package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wood-bison/fluent-question-brain/internal/normalize"
)

type Postgres struct {
	pool *pgxpool.Pool
}

type StoredRevision struct {
	RevisionID string
	Hash       string
	Payload    []byte
}

type DuplicateDecision struct {
	WorkspaceKey   string
	LeftStableKey  string
	RightStableKey string
	ExactScore     float64
	SemanticScore  float64
	Decision       string
	Actor          string
}

func Open(ctx context.Context, databaseURL string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	config.MaxConns = 4
	config.MinConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

func (p *Postgres) UpsertCard(ctx context.Context, card normalize.Card, workspaceKey, workspaceName string) (StoredRevision, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return StoredRevision{}, fmt.Errorf("begin card transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var workspaceID string
	err = tx.QueryRow(ctx, `
		insert into content.workspace (stable_key, display_name)
		values ($1, $2)
		on conflict (stable_key) do update set display_name = excluded.display_name
		returning id::text
	`, workspaceKey, workspaceName).Scan(&workspaceID)
	if err != nil {
		return StoredRevision{}, fmt.Errorf("upsert workspace: %w", err)
	}

	var questionID string
	err = tx.QueryRow(ctx, `
		insert into content.question (workspace_id, stable_key, slug, status)
		values ($1::uuid, $2, $3, 'draft')
		on conflict (workspace_id, stable_key) do update set slug = excluded.slug, updated_at = now()
		returning id::text
	`, workspaceID, card.StableKey, card.Slug).Scan(&questionID)
	if err != nil {
		return StoredRevision{}, fmt.Errorf("upsert question: %w", err)
	}

	var revisionID string
	err = tx.QueryRow(ctx, `
		insert into content.question_revision
		  (question_id, revision_no, content_hash, normalized_payload, source_system, source_ref)
		values ($1::uuid, coalesce((select max(revision_no) + 1 from content.question_revision where question_id = $1::uuid), 1), $2, $3::jsonb, $4, $5)
		on conflict (question_id, content_hash) do nothing
		returning id::text
	`, questionID, card.Hash, card.Payload, "fluent-question-vault", card.SourceRef).Scan(&revisionID)
	revisionCreated := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return StoredRevision{}, fmt.Errorf("upsert revision: %w", err)
	}
	if !revisionCreated {
		err = tx.QueryRow(ctx, `
			select id::text
			from content.question_revision
			where question_id = $1::uuid and content_hash = $2
		`, questionID, card.Hash).Scan(&revisionID)
		if err != nil {
			return StoredRevision{}, fmt.Errorf("find existing revision: %w", err)
		}
	}

	prompt, shortAnswer, explanation := normalize.EnglishFields(card)
	body, err := json.Marshal(map[string]any{
		"source": map[string]string{
			"system": "fluent-question-vault",
			"path":   card.SourceRef,
		},
		"sections": normalize.SectionsForBody(card),
	})
	if err != nil {
		return StoredRevision{}, fmt.Errorf("encode card body: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into content.question_locale (revision_id, locale, prompt, short_answer, explanation, body)
		values ($1::uuid, 'en', $2, $3, $4, $5::jsonb)
		on conflict (revision_id, locale) do update set
		  prompt = excluded.prompt,
		  short_answer = excluded.short_answer,
		  explanation = excluded.explanation,
		  body = excluded.body
	`, revisionID, prompt, shortAnswer, explanation, body)
	if err != nil {
		return StoredRevision{}, fmt.Errorf("upsert english locale: %w", err)
	}

	if _, err = tx.Exec(ctx, `
		update content.question
		set current_revision_id = $1::uuid, updated_at = now()
		where id = $2::uuid
	`, revisionID, questionID); err != nil {
		return StoredRevision{}, fmt.Errorf("set current revision: %w", err)
	}

	if topicKey := normalize.TopicStableKey(card.Topic); topicKey != "" {
		var topicID string
		err = tx.QueryRow(ctx, `
			insert into content.topic (workspace_id, stable_key, title)
			values ($1::uuid, $2, $3)
			on conflict (workspace_id, stable_key) do update set title = excluded.title
			returning id::text
		`, workspaceID, topicKey, card.Topic).Scan(&topicID)
		if err != nil {
			return StoredRevision{}, fmt.Errorf("upsert source topic: %w", err)
		}
		if _, err = tx.Exec(ctx, `
			insert into content.placement_decision (revision_id, topic_id, decision, evidence)
			values ($1::uuid, $2::uuid, 'proposed', $3::jsonb)
			on conflict (revision_id, topic_id) do update set evidence = excluded.evidence
		`, revisionID, topicID, `{"source":"vault-importer","reason":"source_topic"}`); err != nil {
			return StoredRevision{}, fmt.Errorf("record placement proposal: %w", err)
		}
	}

	if revisionCreated {
		if _, err = tx.Exec(ctx, `
			insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
			values ($1::uuid, 'question_revision', $2::uuid, 'question.imported', 'vault-importer', $3::jsonb)
		`, workspaceID, revisionID, `{"source":"fluent-question-vault"}`); err != nil {
			return StoredRevision{}, fmt.Errorf("write audit event: %w", err)
		}
		if _, err = tx.Exec(ctx, `
			insert into content.outbox_event (aggregate_type, aggregate_id, event_type, idempotency_key, payload)
			values ('question_revision', $1::uuid, 'question.revision.imported', $2, $3::jsonb)
			on conflict (idempotency_key) do nothing
		`, revisionID, "question-revision:"+revisionID, `{"reason":"g1-round-trip"}`); err != nil {
			return StoredRevision{}, fmt.Errorf("write outbox event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return StoredRevision{}, fmt.Errorf("commit card transaction: %w", err)
	}
	return p.ExportRevision(ctx, revisionID)
}

func (p *Postgres) ExportRevision(ctx context.Context, revisionID string) (StoredRevision, error) {
	var stored StoredRevision
	var payload string
	err := p.pool.QueryRow(ctx, `
		select id::text, content_hash, normalized_payload::text
		from content.question_revision
		where id = $1::uuid
	`, revisionID).Scan(&stored.RevisionID, &stored.Hash, &payload)
	if err != nil {
		return StoredRevision{}, fmt.Errorf("export revision: %w", err)
	}
	stored.Payload = []byte(payload)
	return stored, nil
}

// RecordDuplicateDecision makes the duplicate review explicit and auditable.
// Stable keys are resolved to the current revisions in one transaction so a
// review can never point at an unversioned or unrelated card.
func (p *Postgres) RecordDuplicateDecision(ctx context.Context, decision DuplicateDecision) error {
	if decision.Decision == "" {
		decision.Decision = "open"
	}
	if decision.Actor == "" {
		decision.Actor = "g1-audit"
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin duplicate decision transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var workspaceID, leftRevisionID, rightRevisionID string
	err = tx.QueryRow(ctx, `
		select w.id::text, left_q.current_revision_id::text, right_q.current_revision_id::text
		from content.workspace w
		join content.question left_q on left_q.workspace_id = w.id and left_q.stable_key = $2
		join content.question right_q on right_q.workspace_id = w.id and right_q.stable_key = $3
		where w.stable_key = $1
	`, decision.WorkspaceKey, decision.LeftStableKey, decision.RightStableKey).Scan(&workspaceID, &leftRevisionID, &rightRevisionID)
	if err != nil {
		return fmt.Errorf("resolve duplicate revisions: %w", err)
	}
	if leftRevisionID > rightRevisionID {
		leftRevisionID, rightRevisionID = rightRevisionID, leftRevisionID
	}
	var candidateID string
	err = tx.QueryRow(ctx, `
		insert into content.duplicate_candidate
		  (workspace_id, left_revision_id, right_revision_id, exact_score, semantic_score, decision, decided_by, decided_at)
		values ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, now())
		on conflict (left_revision_id, right_revision_id) do update set
		  exact_score = excluded.exact_score,
		  semantic_score = excluded.semantic_score,
		  decision = excluded.decision,
		  decided_by = excluded.decided_by,
		  decided_at = excluded.decided_at
		returning id::text
	`, workspaceID, leftRevisionID, rightRevisionID, decision.ExactScore, decision.SemanticScore, decision.Decision, decision.Actor).Scan(&candidateID)
	if err != nil {
		return fmt.Errorf("upsert duplicate candidate: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"left_stable_key":  decision.LeftStableKey,
		"right_stable_key": decision.RightStableKey,
		"exact_score":      decision.ExactScore,
		"semantic_score":   decision.SemanticScore,
		"decision":         decision.Decision,
	})
	if err != nil {
		return fmt.Errorf("encode duplicate evidence: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		insert into content.audit_event
		  (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		values ($1::uuid, 'duplicate_candidate', $2::uuid, 'duplicate_candidate.decided', $3, $4::jsonb)
	`, workspaceID, candidateID, decision.Actor, metadata); err != nil {
		return fmt.Errorf("write duplicate audit event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit duplicate decision: %w", err)
	}
	return nil
}

func (p *Postgres) CloseByStableKey(ctx context.Context, workspaceKey, stableKey string) error {
	_, err := p.pool.Exec(ctx, `
		delete from content.question
		where workspace_id = (select id from content.workspace where stable_key = $1)
		  and stable_key = $2
	`, workspaceKey, stableKey)
	return err
}
