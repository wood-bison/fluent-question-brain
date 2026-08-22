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
	if card.StableKey != "legacy.q001" || card.Slug != "q001" {
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
