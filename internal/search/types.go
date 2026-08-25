package search

import (
	"context"
	"encoding/json"
	"time"
)

type Request struct {
	WorkspaceKey string
	Query        string
	Locale       string
	TopicKey     string
	Level        string
	Company      string
	Limit        int
}

type Result struct {
	QuestionID    string          `json:"question_id"`
	RevisionID    string          `json:"revision_id"`
	StableKey     string          `json:"stable_key"`
	Slug          string          `json:"slug"`
	Company       string          `json:"company,omitempty"`
	Locale        string          `json:"locale"`
	Prompt        string          `json:"prompt"`
	ShortAnswer   string          `json:"short_answer,omitempty"`
	Explanation   string          `json:"explanation,omitempty"`
	TopicKey      string          `json:"topic_key,omitempty"`
	TopicTitle    string          `json:"topic_title,omitempty"`
	MatchStages   []string        `json:"match_stages"`
	ExactScore    float64         `json:"exact_score"`
	FTSScore      float64         `json:"fts_score"`
	TrigramScore  float64         `json:"trigram_score"`
	SemanticScore float64         `json:"semantic_score"`
	RankScore     float64         `json:"rank_score"`
	Task          json.RawMessage `json:"task,omitempty"`
	Rubric        json.RawMessage `json:"rubric,omitempty"`
	Choices       json.RawMessage `json:"choices,omitempty"`
}

type Question struct {
	QuestionID  string          `json:"question_id"`
	RevisionID  string          `json:"revision_id"`
	StableKey   string          `json:"stable_key"`
	Slug        string          `json:"slug"`
	Status      string          `json:"status"`
	ContentHash string          `json:"content_hash"`
	Locale      string          `json:"locale"`
	Prompt      string          `json:"prompt"`
	ShortAnswer string          `json:"short_answer,omitempty"`
	Explanation string          `json:"explanation,omitempty"`
	Body        map[string]any  `json:"body"`
	Task        json.RawMessage `json:"task,omitempty"`
	Rubric      json.RawMessage `json:"rubric,omitempty"`
	Choices     json.RawMessage `json:"choices,omitempty"`
	Topics      []Topic         `json:"topics"`
}

// CatalogRequest is the bounded read contract used by learner projections.
// It exposes the current published revision index, never authoring payloads.
type CatalogRequest struct {
	WorkspaceKey    string
	Locale          string
	TopicKey        string
	Track           string
	Level           string
	Company         string
	Offset          int
	Limit           int
	IncludeFixtures bool
}

type CatalogItem struct {
	QuestionID       string          `json:"question_id"`
	RevisionID       string          `json:"revision_id"`
	StableKey        string          `json:"stable_key"`
	Slug             string          `json:"slug"`
	Status           string          `json:"status"`
	ContentHash      string          `json:"content_hash"`
	Locale           string          `json:"locale"`
	AvailableLocales []string        `json:"available_locales"`
	Prompt           string          `json:"prompt"`
	ShortAnswer      string          `json:"short_answer,omitempty"`
	Explanation      string          `json:"explanation,omitempty"`
	Metadata         CatalogMetadata `json:"metadata"`
	Topics           []Topic         `json:"topics"`
}

// CatalogMetadata is the normalized, non-answer metadata imported from the
// source vault. Keeping it typed lets Lab build a provisional learner index
// without coupling to PostgreSQL or exposing the raw normalized payload.
type CatalogMetadata struct {
	Group         string `json:"group,omitempty"`
	Level         string `json:"level,omitempty"`
	Scope         string `json:"scope,omitempty"`
	Title         string `json:"title,omitempty"`
	Topic         string `json:"topic,omitempty"`
	Track         string `json:"track,omitempty"`
	Company       string `json:"company,omitempty"`
	Priority      string `json:"priority,omitempty"`
	Lang          string `json:"lang,omitempty"`
	Runtime       string `json:"runtime,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"`
	Depth         int    `json:"depth,omitempty"`
	OrderKey      string `json:"order_key,omitempty"`
	// StageKey is a compatibility projection for older Lab clients. New
	// clients must read DomainKey and PathKey; stage is not a second taxonomy.
	StageKey       string `json:"stage_key,omitempty"`
	ProgramKey     string `json:"program_key,omitempty"`
	PathKey        string `json:"path_key,omitempty"`
	DomainKey      string `json:"domain_key,omitempty"`
	CapabilityKey  string `json:"capability_key,omitempty"`
	MappingState   string `json:"mapping_state,omitempty"`
	MappingVersion string `json:"mapping_version,omitempty"`
	MappingSource  string `json:"mapping_source,omitempty"`
}

