package contractmodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func fixture() Bundle {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	return Bundle{
		ContractVersion: ContractVersion,
		QuestionCards:   []QuestionCard{{StableKey: "question.rate-limiter", RevisionID: "qrev-1", ContentHash: hash, Locales: []string{"en", "ru"}, Status: "published"}},
		Capabilities:    []Capability{{Key: "capability.distributed-systems.rate-limiter", Title: Localized{EN: "Rate limiting", RU: "Ограничение частоты"}, Lifecycle: "active"}},
		CapabilityDomainBindings: []CapabilityDomainBinding{
			{CapabilityKey: "capability.distributed-systems.rate-limiter", DomainKey: "domain.distributed-systems", Role: "primary"},
			{CapabilityKey: "capability.distributed-systems.rate-limiter", DomainKey: "domain.http-api", Role: "secondary"},
		},
		QuestionCapabilityBindings: []QuestionCapabilityBinding{{Question: QuestionBinding{StableKey: "question.rate-limiter", RevisionID: "qrev-1", ContentHash: hash}, CapabilityKey: "capability.distributed-systems.rate-limiter", Role: "primary", Provenance: "review"}},
		TaskFamilies:               []TaskFamily{{Key: "task-family.token-bucket", Title: Localized{EN: "Token bucket", RU: "Token bucket"}, CapabilityKeys: []string{"capability.distributed-systems.rate-limiter"}, RevisionIDs: []string{"task.token-bucket-go"}, Status: "released"}},
		TaskRevisions:              []TaskRevision{{TaskID: "task.token-bucket-go", Revision: 1, TaskFamilyKey: "task-family.token-bucket", Language: "go", Profile: "go", Status: "released", ImmutableHash: hash}},
	}
}

func TestBundleAcceptsManyToManyDomainBinding(t *testing.T) {
	if err := fixture().Validate(); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
}

func TestBundleRejectsTaskSequenceCapability(t *testing.T) {
	bundle := fixture()
	bundle.Capabilities[0].Key = "capability.runtime.rate-limiter-001"
	if err := bundle.Validate(); err == nil {
		t.Fatal("task sequence capability key was accepted")
	}
}

func TestWorkspaceFixtureValidatesInQuestionBrain(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate contract fixture")
	}
	payload, err := os.ReadFile(filepath.Join(filepath.Dir(sourceFile), "..", "..", "docs", "contracts", "question-capability-task.v1.fixture.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var bundle Bundle
	if err := json.Unmarshal(payload, &bundle); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if err := bundle.Validate(); err != nil {
		t.Fatalf("workspace fixture rejected: %v", err)
	}
}
