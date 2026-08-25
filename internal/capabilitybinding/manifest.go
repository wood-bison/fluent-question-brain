// Package capabilitybinding owns the reviewed Question -> Capability release
// contract.  It is intentionally separate from curriculum mapping: a Path /
// Domain placement can be complete while a card remains theory-only.
package capabilitybinding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/wood-bison/fluent-question-brain/internal/taxonomy"
)

const ContractVersion = "question-brain.capability-binding.v1"

var validRoles = map[string]struct{}{
	"primary": {}, "prerequisite": {}, "follow_up": {}, "contrast": {},
	"recall": {}, "supporting_evidence": {},
}

var validDispositions = map[string]struct{}{
	"bound": {}, "theory_only": {}, "needs_new_capability": {}, "rejected": {},
}

type Binding struct {
	PathKey       string   `json:"path_key"`
	CapabilityKey string   `json:"capability_key"`
	Role          string   `json:"role"`
	Provenance    string   `json:"provenance"`
	Confidence    *float64 `json:"confidence,omitempty"`
	Evidence      any      `json:"evidence,omitempty"`
}

type Entry struct {
	StableKey   string    `json:"stable_key"`
	RevisionID  string    `json:"revision_id"`
	ContentHash string    `json:"content_hash"`
	Disposition string    `json:"disposition"`
	Rationale   string    `json:"rationale"`
	Bindings    []Binding `json:"bindings,omitempty"`
}

type Manifest struct {
	ContractVersion             string  `json:"contract_version"`
	TaxonomyVersion             string  `json:"taxonomy_version"`
	WorkspaceKey                string  `json:"workspace_key"`
	QuestionReleaseID           string  `json:"question_release_id"`
	CapabilityRegistryReleaseID string  `json:"capability_registry_release_id"`
	Source                      string  `json:"source"`
	Entries                     []Entry `json:"entries"`
}

func Decode(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode capability binding manifest: %w", err)
	}
	return manifest, nil
}

