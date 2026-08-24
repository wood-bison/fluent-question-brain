// Package mapping owns the explicit Question Brain ↔ Fluent Lab curriculum
// mapping release contract. It has no access to legacy Track/Group/Topic
// fields: callers must provide stable keys and canonical v1 placement fields.
package mapping

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

const ContractVersion = "question-brain.curriculum-mapping.v1"

// Entry pins an explicit decision to a stable key. Every complete manifest row
// must pin the current revision and content hash, including an unmapped audit
// row, so a stale editorial file cannot move a mapping to a new revision
// silently.
type Entry struct {
	StableKey      string `json:"stable_key"`
	RevisionID     string `json:"revision_id,omitempty"`
	ContentHash    string `json:"content_hash,omitempty"`
	ProgramKey     string `json:"program_key,omitempty"`
	PathKey        string `json:"path_key,omitempty"`
	DomainKey      string `json:"domain_key,omitempty"`
	CapabilityKey  string `json:"capability_key,omitempty"`
	MappingState   string `json:"mapping_state,omitempty"`
	MappingVersion string `json:"mapping_version,omitempty"`
	Source         string `json:"source,omitempty"`
}

// Manifest is a complete, pinned mapping batch. Complete means every current
// production stable key must appear exactly once when a release is approved.
// Unmapped entries are allowed and are useful for an auditable no-inference
// baseline while editorial coverage is still being reviewed.
type Manifest struct {
	ContractVersion string  `json:"contract_version"`
	TaxonomyVersion string  `json:"taxonomy_version"`
	WorkspaceKey    string  `json:"workspace_key"`
	Source          string  `json:"source"`
	Entries         []Entry `json:"entries"`
}

func Decode(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode mapping manifest: %w", err)
	}
	return manifest, nil
}

// Normalize validates the manifest and returns deterministic canonical rows.
// It deliberately validates only explicit v1 fields; legacy labels are not
// accepted as aliases for a Path, Domain, or Capability.
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
	defaultSource := strings.TrimSpace(m.Source)
	if defaultSource == "" {
		return nil, fmt.Errorf("source is required")
	}
	if len(m.Entries) == 0 {
		return nil, fmt.Errorf("entries must not be empty")
	}

	entries := make([]Entry, 0, len(m.Entries))
	seen := make(map[string]struct{}, len(m.Entries))
	for index, input := range m.Entries {
		entry := input
		entry.StableKey = strings.TrimSpace(entry.StableKey)
		entry.RevisionID = strings.TrimSpace(entry.RevisionID)
		entry.ContentHash = strings.TrimSpace(entry.ContentHash)
		entry.ProgramKey = strings.TrimSpace(entry.ProgramKey)
		entry.PathKey = strings.TrimSpace(entry.PathKey)
		entry.DomainKey = strings.TrimSpace(entry.DomainKey)
		entry.CapabilityKey = strings.TrimSpace(entry.CapabilityKey)
		entry.MappingState = strings.ToLower(strings.TrimSpace(entry.MappingState))
		entry.MappingVersion = strings.TrimSpace(entry.MappingVersion)
		entry.Source = strings.TrimSpace(entry.Source)
		if entry.StableKey == "" {
			return nil, fmt.Errorf("entries[%d].stable_key is required", index)
		}
		if _, exists := seen[entry.StableKey]; exists {
			return nil, fmt.Errorf("duplicate stable_key %q", entry.StableKey)
		}
		seen[entry.StableKey] = struct{}{}
		if entry.Source == "" {
			entry.Source = defaultSource
		}
		if entry.MappingState == "" {
			if entry.ProgramKey == "" && entry.PathKey == "" && entry.DomainKey == "" && entry.CapabilityKey == "" {
				entry.MappingState = "unmapped"
			} else {
				entry.MappingState = "proposed"
			}
		}
		if entry.MappingState == "unmapped" {
			if entry.RevisionID == "" {
				return nil, fmt.Errorf("entries[%d] unmapped row must pin revision_id", index)
			}
			if _, err := uuid.Parse(entry.RevisionID); err != nil {
				return nil, fmt.Errorf("entries[%d].revision_id must be a UUID: %w", index, err)
			}
			if entry.ContentHash == "" {
				return nil, fmt.Errorf("entries[%d] unmapped row must pin content_hash", index)
			}
			if err := validateContentHash(entry.ContentHash); err != nil {
				return nil, fmt.Errorf("entries[%d].content_hash: %w", index, err)
			}
			if entry.ProgramKey != "" || entry.PathKey != "" || entry.DomainKey != "" || entry.CapabilityKey != "" {
				return nil, fmt.Errorf("entries[%d] unmapped row cannot contain curriculum keys", index)
			}
			entry.MappingVersion = ""
		} else {
			if entry.RevisionID == "" {
				return nil, fmt.Errorf("entries[%d] mapped row must pin revision_id", index)
			}
			if _, err := uuid.Parse(entry.RevisionID); err != nil {
				return nil, fmt.Errorf("entries[%d].revision_id must be a UUID: %w", index, err)
			}
			if entry.ContentHash == "" {
				return nil, fmt.Errorf("entries[%d] mapped row must pin content_hash", index)
			}
			if err := validateContentHash(entry.ContentHash); err != nil {
				return nil, fmt.Errorf("entries[%d].content_hash: %w", index, err)
			}
			if entry.MappingVersion != "" && entry.MappingVersion != taxonomy.Version {
				return nil, fmt.Errorf("entries[%d].mapping_version must be %q", index, taxonomy.Version)
			}
			if taxonomy.IsDeprecatedCapabilityKey(entry.CapabilityKey) {
				return nil, fmt.Errorf("entries[%d] capability_key %q is deprecated; use its canonical registry key in a new release", index, entry.CapabilityKey)
			}
			placement, err := taxonomy.ResolvePlacement(
				entry.ProgramKey, entry.PathKey, entry.DomainKey,
				entry.CapabilityKey, entry.MappingState,
			)
			if err != nil {
				return nil, fmt.Errorf("entries[%d] %w", index, err)
			}
			if placement.ProgramKey == "" || placement.PathKey == "" || placement.DomainKey == "" {
				return nil, fmt.Errorf("entries[%d] mapped row requires program_key, path_key, and domain_key", index)
			}
			entry.ProgramKey = placement.ProgramKey
			entry.PathKey = placement.PathKey
			entry.DomainKey = placement.DomainKey
			entry.CapabilityKey = placement.CapabilityKey
			entry.MappingState = placement.MappingState
			entry.MappingVersion = taxonomy.Version
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].StableKey < entries[j].StableKey })
	return entries, nil
}

func validateContentHash(value string) error {
	if len(value) != sha256.Size*2 {
		return fmt.Errorf("must be a %d-character SHA-256 hex string", sha256.Size*2)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("must be a SHA-256 hex string: %w", err)
	}
	return nil
}

// Fingerprint is the deterministic release identity for a normalized batch.
// It includes stable keys, pins, and explicit decisions, but never content
// bodies or legacy labels.
func Fingerprint(workspaceKey string, entries []Entry) string {
	canonical := make([]Entry, len(entries))
	copy(canonical, entries)
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].StableKey < canonical[j].StableKey })
	data, _ := json.Marshal(struct {
		WorkspaceKey string  `json:"workspace_key"`
		Taxonomy     string  `json:"taxonomy_version"`
		Entries      []Entry `json:"entries"`
	}{strings.TrimSpace(workspaceKey), taxonomy.Version, canonical})
	hash := sha256.Sum256(data)
	return "question-mapping-release-" + hex.EncodeToString(hash[:8])
}
