package store

import "testing"

func TestGraphReleaseIDIsDeterministicForStablePairs(t *testing.T) {
	left := graphReleaseID("question-release-1", []string{"question.a=topic.a", "question.b=topic.b"})
	right := graphReleaseID("question-release-1", []string{"question.a=topic.a", "question.b=topic.b"})
	if left != right {
		t.Fatalf("graph release id changed for identical input: %q != %q", left, right)
	}
	if left != "question-graph-release-32e44ddc096c30a7" {
		t.Fatalf("unexpected graph release id %q", left)
	}
}

func TestGraphReleaseIDChangesWhenPinnedReleaseChanges(t *testing.T) {
	left := graphReleaseID("question-release-1", []string{"question.a=topic.a"})
	right := graphReleaseID("question-release-2", []string{"question.a=topic.a"})
	if left == right {
		t.Fatalf("graph release id ignored pinned question release: %q", left)
	}
}
