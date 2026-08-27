package store

import (
	"strings"

	"github.com/wood-bison/fluent-question-brain/internal/search"
)

// catalogLearningLayers derives a deliberately small, answer-free summary
// from the published normalized payload. The catalog must remain cheap and
// must not expose authored answer bodies, while learners still need to know
// whether a card has follow-ups, traps, terms, practice, or a task brief.
// Titles are normalized here once at the Question Brain boundary; clients do
// not infer completeness from localized copy or from a single short answer.
func catalogLearningLayers(
	payload map[string]any,
	availableLocales []string,
	shortAnswer string,
	explanation string,
) search.CatalogLearningLayers {
	layers := search.CatalogLearningLayers{
		ShortAnswer: strings.TrimSpace(shortAnswer) != "",
		Mechanism:   strings.TrimSpace(explanation) != "",
	}
	for _, locale := range availableLocales {
		if strings.EqualFold(strings.TrimSpace(locale), "ru") {
			layers.RussianLayer = true
			break
		}
	}

	if value, ok := payload["task"]; ok {
		layers.Task = meaningfulCatalogValue(value)
	}
	if value, ok := payload["practice"]; ok {
		layers.Practice = layers.Practice || meaningfulCatalogValue(value)
	}
	if value, ok := payload["follow_ups"]; ok {
		layers.FollowUps = layers.FollowUps || meaningfulCatalogValue(value)
	}
	if value, ok := payload["traps"]; ok {
		layers.Traps = layers.Traps || meaningfulCatalogValue(value)
	}
	if value, ok := payload["terms"]; ok {
		layers.Terms = layers.Terms || meaningfulCatalogValue(value)
	}
	if value, ok := payload["project_evidence"]; ok {
		layers.ProjectEvidence = layers.ProjectEvidence || meaningfulCatalogValue(value)
	}

	if sections, ok := payload["sections"].([]any); ok {
		for _, raw := range sections {
			section, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			title, _ := section["title"].(string)
			if !meaningfulCatalogValue(section["body"]) {
				continue
			}
			title = normalizeCatalogTitle(title)
			switch {
			case containsCatalogTerm(title, "follow-up", "follow up"):
				layers.FollowUps = true
			case containsCatalogTerm(title, "trap", "common mistake", "pitfall", "edge case"):
				layers.Traps = true
			case containsCatalogTerm(title, "must-say", "must say", "term", "terminology", "vocabulary"):
				layers.Terms = true
			case containsCatalogTerm(title, "practice", "drill", "code example", "exercise", "task", "walkthrough"):
				layers.Practice = true
			case containsCatalogTerm(title, "go deeper", "deep dive", "mechanism", "core idea", "explanation"):
				layers.Mechanism = true
			}
			if containsCatalogTerm(title, "project evidence") {
				layers.ProjectEvidence = true
			}
		}
	}
	return layers
}

func normalizeCatalogTitle(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func containsCatalogTerm(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func meaningfulCatalogValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}
