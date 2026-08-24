// Package contractmodel contains the Question Brain-owned validation view of
// the cross-repository Question → Capability → Task contract. It is not a
// persistence model and deliberately does not expose task source or learner
// evidence to Question Brain.
package contractmodel

import (
	"fmt"
	"regexp"
	"strings"
)

const ContractVersion = "question-capability-task.v1"

type Localized struct {
	EN string `json:"en"`
	RU string `json:"ru"`
}

type QuestionBinding struct {
	StableKey   string `json:"stableKey"`
	RevisionID  string `json:"revisionId"`
	ContentHash string `json:"contentHash"`
	Role        string `json:"role,omitempty"`
}

type QuestionCard struct {
	StableKey   string   `json:"stableKey"`
	RevisionID  string   `json:"revisionId"`
	ContentHash string   `json:"contentHash"`
	Locales     []string `json:"locales"`
	Status      string   `json:"status"`
}

type Capability struct {
	Key        string    `json:"key"`
	Title      Localized `json:"title"`
	Lifecycle  string    `json:"lifecycle"`
	Aliases    []string  `json:"aliases,omitempty"`
	Supersedes []string  `json:"supersedes,omitempty"`
}

type CapabilityDomainBinding struct {
	CapabilityKey string `json:"capabilityKey"`
	DomainKey     string `json:"domainKey"`
	Role          string `json:"role"`
}

type QuestionCapabilityBinding struct {
	Question      QuestionBinding `json:"question"`
	CapabilityKey string          `json:"capabilityKey"`
	Role          string          `json:"role"`
	Provenance    string          `json:"provenance"`
	Confidence    *float64        `json:"confidence,omitempty"`
}

type TaskFamily struct {
	Key              string            `json:"key"`
	Title            Localized         `json:"title"`
	CapabilityKeys   []string          `json:"capabilityKeys"`
	QuestionBindings []QuestionBinding `json:"questionBindings"`
	RevisionIDs      []string          `json:"revisionIds"`
	Status           string            `json:"status"`
}

type TaskRevision struct {
	TaskID        string `json:"taskId"`
	Revision      int    `json:"revision"`
	TaskFamilyKey string `json:"taskFamilyKey"`
	Language      string `json:"language"`
	Profile       string `json:"profile"`
	Status        string `json:"status"`
	ImmutableHash string `json:"immutableHash"`
}

