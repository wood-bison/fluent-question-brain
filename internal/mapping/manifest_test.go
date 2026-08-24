package mapping

import "testing"

func validManifest(entries []Entry) Manifest {
	return Manifest{
		ContractVersion: ContractVersion,
		TaxonomyVersion: "question-brain.taxonomy.v1",
		WorkspaceKey:    "fluent-interview",
		Source:          "test-editorial",
		Entries:         entries,
	}
}

func TestNormalizeRequiresExplicitPlacementAndRevisionPin(t *testing.T) {
	_, err := validManifest([]Entry{{StableKey: "question.a", MappingState: "proposed", PathKey: "path.go", DomainKey: "domain.runtime", ProgramKey: "program.backend-engineer"}}).Normalize()
	if err == nil {
		t.Fatal("mapped row without revision_id/content_hash was accepted")
	}

	entries, err := validManifest([]Entry{{
		StableKey:   "question.a",
		RevisionID:  "00000000-0000-0000-0000-000000000001",
		ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProgramKey:  "program.backend-engineer",
		PathKey:     "Go",
		DomainKey:   "Runtime",
	}}).Normalize()
	if err != nil {
		t.Fatalf("explicit aliases rejected: %v", err)
	}
	if entries[0].ProgramKey != "program.backend-engineer" || entries[0].PathKey != "path.go" || entries[0].DomainKey != "domain.runtime" || entries[0].MappingState != "proposed" {
		t.Fatalf("normalized entry = %#v", entries[0])
	}
}

func TestNormalizeUnmappedRowDoesNotInventKeys(t *testing.T) {
	entries, err := validManifest([]Entry{{
		StableKey:    "question.a",
		RevisionID:   "00000000-0000-0000-0000-000000000001",
		ContentHash:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MappingState: "unmapped",
	}}).Normalize()
	if err != nil {
		t.Fatalf("unmapped row rejected: %v", err)
	}
	if entries[0].ProgramKey != "" || entries[0].PathKey != "" || entries[0].DomainKey != "" || entries[0].CapabilityKey != "" {
		t.Fatalf("unmapped row acquired inferred keys: %#v", entries[0])
	}
}

func TestNormalizeRejectsLegacyFieldsAsCurriculumKeys(t *testing.T) {
	_, err := validManifest([]Entry{
		{StableKey: "question.a", RevisionID: "00000000-0000-0000-0000-000000000001", ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PathKey: "Backend", DomainKey: "Common Questions", ProgramKey: "program.backend-engineer"},
	}).Normalize()
	if err == nil {
		t.Fatal("legacy Track/Group values were accepted as curriculum aliases")
	}
}

func TestNormalizeRejectsMalformedRevisionPin(t *testing.T) {
	_, err := validManifest([]Entry{{
		StableKey:   "question.a",
		RevisionID:  "not-a-uuid",
		ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProgramKey:  "program.backend-engineer",
		PathKey:     "path.go",
		DomainKey:   "domain.runtime",
	}}).Normalize()
	if err == nil {
		t.Fatal("malformed revision pin was accepted")
	}
}

func TestFingerprintIsDeterministicForEntryOrder(t *testing.T) {
	left := []Entry{
		{StableKey: "question.b", ContentHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", MappingState: "unmapped"},
		{StableKey: "question.a", ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MappingState: "unmapped"},
	}
	right := []Entry{left[1], left[0]}
	if Fingerprint("fluent-interview", left) != Fingerprint("fluent-interview", right) {
		t.Fatal("mapping fingerprint changed with entry order")
	}
}
