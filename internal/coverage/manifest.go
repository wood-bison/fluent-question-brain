// Package coverage owns the release-pinned capability coverage policy.
//
// It does not create a second Question -> Capability relation. Card roles are
// read from the reviewed capability-binding release; this package only
// classifies the pinned cards as core, supplemental, or quarantined and
// verifies that declared primary/supporting targets are actually satisfied.
package coverage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/wood-bison/fluent-question-brain/internal/capabilitybinding"
	"github.com/wood-bison/fluent-question-brain/internal/taxonomy"
)

const ContractVersion = "question-brain.capability-coverage-target.v1"

type PresentationRole string

const (
	PrimaryQuestion  PresentationRole = "primary_question"
	SupportingPrompt PresentationRole = "supporting_prompt"
)

type CardDisposition string

const (
	Core         CardDisposition = "core"
	Supplemental CardDisposition = "supplemental"
	Quarantined  CardDisposition = "quarantined"
)

type ContractError struct {
	Code   string
	Detail string
}

func (e *ContractError) Error() string {
	return fmt.Sprintf("coverage target contract %s: %s", e.Code, e.Detail)
}

type Target struct {
	PathKey                  string `json:"path_key"`
	CapabilityKey            string `json:"capability_key"`
	Mandatory                bool   `json:"mandatory"`
	MinimumPrimaryQuestions  int    `json:"minimum_primary_questions"`
	MinimumSupportingPrompts int    `json:"minimum_supporting_prompts"`
	Rationale                string `json:"rationale"`
}

// CardClassification pins a coverage decision to an immutable question
// revision. It deliberately contains no Path, Capability, or role fields:
// those remain owned by the referenced capability-binding release.
type CardClassification struct {
	StableKey   string          `json:"stable_key"`
	RevisionID  string          `json:"revision_id"`
	ContentHash string          `json:"content_hash"`
	Disposition CardDisposition `json:"disposition"`
	Rationale   string          `json:"rationale"`
}

type Manifest struct {
	ContractVersion             string               `json:"contract_version"`
	TaxonomyVersion             string               `json:"taxonomy_version"`
	WorkspaceKey                string               `json:"workspace_key"`
	QuestionReleaseID           string               `json:"question_release_id"`
	CapabilityRegistryReleaseID string               `json:"capability_registry_release_id"`
	CapabilityBindingReleaseID  string               `json:"capability_binding_release_id"`
	MinimumCoverageScoreBPS     int                  `json:"minimum_coverage_score_bps"`
	Source                      string               `json:"source"`
	Targets                     []Target             `json:"targets"`
	Cards                       []CardClassification `json:"cards"`
}

type TargetCoverage struct {
	PathKey                  string `json:"path_key"`
	CapabilityKey            string `json:"capability_key"`
	Mandatory                bool   `json:"mandatory"`
	PrimaryQuestions         int    `json:"primary_questions"`
	SupportingPrompts        int    `json:"supporting_prompts"`
	MinimumPrimaryQuestions  int    `json:"minimum_primary_questions"`
	MinimumSupportingPrompts int    `json:"minimum_supporting_prompts"`
	Ready                    bool   `json:"ready"`
}

type Report struct {
	ContractVersion            string           `json:"contract_version"`
	CoverageTargetReleaseID    string           `json:"coverage_target_release_id"`
	CapabilityBindingReleaseID string           `json:"capability_binding_release_id"`
	Cards                      int              `json:"cards"`
	Core                       int              `json:"core"`
	Supplemental               int              `json:"supplemental"`
	Quarantined                int              `json:"quarantined"`
	Targets                    []TargetCoverage `json:"targets"`
	Ready                      bool             `json:"ready"`
}

func Decode(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, contractError("invalid_manifest", err.Error())
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return Manifest{}, contractError("invalid_manifest", "manifest must contain exactly one JSON document")
	} else if !errors.Is(err, io.EOF) {
		return Manifest{}, contractError("invalid_manifest", err.Error())
	}
	return manifest, nil
}

