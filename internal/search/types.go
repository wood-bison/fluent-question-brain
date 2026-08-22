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
	WorkspaceKey string
	Locale       string
	TopicKey     string
	Offset       int
	Limit        int
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
	ContractVersion string        `json:"contract_version"`
	WorkspaceKey    string        `json:"workspace_key"`
	Locale          string        `json:"locale"`
	ReleaseID       string        `json:"release_id"`
	GeneratedAt     time.Time     `json:"generated_at"`
	Total           int           `json:"total"`
	Offset          int           `json:"offset"`
	Limit           int           `json:"limit"`
	Questions       []CatalogItem `json:"questions"`
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
