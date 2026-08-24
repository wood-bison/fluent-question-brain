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

	"github.com/wood-bison/fluent-question-brain/internal/taxonomy"
)

type Section struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// RubricLevel is one ordered assessment grade inside a rubric block: what a
// candidate must demonstrate for this level to be awarded (QB-BUG-8).
type RubricLevel struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// ChoiceOption is one answer option inside a choices block (QB-BUG-7).
type ChoiceOption struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// ChoicesBlock stores a screening question with an explicit answer key and
// the reason the remaining options are wrong.
type ChoicesBlock struct {
	Options   []ChoiceOption `json:"options"`
	AnswerKey string         `json:"answer_key"`
	Rationale string         `json:"rationale,omitempty"`
}

// TaskBlock stores a practical exercise as structured data instead of prose:
// the condition, optional starter schema/signature, the reference solution,
// a walkthrough, difficulty and time/memory constraints where applicable.
type TaskBlock struct {
	Condition   string `json:"condition"`
	Starter     string `json:"starter,omitempty"`
	Solution    string `json:"solution,omitempty"`
	Walkthrough string `json:"walkthrough,omitempty"`
	Difficulty  string `json:"difficulty,omitempty"`
	Constraints string `json:"constraints,omitempty"`
}

// Card is the small, source-independent shape used by the first importer.
// The original markdown remains in the source mirror; the canonical payload
// stores normalized sections so no editor-only syntax becomes runtime truth.
type Card struct {
	SourceRef      string
	ID             string
	StableKey      string
	Slug           string
	Title          string
	Track          string
	Topic          string
	Scope          string
	Lang           string
	Priority       string
	Group          string
	Level          string
	Company        string
	Timing         string
	Usage          string
	ProgramKey     string
	PathKey        string
	DomainKey      string
	CapabilityKey  string
	MappingState   string
	MappingVersion string
	Question       string
	Sections       []Section
	Task           *TaskBlock
	Rubric         []RubricLevel
	Choices        *ChoicesBlock
	Payload        []byte
	Hash           string
}

