package store

import "testing"

func TestCatalogMetadataExposesCurriculumMappingWithoutConfusingLegacyFields(t *testing.T) {
	metadata := catalogMetadata(map[string]any{
		"track":           "Backend",
		"group":           "Common Questions",
		"topic":           "Node / Event Loop & Scheduling",
		"program_key":     "program.backend-engineer",
		"path_key":        "path.nodejs-typescript",
		"domain_key":      "domain.runtime",
		"capability_key":  "capability.runtime.event-loop",
		"mapping_state":   "accepted",
		"mapping_version": "question-brain.taxonomy.v1",
	})
	if metadata.ProgramKey != "program.backend-engineer" || metadata.PathKey != "path.nodejs-typescript" || metadata.DomainKey != "domain.runtime" || metadata.CapabilityKey != "capability.runtime.event-loop" {
		t.Fatalf("curriculum metadata = %#v", metadata)
	}
	if metadata.StageKey != "domain.runtime" {
		t.Fatalf("stage compatibility projection = %q", metadata.StageKey)
	}
	if metadata.Group != "Common Questions" || metadata.Topic != "Node / Event Loop & Scheduling" || metadata.DomainKey == metadata.Group || metadata.CapabilityKey == metadata.Topic {
		t.Fatalf("legacy fields were confused with curriculum fields = %#v", metadata)
	}
}

func TestCatalogMetadataPreservesExplicitLegacyStageKey(t *testing.T) {
	metadata := catalogMetadata(map[string]any{"stage_key": "legacy-stage", "capability_key": "legacy-capability"})
	if metadata.StageKey != "legacy-stage" || metadata.DomainKey != "" || metadata.CapabilityKey != "legacy-capability" {
		t.Fatalf("legacy compatibility metadata = %#v", metadata)
	}
}