func (m Manifest) Normalize() ([]Entry, error) {
	if strings.TrimSpace(m.ContractVersion) != ContractVersion {
		return nil, fmt.Errorf("contract_version must be %q", ContractVersion)
	}
	if strings.TrimSpace(m.TaxonomyVersion) != taxonomy.Version {
		return nil, fmt.Errorf("taxonomy_version must be %q", taxonomy.Version)
	}
	if strings.TrimSpace(m.WorkspaceKey) == "" {
		return nil, fmt.Errorf("workspace_key is required")
	}
	if strings.TrimSpace(m.QuestionReleaseID) == "" {
		return nil, fmt.Errorf("question_release_id is required")
	}
	if strings.TrimSpace(m.CapabilityRegistryReleaseID) == "" {
		return nil, fmt.Errorf("capability_registry_release_id is required")
	}
	if strings.TrimSpace(m.Source) == "" {
		return nil, fmt.Errorf("source is required")
	}
	if len(m.Entries) == 0 {
		return nil, fmt.Errorf("entries must not be empty")
	}
	entries := make([]Entry, 0, len(m.Entries))
	seen := make(map[string]struct{}, len(m.Entries))
	for i, input := range m.Entries {
		entry := input
		entry.StableKey = strings.TrimSpace(entry.StableKey)
		entry.RevisionID = strings.TrimSpace(entry.RevisionID)
		entry.ContentHash = strings.TrimSpace(entry.ContentHash)
		entry.Disposition = strings.ToLower(strings.TrimSpace(entry.Disposition))
		entry.Rationale = strings.TrimSpace(entry.Rationale)
		if entry.StableKey == "" || !strings.HasPrefix(entry.StableKey, "question.") {
			return nil, fmt.Errorf("entries[%d].stable_key must be a question key", i)
		}
		if _, ok := seen[entry.StableKey]; ok {
			return nil, fmt.Errorf("duplicate stable_key %q", entry.StableKey)
		}
		seen[entry.StableKey] = struct{}{}
		if _, ok := validDispositions[entry.Disposition]; !ok {
			return nil, fmt.Errorf("entries[%d] has invalid disposition %q", i, entry.Disposition)
		}
		if _, err := uuid.Parse(entry.RevisionID); err != nil {
			return nil, fmt.Errorf("entries[%d].revision_id must be a UUID: %w", i, err)
		}
		if err := validateHash(entry.ContentHash); err != nil {
			return nil, fmt.Errorf("entries[%d].content_hash: %w", i, err)
		}
		if entry.Rationale == "" {
			return nil, fmt.Errorf("entries[%d].rationale is required", i)
		}
		if entry.Disposition == "bound" && len(entry.Bindings) == 0 {
			return nil, fmt.Errorf("entries[%d] bound disposition requires at least one binding", i)
		}
		if entry.Disposition != "bound" && len(entry.Bindings) > 0 {
			return nil, fmt.Errorf("entries[%d] %s disposition cannot carry bindings", i, entry.Disposition)
		}
		seenBindings := map[string]struct{}{}
		for j := range entry.Bindings {
			binding := &entry.Bindings[j]
			binding.PathKey = strings.TrimSpace(binding.PathKey)
			binding.CapabilityKey = strings.TrimSpace(binding.CapabilityKey)
			binding.Role = strings.ToLower(strings.TrimSpace(binding.Role))
			binding.Provenance = strings.TrimSpace(binding.Provenance)
			if binding.PathKey == "" || binding.CapabilityKey == "" {
				return nil, fmt.Errorf("entries[%d].bindings[%d] requires path_key and capability_key", i, j)
			}
			if _, ok := validRoles[binding.Role]; !ok {
				return nil, fmt.Errorf("entries[%d].bindings[%d] has invalid role %q", i, j, binding.Role)
			}
			if binding.Provenance == "" {
				return nil, fmt.Errorf("entries[%d].bindings[%d].provenance is required", i, j)
			}
			if taxonomy.IsDeprecatedCapabilityKey(binding.CapabilityKey) {
				return nil, fmt.Errorf("entries[%d].bindings[%d] capability_key %q is deprecated", i, j, binding.CapabilityKey)
			}
			if binding.Confidence != nil && (*binding.Confidence < 0 || *binding.Confidence > 1) {
				return nil, fmt.Errorf("entries[%d].bindings[%d].confidence is outside 0..1", i, j)
			}
			placement, err := taxonomy.ResolvePlacement(taxonomy.DefaultProgramKey, binding.PathKey, "domain.runtime", binding.CapabilityKey, "accepted")
			if err != nil {
				// Domain is part of the reviewed registry, not derivable from a
				// capability namespace. The store validates the actual domain
				// before approval; here only accept canonical key shape and path.
				if taxonomy.IsDeprecatedCapabilityKey(binding.CapabilityKey) {
					return nil, fmt.Errorf("entries[%d].bindings[%d] capability_key %q is deprecated", i, j, binding.CapabilityKey)
				}
				if !strings.HasPrefix(binding.CapabilityKey, "capability.") {
					return nil, fmt.Errorf("entries[%d].bindings[%d] capability_key must be canonical: %w", i, j, err)
				}
			} else {
				binding.CapabilityKey = placement.CapabilityKey
			}
			key := binding.PathKey + "\x00" + binding.CapabilityKey + "\x00" + binding.Role
			if _, ok := seenBindings[key]; ok {
				return nil, fmt.Errorf("entries[%d] duplicate binding %q", i, key)
			}
			seenBindings[key] = struct{}{}
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].StableKey < entries[j].StableKey })
	return entries, nil
}

func validateHash(value string) error {
	if len(value) != sha256.Size*2 {
		return fmt.Errorf("must be a %d-character SHA-256 hex string", sha256.Size*2)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("must be a SHA-256 hex string: %w", err)
	}
	return nil
}

func Fingerprint(m Manifest, entries []Entry) string {
	data, _ := json.Marshal(struct {
		WorkspaceKey                string  `json:"workspace_key"`
		QuestionReleaseID           string  `json:"question_release_id"`
		CapabilityRegistryReleaseID string  `json:"capability_registry_release_id"`
		Entries                     []Entry `json:"entries"`
	}{strings.TrimSpace(m.WorkspaceKey), strings.TrimSpace(m.QuestionReleaseID), strings.TrimSpace(m.CapabilityRegistryReleaseID), entries})
	hash := sha256.Sum256(data)
	return "question-capability-release-" + hex.EncodeToString(hash[:8])
}