func (m Manifest) Normalize() (Manifest, error) {
	m.ContractVersion = strings.TrimSpace(m.ContractVersion)
	m.TaxonomyVersion = strings.TrimSpace(m.TaxonomyVersion)
	m.WorkspaceKey = strings.TrimSpace(m.WorkspaceKey)
	m.QuestionReleaseID = strings.TrimSpace(m.QuestionReleaseID)
	m.CapabilityRegistryReleaseID = strings.TrimSpace(m.CapabilityRegistryReleaseID)
	m.CapabilityBindingReleaseID = strings.TrimSpace(m.CapabilityBindingReleaseID)
	m.Source = strings.TrimSpace(m.Source)
	if m.ContractVersion != ContractVersion {
		return Manifest{}, contractError("unsupported_contract_version", fmt.Sprintf("expected %q", ContractVersion))
	}
	if m.TaxonomyVersion != taxonomy.Version {
		return Manifest{}, contractError("unsupported_taxonomy_version", fmt.Sprintf("expected %q", taxonomy.Version))
	}
	for name, value := range map[string]string{
		"workspace_key": m.WorkspaceKey, "question_release_id": m.QuestionReleaseID,
		"capability_registry_release_id": m.CapabilityRegistryReleaseID,
		"capability_binding_release_id":  m.CapabilityBindingReleaseID, "source": m.Source,
	} {
		if value == "" {
			return Manifest{}, contractError("missing_release_pin", name+" is required")
		}
	}
	if m.MinimumCoverageScoreBPS < 9000 || m.MinimumCoverageScoreBPS > 10000 {
		return Manifest{}, contractError("invalid_coverage_threshold", "minimum_coverage_score_bps must be within 9000..10000")
	}
	if len(m.Targets) == 0 {
		return Manifest{}, contractError("empty_targets", "targets must not be empty")
	}
	if len(m.Cards) == 0 {
		return Manifest{}, contractError("empty_card_classification", "cards must not be empty")
	}

	seenTargets := make(map[string]struct{}, len(m.Targets))
	for i := range m.Targets {
		target := &m.Targets[i]
		target.PathKey = strings.TrimSpace(target.PathKey)
		target.CapabilityKey = strings.TrimSpace(target.CapabilityKey)
		target.Rationale = strings.TrimSpace(target.Rationale)
		if !strings.HasPrefix(target.PathKey, "path.") || !strings.HasPrefix(target.CapabilityKey, "capability.") {
			return Manifest{}, contractError("invalid_target_identity", fmt.Sprintf("targets[%d] requires canonical path and capability keys", i))
		}
		if taxonomy.IsDeprecatedCapabilityKey(target.CapabilityKey) {
			return Manifest{}, contractError("deprecated_capability", fmt.Sprintf("targets[%d] uses deprecated capability %q", i, target.CapabilityKey))
		}
		if target.MinimumPrimaryQuestions < 0 || target.MinimumSupportingPrompts < 0 {
			return Manifest{}, contractError("invalid_target_count", fmt.Sprintf("targets[%d] counts must be non-negative", i))
		}
		if target.Mandatory && target.MinimumPrimaryQuestions == 0 {
			return Manifest{}, contractError("empty_mandatory_capability", fmt.Sprintf("targets[%d] mandatory capability requires a primary question", i))
		}
		if target.Rationale == "" {
			return Manifest{}, contractError("missing_rationale", fmt.Sprintf("targets[%d].rationale is required", i))
		}
		key := targetKey(target.PathKey, target.CapabilityKey)
		if _, ok := seenTargets[key]; ok {
			return Manifest{}, contractError("duplicate_target", key)
		}
		seenTargets[key] = struct{}{}
	}

	seenCards := make(map[string]struct{}, len(m.Cards))
	for i := range m.Cards {
		card := &m.Cards[i]
		card.StableKey = strings.TrimSpace(card.StableKey)
		card.RevisionID = strings.TrimSpace(card.RevisionID)
		card.ContentHash = strings.ToLower(strings.TrimSpace(card.ContentHash))
		card.Disposition = CardDisposition(strings.ToLower(strings.TrimSpace(string(card.Disposition))))
		card.Rationale = strings.TrimSpace(card.Rationale)
		if !strings.HasPrefix(card.StableKey, "question.") {
			return Manifest{}, contractError("invalid_card_identity", fmt.Sprintf("cards[%d].stable_key must be a question key", i))
		}
		if _, err := uuid.Parse(card.RevisionID); err != nil {
			return Manifest{}, contractError("invalid_card_revision", fmt.Sprintf("cards[%d].revision_id must be a UUID", i))
		}
		if err := validateHash(card.ContentHash); err != nil {
			return Manifest{}, contractError("invalid_card_hash", fmt.Sprintf("cards[%d]: %v", i, err))
		}
		if card.Disposition != Core && card.Disposition != Supplemental && card.Disposition != Quarantined {
			return Manifest{}, contractError("invalid_card_disposition", fmt.Sprintf("cards[%d] has %q", i, card.Disposition))
		}
		if card.Rationale == "" {
			return Manifest{}, contractError("missing_rationale", fmt.Sprintf("cards[%d].rationale is required", i))
		}
		if _, ok := seenCards[card.StableKey]; ok {
			return Manifest{}, contractError("duplicate_card", card.StableKey)
		}
		seenCards[card.StableKey] = struct{}{}
	}

	sort.Slice(m.Targets, func(i, j int) bool {
		return targetKey(m.Targets[i].PathKey, m.Targets[i].CapabilityKey) < targetKey(m.Targets[j].PathKey, m.Targets[j].CapabilityKey)
	})
	sort.Slice(m.Cards, func(i, j int) bool { return m.Cards[i].StableKey < m.Cards[j].StableKey })
	return m, nil
}

