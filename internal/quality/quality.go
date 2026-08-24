// Package quality contains the source-independent content checks shared by
// import/release boundaries and the read-only quality audit.
package quality

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wood-bison/fluent-question-brain/internal/normalize"
)

// PromptIssue is a stable, answer-free explanation of why a prompt cannot be
// released. The message is suitable for dry-run reports; it never contains
// the prompt or answer text itself.
type PromptIssue struct {
	Code    string
	Message string
}

// CardIssues applies the I0 source gate to one normalized card. Missing
// locales are checked by the release audit; this function only rejects a
// locale when it is present but clearly malformed.
func CardIssues(card normalize.Card) []PromptIssue {
	issues := make([]PromptIssue, 0)
	if isPDFHeading(card.Title) {
		issues = append(issues, PromptIssue{
			Code:    "placeholder_title",
			Message: "card title is a PDF section heading or placeholder",
		})
	}
	if isCodeFragmentTitle(card.Title) {
		issues = append(issues, PromptIssue{
			Code:    "code_fragment_title",
			Message: "card title is an extracted code fragment rather than a semantic title",
		})
	}

	prompt, answer, _ := normalize.EnglishFields(card)
	issues = appendPromptIssues(issues, "en", prompt, answer, card.Title, card.Topic)
	if prompt, answer, _ := normalize.RussianFields(card); strings.TrimSpace(prompt) != "" || strings.TrimSpace(answer) != "" {
		issues = appendPromptIssues(issues, "ru", prompt, answer, card.Title, card.Topic)
	}

	for _, section := range card.Sections {
		if HasPDFArtifact(section.Title) || HasPDFArtifact(section.Body) {
			issues = append(issues, PromptIssue{
				Code:    "pdf_artifact",
				Message: fmt.Sprintf("section %q contains an extracted PDF control or replacement character", section.Title),
			})
			break
		}
		if HasPDFLayoutArtifact(card.Scope, section.Body) {
			issues = append(issues, PromptIssue{
				Code:    "pdf_layout_artifact",
				Message: fmt.Sprintf("section %q contains an extracted PDF footer or sidebar marker", section.Title),
			})
			break
		}
	}
	if len(card.Payload) > 0 {
		if JSONHasPDFArtifact(card.Payload) {
			issues = append(issues, PromptIssue{
				Code:    "pdf_artifact",
				Message: "canonical payload contains an extracted PDF control or replacement character",
			})
		}
		if JSONHasPDFLayoutArtifact(card.Payload, card.Scope) {
			issues = append(issues, PromptIssue{
				Code:    "pdf_layout_artifact",
				Message: "canonical payload contains an extracted PDF footer or sidebar marker",
			})
		}
	}
	return deduplicate(issues)
}

// PromptIssues checks one locale prompt without requiring a parsed card. It
// is used by the database audit, where the canonical locale rows are already
// available and re-parsing the source would be the wrong read boundary.
func PromptIssues(prompt, answer, title, topic string) []PromptIssue {
	issues := make([]PromptIssue, 0)
	return appendPromptIssues(issues, "", prompt, answer, title, topic)
}

// JSONHasPDFArtifact checks every string in a canonical payload. This keeps
// the audit independent of the source file while still catching a page-break
// or replacement character that survived normalization inside a section.
func JSONHasPDFArtifact(payload []byte) bool {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return false
	}
	return valueHasPDFArtifact(value)
}

// JSONHasPDFLayoutArtifact checks known PDF footer/sidebar markers in the
// learner-facing parts of the normalized payload. Metadata such as Track:
// Backend is deliberately excluded: the same category labels that indicate a
// PDF sidebar when they appear as a content line are valid taxonomy values.
// These markers are scoped to Ozon's extracted interview sheets; the same
// words in authored cards are not rejected globally.
func JSONHasPDFLayoutArtifact(payload []byte, scope string) bool {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return false
	}
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"question", "sections", "task", "rubric", "choices"} {
		if valueHasPDFLayoutArtifact(object[key], scope) {
			return true
		}
	}
	return false
}

// IsPDFHeading exposes the title/heading check to the database audit.
func IsPDFHeading(value string) bool {
	return isPDFHeading(value)
}

// IsCodeFragmentTitle exposes the title-shape check to the database audit.
// A title may contain code examples, but a bare assignment/function literal
// is extraction debris and not a useful learner-facing label.
func IsCodeFragmentTitle(value string) bool {
	return isCodeFragmentTitle(value)
}

