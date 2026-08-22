package search

import (
	"context"
	"time"
)

type Request struct {
	WorkspaceKey string
	Query        string
	Locale       string
	TopicKey     string
	Limit        int
}

type Result struct {
	QuestionID    string   `json:"question_id"`
	RevisionID    string   `json:"revision_id"`
	StableKey     string   `json:"stable_key"`
	Slug          string   `json:"slug"`
	Locale        string   `json:"locale"`
	Prompt        string   `json:"prompt"`
	ShortAnswer   string   `json:"short_answer,omitempty"`
	Explanation   string   `json:"explanation,omitempty"`
	TopicKey      string   `json:"topic_key,omitempty"`
	TopicTitle    string   `json:"topic_title,omitempty"`
	MatchStages   []string `json:"match_stages"`
	ExactScore    float64  `json:"exact_score"`
	FTSScore      float64  `json:"fts_score"`
	TrigramScore  float64  `json:"trigram_score"`
	SemanticScore float64  `json:"semantic_score"`
	RankScore     float64  `json:"rank_score"`
}

type Question struct {
	QuestionID  string         `json:"question_id"`
	RevisionID  string         `json:"revision_id"`
	StableKey   string         `json:"stable_key"`
	Slug        string         `json:"slug"`
	Status      string         `json:"status"`
	ContentHash string         `json:"content_hash"`
	Locale      string         `json:"locale"`
	Prompt      string         `json:"prompt"`
	ShortAnswer string         `json:"short_answer,omitempty"`
	Explanation string         `json:"explanation,omitempty"`
	Body        map[string]any `json:"body"`
	Topics      []Topic        `json:"topics"`
}

// CatalogRequest is the bounded read contract used by learner projections.
// It exposes the current published revision index, never authoring payloads.
type CatalogRequest struct {
	WorkspaceKey    string
	Locale          string
	TopicKey        string
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
	Priority      string `json:"priority,omitempty"`
	Lang          string `json:"lang,omitempty"`
	Runtime       string `json:"runtime,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"`
	Depth         int    `json:"depth,omitempty"`
	OrderKey      string `json:"order_key,omitempty"`
	StageKey      string `json:"stage_key,omitempty"`
	CapabilityKey string `json:"capability_key,omitempty"`
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
}

type ReleaseResponse struct {
	ContractVersion  string        `json:"contract_version"`
	WorkspaceKey     string        `json:"workspace_key"`
	ReleaseID        string        `json:"release_id"`
	SourceSnapshotID string        `json:"source_snapshot_id"`
	GeneratedAt      time.Time     `json:"generated_at"`
	Total            int           `json:"total"`
	IncludeFixtures  bool          `json:"include_fixtures"`
	ExcludedFixtures int           `json:"excluded_fixtures"`
	ExcludedNonProd  int           `json:"excluded_non_production"`
	Checks           ReleaseChecks `json:"checks"`
	Items            []ReleaseItem `json:"items"`
	Provenance       struct {
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

type QualityResponse struct {
	ContractVersion string                  `json:"contract_version"`
	WorkspaceKey    string                  `json:"workspace_key"`
	ReleaseID       string                  `json:"release_id"`
	GeneratedAt     time.Time               `json:"generated_at"`
	Total           int                     `json:"total"`
	IncludeFixtures bool                    `json:"include_fixtures"`
	Checks          ReleaseChecks           `json:"checks"`
	Locales         []QualityBucket         `json:"locales"`
	Tracks          []QualityBucket         `json:"tracks"`
	Topics          []QualityBucket         `json:"topics"`
	DuplicateGroups []QualityDuplicateGroup `json:"duplicate_groups"`
	DuplicateStates []QualityBucket         `json:"duplicate_states"`
	Warnings        []string                `json:"warnings"`
	Provenance      struct {
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
