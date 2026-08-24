// Package taxonomy contains the small, controlled vocabulary that is shared
// by Question Brain's content projection and Fluent Engineering Lab's learner
// projection.  The legacy Track/Group/Topic fields deliberately do not live
// in this model: they describe a card's source placement and card kind, not a
// learner curriculum.
package taxonomy

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Version is part of the cross-system mapping contract.  A mapping must not
// be interpreted against a different registry without an explicit migration.
const Version = "question-brain.taxonomy.v1"

const DefaultProgramKey = "program.backend-engineer"

type Program struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

type Path struct {
	Key        string `json:"key"`
	Title      string `json:"title"`
	ProgramKey string `json:"program_key"`
}

type Domain struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Shared bool   `json:"shared"`
}

// Placement is an explicit cross-system binding.  Empty fields are allowed
// for legacy cards; a capability always requires a domain and path.
type Placement struct {
	ProgramKey     string `json:"program_key,omitempty"`
	PathKey        string `json:"path_key,omitempty"`
	DomainKey      string `json:"domain_key,omitempty"`
	CapabilityKey  string `json:"capability_key,omitempty"`
	MappingState   string `json:"mapping_state,omitempty"`
	MappingVersion string `json:"mapping_version,omitempty"`
}

var programs = []Program{
	{Key: DefaultProgramKey, Title: "Backend Engineer"},
}

var paths = []Path{
	{Key: "path.nodejs-typescript", Title: "Node.js + TypeScript", ProgramKey: DefaultProgramKey},
	{Key: "path.java-spring", Title: "Java + Spring", ProgramKey: DefaultProgramKey},
	{Key: "path.dotnet-csharp", Title: ".NET + C#", ProgramKey: DefaultProgramKey},
	{Key: "path.go", Title: "Go", ProgramKey: DefaultProgramKey},
	{Key: "path.frontend", Title: "Frontend", ProgramKey: DefaultProgramKey},
	{Key: "path.system-design", Title: "System Design", ProgramKey: DefaultProgramKey},
	{Key: "path.algorithms", Title: "Algorithms", ProgramKey: DefaultProgramKey},
	{Key: "path.behavioral", Title: "Behavioral", ProgramKey: DefaultProgramKey},
	{Key: "path.python", Title: "Python", ProgramKey: DefaultProgramKey},
}

var domains = []Domain{
	{Key: "domain.runtime", Title: "Runtime", Shared: true},
	{Key: "domain.http-api", Title: "HTTP/API", Shared: true},
	{Key: "domain.data-postgresql", Title: "Data/PostgreSQL", Shared: true},
	{Key: "domain.distributed-systems", Title: "Distributed Systems", Shared: true},
	{Key: "domain.os-networking", Title: "OS/Networking", Shared: true},
	{Key: "domain.testing", Title: "Testing", Shared: true},
	{Key: "domain.delivery-observability", Title: "Delivery/Observability", Shared: true},
}

var pathByKey = indexPaths()
var domainByKey = indexDomains()

// Topic aliases are intentionally small and explicit.  Do not add a generic
// case-folding or fuzzy rule here: one alias must point to one canonical
// legacy content topic and every future alias needs review.
var topicAliases = map[string]string{
	"distributed systems / resilience": "Distributed Systems & Resilience",
	"go / channels & select":           "Go / Channels & select",
	"go / sync patterns":               "Go / Sync & Patterns",
}

