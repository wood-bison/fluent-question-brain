package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wood-bison/fluent-question-brain/internal/capabilitybinding"
)

func TestMergeAcceptedCapabilityProposalsPromotesTheoryOnly(t *testing.T) {
	confidence := 0.84
	entries := []capabilitybinding.Entry{{
		StableKey: "question.q099", RevisionID: "rev-q099",
		ContentHash: strings.Repeat("a", 64), Disposition: "theory_only",
		Rationale: "no reviewed station", Bindings: nil,
	}}
	current := map[string]currentCapabilityRevision{
		"question.q099": {StableKey: "question.q099", RevisionID: "rev-q099", PathKey: "path.nodejs-typescript"},
	}
	proposals := []acceptedCapabilityProposal{{
		ID: "proposal-q099", StableKey: "question.q099", RevisionID: "rev-q099",
		PathKey: "path.nodejs-typescript", CapabilityKey: "capability.nodejs.event-loop-ordering",
		Role: "supporting_evidence", Provenance: "semantic-neighbor-v1", Confidence: &confidence,
		Evidence:  json.RawMessage(`{"method":"pgvector-semantic-neighbor"}`),
		Rationale: "Reviewed against the Node event-loop capability.",
	}}

	merged, err := mergeAcceptedCapabilityProposals(entries, proposals, current)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if got := merged[0].Disposition; got != "bound" {
		t.Fatalf("disposition = %q, want bound", got)
	}
	if len(merged[0].Bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(merged[0].Bindings))
	}
	if got := merged[0].Bindings[0].Evidence.(map[string]any)["method"]; got != "pgvector-semantic-neighbor" {
		t.Fatalf("evidence method = %v", got)
	}
	if !strings.Contains(merged[0].Rationale, "proposal-q099") {
		t.Fatalf("rationale does not retain proposal identity: %q", merged[0].Rationale)
	}
}

func TestMergeAcceptedCapabilityProposalsIsIdempotent(t *testing.T) {
	entries := []capabilitybinding.Entry{{
		StableKey: "question.q099", RevisionID: "rev-q099",
		ContentHash: strings.Repeat("a", 64), Disposition: "bound",
		Rationale: "existing review", Bindings: []capabilitybinding.Binding{{
			PathKey: "path.nodejs-typescript", CapabilityKey: "capability.nodejs.event-loop-ordering",
			Role: "supporting_evidence", Provenance: "semantic-neighbor-v1",
		}},
	}}
	current := map[string]currentCapabilityRevision{
		"question.q099": {StableKey: "question.q099", RevisionID: "rev-q099", PathKey: "path.nodejs-typescript"},
	}
	proposal := acceptedCapabilityProposal{
		ID: "proposal-q099", StableKey: "question.q099", RevisionID: "rev-q099",
		PathKey: "path.nodejs-typescript", CapabilityKey: "capability.nodejs.event-loop-ordering",
		Role: "supporting_evidence", Provenance: "semantic-neighbor-v1",
		Evidence:  json.RawMessage(`{"method":"pgvector-semantic-neighbor"}`),
		Rationale: "Reviewed.",
	}
	merged, err := mergeAcceptedCapabilityProposals(entries, []acceptedCapabilityProposal{proposal}, current)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if len(merged[0].Bindings) != 1 {
		t.Fatalf("idempotent merge duplicated binding: %#v", merged[0].Bindings)
	}
}

func TestMergeAcceptedCapabilityProposalsRejectsPathMismatch(t *testing.T) {
	entries := []capabilitybinding.Entry{{
		StableKey: "question.q099", RevisionID: "rev-q099",
		ContentHash: strings.Repeat("a", 64), Disposition: "theory_only",
		Rationale: "no reviewed station",
	}}
	current := map[string]currentCapabilityRevision{
		"question.q099": {StableKey: "question.q099", RevisionID: "rev-q099", PathKey: "path.nodejs-typescript"},
	}
	proposal := acceptedCapabilityProposal{
		ID: "proposal-foreign", StableKey: "question.q099", RevisionID: "rev-q099",
		PathKey: "path.java-spring", CapabilityKey: "capability.nodejs.event-loop-ordering",
		Role: "supporting_evidence", Provenance: "manual-review", Rationale: "wrong path",
	}
	if _, err := mergeAcceptedCapabilityProposals(entries, []acceptedCapabilityProposal{proposal}, current); err == nil || !strings.Contains(err.Error(), "current path") {
		t.Fatalf("expected path mismatch error, got %v", err)
	}
}

func TestMergeAcceptedCapabilityProposalsIgnoresStaleRevision(t *testing.T) {
	entries := []capabilitybinding.Entry{{
		StableKey: "question.q099", RevisionID: "rev-current",
		ContentHash: strings.Repeat("a", 64), Disposition: "theory_only",
		Rationale: "no reviewed station",
	}}
	current := map[string]currentCapabilityRevision{
		"question.q099": {StableKey: "question.q099", RevisionID: "rev-current", PathKey: "path.nodejs-typescript"},
	}
	proposal := acceptedCapabilityProposal{
		ID: "proposal-stale", StableKey: "question.q099", RevisionID: "rev-old",
		PathKey: "path.nodejs-typescript", CapabilityKey: "capability.nodejs.event-loop-ordering",
		Role: "supporting_evidence", Provenance: "manual-review", Rationale: "old revision",
	}
	merged, err := mergeAcceptedCapabilityProposals(entries, []acceptedCapabilityProposal{proposal}, current)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if merged[0].Disposition != "theory_only" || len(merged[0].Bindings) != 0 {
		t.Fatalf("stale proposal changed current entry: %#v", merged[0])
	}
}