type canonicalCard struct {
	StableKey      string        `json:"stable_key"`
	Slug           string        `json:"slug"`
	Title          string        `json:"title"`
	Track          string        `json:"track,omitempty"`
	Topic          string        `json:"topic,omitempty"`
	Scope          string        `json:"scope,omitempty"`
	Lang           string        `json:"lang,omitempty"`
	Priority       string        `json:"priority,omitempty"`
	Group          string        `json:"group,omitempty"`
	Level          string        `json:"level,omitempty"`
	Company        string        `json:"company,omitempty"`
	Timing         string        `json:"timing,omitempty"`
	Usage          string        `json:"usage,omitempty"`
	ProgramKey     string        `json:"program_key,omitempty"`
	PathKey        string        `json:"path_key,omitempty"`
	DomainKey      string        `json:"domain_key,omitempty"`
	CapabilityKey  string        `json:"capability_key,omitempty"`
	MappingState   string        `json:"mapping_state,omitempty"`
	MappingVersion string        `json:"mapping_version,omitempty"`
	StageKey       string        `json:"stage_key,omitempty"`
	Question       string        `json:"question"`
	Sections       []Section     `json:"sections"`
	Task           *TaskBlock    `json:"task,omitempty"`
	Rubric         []RubricLevel `json:"rubric,omitempty"`
	Choices        *ChoicesBlock `json:"choices,omitempty"`
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
	inFence := false
	for _, rawLine := range lines[1:] {
		line := strings.TrimRight(rawLine, " \t")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(trimmed, "# ") {
			return Card{}, fmt.Errorf("%s: multiple H1 headings (%q); one file must hold exactly one card", sourceRef, trimmed)
		}
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
	// The explicit metadata ID is the source-of-truth identity. A copied or
	// stale H1 prefix must never be allowed to change the canonical key.
	if metadataID := strings.TrimSpace(meta["id"]); metadataID != "" {
		id = metadataID
	}
	question := firstNonEmpty(meta["question"], title)
	stableKey := "question." + strings.ToLower(strings.TrimSpace(id))
	placement, err := placementFromMetadata(meta)
	if err != nil {
		return Card{}, fmt.Errorf("%s: taxonomy placement: %w", sourceRef, err)
	}
	card := Card{
		SourceRef:      sourceRef,
		ID:             id,
		StableKey:      stableKey,
		Slug:           slugify(id),
		Title:          title,
		Track:          meta["track"],
		Topic:          meta["topic"],
		Scope:          meta["scope"],
		Lang:           meta["lang"],
		Priority:       meta["priority"],
		Group:          meta["group"],
		Level:          meta["level"],
		Company:        firstNonEmpty(meta["company"], meta["организация"]),
		Timing:         meta["timing"],
		Usage:          meta["usage"],
		ProgramKey:     placement.ProgramKey,
		PathKey:        placement.PathKey,
		DomainKey:      placement.DomainKey,
		CapabilityKey:  placement.CapabilityKey,
		MappingState:   placement.MappingState,
		MappingVersion: placement.MappingVersion,
		Question:       question,
		Sections:       sections,
	}
	card.Task = taskBlock(card, meta["difficulty"])
	card.Rubric = rubricBlock(card)
	card.Choices = choicesBlock(card)
	// A file holding many standalone questions under one H1 used to collapse
	// silently into a single card (QB-BUG-6). Without identity or taxonomy
	// metadata such a file cannot be one card; refuse instead of corrupting.
	if strings.TrimSpace(meta["id"]) == "" && card.Track == "" && card.Topic == "" && len(sections) >= 3 {
		return Card{}, fmt.Errorf("%s: %d sections but no ID/Track/Topic metadata — looks like a multi-question dump; split into one card per file", sourceRef, len(sections))
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
		StableKey:      card.StableKey,
		Slug:           card.Slug,
		Title:          card.Title,
		Track:          card.Track,
		Topic:          card.Topic,
		Scope:          card.Scope,
		Lang:           card.Lang,
		Priority:       card.Priority,
		Group:          card.Group,
		Level:          card.Level,
		Company:        card.Company,
		Timing:         card.Timing,
		Usage:          card.Usage,
		ProgramKey:     card.ProgramKey,
		PathKey:        card.PathKey,
		DomainKey:      card.DomainKey,
		CapabilityKey:  card.CapabilityKey,
		MappingState:   card.MappingState,
		MappingVersion: card.MappingVersion,
		Question:       card.Question,
		Sections:       append([]Section(nil), card.Sections...),
		Task:           card.Task,
		Rubric:         append([]RubricLevel(nil), card.Rubric...),
		Choices:        card.Choices,
	}
	return json.Marshal(payload)
}

// taskSectionTitles maps the accepted markdown section titles onto task
// block fields. Both English and Russian conventions are recognized; a
// section keeps its raw body so code blocks survive verbatim.
var taskSectionTitles = []struct {
	title string
	field func(block *TaskBlock) *string
}{
	{"Task", func(b *TaskBlock) *string { return &b.Condition }},
	{"Задача", func(b *TaskBlock) *string { return &b.Condition }},
	{"Условие", func(b *TaskBlock) *string { return &b.Condition }},
	{"Условие задачи", func(b *TaskBlock) *string { return &b.Condition }},
	{"Starter", func(b *TaskBlock) *string { return &b.Starter }},
	{"Schema", func(b *TaskBlock) *string { return &b.Starter }},
	{"DDL", func(b *TaskBlock) *string { return &b.Starter }},
	{"Схема", func(b *TaskBlock) *string { return &b.Starter }},
	{"Входные данные", func(b *TaskBlock) *string { return &b.Starter }},
	{"Solution", func(b *TaskBlock) *string { return &b.Solution }},
	{"Решение", func(b *TaskBlock) *string { return &b.Solution }},
	{"Эталонное решение", func(b *TaskBlock) *string { return &b.Solution }},
	{"Walkthrough", func(b *TaskBlock) *string { return &b.Walkthrough }},
	{"Разбор", func(b *TaskBlock) *string { return &b.Walkthrough }},
	{"Разбор решения", func(b *TaskBlock) *string { return &b.Walkthrough }},
	{"Constraints", func(b *TaskBlock) *string { return &b.Constraints }},
	{"Ограничения", func(b *TaskBlock) *string { return &b.Constraints }},
}

// taskBlock assembles the optional typed task block from recognized sections
// (QB-BUG-6 prerequisite for importing practical exercises). Cards without
// any task section get nil, which serializes to nothing — existing payloads
// and their content hashes stay byte-identical.
func taskBlock(card Card, difficulty string) *TaskBlock {
	block := &TaskBlock{Difficulty: strings.ToUpper(strings.TrimSpace(difficulty))}
	found := false
	for _, section := range card.Sections {
		for _, mapping := range taskSectionTitles {
			if !strings.EqualFold(section.Title, mapping.title) {
				continue
			}
			if strings.TrimSpace(section.Body) == "" {
				continue
			}
			*mapping.field(block) = section.Body
			found = true
		}
	}
	if !found {
		return nil
	}
	// A real exercise carries evidence of being solvable: a reference
	// solution, a walkthrough, or a starter schema/signature. A section
	// merely *called* "Task" inside a narrative card is prose, not a task;
	// treating it as one would silently corrupt the card.
	if block.Condition == "" || (block.Solution == "" && block.Walkthrough == "" && block.Starter == "") {
		return nil
	}
	return block
}

// rubricBlock parses an ordered assessment rubric (QB-BUG-8). Lines may be
// written as `- Развёрнуто: …`, `2 — …`, or `2. …`; the source order is kept.
func rubricBlock(card Card) []RubricLevel {
	body := firstNonEmpty(
		sectionBody(card, "Rubric"),
		sectionBody(card, "Рубрика"),
		sectionBody(card, "Рубрика оценки"),
	)
	if body == "" {
		return nil
	}
	var levels []RubricLevel
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if l, t, ok := cutRubricEntry(line); ok && strings.TrimSpace(t) != "" && !listIndexLabel(l) {
			levels = append(levels, RubricLevel{Label: strings.TrimSpace(l), Text: strings.TrimSpace(t)})
			continue
		}
		// Not an entry header: a continuation of the previous level's text
		// (grade answers in source material span multiple lines and often
		// carry numbered sub-questions).
		if len(levels) > 0 {
			levels[len(levels)-1].Text += "\n" + line
		}
	}
	if len(levels) == 0 {
		return nil
	}
	return levels
}