type ContentRelation struct {
	RelationID string   `json:"relationId"`
	From       string   `json:"from"`
	To         string   `json:"to"`
	Kind       string   `json:"kind"`
	Status     string   `json:"status"`
	Provenance string   `json:"provenance"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type Run struct {
	RunID        string `json:"runId"`
	TaskID       string `json:"taskId"`
	TaskRevision int    `json:"taskRevision"`
	Status       string `json:"status"`
	ResultHash   string `json:"resultHash"`
}

type Evidence struct {
	EvidenceID string `json:"evidenceId"`
	RunID      string `json:"runId"`
	State      string `json:"state"`
	RecordedAt string `json:"recordedAt"`
}

type Bundle struct {
	ContractVersion            string                      `json:"contractVersion"`
	QuestionCards              []QuestionCard              `json:"questionCards"`
	Capabilities               []Capability                `json:"capabilities"`
	CapabilityDomainBindings   []CapabilityDomainBinding   `json:"capabilityDomainBindings"`
	QuestionCapabilityBindings []QuestionCapabilityBinding `json:"questionCapabilityBindings"`
	TaskFamilies               []TaskFamily                `json:"taskFamilies"`
	TaskRevisions              []TaskRevision              `json:"taskRevisions"`
	ContentRelations           []ContentRelation           `json:"contentRelations"`
	Runs                       []Run                       `json:"runs"`
	Evidence                   []Evidence                  `json:"evidence"`
}

var (
	capabilityKey = regexp.MustCompile(`^capability\.[a-z0-9-]+\.[a-z0-9][a-z0-9-]*$`)
	questionKey   = regexp.MustCompile(`^question\.[a-z0-9][a-z0-9._-]*$`)
	domainKey     = regexp.MustCompile(`^domain\.[a-z0-9][a-z0-9-]*$`)
	sha256        = regexp.MustCompile(`^[a-f0-9]{64}$`)
	familyKey     = regexp.MustCompile(`^task-family\.[a-z0-9][a-z0-9-]*$`)
)

func validCapability(value string) bool {
	return capabilityKey.MatchString(value) && !regexp.MustCompile(`-\d{3,}$`).MatchString(value)
}

func validBinding(value QuestionBinding) bool {
	return questionKey.MatchString(value.StableKey) && value.RevisionID != "" && sha256.MatchString(value.ContentHash)
}

// Validate enforces identity and ownership invariants before a cross-system
// release is accepted. Graph semantics and editorial decisions stay in the
// Question Brain store/review workflow; this function only validates shape.
func (b Bundle) Validate() error {
	if b.ContractVersion != ContractVersion {
		return fmt.Errorf("unsupported contract version %q", b.ContractVersion)
	}
	questions := map[string]struct{}{}
	for i, card := range b.QuestionCards {
		if !questionKey.MatchString(card.StableKey) || card.RevisionID == "" || !sha256.MatchString(card.ContentHash) {
			return fmt.Errorf("questionCards[%d] has invalid revision identity", i)
		}
		questions[card.StableKey] = struct{}{}
	}
	capabilities := map[string]struct{}{}
	for i, capability := range b.Capabilities {
		if !validCapability(capability.Key) || strings.TrimSpace(capability.Title.EN) == "" || strings.TrimSpace(capability.Title.RU) == "" {
			return fmt.Errorf("capabilities[%d] has invalid identity/title", i)
		}
		capabilities[capability.Key] = struct{}{}
	}
	for i, binding := range b.CapabilityDomainBindings {
		if _, ok := capabilities[binding.CapabilityKey]; !ok || !domainKey.MatchString(binding.DomainKey) {
			return fmt.Errorf("capabilityDomainBindings[%d] has an unknown capability/domain", i)
		}
	}
	for i, binding := range b.QuestionCapabilityBindings {
		if _, ok := questions[binding.Question.StableKey]; !ok || !validBinding(binding.Question) {
			return fmt.Errorf("questionCapabilityBindings[%d] has an unknown question", i)
		}
		if _, ok := capabilities[binding.CapabilityKey]; !ok || strings.TrimSpace(binding.Provenance) == "" {
			return fmt.Errorf("questionCapabilityBindings[%d] has an unknown capability/provenance", i)
		}
		if binding.Confidence != nil && (*binding.Confidence < 0 || *binding.Confidence > 1) {
			return fmt.Errorf("questionCapabilityBindings[%d] confidence is outside 0..1", i)
		}
	}
	families := map[string]struct{}{}
	for i, family := range b.TaskFamilies {
		if !familyKey.MatchString(family.Key) || len(family.CapabilityKeys) == 0 || len(family.RevisionIDs) == 0 || family.Title.EN == "" || family.Title.RU == "" {
			return fmt.Errorf("taskFamilies[%d] has invalid identity/cardinality", i)
		}
		families[family.Key] = struct{}{}
		for _, capability := range family.CapabilityKeys {
			if _, ok := capabilities[capability]; !ok {
				return fmt.Errorf("taskFamilies[%d] references unknown capability", i)
			}
		}
		for _, binding := range family.QuestionBindings {
			if !validBinding(binding) {
				return fmt.Errorf("taskFamilies[%d] has invalid question binding", i)
			}
		}
	}
	for i, revision := range b.TaskRevisions {
		if _, ok := families[revision.TaskFamilyKey]; !ok || revision.Revision < 1 || revision.Language == "" || revision.Profile == "" || !sha256.MatchString(revision.ImmutableHash) {
			return fmt.Errorf("taskRevisions[%d] is not immutable or has an unknown family", i)
		}
	}
	return nil
}
