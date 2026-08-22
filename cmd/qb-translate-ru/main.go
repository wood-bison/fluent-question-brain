package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/wood-bison/fluent-question-brain/internal/normalize"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

const defaultModel = "qwen3.8:27b-mlx"

type translationInput struct {
	Question string              `json:"question"`
	Sections []normalize.Section `json:"sections"`
}

type translationOutput struct {
	Question    string              `json:"question"`
	ShortAnswer string              `json:"short_answer"`
	Explanation string              `json:"explanation"`
	Sections    []normalize.Section `json:"sections"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

type reportItem struct {
	StableKey string `json:"stable_key"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type translationReport struct {
	ContractVersion string       `json:"contract_version"`
	WorkspaceKey    string       `json:"workspace_key"`
	Model           string       `json:"model"`
	Provider        string       `json:"provider"`
	Approved        bool         `json:"approved"`
	StartedAt       time.Time    `json:"started_at"`
	FinishedAt      time.Time    `json:"finished_at"`
	SourceCount     int          `json:"source_count"`
	TargetCount     int          `json:"target_count"`
	ExistingBefore  int          `json:"existing_before"`
	RemainingAfter  int          `json:"remaining_after"`
	Translated      int          `json:"translated"`
	Failed          int          `json:"failed"`
	Items           []reportItem `json:"items"`
}

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	workspaceKey := flag.String("workspace", "fluent-interview", "stable workspace key")
	ollamaURL := flag.String("ollama-url", "http://127.0.0.1:11434", "Ollama API base URL")
	model := flag.String("model", defaultModel, "Ollama completion model")
	provider := flag.String("provider", "google", "translation provider: google or ollama (ollama is legacy and opt-in)")
	actor := flag.String("actor", "translation-editorial-2026-08-22", "audit actor")
	workers := flag.Int("workers", 2, "concurrent translation requests")
	limit := flag.Int("limit", 0, "translate at most N cards; zero means all")
	approve := flag.Bool("approve", false, "write generated Russian locales to Postgres")
	reportPath := flag.String("report", "", "write a machine-readable JSON report")
	flag.Parse()

	if strings.TrimSpace(*databaseURL) == "" {
		fatal("database-url is required")
	}
	if *workers < 1 {
		fatal("workers must be at least 1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
	defer cancel()

	db, err := store.Open(ctx, *databaseURL)
	if err != nil {
		fatal("open database: %v", err)
	}
	defer db.Close()
	sources, err := db.MissingRussianSources(ctx, *workspaceKey)
	if err != nil {
		fatal("read translation sources: %v", err)
	}
	if *limit > 0 && len(sources) > *limit {
		sources = sources[:*limit]
	}
	targetCount, existingBefore, err := db.TranslationCoverage(ctx, *workspaceKey)
	if err != nil {
		fatal("read translation coverage: %v", err)
	}
	report := translationReport{
		ContractVersion: "question-brain.translation-run.v1",
		WorkspaceKey:    *workspaceKey,
		Model:           translationModelLabel(*provider, *model),
		Provider:        translationProviderLabel(*provider, *model),
		Approved:        *approve,
		StartedAt:       time.Now().UTC(),
		SourceCount:     len(sources),
		TargetCount:     targetCount,
		ExistingBefore:  existingBefore,
		Items:           make([]reportItem, 0, len(sources)),
	}
	if *provider != "google" && *provider != "ollama" {
		fatal("provider must be google or ollama")
	}
	items := translateAll(ctx, db, sources, *ollamaURL, *model, *provider, *actor, *workers, *approve)
	report.Items = items
	for _, item := range items {
		if item.Status == "translated" || item.Status == "stored" {
			report.Translated++
		} else if item.Status == "failed" {
			report.Failed++
		}
	}
	report.FinishedAt = time.Now().UTC()
	_, existingAfter, err := db.TranslationCoverage(ctx, *workspaceKey)
	if err != nil {
		fatal("read post-run translation coverage: %v", err)
	}
	report.RemainingAfter = targetCount - existingAfter
	if *reportPath != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fatal("encode report: %v", err)
		}
		if err := os.WriteFile(*reportPath, append(data, '\n'), 0o644); err != nil {
			fatal("write report: %v", err)
		}
	}
	if report.Failed > 0 {
		os.Exit(1)
	}
}

func translationProviderLabel(provider, model string) string {
	if provider == "ollama" {
		return "ollama:" + model
	}
	return "google-translate:text-endpoint"
}