// IsSemanticShapeIssue identifies the answer-free semantic/shape failures
// surfaced by /v1/quality. PDF control/layout failures remain separate so the
// audit can distinguish malformed prose from extraction debris.
func IsSemanticShapeIssue(code string) bool {
	switch code {
	case "empty_prompt", "prompt_equals_answer", "prompt_matches_title",
		"prompt_matches_topic", "pdf_heading_prompt", "placeholder_title",
		"code_fragment_title", "single_token_prompt", "short_label_prompt",
		"fragment_prompt":
		return true
	default:
		return false
	}
}

func appendPromptIssues(issues []PromptIssue, locale, prompt, answer, title, topic string) []PromptIssue {
	add := func(code, message string) {
		if locale != "" {
			message = locale + " locale: " + message
		}
		issues = append(issues, PromptIssue{Code: code, Message: message})
	}

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		add("empty_prompt", "prompt is empty")
		return issues
	}
	normalizedPrompt := normalizeComparable(prompt)
	if normalizedAnswer := normalizeComparable(answer); normalizedAnswer != "" && normalizedPrompt == normalizedAnswer {
		add("prompt_equals_answer", "prompt is identical to the answer")
	}
	if normalizedTitle := normalizeComparable(title); normalizedTitle != "" && normalizedPrompt == normalizedTitle && isCompactLabel(prompt) {
		add("prompt_matches_title", "prompt is identical to the card title")
	}
	if normalizedTopic := normalizeComparable(topic); normalizedTopic != "" && normalizedPrompt == normalizedTopic && isCompactLabel(prompt) {
		add("prompt_matches_topic", "prompt is identical to the card topic")
	}
	if isPDFHeading(prompt) {
		add("pdf_heading_prompt", "prompt is a PDF section heading or placeholder")
	}
	if HasPDFArtifact(prompt) {
		add("pdf_artifact", "prompt contains an extracted PDF control or replacement character")
	}
	if isPromptFragment(prompt) {
		add("fragment_prompt", "prompt ends like an incomplete extracted sentence")
	}

	// A single bare token is almost always a heading, not an interview
	// question. A question mark is an explicit escape hatch for concise real
	// questions such as “What is SLA?”.
	words := strings.Fields(prompt)
	if len(words) == 1 && !hasQuestionMark(prompt) && !hasPunctuation(prompt) {
		add("single_token_prompt", "prompt is one unpunctuated word")
	} else if len(words) == 2 && runeCount(prompt) < 20 && !hasQuestionMark(prompt) && !hasPunctuation(prompt) {
		add("short_label_prompt", "prompt is shorter than 20 characters and is not phrased as a question")
	}
	return issues
}

// HasPDFArtifact detects characters that should not survive pdftotext/HTML
// extraction into learner content. Newlines, tabs and ordinary spaces remain
// legal; page breaks, replacement characters, soft hyphens and zero-width
// layout markers do not.
func HasPDFArtifact(value string) bool {
	for _, r := range value {
		switch r {
		case '\u00ad', '\u200b', '\u200c', '\u200d', '\u2060', '\ufffd':
			return true
		}
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}

// HasPDFLayoutArtifact detects the visible remnants of the Ozon PDF sidebar
// and footer: page counters, the "Задачник" footer, and isolated category
// labels that were inserted between the actual task and rubric.
func HasPDFLayoutArtifact(scope, value string) bool {
	if !strings.EqualFold(strings.TrimSpace(scope), "ozon") {
		return false
	}
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		if isPDFLayoutLine(line) {
			return true
		}
	}
	return false
}

func isPDFHeading(value string) bool {
	value = normalizeComparable(value)
	if value == "" {
		return true
	}
	if strings.HasSuffix(value, "—") || strings.Trim(value, " .,:;—-") == "" {
		return true
	}
	switch value {
	case "c", "sql", "-", ".", ":", ";", "указатели", "jquery", "deepcopy":
		return true
	default:
		return false
	}
}

func isCodeFragmentTitle(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	// These patterns are intentionally narrow: code in a descriptive title is
	// allowed, while a standalone assignment/map/function expression is a
	// reliable signal that a PDF code column was promoted to the title.
	if strings.Contains(value, ":=") || strings.Contains(value, "map[") {
		return true
	}
	if strings.Contains(value, "func ") && strings.Contains(value, "(") {
		return true
	}
	return strings.ContainsAny(value, "{}") && strings.Contains(value, "=")
}

