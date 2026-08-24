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