// The list is the 2026-08-24 legacy registry snapshot.  It exists only to
// validate new Topic metadata; it does not rewrite old payloads or create a
// second content graph.
var legacyTopicTitles = []string{
	"AI-assisted Development",
	"AI / RAG / Search",
	"Algorithms",
	"Algorithms / Binary Search",
	"Algorithms / Design",
	"Algorithms / Graph",
	"Algorithms / Greedy",
	"Algorithms / Hash Table",
	"Algorithms / Sliding Window",
	"Algorithms / Stack",
	"Algorithms / Two Pointers",
	"Algorithms / Union-Find",
	"Angular / Architecture",
	"Angular / Build & Tooling",
	"Angular / Change Detection",
	"Angular / Change Detection & Lifecycle",
	"Angular / Code Review",
	"Angular / Components",
	"Angular / Components & DI",
	"Angular / Components & Templates",
	"Angular / Content Projection",
	"Angular / Dependency Injection",
	"Angular / Directives",
	"Angular / DI & RxJS",
	"Angular / DI scopes",
	"Angular / Forms",
	"Angular / Forms & DI",
	"Angular / HttpClient",
	"Angular / Ivy",
	"Angular / Lifecycle",
	"Angular / Migration",
	"Angular / NgRx",
	"Angular / Pipes",
	"Angular / Pipes & CD",
	"Angular / Routing",
	"Angular / RxJS",
	"Angular / Services & DI",
	"Angular / Signals",
	"Angular / Signals & Pipes",
	"Angular / Templates",
	"Angular / Testing",
	"Angular / zone.js",
	"Answer Technique",
	"API Design",
	"Architecture & Design Patterns",
	"Auth & Backend Security",
	"Behavioral",
	"Behavioral / Competency",
	"Behavioral / Conflict & Teamwork",
	"Behavioral / Leadership Principles",
	"Behavioral / Project Deep Dive",
	"Behavioral / Recruiter Screen",
	"Behavioral / Self & Motivation",
	"Caching",
	"Databases & Data Modeling",
	"Delivery, Observability & Cloud",
	"Distributed Systems & Resilience",
	"Frontend Performance",
	"Frontend Security / Accessibility",
	"Frontend / Styling",
	"Frontend System Design",
	"Git / Tooling",
	"Go / Channels & select",
	"Go / Errors & defer & panic",
	"Go / Goroutines & Scheduler",
	"Go / Graceful Shutdown",
	"Go / Interfaces & Generics",
	"Go / Memory & GC",
	"Go / Runtime & Language",
	"Go / Package API Design",
	"Go / Sync & Patterns",
	"Go / Tooling & Testing",
	"Java / Concurrency & Async",
	"Java / Core Language",
	"Java / JVM & GC",
	"Java / Spring & Spring Boot",
	"JavaScript / DOM Events",
	"JS Async Internals",
	"Messaging & Event Streaming",
	".NET / ASP.NET Core",
	".NET / Async & Concurrency",
	".NET / C# Language & Type System",
	".NET / CLR & GC",
	".NET / DI & Lifetimes",
	".NET / EF Core",
	".NET / LINQ & Collections",
	".NET / Performance & Diagnostics",
	"Networking / Browser",
	"Node / Async & Promises",
	"Node / Bun",
	"Node / Event Loop & Scheduling",
	"Node / JS Fundamentals",
	"Node / Modules & Packaging",
	"Node / NestJS / Auth & Security",
	"Node / NestJS / Data & TypeORM",
	"Node / NestJS / DI & Architecture",
	"Node / NestJS / Performance & Realtime",
	"Node / NestJS / Request Pipeline",
	"Node / NestJS / Testing & Production",
	"Node / NestJS / Validation & Errors",
	"Node / Runtime Ops & Debugging",
	"Node / Streams & Backpressure",
	"Node / TS Applied",
	"Node / TS Tooling",
	"Node / TS Type System",
	"Node / V8 & GC",
	"Node / Workers",
	"Node Runtime & Streams",
	"Oracle / SQL Craft & Analytics",
	"ORM & Persistence Performance",
	"OS, Networking & Concurrency Fundamentals",
	"Python / Concurrency & GIL",
	"Python / Iterators & Generators",
	"Python / Metaprogramming",
	"Python / OOP & Data Model",
	"React Rendering / Hooks",
	"RxJS / Caching",
	"RxJS / Combination",
	"RxJS / Completion",
	"RxJS / Custom Operators",
	"RxJS / Error Handling",
	"RxJS / Higher-order Mapping",
	"RxJS / Hot vs Cold",
	"RxJS / Multicasting",
	"RxJS / Observable Creation",
	"RxJS / Subjects",
	"SQL / Joins & Aggregation",
	"State Management",
	"System Design",
	"System Design / Product Cases",
	"Testing",
	"Web Architecture / Browser Storage",
	"systems",
}

var topicByLabel = indexTopicTitles()

var capabilityPattern = regexp.MustCompile(`^capability\.([a-z0-9]+(?:-[a-z0-9]+)*)\.([a-z0-9]+(?:-[a-z0-9]+|\.[a-z0-9]+)*)$`)

// Capability aliases are explicit migrations, not fuzzy normalisation. Old
// task-shaped keys remain readable for historical manifests while new writes
// resolve to the stable observable-skill key.
var capabilityAliases = map[string]string{
	"capability.runtime.node-event-loop-001":              "capability.nodejs.event-loop-ordering",
	"capability.runtime.node-cpu-bound-002":               "capability.nodejs.cpu-bound-work",
	"capability.runtime.node-streams-003":                 "capability.nodejs.streams-backpressure",
	"capability.runtime.node-memory-004":                  "capability.nodejs.memory-retention",
	"capability.runtime.node-concurrency-012":             "capability.nodejs.bounded-concurrency",
	"capability.runtime.dotnet-cancellation-001":          "capability.dotnet.cancellation-boundary",
	"capability.http-api.node-auth-015":                   "capability.http-api.authentication-authorization",
	"capability.data-postgresql.pg-indexes-008":           "capability.postgresql.query-planning",
	"capability.data-postgresql.pg-locks-016":             "capability.postgresql.row-locks",
	"capability.distributed-systems.node-idempotency-013": "capability.distributed-systems.idempotent-delivery",
	"capability.delivery-observability.node-cache-014":    "capability.delivery-observability.cache-invalidation",
}

