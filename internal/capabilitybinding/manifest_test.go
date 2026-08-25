package capabilitybinding

import "testing"

func testManifest(entries []Entry) Manifest {
	return Manifest{
		ContractVersion:             ContractVersion,
		TaxonomyVersion:             "question-brain.taxonomy.v1",
		WorkspaceKey:                "fluent-interview",
		QuestionReleaseID:           "question-release-test",
		CapabilityRegistryReleaseID: "capability-registry-test",
		Source:                      "test-review",
		Entries:                     entries,
	}
}

func testEntry(disposition string, bindings []Binding) Entry {
	return Entry{
		StableKey:   "question.rate-limiter",
		RevisionID:  "00000000-0000-0000-0000-000000000001",
		ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Disposition: disposition,
		Rationale:   "explicit review decision",
		Bindings:    bindings,
	}
}

func TestNormalizeAllowsManyCapabilitiesForOneQuestion(t *testing.T) {
	entries, err := testManifest([]Entry{testEntry("bound", []Binding{
		{PathKey: "path.nodejs-typescript", CapabilityKey: "capability.nodejs.event-loop-ordering", Role: "primary", Provenance: "editorial"},
		{PathKey: "path.nodejs-typescript", CapabilityKey: "capability.http-api.authentication-authorization", Role: "supporting_evidence", Provenance: "editorial"},
	})}).Normalize()
	if err != nil {
		t.Fatalf("many-to-many binding rejected: %v", err)
	}
	if len(entries[0].Bindings) != 2 {
		t.Fatalf("bindings = %d, want 2", len(entries[0].Bindings))
	}
}

func TestNormalizeAllowsTheoryOnlyWithoutFabricatingBinding(t *testing.T) {
	entries, err := testManifest([]Entry{testEntry("theory_only", nil)}).Normalize()
	if err != nil {
		t.Fatalf("theory-only disposition rejected: %v", err)
	}
	if len(entries[0].Bindings) != 0 {
		t.Fatalf("theory-only entry unexpectedly has bindings: %#v", entries[0].Bindings)
	}
}

func TestNormalizeRejectsBindingOnNonBoundDisposition(t *testing.T) {
	_, err := testManifest([]Entry{testEntry("needs_new_capability", []Binding{{
		PathKey: "path.go", CapabilityKey: "capability.distributed-systems.rate-limiter", Role: "primary", Provenance: "editorial",
	}})}).Normalize()
	if err == nil {
		t.Fatal("needs_new_capability accepted a fabricated binding")
	}
}

func TestNormalizeRejectsDeprecatedCapability(t *testing.T) {
	_, err := testManifest([]Entry{testEntry("bound", []Binding{{
		PathKey: "path.nodejs-typescript", CapabilityKey: "capability.runtime.node-event-loop-001", Role: "primary", Provenance: "legacy",
	}})}).Normalize()
	if err == nil {
		t.Fatal("deprecated capability key accepted in a new binding release")
	}
}

func TestFingerprintIsStableForNormalizedEntries(t *testing.T) {
	manifest := testManifest([]Entry{testEntry("theory_only", nil)})
	entries, err := manifest.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if Fingerprint(manifest, entries) != Fingerprint(manifest, entries) {
		t.Fatal("fingerprint is not deterministic")
	}
}