type CatalogResponse struct {
	ContractVersion  string        `json:"contract_version"`
	WorkspaceKey     string        `json:"workspace_key"`
	Locale           string        `json:"locale"`
	ReleaseID        string        `json:"release_id"`
	GeneratedAt      time.Time     `json:"generated_at"`
	Total            int           `json:"total"`
	Offset           int           `json:"offset"`
	Limit            int           `json:"limit"`
	IncludeFixtures  bool          `json:"include_fixtures"`
	ExcludedFixtures int           `json:"excluded_fixtures"`
	ExcludedNonProd  int           `json:"excluded_non_production"`
	Questions        []CatalogItem `json:"questions"`
	Provenance       struct {
		Explainable bool     `json:"explainable"`
		Source      string   `json:"source"`
		Pipeline    []string `json:"pipeline"`
	} `json:"provenance"`
}

// ReleaseRequest asks for the complete, learner-safe identity manifest. It is
// intentionally separate from CatalogRequest: a catalog may be paged, while a
// release manifest is the auditable pinned set used by operators and clients.
type ReleaseRequest struct {
	WorkspaceKey    string
	IncludeFixtures bool
}

type ReleaseItem struct {
	QuestionID       string   `json:"question_id"`
	RevisionID       string   `json:"revision_id"`
	StableKey        string   `json:"stable_key"`
	ContentHash      string   `json:"content_hash"`
	Status           string   `json:"status"`
	ContentKind      string   `json:"content_kind"`
	AvailableLocales []string `json:"available_locales"`
	SourceSystem     string   `json:"source_system"`
	QualityState     string   `json:"quality_state"`
	GraphState       string   `json:"graph_state"`
}

type ReleaseChecks struct {
	Published            int `json:"published"`
	Fixtures             int `json:"fixtures"`
	GraphReleased        int `json:"graph_released"`
	GraphAcceptedPending int `json:"graph_accepted_pending"`
	GraphProposed        int `json:"graph_proposed"`
	GraphUnplaced        int `json:"graph_unplaced"`
	MissingEnglish       int `json:"missing_english"`
	MissingRussian       int `json:"missing_russian"`
	// Index-freshness counters are computed by the quality audit surface
	// (/v1/quality) only; /v1/release omits them. Pointers keep the release
	// contract byte-identical while letting the audit report index lag.
	OutboxPending           *int `json:"outbox_pending,omitempty"`
	LocalesWithoutEmbedding *int `json:"locales_without_embedding,omitempty"`
}

// QualityChecks is the /v1/quality view of ReleaseChecks plus audit-only
// content-quality counters. The embedded release fields stay byte-identical;
// quality-only counters are plain ints so they always serialize, including 0.
type QualityChecks struct {
	ReleaseChecks
	// RuPromptEqualsAnswer is retained for clients that consumed the original
	// audit field. DegeneratePrompts is the I0 gate and covers both locales plus
	// shape/PDF extraction failures.
	RuPromptEqualsAnswer int `json:"ru_prompt_equals_answer"`
	// DegeneratePrompts counts production cards whose RU or EN prompt is not
	// a usable question: empty, equal to the answer, equal to the card title
	// or topic, a single unpunctuated word, a truncated sentence fragment, or
	// shorter than ~20 characters without a question mark. Equality with the answer
	// alone was not enough: a PDF section heading leaked into the prompt
	// ("SQL", "Указатели", ":") is also not equal to the answer, which let
	// 70 unusable cards pass the old check.
	DegeneratePrompts int `json:"degenerate_prompts"`
	// SemanticShapeIssues is the subset of degenerate cards whose learner
	// prompt/title shape is malformed. PDF control/layout debris is counted by
	// DegeneratePrompts but intentionally not repeated here.
	SemanticShapeIssues int `json:"semantic_shape_issues"`
	// Curriculum mapping counters are revision-scoped and never inferred from
	// the legacy content graph. Unmapped includes current revisions with no
	// explicit mapping row or an explicit mapping_state=unmapped row.
	CurriculumMapped       int `json:"curriculum_mapped"`
	CurriculumUnmapped     int `json:"curriculum_unmapped"`
	CurriculumProposed     int `json:"curriculum_proposed"`
	CurriculumAccepted     int `json:"curriculum_accepted"`
	CurriculumRejected     int `json:"curriculum_rejected"`
	CurriculumCapabilities int `json:"curriculum_capabilities"`
	// Task boundary counters make historical duplication visible without
	// exposing task bodies. New versioned TaskBrief cards must reference a
	// TaskFamily; executable solutions remain Runtime-owned.
	TaskBlocks             int `json:"task_blocks"`
	TaskFamilyReferences   int `json:"task_family_references"`
	EmbeddedSolutions      int `json:"embedded_solutions"`
	TaskBoundaryViolations int `json:"task_boundary_violations"`
}

