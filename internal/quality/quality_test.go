package quality

import (
	"testing"

	"github.com/wood-bison/fluent-question-brain/internal/normalize"
)

func TestPromptIssuesRejectsPDFHeadings(t *testing.T) {
	for _, prompt := range []string{"C", "SQL", "-", ".", "OZ-146 —", "Указатели", "Jquery", "DeepCopy"} {
		if len(PromptIssues(prompt, "A useful answer", "Memory", "Runtime")) == 0 {
			t.Errorf("prompt %q passed the shape gate", prompt)
		}
	}
}

func TestPromptIssuesKeepsConciseQuestions(t *testing.T) {
	for _, prompt := range []string{"Что такое SLA?", "Что такое Сага?", "What is SLA?"} {
		if got := PromptIssues(prompt, "A useful answer", "SLA", "System Design"); len(got) != 0 {
			t.Errorf("prompt %q was rejected: %#v", prompt, got)
		}
	}
}

func TestPromptIssuesRejectsAnswerAndMetadataCopies(t *testing.T) {
	if got := PromptIssues("A useful answer", "A useful answer", "A card", "A topic"); len(got) == 0 {
		t.Fatal("prompt equal to answer passed")
	}
	if got := PromptIssues("A card", "A useful answer", "A card", "A topic"); len(got) == 0 {
		t.Fatal("prompt equal to title passed")
	}
	if got := PromptIssues("A topic", "A useful answer", "A card", "A topic"); len(got) == 0 {
		t.Fatal("prompt equal to topic passed")
	}
}

func TestJSONHasPDFArtifactFindsPageBreak(t *testing.T) {
	if !JSONHasPDFArtifact([]byte(`{"sections":[{"body":"before\fafter"}]}`)) {
		t.Fatal("form-feed in canonical payload was not detected")
	}
	if JSONHasPDFArtifact([]byte(`{"sections":[{"body":"ordinary text"}]}`)) {
		t.Fatal("ordinary canonical payload was rejected")
	}
}

func TestJSONHasPDFLayoutArtifactIgnoresTaxonomyMetadata(t *testing.T) {
	if JSONHasPDFLayoutArtifact([]byte(`{"scope":"Ozon","track":"Backend","title":"A useful question"}`), "Ozon") {
		t.Fatal("taxonomy metadata was treated as a PDF sidebar marker")
	}
	if !JSONHasPDFLayoutArtifact([]byte(`{"scope":"Ozon","track":"Backend","sections":[{"title":"Task","body":"SQL\nExplain the query"}]}`), "Ozon") {
		t.Fatal("learner-facing PDF sidebar marker was not detected")
	}
}

func TestCardIssuesRejectsPlaceholderTitleAndPDFText(t *testing.T) {
	card := normalize.Card{
		Title:    "SQL",
		Question: "What is a syscall?",
		Topic:    "OS, Networking & Concurrency Fundamentals",
		Sections: []normalize.Section{{Title: "Task", Body: "before\fafter"}},
	}
	if got := CardIssues(card); len(got) < 2 {
		t.Fatalf("card issues = %#v, want title and PDF issues", got)
	}
}

func TestCardIssuesRejectsOzonPDFLayoutMarkers(t *testing.T) {
	card := normalize.Card{
		Scope:    "Ozon",
		Title:    "What is a syscall?",
		Sections: []normalize.Section{{Title: "Task", Body: "SQL\nWhat is a syscall?\n\n1/40\nЗадачник"}},
	}

	issues := CardIssues(card)
	for _, issue := range issues {
		if issue.Code == "pdf_layout_artifact" {
			return
		}
	}
	t.Fatalf("expected pdf_layout_artifact, got %#v", issues)
}

func TestHasPDFLayoutArtifactDistinguishesSidebarPrefixFromGoCode(t *testing.T) {
	if !HasPDFLayoutArtifact("Ozon", "GO Go (теория)") {
		t.Fatal("uppercase PDF sidebar prefix was not detected")
	}
	if !HasPDFLayoutArtifact("Ozon", ": : Q Зачем нужен TCP?") {
		t.Fatal("PDF colon prefix was not detected")
	}
	if HasPDFLayoutArtifact("Ozon", "go func() { run() }") {
		t.Fatal("authored lowercase Go code was rejected")
	}
}
