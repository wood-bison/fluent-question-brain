package normalize

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Section struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Card is the small, source-independent shape used by the first importer.
// The original markdown remains in the source mirror; the canonical payload
// stores normalized sections so no editor-only syntax becomes runtime truth.
type Card struct {
	SourceRef string
	ID        string
	StableKey string
	Slug      string
	Title     string
	Track     string
	Topic     string
	Scope     string
	Lang      string
	Priority  string
	Group     string
	Level     string
	Question  string
	Sections  []Section
	Payload   []byte
	Hash      string
}

type canonicalCard struct {
	StableKey string    `json:"stable_key"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Track     string    `json:"track,omitempty"`
	Topic     string    `json:"topic,omitempty"`
	Scope     string    `json:"scope,omitempty"`
	Lang      string    `json:"lang,omitempty"`
	Priority  string    `json:"priority,omitempty"`
	Group     string    `json:"group,omitempty"`
	Level     string    `json:"level,omitempty"`
	Question  string    `json:"question"`
	Sections  []Section `json:"sections"`
}

var headingPattern = regexp.MustCompile(`^#\s+(.+?)\s*$`)

func ParseMarkdown(sourceRef string, input []byte) (Card, error) {
	text := strings.ReplaceAll(strings.TrimPrefix(string(input), "\ufeff"), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return Card{}, fmt.Errorf("%s: empty markdown", sourceRef)
	}

	match := headingPattern.FindStringSubmatch(strings.TrimSpace(lines[0]))
	if len(match) != 2 {
		return Card{}, fmt.Errorf("%s: first line must be an H1", sourceRef)
	}
	title := strings.TrimSpace(match[1])
	id, title := splitIDTitle(title)
	meta := make(map[string]string)
	var sections []Section
	var current *Section
	for _, rawLine := range lines[1:] {
		line := strings.TrimRight(rawLine, " \t")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if current != nil {
				current.Body = normalizeText(current.Body)
				sections = append(sections, *current)
			}
			current = &Section{Title: strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))}
			continue
		}
		if current != nil {
			current.Body += line + "\n"
			continue
		}
		if key, value, ok := strings.Cut(trimmed, ":"); ok && key != "" {
			meta[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
		}
	}
	if current != nil {
		current.Body = normalizeText(current.Body)
		sections = append(sections, *current)
	}
	if id == "" {
		id = slugify(title)
	}
	question := firstNonEmpty(meta["question"], title)
	stableKey := "legacy." + strings.ToLower(strings.TrimSpace(id))
	card := Card{
		SourceRef: sourceRef,
		ID:        id,
		StableKey: stableKey,
		Slug:      slugify(id),
		Title:     title,
		Track:     meta["track"],
		Topic:     meta["topic"],
		Scope:     meta["scope"],
		Lang:      meta["lang"],
		Priority:  meta["priority"],
		Group:     meta["group"],
		Level:     meta["level"],
		Question:  question,
		Sections:  sections,
	}
	rawPayload, err := canonicalPayload(card)
	if err != nil {
		return Card{}, err
	}
	payload, err := CanonicalJSON(rawPayload)
	if err != nil {
		return Card{}, fmt.Errorf("canonicalize payload: %w", err)
	}
	card.Payload = payload
	card.Hash = HashCanonicalJSON(payload)
	return card, nil
}

func CanonicalJSON(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(out.Bytes()), nil
}

func HashCanonicalJSON(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func canonicalPayload(card Card) ([]byte, error) {
	payload := canonicalCard{
		StableKey: card.StableKey,
		Slug:      card.Slug,
		Title:     card.Title,
		Track:     card.Track,
		Topic:     card.Topic,
		Scope:     card.Scope,
		Lang:      card.Lang,
		Priority:  card.Priority,
		Group:     card.Group,
		Level:     card.Level,
		Question:  card.Question,
		Sections:  append([]Section(nil), card.Sections...),
	}
	return json.Marshal(payload)
}

func splitIDTitle(raw string) (string, string) {
	if id, title, ok := strings.Cut(raw, " — "); ok {
		return strings.TrimSpace(id), strings.TrimSpace(title)
	}
	if id, title, ok := strings.Cut(raw, " - "); ok && strings.HasPrefix(strings.TrimSpace(id), "Q") {
		return strings.TrimSpace(id), strings.TrimSpace(title)
	}
	return "", strings.TrimSpace(raw)
}

func sectionBody(card Card, title string) string {
	for _, section := range card.Sections {
		if strings.EqualFold(section.Title, title) {
			return section.Body
		}
	}
	return ""
}

func normalizeText(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && out.Len() > 0 {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

// SectionsForBody returns a stable copy for callers that need to embed the
// source sections in a JSONB body without mutating the parsed card.
func SectionsForBody(card Card) []Section {
	sections := append([]Section(nil), card.Sections...)
	sort.SliceStable(sections, func(i, j int) bool { return i < j })
	return sections
}

func EnglishFields(card Card) (prompt, shortAnswer, explanation string) {
	prompt = firstNonEmpty(card.Question, sectionBody(card, "Question"))
	shortAnswer = firstNonEmpty(sectionBody(card, "Core Idea"), sectionBody(card, "Answer"))
	explanation = firstNonEmpty(
		sectionBody(card, "English Explanation"),
		sectionBody(card, "Explanation"),
		sectionBody(card, "Go Deeper"),
	)
	return prompt, shortAnswer, explanation
}

// RussianFields extracts the Russian understanding layer as a first-class
// locale. Cards without Russian sections keep an explicit English fallback at
// read time instead of duplicating source text during import.
func RussianFields(card Card) (prompt, shortAnswer, explanation string) {
	prompt = firstNonEmpty(sectionBody(card, "Question (RU)"), sectionBody(card, "Core Idea (RU)"))
	shortAnswer = firstNonEmpty(sectionBody(card, "Core Idea (RU)"), sectionBody(card, "Russian Answer"))
	explanation = firstNonEmpty(
		sectionBody(card, "Russian Explanation"),
		sectionBody(card, "Explanation (RU)"),
	)
	return prompt, shortAnswer, explanation
}

func TopicStableKey(topic string) string {
	if strings.TrimSpace(topic) == "" {
		return ""
	}
	return "legacy." + slugify(topic)
}