var trailingFragmentWords = map[string]struct{}{
	"для": {}, "у": {}, "нас": {}, "в": {}, "во": {}, "на": {}, "с": {},
	"со": {}, "к": {}, "ко": {}, "по": {}, "из": {}, "от": {}, "до": {},
	"без": {}, "над": {}, "под": {}, "при": {}, "об": {}, "о": {},
	"который": {}, "которая": {}, "которое": {}, "которые": {},
	"если": {}, "когда": {}, "чтобы": {},
	"for": {}, "to": {}, "with": {}, "and": {}, "or": {}, "that": {},
	"which": {}, "if": {}, "when": {},
}

func isPromptFragment(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || hasQuestionMark(value) {
		return false
	}
	trimmed := strings.TrimRightFunc(value, unicode.IsSpace)
	if len(trimmed) == 0 {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(trimmed)
	if strings.ContainsRune(",:;—-", last) {
		return true
	}
	// A terminal full stop is a complete statement even when its last word is
	// a preposition (for example, "What is this tool used for.").
	if strings.ContainsRune(".!?。！？", last) {
		return false
	}
	words := strings.Fields(trimmed)
	if len(words) < 2 {
		return false
	}
	lastWord := strings.Trim(strings.ToLower(words[len(words)-1]), " \t\r\n.,!?;:()[]{}\"'«»`")
	if _, ok := trailingFragmentWords[lastWord]; ok {
		return true
	}
	return strings.HasSuffix(strings.ToLower(trimmed), " у нас")
}

func valueHasPDFArtifact(value any) bool {
	switch typed := value.(type) {
	case string:
		return HasPDFArtifact(typed)
	case []any:
		for _, child := range typed {
			if valueHasPDFArtifact(child) {
				return true
			}
		}
	case map[string]any:
		for _, child := range typed {
			if valueHasPDFArtifact(child) {
				return true
			}
		}
	}
	return false
}

func valueHasPDFLayoutArtifact(value any, scope string) bool {
	switch typed := value.(type) {
	case string:
		return HasPDFLayoutArtifact(scope, typed)
	case []any:
		for _, child := range typed {
			if valueHasPDFLayoutArtifact(child, scope) {
				return true
			}
		}
	case map[string]any:
		for _, child := range typed {
			if valueHasPDFLayoutArtifact(child, scope) {
				return true
			}
		}
	}
	return false
}

func isPDFLayoutLine(value string) bool {
	original := strings.TrimSpace(value)
	// The extracted Ozon sheets prefix sidebar categories with an uppercase
	// `GO`; lowercase `go func` lines are authored code and must remain valid.
	if strings.HasPrefix(original, "GO ") || original == "GO" || strings.HasPrefix(original, ": :") {
		return true
	}
	value = normalizeComparable(original)
	if value == "" {
		return false
	}
	if strings.Contains(value, "https://matrix.o3.ru/trials") || strings.Contains(value, "задачник") || strings.Contains(value, "go ос, сети и эксплуатация") || strings.HasPrefix(value, "аналитика данных ") {
		return true
	}
	if isPageCounter(value) {
		return true
	}
	switch value {
	case "c", "sql", "bi", "java", "frontend", "backend", "product", "ds", "scala", "go", ":", "-":
		return true
	default:
		return false
	}
}

func isPageCounter(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	for _, part := range parts {
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func normalizeComparable(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func hasQuestionMark(value string) bool {
	return strings.ContainsAny(value, "?？")
}

func hasPunctuation(value string) bool {
	for _, r := range value {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return true
		}
	}
	return false
}

func isCompactLabel(value string) bool {
	return len(strings.Fields(value)) <= 2 && !hasQuestionMark(value)
}

func runeCount(value string) int {
	count := 0
	for range value {
		count++
	}
	return count
}

func deduplicate(issues []PromptIssue) []PromptIssue {
	seen := make(map[string]struct{}, len(issues))
	out := make([]PromptIssue, 0, len(issues))
	for _, issue := range issues {
		if _, ok := seen[issue.Code]; ok {
			continue
		}
		seen[issue.Code] = struct{}{}
		out = append(out, issue)
	}
	return out
}