// PresentationRoleForBinding derives learner presentation from the canonical
// binding role. The derived value is never stored as a second placement role.
func PresentationRoleForBinding(role string) (PresentationRole, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "primary":
		return PrimaryQuestion, nil
	case "prerequisite", "follow_up", "contrast", "recall", "supporting_evidence":
		return SupportingPrompt, nil
	default:
		return "", contractError("unknown_binding_role", fmt.Sprintf("role %q has no presentation mapping", role))
	}
}

// ValidateAgainstBindings proves that the target is a complete overlay on one
// exact capability-binding release. It rejects raw-count-only targets,
// quarantined learner bindings, stale pins, and incomplete card ledgers.
func ValidateAgainstBindings(input Manifest, bindings capabilitybinding.Manifest) (Report, error) {
	manifest, err := input.Normalize()
	if err != nil {
		return Report{}, err
	}
	bindingEntries, err := bindings.Normalize()
	if err != nil {
		return Report{}, contractError("invalid_binding_manifest", err.Error())
	}
	bindingReleaseID := capabilitybinding.Fingerprint(bindings, bindingEntries)
	if manifest.WorkspaceKey != strings.TrimSpace(bindings.WorkspaceKey) ||
		manifest.QuestionReleaseID != strings.TrimSpace(bindings.QuestionReleaseID) ||
		manifest.CapabilityRegistryReleaseID != strings.TrimSpace(bindings.CapabilityRegistryReleaseID) ||
		manifest.CapabilityBindingReleaseID != bindingReleaseID {
		return Report{}, contractError("stale_release_join", "coverage and capability-binding release pins do not match")
	}

	byStableKey := make(map[string]capabilitybinding.Entry, len(bindingEntries))
	for _, entry := range bindingEntries {
		byStableKey[entry.StableKey] = entry
	}
	if len(manifest.Cards) != len(bindingEntries) {
		return Report{}, contractError("incomplete_card_ledger", fmt.Sprintf("coverage cards=%d binding entries=%d", len(manifest.Cards), len(bindingEntries)))
	}

	targets := make(map[string]Target, len(manifest.Targets))
	primary := make(map[string]map[string]struct{}, len(manifest.Targets))
	supporting := make(map[string]map[string]struct{}, len(manifest.Targets))
	for _, target := range manifest.Targets {
		key := targetKey(target.PathKey, target.CapabilityKey)
		targets[key] = target
		primary[key] = map[string]struct{}{}
		supporting[key] = map[string]struct{}{}
	}

	report := Report{ContractVersion: ContractVersion, CapabilityBindingReleaseID: bindingReleaseID, Cards: len(manifest.Cards)}
	for _, card := range manifest.Cards {
		entry, ok := byStableKey[card.StableKey]
		if !ok {
			return Report{}, contractError("unknown_card", card.StableKey+" is not in the pinned binding release")
		}
		if card.RevisionID != entry.RevisionID || card.ContentHash != strings.ToLower(entry.ContentHash) {
			return Report{}, contractError("stale_card_pin", card.StableKey)
		}
		switch card.Disposition {
		case Core:
			report.Core++
			if entry.Disposition != "bound" || len(entry.Bindings) == 0 {
				return Report{}, contractError("unbound_core_card", card.StableKey)
			}
			for _, binding := range entry.Bindings {
				key := targetKey(binding.PathKey, binding.CapabilityKey)
				if _, ok := targets[key]; !ok {
					return Report{}, contractError("core_binding_without_target", card.StableKey+" -> "+key)
				}
				presentationRole, err := PresentationRoleForBinding(binding.Role)
				if err != nil {
					return Report{}, err
				}
				if presentationRole == PrimaryQuestion {
					primary[key][card.StableKey] = struct{}{}
				} else {
					supporting[key][card.StableKey] = struct{}{}
				}
			}
		case Supplemental:
			report.Supplemental++
			if entry.Disposition == "rejected" {
				return Report{}, contractError("rejected_card_marked_supplemental", card.StableKey)
			}
		case Quarantined:
			report.Quarantined++
			if entry.Disposition == "bound" || len(entry.Bindings) > 0 {
				return Report{}, contractError("quarantined_card_is_bound", card.StableKey)
			}
		}
		delete(byStableKey, card.StableKey)
	}
	if len(byStableKey) != 0 {
		return Report{}, contractError("incomplete_card_ledger", "one or more binding entries have no classification")
	}

	for _, target := range manifest.Targets {
		key := targetKey(target.PathKey, target.CapabilityKey)
		item := TargetCoverage{
			PathKey: target.PathKey, CapabilityKey: target.CapabilityKey, Mandatory: target.Mandatory,
			PrimaryQuestions: len(primary[key]), SupportingPrompts: len(supporting[key]),
			MinimumPrimaryQuestions: target.MinimumPrimaryQuestions, MinimumSupportingPrompts: target.MinimumSupportingPrompts,
		}
		item.Ready = item.PrimaryQuestions >= item.MinimumPrimaryQuestions && item.SupportingPrompts >= item.MinimumSupportingPrompts
		if !item.Ready {
			return Report{}, contractError("target_role_gap", fmt.Sprintf("%s has primary=%d/%d supporting=%d/%d", key, item.PrimaryQuestions, item.MinimumPrimaryQuestions, item.SupportingPrompts, item.MinimumSupportingPrompts))
		}
		report.Targets = append(report.Targets, item)
	}
	report.CoverageTargetReleaseID = Fingerprint(manifest)
	report.Ready = true
	return report, nil
}

