package store

import "testing"

func TestQualityDuplicatePairKeyIsOrderIndependent(t *testing.T) {
	left := qualityDuplicatePairKey("question.c010", "question.q195")
	right := qualityDuplicatePairKey("question.q195", "question.c010")
	if left != right {
		t.Fatalf("pair key depends on candidate order: %q != %q", left, right)
	}
}

func TestTerminalDuplicateDecision(t *testing.T) {
	for _, decision := range []string{"not_duplicate", "keep_separate", "merge"} {
		if !terminalDuplicateDecision(decision) {
			t.Fatalf("%q should be terminal", decision)
		}
	}
	if terminalDuplicateDecision("open") {
		t.Fatal("open duplicate decision should remain review debt")
	}
}