// Technology namespaces are valid only for reviewed canonical keys. Their
// domain is explicit data, not inferred from the namespace by the caller.
var canonicalCapabilityDomains = map[string]string{
	"capability.nodejs.event-loop-ordering":                "domain.runtime",
	"capability.nodejs.cpu-bound-work":                     "domain.runtime",
	"capability.nodejs.streams-backpressure":               "domain.runtime",
	"capability.nodejs.memory-retention":                   "domain.runtime",
	"capability.nodejs.bounded-concurrency":                "domain.runtime",
	"capability.dotnet.cancellation-boundary":              "domain.runtime",
	"capability.http-api.authentication-authorization":     "domain.http-api",
	"capability.postgresql.query-planning":                 "domain.data-postgresql",
	"capability.postgresql.row-locks":                      "domain.data-postgresql",
	"capability.distributed-systems.idempotent-delivery":   "domain.distributed-systems",
	"capability.delivery-observability.cache-invalidation": "domain.delivery-observability",
}

func Programs() []Program { return append([]Program(nil), programs...) }

func Paths() []Path { return append([]Path(nil), paths...) }

func Domains() []Domain { return append([]Domain(nil), domains...) }

func LegacyTopics() []string {
	result := append([]string(nil), legacyTopicTitles...)
	sort.Strings(result)
	return result
}

// IsDeprecatedCapabilityKey reports whether input is a historical capability
// alias. Historical ingest and evidence lookups may resolve these aliases,
// but a new release manifest must spell the canonical key explicitly.
func IsDeprecatedCapabilityKey(input string) bool {
	_, ok := capabilityAliases[normalizeKey(input)]
	return ok
}

func CanonicalTopicTitle(input string) (string, bool) {
	label := normalizeLabel(input)
	if canonical, ok := topicByLabel[label]; ok {
		return canonical, true
	}
	canonical, ok := topicAliases[label]
	return canonical, ok
}

func CanonicalTopicKey(input string) (string, bool) {
	title, ok := CanonicalTopicTitle(input)
	if !ok {
		return "", false
	}
	return "topic." + slugify(title), true
}

func ResolvePlacement(program, path, domain, capability, state string) (Placement, error) {
	program = strings.TrimSpace(program)
	path = strings.TrimSpace(path)
	domain = strings.TrimSpace(domain)
	capability = strings.TrimSpace(capability)
	state = strings.TrimSpace(strings.ToLower(state))
	if program == "" && (path != "" || domain != "" || capability != "") {
		program = DefaultProgramKey
	}
	result := Placement{MappingVersion: Version}
	if program != "" {
		resolved, ok := resolveProgram(program)
		if !ok {
			return Placement{}, fmt.Errorf("unknown program %q; use an explicit taxonomy alias", program)
		}
		result.ProgramKey = resolved.Key
	}
	if path != "" {
		resolved, ok := resolvePath(path)
		if !ok {
			return Placement{}, fmt.Errorf("unknown path %q; use an explicit taxonomy alias", path)
		}
		if result.ProgramKey != "" && resolved.ProgramKey != result.ProgramKey {
			return Placement{}, fmt.Errorf("path %q does not belong to program %q", path, result.ProgramKey)
		}
		result.PathKey = resolved.Key
	}
	if domain != "" {
		resolved, ok := resolveDomain(domain)
		if !ok {
			return Placement{}, fmt.Errorf("unknown domain %q; use an explicit taxonomy alias", domain)
		}
		result.DomainKey = resolved.Key
	}
	if capability != "" {
		if result.PathKey == "" || result.DomainKey == "" {
			return Placement{}, fmt.Errorf("capability_key requires both path_key and domain_key")
		}
		resolved, err := canonicalCapability(capability, result.DomainKey)
		if err != nil {
			return Placement{}, err
		}
		result.CapabilityKey = resolved
	}
	if result.ProgramKey == "" && result.PathKey == "" && result.DomainKey == "" && result.CapabilityKey == "" {
		return Placement{}, nil
	}
	if state == "" {
		state = "proposed"
	}
	switch state {
	case "proposed", "accepted", "rejected":
		result.MappingState = state
	default:
		return Placement{}, fmt.Errorf("invalid mapping_state %q; use proposed, accepted, or rejected", state)
	}
	return result, nil
}

