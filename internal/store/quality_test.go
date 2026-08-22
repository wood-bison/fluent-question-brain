package store

import "testing"

func TestQualityDuplicatePairKeyIsOrderIndependent(t *testing.T) {
	left := qualityDuplicatePairKey("legacy.c010", "legacy.q195")
	right := qualityDuplicatePairKey("legacy.q195", "legacy.c010")
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
