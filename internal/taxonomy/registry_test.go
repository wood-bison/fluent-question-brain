package taxonomy

import "testing"

func TestRegistryHasExpectedLabShapeAndLegacySnapshot(t *testing.T) {
	if len(Programs()) != 1 || Programs()[0].Key != DefaultProgramKey {
		t.Fatalf("program registry = %#v", Programs())
	}
	if len(Paths()) != 8 {
		t.Fatalf("path registry length = %d, want 8", len(Paths()))
	}
	if len(Domains()) != 7 {
		t.Fatalf("domain registry length = %d, want 7", len(Domains()))
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
