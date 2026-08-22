package store

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wood-bison/fluent-question-brain/internal/normalize"
)

type Postgres struct {
	pool *pgxpool.Pool
}

type StoredRevision struct {
	QuestionID      string
	RevisionID      string
	Hash            string
	Payload         []byte
	Action          string
	RevisionCreated bool
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

type ImportItem struct {
	RunID       string
	SourceRef   string
	StableKey   string
	ContentHash string
	Action      string
	QuestionID  string
	Error       string
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
	var existingHash string
	questionExists := true
	err = tx.QueryRow(ctx, `
		select qr.content_hash
		from content.question q
		left join content.question_revision qr on qr.id = q.current_revision_id
		where q.workspace_id = $1::uuid and q.stable_key = $2
	`, workspaceID, card.StableKey).Scan(&existingHash)
	if err == pgx.ErrNoRows {
		questionExists = false
	} else if err != nil {
		return StoredRevision{}, fmt.Errorf("read existing question: %w", err)
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
	locales := []struct {
		locale, prompt, shortAnswer, explanation string
	}{
		{locale: "en", prompt: prompt, shortAnswer: shortAnswer, explanation: explanation},
	}
	ruPrompt, ruShortAnswer, ruExplanation := normalize.RussianFields(card)
	if ruPrompt != "" || ruShortAnswer != "" || ruExplanation != "" {
		if ruPrompt == "" {
			ruPrompt = prompt
		}
		locales = append(locales, struct {
			locale, prompt, shortAnswer, explanation string
		}{locale: "ru", prompt: ruPrompt, shortAnswer: ruShortAnswer, explanation: ruExplanation})
	}
	for _, locale := range locales {
		_, err = tx.Exec(ctx, `
			insert into content.question_locale (revision_id, locale, prompt, short_answer, explanation, body)
			values ($1::uuid, $2, $3, $4, $5, $6::jsonb)
			on conflict (revision_id, locale) do update set
			  prompt = excluded.prompt,
			  short_answer = excluded.short_answer,
			  explanation = excluded.explanation,
			  body = excluded.body
		`, revisionID, locale.locale, locale.prompt, locale.shortAnswer, locale.explanation, body)
		if err != nil {
			return StoredRevision{}, fmt.Errorf("upsert %s locale: %w", locale.locale, err)
		}
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
	stored, err := p.ExportRevision(ctx, revisionID)
	if err != nil {
		return StoredRevision{}, err
	}
	stored.QuestionID = questionID
	stored.RevisionCreated = revisionCreated
	if !questionExists {
		stored.Action = "created"
	} else if revisionCreated && existingHash != card.Hash {
		stored.Action = "updated"
	} else {
		stored.Action = "unchanged"
	}
	return stored, nil
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

func (p *Postgres) StartImportRun(ctx context.Context, workspaceKey, workspaceName, sourceSystem, sourceRoot, mode string) (string, error) {
	var runID string
	err := p.pool.QueryRow(ctx, `
		with workspace as (
			insert into content.workspace (stable_key, display_name)
			values ($1, $2)
			on conflict (stable_key) do update set display_name = excluded.display_name
			returning id
		)
		insert into content.import_run (workspace_id, source_system, source_root, mode)
		select id, $3, $4, $5 from workspace
		returning id::text
	`, workspaceKey, workspaceName, sourceSystem, sourceRoot, mode).Scan(&runID)
	if err != nil {
		return "", fmt.Errorf("start import run: %w", err)
	}
	return runID, nil
}

func (p *Postgres) RecordImportItem(ctx context.Context, item ImportItem) error {
	_, err := p.pool.Exec(ctx, `
		insert into content.import_item (run_id, source_ref, stable_key, content_hash, action, question_id, error)
		values ($1::uuid, $2, nullif($3, ''), nullif($4, ''), $5, nullif($6, '')::uuid, nullif($7, ''))
		on conflict (run_id, source_ref) do update set
		  stable_key = excluded.stable_key,
		  content_hash = excluded.content_hash,
		  action = excluded.action,
		  question_id = excluded.question_id,
		  error = excluded.error
	`, item.RunID, item.SourceRef, item.StableKey, item.ContentHash, item.Action, item.QuestionID, item.Error)
	if err != nil {
		return fmt.Errorf("record import item: %w", err)
	}
	return nil
}

func (p *Postgres) ArchiveMissingSourceQuestions(ctx context.Context, workspaceKey, sourceSystem, sourceRoot, runID string) (int64, error) {
	prefix := strings.TrimRight(sourceRoot, string(filepath.Separator)) + string(filepath.Separator) + "%"
	result, err := p.pool.Exec(ctx, `
		update content.question q
		set status = 'archived', updated_at = now()
		where q.workspace_id = (select id from content.workspace where stable_key = $1)
		  and q.current_revision_id in (
			select qr.id
			from content.question_revision qr
			where qr.source_system = $2 and qr.source_ref like $3
		  )
		  and not exists (
			select 1
			from content.import_item item
			join content.question_revision qr on qr.source_ref = item.source_ref
			where item.run_id = $4::uuid and qr.id = q.current_revision_id
		  )
	`, workspaceKey, sourceSystem, prefix, runID)
	if err != nil {
		return 0, fmt.Errorf("archive missing source questions: %w", err)
	}
	return result.RowsAffected(), nil
}

func (p *Postgres) FinishImportRun(ctx context.Context, runID, status string, totals any) error {
	encoded, err := json.Marshal(totals)
	if err != nil {
		return fmt.Errorf("encode import totals: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		update content.import_run
		set status = $2, totals = $3::jsonb, completed_at = now()
		where id = $1::uuid
	`, runID, status, encoded)
	if err != nil {
		return fmt.Errorf("finish import run: %w", err)
	}
	return nil
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
