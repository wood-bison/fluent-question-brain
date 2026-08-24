package normalize

import (
	"strings"
	"testing"
)

func TestParseMarkdownProducesStableHash(t *testing.T) {
	input := []byte("# Q001 — Why?\r\nID: Q001\r\nTrack: Backend\r\nQuestion: Why?\r\n\r\n## Core Idea\r\n\r\nBecause.  \r\n")
	card, err := ParseMarkdown("Question Cards/Q001.md", input)
	if err != nil {
		t.Fatalf("ParseMarkdown() error = %v", err)
	}
	if card.StableKey != "question.q001" || card.Slug != "q001" {
		t.Fatalf("identity = %q/%q", card.StableKey, card.Slug)
	}
	if !strings.Contains(string(card.Payload), "Because.") {
		t.Fatalf("payload does not contain normalized section: %s", card.Payload)
	}
	if card.Hash != HashCanonicalJSON(card.Payload) {
		t.Fatalf("hash is not derived from payload")
	}
}

func TestCanonicalJSONSortsObjectKeys(t *testing.T) {
	canonical, err := CanonicalJSON([]byte(`{"z":1,"a":2}`))
	if err != nil {
		t.Fatalf("CanonicalJSON() error = %v", err)
	}
	if string(canonical) != `{"a":2,"z":1}` {
		t.Fatalf("canonical = %s", canonical)
	}
}

func TestParseMarkdownPrefersExplicitMetadataID(t *testing.T) {
	input := []byte("# B011 — copied heading\nID: B012\nQuestion: Tell me about a time you learned quickly.\n\n## Situation\n\nThe heading carried a stale id, but the metadata identifies this card.\n")
	card, err := ParseMarkdown("Behavioral Cards/B012.md", input)
	if err != nil {
		t.Fatalf("ParseMarkdown() error = %v", err)
	}
	if card.StableKey != "question.b012" || card.Slug != "b012" {
		t.Fatalf("identity = %q/%q, want question.b012/b012", card.StableKey, card.Slug)
	}
}

func TestLegacyCardHashDoesNotGainCurriculumFields(t *testing.T) {
	input := []byte("# Q001 — Why?\nID: Q001\nTrack: Backend\nTopic: Node / Event Loop & Scheduling\nQuestion: Why?\n\n## Core Idea\n\nBecause.\n")
	card, err := ParseMarkdown("Question Cards/Q001.md", input)
	if err != nil {
		t.Fatalf("ParseMarkdown() error = %v", err)
	}
	for _, field := range []string{"program_key", "path_key", "domain_key", "capability_key", "mapping_state"} {
		if strings.Contains(string(card.Payload), `"`+field+`"`) {
			t.Fatalf("legacy payload unexpectedly contains %s: %s", field, card.Payload)
		}
	}
}

func TestParseMarkdownCanonicalizesExplicitCurriculumPlacement(t *testing.T) {
	input := []byte("# C001 — Event loop\nID: C001\nProgram-Key: Backend Engineer\nPath-Key: Node.js+TypeScript\nDomain-Key: stage.runtime\nCapability-Key: capability.runtime.event-loop\nMapping-State: accepted\nQuestion: Explain the event loop.\n\n## Core Idea\n\nIt schedules work.\n")
	card, err := ParseMarkdown("Question Cards/C001.md", input)
	if err != nil {
		t.Fatalf("ParseMarkdown() error = %v", err)
	}
	if card.ProgramKey != "program.backend-engineer" || card.PathKey != "path.nodejs-typescript" || card.DomainKey != "domain.runtime" || card.CapabilityKey != "capability.runtime.event-loop" || card.MappingState != "accepted" {
		t.Fatalf("placement = %#v", card)
	}
	for _, expected := range []string{`"program_key":"program.backend-engineer"`, `"path_key":"path.nodejs-typescript"`, `"domain_key":"domain.runtime"`, `"capability_key":"capability.runtime.event-loop"`, `"mapping_state":"accepted"`} {
		if !strings.Contains(string(card.Payload), expected) {
			t.Fatalf("payload missing %s: %s", expected, card.Payload)
		}
	}
}

func TestParseMarkdownRejectsUncontrolledCurriculumPlacement(t *testing.T) {
	input := []byte("# C002 — Unknown\nID: C002\nPath: Backend\nDomain: Runtime\nQuestion: Explain.\n\n## Core Idea\n\nAnswer.\n")
	if _, err := ParseMarkdown("Question Cards/C002.md", input); err == nil {
		t.Fatal("uncontrolled path was accepted")
	}
}

func TestCardFromPayloadPreservesLegacyFieldsAndHash(t *testing.T) {
	input := []byte(`{"stable_key":"question.legacy","slug":"legacy","title":"Legacy","track":"Backend","topic":"Node / Event Loop & Scheduling","stage_key":"legacy-stage","runtime_task_ref":"task-001","question":"Why?","sections":[]}`)
	card, err := CardFromPayload("payload://legacy", input)
	if err != nil {
		t.Fatalf("CardFromPayload() error = %v", err)
	}
	canonical, err := CanonicalJSON(input)
	if err != nil {
		t.Fatalf("CanonicalJSON() error = %v", err)
	}
	if string(card.Payload) != string(canonical) {
		t.Fatalf("legacy payload was rewritten: got %s, want %s", card.Payload, canonical)
	}
	if !strings.Contains(string(card.Payload), `"stage_key":"legacy-stage"`) || !strings.Contains(string(card.Payload), `"runtime_task_ref":"task-001"`) {
		t.Fatalf("legacy fields were dropped: %s", card.Payload)
	}
}

func TestRussianFieldsPrefersQuestionRUOverCoreIdea(t *testing.T) {
	card, err := ParseMarkdown("test.md", []byte(`# Q-1 — Sample

ID: Q-1
Question: Sample question?

## Question (RU)

Примерный вопрос на русском?

## Core Idea (RU)

Суть решения одной строкой.
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prompt, shortAnswer, _ := RussianFields(card)
	if prompt != "Примерный вопрос на русском?" {
		t.Fatalf("ru prompt = %q, want the Question (RU) text", prompt)
	}
	if shortAnswer != "Суть решения одной строкой." {
		t.Fatalf("ru short answer = %q, want the Core Idea (RU) text", shortAnswer)
	}
}

func TestRussianFieldsFallsBackToCoreIdeaWhenNoQuestionRU(t *testing.T) {
	card, err := ParseMarkdown("test.md", []byte(`# Q-2 — Legacy style

ID: Q-2
Question: Legacy question?

## Core Idea (RU)

Единый текст вместо вопроса и ответа.
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prompt, shortAnswer, _ := RussianFields(card)
	if prompt != shortAnswer || prompt == "" {
		t.Fatalf("expected degenerate fallback prompt==answer, got %q vs %q", prompt, shortAnswer)
	}
}

func TestParseMarkdownCapturesTimingAndUsage(t *testing.T) {
	card, err := ParseMarkdown("t.md", []byte("# T-1 — Sample\n\nID: T-1\nTopic: Go / Sync & Patterns\nTiming: 5 мин\nUsage: 6 раз\nQuestion: Q?\n\n## Core Idea (RU)\n\nX.\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if card.Timing != "5 мин" || card.Usage != "6 раз" {
		t.Fatalf("timing=%q usage=%q", card.Timing, card.Usage)
	}
}
