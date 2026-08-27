package taxonomy

import "testing"

func TestRegistryHasExpectedLabShapeAndLegacySnapshot(t *testing.T) {
	if len(Programs()) != 1 || Programs()[0].Key != DefaultProgramKey {
		t.Fatalf("program registry = %#v", Programs())
	}
	if len(Paths()) != 9 {
		t.Fatalf("path registry length = %d, want 9", len(Paths()))
	}
	if len(Domains()) != 9 {
		t.Fatalf("domain registry length = %d, want 9", len(Domains()))
	}
	topics := LegacyTopics()
	if len(topics) != 133 {
		t.Fatalf("legacy topic snapshot length = %d, want 133", len(topics))
	}
	for index := 1; index < len(topics); index++ {
		if topics[index] == topics[index-1] {
			t.Fatalf("duplicate legacy topic %q", topics[index])
		}
	}
}

func TestLegacyTopicAliasesResolveToOneCanonicalTopic(t *testing.T) {
	tests := map[string]string{
		"Distributed Systems / Resilience": "Distributed Systems & Resilience",
		"Go / Channels & Select":           "Go / Channels & select",
		"Go / Sync Patterns":               "Go / Sync & Patterns",
	}
	for input, want := range tests {
		got, ok := CanonicalTopicTitle(input)
		if !ok || got != want {
			t.Fatalf("CanonicalTopicTitle(%q) = %q, %v; want %q, true", input, got, ok, want)
		}
	}
}

func TestUnknownTopicIsNotSilentlyAccepted(t *testing.T) {
	if _, ok := CanonicalTopicTitle("Node / Made Up"); ok {
		t.Fatal("unknown legacy topic unexpectedly resolved")
	}
}

func TestResolvePlacementUsesExplicitPathDomainCapability(t *testing.T) {
	placement, err := ResolvePlacement(
		"Backend Engineer",
		"Node.js+TypeScript",
		"stage.runtime",
		"capability.runtime.event-loop",
		"accepted",
	)
	if err != nil {
		t.Fatalf("ResolvePlacement() error = %v", err)
	}
	if placement.ProgramKey != DefaultProgramKey || placement.PathKey != "path.nodejs-typescript" || placement.DomainKey != "domain.runtime" || placement.CapabilityKey != "capability.runtime.event-loop" {
		t.Fatalf("placement = %#v", placement)
	}
	if placement.MappingState != "accepted" || placement.MappingVersion != Version {
		t.Fatalf("mapping metadata = %#v", placement)
	}
}

func TestResolvePlacementDoesNotTreatGroupOrTopicAsDomain(t *testing.T) {
	if _, err := ResolvePlacement("Backend Engineer", "Backend", "Common Questions", "", ""); err == nil {
		t.Fatal("Group/Track values were accepted as curriculum placement")
	}
	if _, err := ResolvePlacement("Backend Engineer", "Frontend", "Messaging & Event Streaming", "", ""); err == nil {
		t.Fatal("legacy Topic value was accepted as a shared domain")
	}
}

func TestCapabilityRequiresPathAndMatchingDomain(t *testing.T) {
	if _, err := ResolvePlacement("", "Go", "Runtime", "capability.runtime.goroutines", ""); err != nil {
		t.Fatalf("valid capability mapping rejected: %v", err)
	}
	if _, err := ResolvePlacement("", "Go", "Runtime", "capability.http-api.rest", ""); err == nil {
		t.Fatal("capability outside declared domain accepted")
	}
	if _, err := ResolvePlacement("", "Go", "", "capability.runtime.goroutines", ""); err == nil {
		t.Fatal("capability without domain accepted")
	}
}

func TestCanonicalCapabilityAliasesRemoveTaskSequence(t *testing.T) {
	placement, err := ResolvePlacement("", "Node.js", "Runtime", "capability.runtime.node-event-loop-001", "accepted")
	if err != nil {
		t.Fatalf("legacy capability alias rejected: %v", err)
	}
	if placement.CapabilityKey != "capability.nodejs.event-loop-ordering" {
		t.Fatalf("legacy key was not migrated: %#v", placement)
	}
	if !IsDeprecatedCapabilityKey("capability.runtime.node-event-loop-001") {
		t.Fatal("historical capability alias was not marked deprecated")
	}
	if IsDeprecatedCapabilityKey("capability.nodejs.event-loop-ordering") {
		t.Fatal("canonical capability key was incorrectly marked deprecated")
	}
}

func TestCanonicalTechnologyCapabilityRequiresReviewedDomain(t *testing.T) {
	if _, err := ResolvePlacement("", "Node.js", "Runtime", "capability.nodejs.event-loop-ordering", "accepted"); err != nil {
		t.Fatalf("reviewed technology capability rejected: %v", err)
	}
	if _, err := ResolvePlacement("", "Node.js", "HTTP/API", "capability.nodejs.event-loop-ordering", "accepted"); err == nil {
		t.Fatal("technology capability was accepted in an unrelated domain")
	}
}

func TestAlgorithmsAndBehavioralUseDedicatedDomains(t *testing.T) {
	algorithms, err := ResolvePlacement("", "Algorithms", "Algorithms", "", "accepted")
	if err != nil {
		t.Fatalf("algorithms placement rejected: %v", err)
	}
	if algorithms.PathKey != "path.algorithms" || algorithms.DomainKey != "domain.algorithms" {
		t.Fatalf("algorithms placement = %#v", algorithms)
	}
	behavioral, err := ResolvePlacement("", "Behavioral", "Behavioral/English", "", "accepted")
	if err != nil {
		t.Fatalf("behavioral placement rejected: %v", err)
	}
	if behavioral.PathKey != "path.behavioral" || behavioral.DomainKey != "domain.behavioral" {
		t.Fatalf("behavioral placement = %#v", behavioral)
	}
}

func TestPathDomainShapeRejectsDedicatedLaneLeakage(t *testing.T) {
	cases := []struct {
		name, path, domain string
	}{
		{name: "algorithms into runtime", path: "Algorithms", domain: "Runtime"},
		{name: "behavioral into testing", path: "Behavioral", domain: "Testing"},
		{name: "shared algorithms domain on node", path: "Node.js", domain: "Algorithms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolvePlacement("", tc.path, tc.domain, "", "accepted"); err == nil {
				t.Fatalf("ResolvePlacement(%q, %q) unexpectedly accepted dedicated lane leakage", tc.path, tc.domain)
			}
		})
	}
}