func Fingerprint(manifest Manifest) string {
	normalized, err := manifest.Normalize()
	if err != nil {
		return ""
	}
	payload, _ := json.Marshal(struct {
		WorkspaceKey                string               `json:"workspace_key"`
		QuestionReleaseID           string               `json:"question_release_id"`
		CapabilityRegistryReleaseID string               `json:"capability_registry_release_id"`
		CapabilityBindingReleaseID  string               `json:"capability_binding_release_id"`
		MinimumCoverageScoreBPS     int                  `json:"minimum_coverage_score_bps"`
		Targets                     []Target             `json:"targets"`
		Cards                       []CardClassification `json:"cards"`
	}{normalized.WorkspaceKey, normalized.QuestionReleaseID, normalized.CapabilityRegistryReleaseID, normalized.CapabilityBindingReleaseID, normalized.MinimumCoverageScoreBPS, normalized.Targets, normalized.Cards})
	hash := sha256.Sum256(payload)
	return "capability-coverage-target-release-" + hex.EncodeToString(hash[:8])
}

func targetKey(pathKey, capabilityKey string) string {
	return strings.TrimSpace(pathKey) + "\x00" + strings.TrimSpace(capabilityKey)
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

func contractError(code, detail string) error {
	return &ContractError{Code: code, Detail: detail}
}
