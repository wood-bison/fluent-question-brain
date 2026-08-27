package coverage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wood-bison/fluent-question-brain/internal/capabilitybinding"
)

const (
	questionRelease = "question-release-test"
	registryRelease = "capability-registry-test"
	primaryHash     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	supportHash     = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	extraHash       = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

func bindingFixture() capabilitybinding.Manifest {
	return capabilitybinding.Manifest{
		ContractVersion: capabilitybinding.ContractVersion, TaxonomyVersion: "question-brain.taxonomy.v1",
		WorkspaceKey: "fluent-interview", QuestionReleaseID: questionRelease,
		CapabilityRegistryReleaseID: registryRelease, Source: "coverage-test",
		Entries: []capabilitybinding.Entry{
			{
				StableKey: "question.primary", RevisionID: "00000000-0000-0000-0000-000000000001", ContentHash: primaryHash,
				Disposition: "bound", Rationale: "reviewed primary", Bindings: []capabilitybinding.Binding{{
					PathKey: "path.nodejs-typescript", CapabilityKey: "capability.nodejs.event-loop-ordering", Role: "primary", Provenance: "editorial",
				}},
			},
			{
				StableKey: "question.support", RevisionID: "00000000-0000-0000-0000-000000000002", ContentHash: supportHash,
				Disposition: "bound", Rationale: "reviewed follow-up", Bindings: []capabilitybinding.Binding{{
					PathKey: "path.nodejs-typescript", CapabilityKey: "capability.nodejs.event-loop-ordering", Role: "follow_up", Provenance: "editorial",
				}},
			},
			{
				StableKey: "question.extra", RevisionID: "00000000-0000-0000-0000-000000000003", ContentHash: extraHash,
				Disposition: "theory_only", Rationale: "reviewed supplementary material",
			},
		},
	}
}

func coverageFixture() Manifest {
	bindings := bindingFixture()
	entries, err := bindings.Normalize()
	if err != nil {
		panic(err)
	}
	return Manifest{
		ContractVersion: ContractVersion, TaxonomyVersion: "question-brain.taxonomy.v1",
		WorkspaceKey: "fluent-interview", QuestionReleaseID: questionRelease,
		CapabilityRegistryReleaseID: registryRelease,
		CapabilityBindingReleaseID:  capabilitybinding.Fingerprint(bindings, entries),
		MinimumCoverageScoreBPS:     9000, Source: "reviewed-coverage-policy",
		Targets: []Target{{
			PathKey: "path.nodejs-typescript", CapabilityKey: "capability.nodejs.event-loop-ordering", Mandatory: true,
			MinimumPrimaryQuestions: 1, MinimumSupportingPrompts: 1, Rationale: "one complete pilot bundle",
		}},
		Cards: []CardClassification{
			{StableKey: "question.primary", RevisionID: "00000000-0000-0000-0000-000000000001", ContentHash: primaryHash, Disposition: Core, Rationale: "primary core evidence"},
			{StableKey: "question.support", RevisionID: "00000000-0000-0000-0000-000000000002", ContentHash: supportHash, Disposition: Core, Rationale: "supporting core evidence"},
			{StableKey: "question.extra", RevisionID: "00000000-0000-0000-0000-000000000003", ContentHash: extraHash, Disposition: Supplemental, Rationale: "useful but not denominator-bearing"},
		},
	}
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var contractErr *ContractError
	if !errors.As(err, &contractErr) || contractErr.Code != code {
		t.Fatalf("error = %v, want code %q", err, code)
	}
}

func TestPresentationRoleIsDerivedFromCanonicalBindingRole(t *testing.T) {
	if role, err := PresentationRoleForBinding("primary"); err != nil || role != PrimaryQuestion {
		t.Fatalf("primary role = %q, %v", role, err)
	}
	for _, input := range []string{"prerequisite", "follow_up", "contrast", "recall", "supporting_evidence"} {
		if role, err := PresentationRoleForBinding(input); err != nil || role != SupportingPrompt {
			t.Fatalf("%s role = %q, %v", input, role, err)
		}
	}
	_, err := PresentationRoleForBinding("diagnostic-from-title")
	requireCode(t, err, "unknown_binding_role")
}

func TestCoverageTargetValidatesCompletePinnedRoleCoverage(t *testing.T) {
	report, err := ValidateAgainstBindings(coverageFixture(), bindingFixture())
	if err != nil {
		t.Fatalf("valid coverage rejected: %v", err)
	}
	if !report.Ready || report.Core != 2 || report.Supplemental != 1 || report.Quarantined != 0 {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Targets) != 1 || report.Targets[0].PrimaryQuestions != 1 || report.Targets[0].SupportingPrompts != 1 {
		t.Fatalf("target report = %#v", report.Targets)
	}
	if !strings.HasPrefix(report.CoverageTargetReleaseID, "capability-coverage-target-release-") {
		t.Fatalf("release id = %q", report.CoverageTargetReleaseID)
	}
}

func TestCoverageTargetRejectsRawCountWithoutPrimaryRole(t *testing.T) {
	manifest := coverageFixture()
	manifest.Targets[0].MinimumPrimaryQuestions = 2
	_, err := ValidateAgainstBindings(manifest, bindingFixture())
	requireCode(t, err, "target_role_gap")
}

func TestCoverageTargetRejectsEmptyMandatoryCapability(t *testing.T) {
	manifest := coverageFixture()
	manifest.Targets[0].MinimumPrimaryQuestions = 0
	_, err := manifest.Normalize()
	requireCode(t, err, "empty_mandatory_capability")
}

func TestCoverageTargetRejectsBoundQuarantine(t *testing.T) {
	manifest := coverageFixture()
	manifest.Cards[0].Disposition = Quarantined
	_, err := ValidateAgainstBindings(manifest, bindingFixture())
	requireCode(t, err, "quarantined_card_is_bound")
}

func TestCoverageTargetRejectsStaleOrIncompleteReleaseJoin(t *testing.T) {
	manifest := coverageFixture()
	manifest.QuestionReleaseID = "question-release-stale"
	_, err := ValidateAgainstBindings(manifest, bindingFixture())
	requireCode(t, err, "stale_release_join")

	manifest = coverageFixture()
	manifest.Cards = manifest.Cards[:2]
	_, err = ValidateAgainstBindings(manifest, bindingFixture())
	requireCode(t, err, "incomplete_card_ledger")
}

func TestCoverageTargetDecodeRejectsUnknownFields(t *testing.T) {
	payload, err := json.Marshal(coverageFixture())
	if err != nil {
		t.Fatal(err)
	}
	payload = []byte(strings.Replace(string(payload), `"source":"reviewed-coverage-policy"`, `"source":"reviewed-coverage-policy","guessed_count":2300`, 1))
	_, err = Decode(payload)
	requireCode(t, err, "invalid_manifest")

	_, err = Decode(append(payload, []byte(` {}`)...))
	requireCode(t, err, "invalid_manifest")
}

func TestCoverageTargetMigrationKeepsClassificationAttachedToBindingRelease(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	path := filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations", "0022_capability_coverage_targets.sql")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(payload)
	for _, required := range []string{
		"content.capability_coverage_target_release",
		"content.capability_coverage_target",
		"content.question_coverage_classification",
		"references content.question_capability_binding_release(binding_release_id)",
		"check (disposition in ('core', 'supplemental', 'quarantined'))",
		"validate_question_coverage_classification",
		"validate_capability_coverage_target_release",
		"binding.question_release_id <> new.question_release_id",
		"binding.capability_registry_release_id <> new.capability_registry_release_id",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"insert into content.question ", "update content.question ", "delete from content.question "} {
		if strings.Contains(strings.ToLower(sql), forbidden) {
			t.Fatalf("migration mutates current question release via %q", forbidden)
		}
	}
}
