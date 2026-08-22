package normalize

import (
	"encoding/json"
	"fmt"
	"strings"
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
		Question:  value.Question,
		Sections:  append([]Section(nil), value.Sections...),
	}
	card.Payload, err = CanonicalJSON(canonical)
	if err != nil {
		return Card{}, fmt.Errorf("canonicalize payload: %w", err)
	}
	card.Hash = HashCanonicalJSON(card.Payload)
	return card, nil
}
