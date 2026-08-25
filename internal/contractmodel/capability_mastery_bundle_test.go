package contractmodel

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func goldenCapabilityMasteryBundle(t *testing.T) map[string]any {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate fixture")
	}
	paths := []string{filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", "docs", "contracts", "capability-mastery-bundle.v1.fixture.json"), filepath.Join(filepath.Dir(sourceFile), "..", "..", "docs", "contracts", "capability-mastery-bundle.v1.fixture.json")}
	var payload []byte
	var err error
	for _, path := range paths {
		payload, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestCapabilityMasteryGoldenFixtureValidInQuestionBrain(t *testing.T) {
	payload, err := json.Marshal(goldenCapabilityMasteryBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCapabilityMasteryBundleJSON(payload); err != nil {
		t.Fatalf("golden fixture rejected: %v", err)
	}
}

func TestCapabilityMasteryQuestionBrainRejectsStaleTaskAndVersion(t *testing.T) {
	value := goldenCapabilityMasteryBundle(t)
	value["learningSession"].(map[string]any)["taskRevision"].(map[string]any)["immutableHash"] = strings.Repeat("b", 64)
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var contractError *CapabilityMasteryContractError
	if !errors.As(ValidateCapabilityMasteryBundleJSON(payload), &contractError) || contractError.Code != "stale-task-revision-hash" {
		t.Fatalf("unexpected stale result: %v", ValidateCapabilityMasteryBundleJSON(payload))
	}
	value = goldenCapabilityMasteryBundle(t)
	value["contractVersion"] = "capability-mastery-bundle.v0"
	payload, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.As(ValidateCapabilityMasteryBundleJSON(payload), &contractError) || contractError.Code != "unsupported_contract_version" {
		t.Fatalf("unexpected version result: %v", ValidateCapabilityMasteryBundleJSON(payload))
	}
}
