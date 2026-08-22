package search

import "context"

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

type Topic struct {
	StableKey string `json:"stable_key"`
	Title     string `json:"title"`
	Relation  string `json:"relation"`
}

type Service interface {
	Search(context.Context, Request) ([]Result, error)
	GetQuestion(context.Context, string, string, string) (Question, error)
}
