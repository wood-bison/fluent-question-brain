package quality

import (
	"testing"

	"github.com/wood-bison/fluent-question-brain/internal/normalize"
)

func TestTaskBoundaryRejectsSolutionInVersionedRuntimeBrief(t *testing.T) {
	card, err := normalize.ParseMarkdown("task.md", []byte(`# T-1 — Rate limiter

ID: T-1
Question: Implement a rate limiter.
Task-Contract-Version: question-brain.task-brief.v1
Task-Kind: runtime_task_reference
Task-Family-Key: task-family.rate-limiter

## Task

Implement it.

## Solution

This must stay in Task Runtime.
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	issues := TaskBoundaryIssues(card)
	if len(issues) != 1 || issues[0].Code != "task_solution_forbidden" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestTaskBoundaryRequiresFamilyForRuntimeReference(t *testing.T) {
	card, err := normalize.ParseMarkdown("task.md", []byte(`# T-2 — Runtime task

ID: T-2
Question: Implement it.
Task-Contract-Version: question-brain.task-brief.v1
Task-Kind: runtime_task_reference

## Task

Implement it.

## Walkthrough

Explain it.
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	issues := TaskBoundaryIssues(card)
	if len(issues) != 1 || issues[0].Code != "task_family_required" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestLegacyTaskBlockRemainsReadable(t *testing.T) {
	card, err := normalize.ParseMarkdown("legacy.md", []byte(`# Legacy — exercise

ID: Legacy
Question: Explain the exercise.

## Task

Historical prose.

## Solution

Historical solution retained in the immutable revision.
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(TaskBoundaryIssues(card)) != 0 {
		t.Fatalf("legacy block should not be rewritten by additive validator")
	}
}

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

func TestPromptIssuesRejectsExtractedSentenceFragments(t *testing.T) {
	for _, prompt := range []string{
		"Построить оптимальный индекс для",
		"Мой сервис использует внешний API, в рамках тарифа у нас",
		"Это обёртка над кешем,",
		"Design a rate limiter for",
	} {
		if got := PromptIssues(prompt, "A useful answer", "A card", "A topic"); !hasIssueCode(got, "fragment_prompt") {
			t.Errorf("prompt %q passed the fragment gate: %#v", prompt, got)
		}
	}
}

func TestPromptIssuesKeepsCompleteImperatives(t *testing.T) {
	for _, prompt := range []string{
		"Напишите функцию для слияния каналов",
		"Рассказать, как устроен LRU cache",
		"Design a rate limiter for an external API",
	} {
		if got := PromptIssues(prompt, "A useful answer", "A card", "A topic"); hasIssueCode(got, "fragment_prompt") {
			t.Errorf("complete prompt %q was classified as a fragment: %#v", prompt, got)
		}
	}
}

func TestCardIssuesRejectsCodeFragmentTitle(t *testing.T) {
	if !IsCodeFragmentTitle("a = map[B]int{}") {
		t.Fatal("map assignment title was not classified as code debris")
	}
	issues := CardIssues(normalize.Card{
		Title:    "a = map[B]int{}",
		Question: "How does comma-ok work?",
	})
	if !hasIssueCode(issues, "code_fragment_title") {
		t.Fatalf("code title passed shape gate: %#v", issues)
	}
	if !IsSemanticShapeIssue("code_fragment_title") {
		t.Fatal("code fragment title is not part of semantic shape gate")
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

func hasIssueCode(issues []PromptIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
