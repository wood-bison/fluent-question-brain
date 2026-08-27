package store

import "testing"

func TestNormalizeGraphRequestDefaultsAndPreservesContract(t *testing.T) {
	confidence := 0.75
	request, err := normalizeGraphRequest(EdgeProposalRequest{
		FromStableKey: " question.q315 ",
		ToStableKey:   " question.c009 ",
		Kind:          " prerequisite ",
		Confidence:    &confidence,
	})
	if err != nil {
		t.Fatalf("normalize graph request: %v", err)
	}
	if request.WorkspaceKey != "fluent-interview" || request.Source != "question-brain-editorial" {
		t.Fatalf("defaults = %#v", request)
	}
	if request.FromStableKey != "question.q315" || request.ToStableKey != "question.c009" || request.Kind != "prerequisite" {
		t.Fatalf("trimmed identity = %#v", request)
	}
}

func TestNormalizeGraphRequestRejectsInvalidEdges(t *testing.T) {
	confidence := 1.1
	cases := []EdgeProposalRequest{
		{FromStableKey: "question.q315", ToStableKey: "question.q315", Kind: "related"},
		{FromStableKey: "question.q315", ToStableKey: "question.c009", Kind: "unknown"},
		{FromStableKey: "question.q315", ToStableKey: "question.c009", Kind: "related", Confidence: &confidence},
	}
	for index, request := range cases {
		if _, err := normalizeGraphRequest(request); err == nil {
			t.Errorf("case %d unexpectedly accepted", index)
		}
	}
}

func TestQuestionGraphEdgeKindRegistryDefinesLearnerSemantics(t *testing.T) {
	registry := QuestionGraphEdgeKindRegistry()
	if len(registry) != 7 {
		t.Fatalf("registry length = %d, want 7", len(registry))
	}
	wantKinds := []string{"contrast", "duplicate", "follow_up", "prerequisite", "related", "supersedes", "variant"}
	for index, policy := range registry {
		if policy.Kind != wantKinds[index] {
			t.Fatalf("registry[%d].Kind = %q, want %q", index, policy.Kind, wantKinds[index])
		}
		if policy.Description == "" || policy.LearnerEffect == "" {
			t.Fatalf("registry[%d] has incomplete semantics: %#v", index, policy)
		}
	}
	if !registry[3].RequiresAcyclic || registry[0].RequiresAcyclic {
		t.Fatalf("only prerequisite should require acyclic semantics: %#v", registry)
	}
}

func TestNormalizeGraphRequestRejectsUnprovenConfidence(t *testing.T) {
	confidence := 1.0
	for _, request := range []EdgeProposalRequest{
		{FromStableKey: "question.a", ToStableKey: "question.b", Kind: "related", Confidence: &confidence},
		{FromStableKey: "question.a", ToStableKey: "question.b", Kind: "related", Confidence: &confidence, Source: "editorial-source-v1"},
	} {
		if _, err := normalizeGraphRequest(request); err == nil {
			t.Fatalf("confidence 1 proposal unexpectedly accepted: %#v", request)
		}
	}
	accepted, err := normalizeGraphRequest(EdgeProposalRequest{
		FromStableKey: "question.a", ToStableKey: "question.b", Kind: "related",
		Confidence: &confidence, Rationale: "reviewed by editor", Source: "editorial-source-v1",
	})
	if err != nil {
		t.Fatalf("confidence 1 proposal with evidence rejected: %v", err)
	}
	if accepted.Rationale != "reviewed by editor" || accepted.Source != "editorial-source-v1" {
		t.Fatalf("evidence was not preserved: %#v", accepted)
	}
}
