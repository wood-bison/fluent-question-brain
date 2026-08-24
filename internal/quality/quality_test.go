package quality

import (
	"testing"

	"github.com/wood-bison/fluent-question-brain/internal/normalize"
)

func TestPromptIssuesRejectsPDFHeadings(t *testing.T) {
	for _, prompt := range []string{"C", "SQL", "-", "Указатели", "Jquery", "DeepCopy"} {
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