func resolveProgram(input string) (Program, bool) {
	key := normalizeKey(input)
	for _, item := range programs {
		if key == item.Key || key == normalizeKey(item.Title) {
			return item, true
		}
	}
	return Program{}, false
}

var pathAliases = map[string]string{
	"node":                 "path.nodejs-typescript",
	"node.js":              "path.nodejs-typescript",
	"node.js + typescript": "path.nodejs-typescript",
	"node.js+typescript":   "path.nodejs-typescript",
	"nodejs + typescript":  "path.nodejs-typescript",
	"nodejs+typescript":    "path.nodejs-typescript",
	"java":                 "path.java-spring",
	"java + spring":        "path.java-spring",
	"java+spring":          "path.java-spring",
	".net":                 "path.dotnet-csharp",
	".net + c#":            "path.dotnet-csharp",
	".net+c#":              "path.dotnet-csharp",
	"c#":                   "path.dotnet-csharp",
	"go":                   "path.go",
	"frontend":             "path.frontend",
	"system design":        "path.system-design",
	"algorithms":           "path.algorithms",
	"behavioral":           "path.behavioral",
	"python":               "path.python",
}

var domainAliases = map[string]string{
	"runtime":                       "domain.runtime",
	"stage.runtime":                 "domain.runtime",
	"domain.runtime":                "domain.runtime",
	"http/api":                      "domain.http-api",
	"http / api":                    "domain.http-api",
	"http-api":                      "domain.http-api",
	"stage.http-api":                "domain.http-api",
	"stage.http/api":                "domain.http-api",
	"domain.http-api":               "domain.http-api",
	"data/postgresql":               "domain.data-postgresql",
	"data / postgresql":             "domain.data-postgresql",
	"data-postgresql":               "domain.data-postgresql",
	"stage.data-postgresql":         "domain.data-postgresql",
	"domain.data-postgresql":        "domain.data-postgresql",
	"distributed systems":           "domain.distributed-systems",
	"distributed-systems":           "domain.distributed-systems",
	"stage.distributed-systems":     "domain.distributed-systems",
	"domain.distributed-systems":    "domain.distributed-systems",
	"os/networking":                 "domain.os-networking",
	"os / networking":               "domain.os-networking",
	"os-networking":                 "domain.os-networking",
	"stage.os-networking":           "domain.os-networking",
	"domain.os-networking":          "domain.os-networking",
	"testing":                       "domain.testing",
	"stage.testing":                 "domain.testing",
	"domain.testing":                "domain.testing",
	"delivery/observability":        "domain.delivery-observability",
	"delivery / observability":      "domain.delivery-observability",
	"delivery-observability":        "domain.delivery-observability",
	"stage.delivery-observability":  "domain.delivery-observability",
	"domain.delivery-observability": "domain.delivery-observability",
}

func resolvePath(input string) (Path, bool) {
	key := normalizeKey(input)
	if alias, ok := pathAliases[key]; ok {
		key = alias
	}
	item, ok := pathByKey[key]
	return item, ok
}

func resolveDomain(input string) (Domain, bool) {
	key := normalizeKey(input)
	if alias, ok := domainAliases[key]; ok {
		key = alias
	}
	item, ok := domainByKey[key]
	return item, ok
}

func canonicalCapability(input, domainKey string) (string, error) {
	key := normalizeKey(input)
	match := capabilityPattern.FindStringSubmatch(key)
	if len(match) != 3 {
		return "", fmt.Errorf("invalid capability_key %q; use capability.<domain>.<slug>", input)
	}
	if alias, ok := capabilityAliases[key]; ok {
		key = alias
	}
	canonicalDomain := "domain." + match[1]
	if mappedDomain, ok := canonicalCapabilityDomains[key]; ok {
		canonicalDomain = mappedDomain
	}
	if canonicalDomain != domainKey {
		return "", fmt.Errorf("capability_key %q is outside domain %q", input, domainKey)
	}
	return key, nil
}

func normalizeLabel(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && result.Len() > 0 {
			result.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(result.String(), "-")
}

func indexPaths() map[string]Path {
	result := make(map[string]Path, len(paths))
	for _, item := range paths {
		result[item.Key] = item
	}
	return result
}

func indexDomains() map[string]Domain {
	result := make(map[string]Domain, len(domains))
	for _, item := range domains {
		result[item.Key] = item
	}
	return result
}

func indexTopicTitles() map[string]string {
	result := make(map[string]string, len(legacyTopicTitles))
	for _, item := range legacyTopicTitles {
		result[normalizeLabel(item)] = item
	}
	return result
}