// listIndexLabel reports whether a rubric label is really a numbered list
// index ("1", "2.") rather than an assessment level name — such lines are
// continuations of the previous level's text.
func listIndexLabel(label string) bool {
	return regexp.MustCompile(`^\d+[.)]?$`).MatchString(strings.TrimSpace(label))
}

func cutRubricEntry(line string) (label, text string, ok bool) {
	for _, separator := range []string{"\u2014", "\u2013", ": ", " - ", ". "} {
		if label, text, found := strings.Cut(line, separator); found {
			return label, text, true
		}
	}
	return "", "", false
}

// choicesBlock parses a screening question with answer options (QB-BUG-7):
// option lines `А) text` / `A) text` / `- Б) text`, a key line `Ключ: А`
// or `Key: A`, and free text after the key explaining why the rest are wrong.
func choicesBlock(card Card) *ChoicesBlock {
	body := firstNonEmpty(
		sectionBody(card, "Options"),
		sectionBody(card, "Варианты"),
		sectionBody(card, "Варианты ответа"),
		sectionBody(card, "Choices"),
	)
	if body == "" {
		return nil
	}
	optionPattern := regexp.MustCompile(`^[-*]?\s*([АБВГДЕA-F])\)\s*(.+)$`)
	keyPattern := regexp.MustCompile(`(?i)^(?:ключ|key|ответ|answer)\s*:\s*([АБВГДЕA-F])\s*$`)
	block := &ChoicesBlock{}
	var rationale strings.Builder
	inRationale := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if match := keyPattern.FindStringSubmatch(trimmed); match != nil {
			block.AnswerKey = strings.ToUpper(match[1])
			inRationale = true
			continue
		}
		if !inRationale {
			if match := optionPattern.FindStringSubmatch(trimmed); match != nil {
				block.Options = append(block.Options, ChoiceOption{
					Label: strings.ToUpper(match[1]),
					Text:  strings.TrimSpace(match[2]),
				})
				continue
			}
		}
		if inRationale {
			rationale.WriteString(trimmed + "\n")
		}
	}
	if len(block.Options) == 0 || block.AnswerKey == "" {
		return nil
	}
	block.Rationale = strings.TrimSpace(rationale.String())
	return block
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
// locale. Cards without Russian sections remain explicitly incomplete; the
// read API never substitutes another locale.
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
	if canonical, ok := taxonomy.CanonicalTopicKey(topic); ok {
		return canonical
	}
	return "topic." + slugify(topic)
}

// placementFromMetadata is intentionally opt-in.  Legacy cards have no
// curriculum fields and therefore produce the exact same canonical payload
// and content hash as before taxonomy v1 was introduced.
func placementFromMetadata(meta map[string]string) (taxonomy.Placement, error) {
	domain := firstNonEmpty(meta["domain_key"], meta["domain-key"], meta["domain"])
	stage := firstNonEmpty(meta["stage_key"], meta["stage-key"], meta["stage"])
	if domain != "" && stage != "" {
		domainPlacement, err := taxonomy.ResolvePlacement("", "", domain, "", "")
		if err != nil {
			return taxonomy.Placement{}, err
		}
		stagePlacement, err := taxonomy.ResolvePlacement("", "", stage, "", "")
		if err != nil {
			return taxonomy.Placement{}, err
		}
		if domainPlacement.DomainKey != stagePlacement.DomainKey {
			return taxonomy.Placement{}, fmt.Errorf("domain_key %q conflicts with deprecated stage_key %q", domain, stage)
		}
	}
	return taxonomy.ResolvePlacement(
		firstNonEmpty(meta["program_key"], meta["program-key"], meta["program"]),
		firstNonEmpty(meta["path_key"], meta["path-key"], meta["path"]),
		firstNonEmpty(domain, stage),
		firstNonEmpty(meta["capability_key"], meta["capability-key"], meta["capability"]),
		firstNonEmpty(meta["mapping_state"], meta["mapping-state"], meta["mapping state"]),
	)
}
