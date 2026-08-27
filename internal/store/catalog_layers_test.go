package store

import (
	"testing"

	"github.com/wood-bison/fluent-question-brain/internal/search"
)

func TestCatalogLearningLayersSummarizesPublishedSections(t *testing.T) {
	layers := catalogLearningLayers(map[string]any{
		"task": map[string]any{"condition": "implement"},
		"sections": []any{
			map[string]any{"title": "Follow-ups", "body": "How would you scale it?"},
			map[string]any{"title": "Common Mistakes", "body": "Do not ignore retries."},
			map[string]any{"title": "Must-Say Terms", "body": "idempotency"},
			map[string]any{"title": "Drill Prompts", "body": "Explain the invariant."},
			map[string]any{"title": "Project Evidence", "body": "Describe your trade-off."},
		},
	}, []string{"en", "ru"}, "short answer", "mechanism")

	want := search.CatalogLearningLayers{
		ShortAnswer: true, Mechanism: true, RussianLayer: true,
		FollowUps: true, Traps: true, Terms: true, Practice: true,
		ProjectEvidence: true, Task: true,
	}
	if layers != want {
		t.Fatalf("layers = %#v, want %#v", layers, want)
	}
}

func TestCatalogLearningLayersDoesNotTreatEmptySectionsAsPresent(t *testing.T) {
	layers := catalogLearningLayers(map[string]any{
		"sections": []any{
			map[string]any{"title": "Follow-ups", "body": "  "},
			map[string]any{"title": "Practice", "body": ""},
		},
	}, []string{"en"}, "", "")

	if layers != (search.CatalogLearningLayers{}) {
		t.Fatalf("empty sections produced false positive: %#v", layers)
	}
}
