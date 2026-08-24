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
	provider := flag.String("provider", "google", "translation provider; only the non-LLM Google text endpoint is supported")
	actor := flag.String("actor", "translation-editorial-2026-08-22", "audit actor")
	workers := flag.Int("workers", 2, "concurrent translation requests")
	limit := flag.Int("limit", 0, "translate at most N cards; zero means all")
	repairDegenerate := flag.Bool("repair-degenerate-prompts", false, "re-translate the question for production cards whose Russian prompt degenerated into the answer text, updating only the ru prompt column")
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
	var sources []store.TranslationSource
	if *repairDegenerate {
		sources, err = db.DegenerateRussianSources(ctx, *workspaceKey)
	} else {
		sources, err = db.MissingRussianSources(ctx, *workspaceKey)
	}
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
		Model:           "google-translate",
		Provider:        "google-translate:text-endpoint",
		Approved:        *approve,
		StartedAt:       time.Now().UTC(),
		SourceCount:     len(sources),
		TargetCount:     targetCount,
		ExistingBefore:  existingBefore,
		Items:           make([]reportItem, 0, len(sources)),
	}
	if *provider != "google" {
		fatal("LLM translation providers are disabled; provider must be google")
	}
	items := translateAll(ctx, db, sources, *actor, *workers, *approve, *repairDegenerate)
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

func translateAll(ctx context.Context, db *store.Postgres, sources []store.TranslationSource, actor string, workers int, approve, repairDegenerate bool) []reportItem {
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
				if repairDegenerate {
					var input translationInput
					if err := json.Unmarshal(source.Payload, &input); err != nil {
						item.Status, item.Error = "failed", err.Error()
						results <- result{index: index, item: item}
						continue
					}
					questionOnly := strings.TrimSpace(input.Question)
					if questionOnly == "" {
						item.Status, item.Error = "failed", "canonical payload has no question"
						results <- result{index: index, item: item}
						continue
					}
					translated, err := translateWithGoogle(ctx, translationInput{Question: questionOnly})
					if err != nil {
						item.Status, item.Error = "failed", err.Error()
						results <- result{index: index, item: item}
						continue
					}
					if approve {
						err = db.RepairRussianPrompt(ctx, source, translated.Question, actor, "google-translate:text-endpoint:degenerate-prompt-repair")
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
					continue
				}
				translated, err := translateSource(ctx, source)
				if err != nil {
					item.Status, item.Error = "failed", err.Error()
					results <- result{index: index, item: item}
					continue
				}
				if approve {
					err = db.StoreRussianTranslation(ctx, source, translated.Question, translated.ShortAnswer, translated.Explanation, translated.Sections, actor, "google-translate:text-endpoint")
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

func translateSource(ctx context.Context, source store.TranslationSource) (translationOutput, error) {
	var input translationInput
	if err := json.Unmarshal(source.Payload, &input); err != nil {
		return translationOutput{}, fmt.Errorf("decode canonical payload: %w", err)
	}
	if strings.TrimSpace(input.Question) == "" {
		return translationOutput{}, errors.New("canonical payload has no question")
	}
	return translateWithGoogle(ctx, input)
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
	// The public endpoint rate-limits bursts with 429; a bounded backoff
	// keeps long editorial batches alive without hammering the service.
	backoff := 3 * time.Second
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 4
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://translate.googleapis.com/translate_a/single?"+values.Encode(), nil)
		if err != nil {
			return "", fmt.Errorf("create translation request: %w", err)
		}
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			lastErr = fmt.Errorf("call translation endpoint: %w", err)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("translation endpoint returned %s", resp.Status)
			continue
		}
		if resp.StatusCode == http.StatusBadRequest {
			resp.Body.Close()
			return "", errGoogleRequestTooLong
		}
		if resp.StatusCode >= http.StatusBadRequest {
			resp.Body.Close()
			return "", fmt.Errorf("translation endpoint returned %s", resp.Status)
		}
		return decodeGoogleTranslation(resp)
	}
	return "", lastErr
}

func decodeGoogleTranslation(resp *http.Response) (string, error) {
	defer resp.Body.Close()
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

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