type ReleaseResponse struct {
	ContractVersion             string        `json:"contract_version"`
	WorkspaceKey                string        `json:"workspace_key"`
	ReleaseID                   string        `json:"release_id"`
	SourceSnapshotID            string        `json:"source_snapshot_id"`
	CapabilityRegistryReleaseID string        `json:"capability_registry_release_id,omitempty"`
	CapabilityBindingReleaseID  string        `json:"capability_binding_release_id,omitempty"`
	CapabilityKeys              []string      `json:"capability_keys,omitempty"`
	GeneratedAt                 time.Time     `json:"generated_at"`
	Total                       int           `json:"total"`
	IncludeFixtures             bool          `json:"include_fixtures"`
	ExcludedFixtures            int           `json:"excluded_fixtures"`
	ExcludedNonProd             int           `json:"excluded_non_production"`
	Checks                      ReleaseChecks `json:"checks"`
	Items                       []ReleaseItem `json:"items"`
	Provenance                  struct {
		Explainable bool     `json:"explainable"`
		Source      string   `json:"source"`
		Pipeline    []string `json:"pipeline"`
	} `json:"provenance"`
}

// QualityRequest asks for an aggregate, answer-free audit of one published
// release. It is intentionally separate from CatalogResponse: clients can
// inspect coverage and review debt without downloading question bodies.
type QualityRequest struct {
	WorkspaceKey    string
	IncludeFixtures bool
}

type QualityBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type QualityDuplicateGroup struct {
	Fingerprint string   `json:"fingerprint"`
	StableKeys  []string `json:"stable_keys"`
}

// QualityResolvedDuplicateGroup keeps an explicit terminal review decision
// visible without treating it as unresolved quality debt. The source prompt
// fingerprint remains opaque; stable keys are present so an operator can
// follow the auditable decision back to the canonical records.
type QualityResolvedDuplicateGroup struct {
	Fingerprint string   `json:"fingerprint"`
	StableKeys  []string `json:"stable_keys"`
	Decisions   []string `json:"decisions"`
}

type QualityResponse struct {
	ContractVersion         string                          `json:"contract_version"`
	WorkspaceKey            string                          `json:"workspace_key"`
	ReleaseID               string                          `json:"release_id"`
	GeneratedAt             time.Time                       `json:"generated_at"`
	Total                   int                             `json:"total"`
	IncludeFixtures         bool                            `json:"include_fixtures"`
	Checks                  QualityChecks                   `json:"checks"`
	Locales                 []QualityBucket                 `json:"locales"`
	Tracks                  []QualityBucket                 `json:"tracks"`
	Topics                  []QualityBucket                 `json:"topics"`
	Levels                  []QualityBucket                 `json:"levels"`
	Companies               []QualityBucket                 `json:"companies"`
	DuplicateGroups         []QualityDuplicateGroup         `json:"duplicate_groups"`
	ResolvedDuplicateGroups []QualityResolvedDuplicateGroup `json:"resolved_duplicate_groups"`
	DuplicateStates         []QualityBucket                 `json:"duplicate_states"`
	Warnings                []string                        `json:"warnings"`
	Provenance              struct {
		Explainable bool     `json:"explainable"`
		Source      string   `json:"source"`
		Pipeline    []string `json:"pipeline"`
	} `json:"provenance"`
}

type Topic struct {
	StableKey string `json:"stable_key"`
	Title     string `json:"title"`
	Relation  string `json:"relation"`
}

type Service interface {
	Search(context.Context, Request) ([]Result, error)
	GetQuestion(context.Context, string, string, string) (Question, error)
	Catalog(context.Context, CatalogRequest) (CatalogResponse, error)
}
