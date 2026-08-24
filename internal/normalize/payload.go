package normalize

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wood-bison/fluent-question-brain/internal/taxonomy"
)

// CardFromPayload converts the canonical JSON shape used by the CMS promote
// boundary back into the same normalized card used by the vault importer. The
// conversion deliberately stays in this package so a second writer cannot
// invent a subtly different hash or locale convention.
func CardFromPayload(sourceRef string, payload []byte) (Card, error) {
	var value canonicalCard
	canonical, err := CanonicalJSON(payload)
	if err != nil {
		return Card{}, fmt.Errorf("canonicalize payload: %w", err)
	}
	if err := json.Unmarshal(canonical, &value); err != nil {
		return Card{}, fmt.Errorf("decode canonical payload: %w", err)
	}
	value.StableKey = strings.TrimSpace(value.StableKey)
	value.Slug = strings.TrimSpace(value.Slug)
	value.Title = strings.TrimSpace(value.Title)
	value.Question = strings.TrimSpace(value.Question)
	if value.StableKey == "" || value.Slug == "" || value.Title == "" || value.Question == "" {
		return Card{}, fmt.Errorf("stable_key, slug, title, and question are required")
	}
	if sourceRef == "" {
		sourceRef = "payload://question/" + value.StableKey
	}
	// A legacy promote payload may contain fields owned by another projection
	// (for example stage_key or runtime metadata) that this package does not
	// model. Preserve its canonical bytes and hash exactly; taxonomy v1 is an
	// opt-in additive contract, not a reason to rewrite old revisions.
	if !hasExplicitTaxonomy(value) {
		card := Card{
			SourceRef: sourceRef,
			ID:        value.StableKey,
			StableKey: value.StableKey,
			Slug:      value.Slug,
			Title:     value.Title,
			Track:     value.Track,
			Topic:     value.Topic,
			Scope:     value.Scope,
			Lang:      value.Lang,
			Priority:  value.Priority,
			Group:     value.Group,
			Level:     value.Level,
			Company:   value.Company,
			Question:  value.Question,
			Sections:  append([]Section(nil), value.Sections...),
			Task:      value.Task,
			Rubric:    append([]RubricLevel(nil), value.Rubric...),
			Choices:   value.Choices,
			Payload:   canonical,
		}
		card.Hash = HashCanonicalJSON(card.Payload)
		return card, nil
	}
	placement, err := taxonomyPlacementFromPayload(value)
	if err != nil {
		return Card{}, fmt.Errorf("taxonomy placement: %w", err)
	}
	card := Card{
		SourceRef:      sourceRef,
		ID:             value.StableKey,
		StableKey:      value.StableKey,
		Slug:           value.Slug,
		Title:          value.Title,
		Track:          value.Track,
		Topic:          value.Topic,
		Scope:          value.Scope,
		Lang:           value.Lang,
		Priority:       value.Priority,
		Group:          value.Group,
		Level:          value.Level,
		Company:        value.Company,
		ProgramKey:     placement.ProgramKey,
		PathKey:        placement.PathKey,
		DomainKey:      placement.DomainKey,
		CapabilityKey:  placement.CapabilityKey,
		MappingState:   placement.MappingState,
		MappingVersion: placement.MappingVersion,
		Question:       value.Question,
		Sections:       append([]Section(nil), value.Sections...),
		Task:           value.Task,
		Rubric:         append([]RubricLevel(nil), value.Rubric...),
		Choices:        value.Choices,
	}
	rawPayload, err := canonicalPayload(card)
	if err != nil {
		return Card{}, fmt.Errorf("encode canonical payload: %w", err)
	}
	card.Payload, err = CanonicalJSON(rawPayload)
	if err != nil {
		return Card{}, fmt.Errorf("canonicalize payload: %w", err)
	}
	card.Hash = HashCanonicalJSON(card.Payload)
	return card, nil
}

func taxonomyPlacementFromPayload(value canonicalCard) (taxonomy.Placement, error) {
	domain := value.DomainKey
	if domain == "" {
		domain = value.StageKey
	}
	return taxonomy.ResolvePlacement(
		value.ProgramKey,
		value.PathKey,
		domain,
		value.CapabilityKey,
		value.MappingState,
	)
}

func hasExplicitTaxonomy(value canonicalCard) bool {
	// A state/version without a placement is not a curriculum binding. Treat
	// it as legacy metadata so an older writer cannot force a payload rewrite.
	return value.ProgramKey != "" || value.PathKey != "" || value.DomainKey != "" || value.CapabilityKey != ""
}