func translationModelLabel(provider, model string) string {
	if provider == "ollama" {
		return model
	}
	return "google-translate"
}

func translateAll(ctx context.Context, db *store.Postgres, sources []store.TranslationSource, ollamaURL, model, provider, actor string, workers int, approve bool) []reportItem {
	type result struct {
		index int
		item  reportItem
	}
	jobs := make(chan int)
	results := make(chan result, len(sources))
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				source := sources[index]
				item := reportItem{StableKey: source.StableKey}
				translated, err := translateSource(ctx, source, ollamaURL, model, provider)
				if err != nil {
					item.Status, item.Error = "failed", err.Error()
					results <- result{index: index, item: item}
					continue
				}
				if approve {
					err = db.StoreRussianTranslation(ctx, source, translated.Question, translated.ShortAnswer, translated.Explanation, translated.Sections, actor, translationProviderLabel(provider, model))
					if err != nil {
						item.Status, item.Error = "failed", err.Error()
						results <- result{index: index, item: item}
						continue
					}
					item.Status = "stored"
				} else {
					item.Status = "translated"
				}
				results <- result{index: index, item: item}
			}
		}()
	}
	go func() {
		for index := range sources {
			jobs <- index
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	ordered := make([]reportItem, len(sources))
	for result := range results {
		ordered[result.index] = result.item
	}
	return ordered
}

func translateSource(ctx context.Context, source store.TranslationSource, ollamaURL, model, provider string) (translationOutput, error) {
	var input translationInput
	if err := json.Unmarshal(source.Payload, &input); err != nil {
		return translationOutput{}, fmt.Errorf("decode canonical payload: %w", err)
	}
	if strings.TrimSpace(input.Question) == "" {
		return translationOutput{}, errors.New("canonical payload has no question")
	}
	if provider == "google" {
		return translateWithGoogle(ctx, input)
	}
	return translateWithOllama(ctx, input, ollamaURL, model)
}

func translateWithOllama(ctx context.Context, input translationInput, ollamaURL, model string) (translationOutput, error) {
	requestBody := map[string]any{
		"model":  model,
		"stream": false,
		"think":  false,
		"format": "json",
		"options": map[string]any{
			"temperature": 0.1,
		},
		"prompt": translationPrompt(input),
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return translationOutput{}, fmt.Errorf("encode Ollama request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(ollamaURL, "/")+"/api/generate", strings.NewReader(string(body)))
	if err != nil {
		return translationOutput{}, fmt.Errorf("create Ollama request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return translationOutput{}, fmt.Errorf("call Ollama: %w", err)
	}
	defer resp.Body.Close()
	var envelope ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return translationOutput{}, fmt.Errorf("decode Ollama response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest || envelope.Error != "" {
		return translationOutput{}, fmt.Errorf("Ollama error: %s", firstNonEmpty(envelope.Error, resp.Status))
	}
	var output translationOutput
	if err := json.Unmarshal([]byte(envelope.Response), &output); err != nil {
		return translationOutput{}, fmt.Errorf("decode translated JSON: %w", err)
	}
	if err := validateTranslation(input, output); err != nil {
		return translationOutput{}, err
	}
	return output, nil
}

// translateWithGoogle is the fast, non-LLM editorial pass. It uses the
// public Google Translate text endpoint one card at a time, keeping explicit
// field markers so the response can be reconstructed without changing the
// number or order of sections. Code identifiers and markdown backticks are
// preserved by the provider in the same way as the source text.
func translateWithGoogle(ctx context.Context, input translationInput) (translationOutput, error) {
	markers := make([]string, 0, 1+len(input.Sections)*2)
	parts := make([]string, 0, cap(markers))
	add := func(marker, value string) {
		markers = append(markers, marker)
		parts = append(parts, marker+" "+strings.TrimSpace(value))
	}
	add("__QB_QUESTION__", input.Question)
	for index, section := range input.Sections {
		add(fmt.Sprintf("__QB_SECTION_%d_TITLE__", index), section.Title)
		add(fmt.Sprintf("__QB_SECTION_%d_BODY__", index), section.Body)
	}
	translated, err := googleTranslateText(ctx, strings.Join(parts, "\n"))
	if errors.Is(err, errGoogleRequestTooLong) {
		return translateGoogleFields(ctx, input)
	}
	if err != nil {
		return translationOutput{}, err
	}
	valuesByMarker := make(map[string]string, len(markers))
	for index, marker := range markers {
		start := strings.Index(translated, marker)
		if start < 0 {
			return translationOutput{}, fmt.Errorf("translation marker %q is missing", marker)
		}
		start += len(marker)
		end := len(translated)
		for _, next := range markers[index+1:] {
			if candidate := strings.Index(translated[start:], next); candidate >= 0 {
				end = start + candidate
				break
			}
		}
		valuesByMarker[marker] = strings.TrimSpace(translated[start:end])
	}
	output := translationOutput{Question: valuesByMarker[markers[0]], Sections: make([]normalize.Section, len(input.Sections))}
	for index := range input.Sections {
		output.Sections[index] = normalize.Section{
			Title: valuesByMarker[fmt.Sprintf("__QB_SECTION_%d_TITLE__", index)],
			Body:  valuesByMarker[fmt.Sprintf("__QB_SECTION_%d_BODY__", index)],
		}
	}
	if err := validateTranslation(input, output); err != nil {
		return translationOutput{}, err
	}
	return output, nil
}

var errGoogleRequestTooLong = errors.New("google translation request is too long")

func googleTranslateText(ctx context.Context, text string) (string, error) {
	values := url.Values{}
	values.Set("client", "gtx")
	values.Set("sl", "en")
	values.Set("tl", "ru")
	values.Set("dt", "t")
	values.Set("q", text)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://translate.googleapis.com/translate_a/single?"+values.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("create translation request: %w", err)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("call translation endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusBadRequest {
		return "", errGoogleRequestTooLong
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("translation endpoint returned %s", resp.Status)
	}
	var root []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return "", fmt.Errorf("decode translation response: %w", err)
	}
	if len(root) == 0 {
		return "", errors.New("translation response is empty")
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(root[0], &rows); err != nil {
		return "", fmt.Errorf("decode translation rows: %w", err)
	}
	translatedParts := make([]string, 0, len(rows))
	for _, row := range rows {
		var fields []json.RawMessage
		if err := json.Unmarshal(row, &fields); err != nil || len(fields) == 0 {
			continue
		}
		var translated string
		if err := json.Unmarshal(fields[0], &translated); err == nil {
			translatedParts = append(translatedParts, translated)
		}
	}
	return strings.Join(translatedParts, "\n"), nil
}

func translateGoogleFields(ctx context.Context, input translationInput) (translationOutput, error) {
	output := translationOutput{Sections: make([]normalize.Section, len(input.Sections))}
	var err error
	if output.Question, err = googleTranslateText(ctx, input.Question); err != nil {
		return translationOutput{}, err
	}
	for index, section := range input.Sections {
		if output.Sections[index].Title, err = googleTranslateText(ctx, section.Title); err != nil {
			return translationOutput{}, err
		}
		if strings.TrimSpace(section.Body) != "" {
			if output.Sections[index].Body, err = googleTranslateText(ctx, section.Body); err != nil {
				return translationOutput{}, err
			}
		}
	}
	if err := validateTranslation(input, output); err != nil {
		return translationOutput{}, err
	}
	return output, nil
}

func translationPrompt(input translationInput) string {
	encoded, _ := json.Marshal(input)
	return `Translate this technical learning card from English to natural, concise Russian.
Return ONLY valid JSON with exactly these keys: question, short_answer, explanation, sections.
The sections array must contain the same number of objects as the input, each with title and body.
Preserve code, identifiers, API names, numbers, markdown bullets, and inline English technical terms.
Do not invent facts, examples, or sections. Translate section titles as well.
If a field is empty in the input, keep it empty. Input JSON:
` + string(encoded)
}

func validateTranslation(input translationInput, output translationOutput) error {
	if strings.TrimSpace(output.Question) == "" || !hasCyrillic(output.Question) {
		return errors.New("translated question is empty or not Russian")
	}
	if len(output.Sections) != len(input.Sections) {
		return fmt.Errorf("translated section count %d does not match source %d", len(output.Sections), len(input.Sections))
	}
	for index, section := range output.Sections {
		if strings.TrimSpace(section.Title) == "" {
			return fmt.Errorf("translated section %d has no title", index)
		}
		if strings.TrimSpace(input.Sections[index].Body) != "" && strings.TrimSpace(section.Body) == "" {
			return fmt.Errorf("translated section %d is incomplete", index)
		}
		if !hasCyrillic(section.Title + " " + section.Body) {
			return fmt.Errorf("translated section %d has no Russian text", index)
		}
	}
	return nil
}

func hasCyrillic(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
