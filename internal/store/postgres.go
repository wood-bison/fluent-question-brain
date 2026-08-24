package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wood-bison/fluent-question-brain/internal/embedding"
	"github.com/wood-bison/fluent-question-brain/internal/normalize"
	"github.com/wood-bison/fluent-question-brain/internal/quality"
	"github.com/wood-bison/fluent-question-brain/internal/search"
	"github.com/wood-bison/fluent-question-brain/internal/taxonomy"
)

type Postgres struct {
	pool *pgxpool.Pool
	// embedder produces query-time vectors for semantic retrieval; the
	// profile key selects which generation of stored vectors it compares
	// against. Defaults keep the deterministic hash pipeline working for
	// tests and offline runs.
	embedder         embedding.Provider
	embeddingProfile string
	// Relevance cutoff (QB-BUG-3). A candidate is returned when it matches a
	// stable key exactly, or the fused rank score reaches minRankScore (two
	// stages agreeing under reciprocal-rank fusion), or its semantic score
	// alone reaches minSemanticScore (one strong semantic match).
	searchMinRankScore     float64
	searchMinSemanticScore float64
}

// UseRelevanceThresholds overrides the default relevance cutoffs. It must be
// called before serving traffic.
func (p *Postgres) UseRelevanceThresholds(minRankScore, minSemanticScore float64) {
	if minRankScore >= 0 {
		p.searchMinRankScore = minRankScore
	}
	if minSemanticScore >= 0 {
		p.searchMinSemanticScore = minSemanticScore
	}
}

// UseEmbedding switches the retrieval provider and the embedding profile the
// search stage reads. It must be called before serving traffic.
func (p *Postgres) UseEmbedding(provider embedding.Provider, profileKey string) {
	if provider != nil {
		p.embedder = provider
	}
	if strings.TrimSpace(profileKey) != "" {
		p.embeddingProfile = strings.TrimSpace(profileKey)
	}
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

type OutboxItem struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	Attempts      int
}

type LocaleText struct {
	ID          string
	ContentHash string
	Text        string
}

// TranslationSource is the immutable English revision that needs a Russian
// locale. The translation command never creates a new question revision; a
// locale is an additive representation of the same pinned revision.
type TranslationSource struct {
	QuestionID  string
	RevisionID  string
	StableKey   string
	ContentHash string
	Payload     []byte
}

// TranslationCoverage returns the current published production denominator
// and the number of those revisions that already have a Russian locale. It
// deliberately excludes fixture content so the release gate cannot look green
// because of smoke records.
func (p *Postgres) TranslationCoverage(ctx context.Context, workspaceKey string) (total, russian int, err error) {
	err = p.pool.QueryRow(ctx, `
		select count(*)::int, count(ru.id)::int
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		left join content.question_locale ru on ru.revision_id = qr.id and ru.locale = 'ru'
		where q.workspace_id = (select id from content.workspace where stable_key = $1)
		  and q.status = 'published'
		  and q.content_kind = 'production'
	`, strings.TrimSpace(workspaceKey)).Scan(&total, &russian)
	if err != nil {
		return 0, 0, fmt.Errorf("query translation coverage: %w", err)
	}
	return total, russian, nil
}

// MissingRussianSources returns current published production revisions with
// no Russian locale. The ordering is stable so a dry-run report and a resumed
// translation batch can be compared without hidden pagination state.
func (p *Postgres) MissingRussianSources(ctx context.Context, workspaceKey string) ([]TranslationSource, error) {
	rows, err := p.pool.Query(ctx, `
		select q.id::text, qr.id::text, q.stable_key, qr.content_hash, qr.normalized_payload
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		left join content.question_locale ru on ru.revision_id = qr.id and ru.locale = 'ru'
		where q.workspace_id = (select id from content.workspace where stable_key = $1)
		  and q.status = 'published'
		  and q.content_kind = 'production'
		  and ru.id is null
		order by q.stable_key
	`, strings.TrimSpace(workspaceKey))
	if err != nil {
		return nil, fmt.Errorf("query missing Russian locales: %w", err)
	}
	defer rows.Close()
	items := make([]TranslationSource, 0)
	for rows.Next() {
		var item TranslationSource
		if err := rows.Scan(&item.QuestionID, &item.RevisionID, &item.StableKey, &item.ContentHash, &item.Payload); err != nil {
			return nil, fmt.Errorf("scan missing Russian locale: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate missing Russian locales: %w", err)
	}
	return items, nil
}

// DegenerateRussianSources returns current published production revisions
// whose Russian locale exists but carries answer text standing in for the
// question (empty prompt, or prompt equal to the short answer). These rows
// were written by imports of legacy-style cards; no translated question was
// ever stored for them, so they cannot be restored from history.
func (p *Postgres) DegenerateRussianSources(ctx context.Context, workspaceKey string) ([]TranslationSource, error) {
	rows, err := p.pool.Query(ctx, `
		select q.id::text, qr.id::text, q.stable_key, qr.content_hash, qr.normalized_payload
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		join content.question_locale ru on ru.revision_id = qr.id and ru.locale = 'ru'
		where q.workspace_id = (select id from content.workspace where stable_key = $1)
		  and q.status = 'published'
		  and q.content_kind = 'production'
		  and (trim(ru.prompt) = '' or trim(ru.prompt) = trim(coalesce(ru.short_answer, '')))
		order by q.stable_key
	`, strings.TrimSpace(workspaceKey))
	if err != nil {
		return nil, fmt.Errorf("query degenerate Russian prompts: %w", err)
	}
	defer rows.Close()
	items := make([]TranslationSource, 0)
	for rows.Next() {
		var item TranslationSource
		if err := rows.Scan(&item.QuestionID, &item.RevisionID, &item.StableKey, &item.ContentHash, &item.Payload); err != nil {
			return nil, fmt.Errorf("scan degenerate Russian locale: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate degenerate Russian locales: %w", err)
	}
	return items, nil
}

// RepairRussianPrompt replaces a degenerate Russian prompt with a translated
// question. The update is guarded: it only applies while the stored prompt is
// still degenerate, so a concurrent editorial fix always wins. The payload,
// revision, and content_hash are untouched; an audit record and an outbox
// event let the indexer re-embed the changed locale text.
func (p *Postgres) RepairRussianPrompt(ctx context.Context, source TranslationSource, question, actor, provider string) error {
	question = strings.TrimSpace(question)
	if question == "" {
		return fmt.Errorf("repaired russian prompt is empty")
	}
	metadata, err := json.Marshal(map[string]string{
		"stable_key":  source.StableKey,
		"revision_id": source.RevisionID,
		"source_hash": source.ContentHash,
		"locale":      "ru",
		"provider":    provider,
		"actor":       actor,
		"mode":        "degenerate-prompt-repair",
	})
	if err != nil {
		return fmt.Errorf("encode repair audit: %w", err)
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin repair transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		update content.question_locale ru
		set prompt = $3
		from content.question_revision qr
		where qr.id = ru.revision_id
		  and ru.locale = 'ru'
		  and qr.id = $2::uuid
		  and (trim(ru.prompt) = '' or trim(ru.prompt) = trim(coalesce(ru.short_answer, '')))
	`, source.QuestionID, source.RevisionID, question)
	if err != nil {
		return fmt.Errorf("repair russian prompt: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil // already fixed elsewhere; nothing to audit
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		select q.workspace_id, 'question_locale', q.id, 'question.locale.translated', $2, $3::jsonb
		from content.question q
		where q.id = $1::uuid
	`, source.QuestionID, actor, metadata); err != nil {
		return fmt.Errorf("write repair audit: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.outbox_event (aggregate_type, aggregate_id, event_type, idempotency_key, payload)
		values ('question_locale', $1::uuid, 'question.locale.translated', $2, $3::jsonb)
		on conflict (idempotency_key) do nothing
	`, source.QuestionID, "question-locale-ru-prompt:"+source.RevisionID, metadata); err != nil {
		return fmt.Errorf("write repair outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit repair transaction: %w", err)
	}
	return nil
}

// StoreRussianTranslation adds one validated locale to an existing
// immutable revision. It is intentionally insert-only: correcting a draft
// requires a new explicit translation review rather than silently replacing
// a published locale. The outbox event lets embeddings/index workers catch up.
func (p *Postgres) StoreRussianTranslation(ctx context.Context, source TranslationSource, prompt, shortAnswer, explanation string, sections []normalize.Section, actor, provider string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("Russian translation prompt is required")
	}
	if strings.TrimSpace(actor) == "" {
		actor = "translation-worker"
	}
	if strings.TrimSpace(provider) == "" {
		provider = "unknown"
	}
	body, err := json.Marshal(map[string]any{
		"sections": sections,
		"translation": map[string]string{
			"source_locale": "en",
			"provider":      provider,
			"actor":         actor,
		},
	})
	if err != nil {
		return fmt.Errorf("encode Russian translation body: %w", err)
	}
	metadata, err := json.Marshal(map[string]string{
		"stable_key":    source.StableKey,
		"revision_id":   source.RevisionID,
		"source_hash":   source.ContentHash,
		"source_locale": "en",
		"locale":        "ru",
		"provider":      provider,
		"actor":         actor,
	})
	if err != nil {
		return fmt.Errorf("encode Russian translation audit: %w", err)
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin Russian translation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		insert into content.question_locale (revision_id, locale, prompt, short_answer, explanation, body)
		values ($1::uuid, 'ru', $2, nullif($3, ''), nullif($4, ''), $5::jsonb)
		on conflict (revision_id, locale) do nothing
	`, source.RevisionID, prompt, strings.TrimSpace(shortAnswer), strings.TrimSpace(explanation), body)
	if err != nil {
		return fmt.Errorf("insert Russian locale: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		select q.workspace_id, 'question_locale', q.id, 'question.locale.translated', $2, $3::jsonb
		from content.question q
		where q.id = $1::uuid
	`, source.QuestionID, actor, metadata); err != nil {
		return fmt.Errorf("write Russian translation audit: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.outbox_event (aggregate_type, aggregate_id, event_type, idempotency_key, payload)
		values ('question_locale', $1::uuid, 'question.locale.translated', $2, $3::jsonb)
		on conflict (idempotency_key) do nothing
	`, source.QuestionID, "question-locale-ru:"+source.RevisionID, metadata); err != nil {
		return fmt.Errorf("write Russian translation outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Russian translation: %w", err)
	}
	return nil
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
	return &Postgres{
		pool:                   pool,
		embedder:               embedding.HashProvider{},
		embeddingProfile:       embedding.ProfileKey,
		searchMinRankScore:     0.02,
		searchMinSemanticScore: 0.55,
	}, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

func (p *Postgres) UpsertCard(ctx context.Context, card normalize.Card, workspaceKey, workspaceName string) (StoredRevision, error) {
	return p.upsertCard(ctx, card, workspaceKey, workspaceName, "fluent-question-vault", "vault-importer", false)
}

// PromoteCard is the only write path exposed to an authoring surface. Payload
// owns drafts in cms; this transaction owns the published revision in content
// and emits the same outbox event used by imports.
func (p *Postgres) PromoteCard(ctx context.Context, card normalize.Card, workspaceKey, workspaceName, actor string) (StoredRevision, error) {
	if strings.TrimSpace(actor) == "" {
		actor = "payload-cms"
	}
	return p.upsertCard(ctx, card, workspaceKey, workspaceName, "payload-cms", actor, true)
}

// PublishImportedCard promotes an already imported vault revision into the
// learner-visible published projection through the same transactional writer
// used by Payload. It is intentionally a separate method so a regular vault
// reconcile can never publish content by accident.
func (p *Postgres) PublishImportedCard(ctx context.Context, card normalize.Card, workspaceKey, workspaceName, actor string) (StoredRevision, error) {
	if strings.TrimSpace(actor) == "" {
		actor = "vault-release"
	}
	return p.upsertCard(ctx, card, workspaceKey, workspaceName, "fluent-question-vault", actor, true)
}

func (p *Postgres) upsertCard(ctx context.Context, card normalize.Card, workspaceKey, workspaceName, sourceSystem, actor string, publish bool) (StoredRevision, error) {
	eventType := "question.imported"
	if sourceSystem == "payload-cms" {
		eventType = "question.promoted"
	}
	if publish && sourceSystem != "payload-cms" {
		eventType = "question.published"
	}
	auditMetadata, err := json.Marshal(map[string]string{"source": sourceSystem, "actor": actor})
	if err != nil {
		return StoredRevision{}, fmt.Errorf("encode audit metadata: %w", err)
	}
	outboxPayload, err := json.Marshal(map[string]string{"reason": "published", "source": sourceSystem})
	if err != nil {
		return StoredRevision{}, fmt.Errorf("encode outbox payload: %w", err)
	}
	status := "draft"
	if publish {
		status = "published"
	}
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
	var existingHash, existingStatus string
	questionExists := true
	err = tx.QueryRow(ctx, `
		select qr.content_hash, q.status
		from content.question q
		left join content.question_revision qr on qr.id = q.current_revision_id
		where q.workspace_id = $1::uuid and q.stable_key = $2
	`, workspaceID, card.StableKey).Scan(&existingHash, &existingStatus)
	if err == pgx.ErrNoRows {
		questionExists = false
	} else if err != nil {
		return StoredRevision{}, fmt.Errorf("read existing question: %w", err)
	}

	var questionID string
	err = tx.QueryRow(ctx, `
		insert into content.question (workspace_id, stable_key, slug, status)
		values ($1::uuid, $2, $3, $4)
		on conflict (workspace_id, stable_key) do update set
		  slug = excluded.slug,
		  status = case when excluded.status = 'published' then 'published' else content.question.status end,
		  updated_at = now()
		returning id::text
	`, workspaceID, card.StableKey, card.Slug, status).Scan(&questionID)
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
	`, questionID, card.Hash, card.Payload, sourceSystem, card.SourceRef).Scan(&revisionID)
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
			"system": sourceSystem,
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
		if locale.locale == "ru" && degenerateRussianPrompt(locale.prompt, locale.shortAnswer) {
			// A card without an explicit "## Question (RU)" section parses
			// with the answer text standing in for the question (both fall
			// back to Core Idea (RU)). Re-publishing such a card must never
			// clobber a translated question that was written onto this
			// revision by the translation flow or a repair.
			var existingPrompt string
			err := tx.QueryRow(ctx, `
				select prompt from content.question_locale
				where revision_id = $1::uuid and locale = 'ru'
			`, revisionID).Scan(&existingPrompt)
			switch {
			case err == nil:
				if strings.TrimSpace(existingPrompt) != "" &&
					strings.TrimSpace(existingPrompt) != strings.TrimSpace(locale.prompt) {
					continue
				}
			case !errors.Is(err, pgx.ErrNoRows):
				return StoredRevision{}, fmt.Errorf("read existing ru locale: %w", err)
			}
		}
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
		set current_revision_id = $1::uuid,
		    level = nullif($3, ''),
		    company = nullif($4, ''),
		    updated_at = now()
		where id = $2::uuid
	`, revisionID, questionID, card.Level, card.Company); err != nil {
		return StoredRevision{}, fmt.Errorf("set current revision: %w", err)
	}

	if topicKey := normalize.TopicStableKey(card.Topic); topicKey != "" {
		topicTitle := strings.TrimSpace(card.Topic)
		if canonicalTitle, ok := taxonomy.CanonicalTopicTitle(card.Topic); ok {
			topicTitle = canonicalTitle
		}
		var topicID string
		err = tx.QueryRow(ctx, `
				insert into content.topic (workspace_id, stable_key, title)
				values ($1::uuid, $2, $3)
				on conflict (workspace_id, stable_key) do nothing
				returning id::text
			`, workspaceID, topicKey, topicTitle).Scan(&topicID)
		if err == pgx.ErrNoRows {
			err = tx.QueryRow(ctx, `
					select id::text from content.topic where workspace_id = $1::uuid and stable_key = $2
				`, workspaceID, topicKey).Scan(&topicID)
		}
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

	publicationChanged := publish && existingStatus != "published"
	if revisionCreated || publicationChanged {
		if _, err = tx.Exec(ctx, `
			insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
			values ($1::uuid, 'question_revision', $2::uuid, $3, $4, $5::jsonb)
		`, workspaceID, revisionID, eventType, actor, auditMetadata); err != nil {
			return StoredRevision{}, fmt.Errorf("write audit event: %w", err)
		}
		if _, err = tx.Exec(ctx, `
			insert into content.outbox_event (aggregate_type, aggregate_id, event_type, idempotency_key, payload)
			values ('question_revision', $1::uuid, $3, $2, $4::jsonb)
			on conflict (idempotency_key) do nothing
		`, revisionID, fmt.Sprintf("question-publication:%s", revisionID), "question.revision.published", outboxPayload); err != nil {
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
	} else if publicationChanged {
		stored.Action = "published"
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

// RollbackQuestion moves the published pointer to an existing immutable
// revision. It is deliberately a pointer update, never a content rewrite:
// the previous state remains available for audit, replay, and another
// rollback. The audit event and indexer outbox entry commit atomically with
// the pointer so a successful API response is always observable.
func (p *Postgres) RollbackQuestion(ctx context.Context, workspaceKey, stableKey, revisionID, actor string) (StoredRevision, error) {
	if strings.TrimSpace(actor) == "" {
		actor = "question-brain-operator"
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return StoredRevision{}, fmt.Errorf("begin rollback transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var workspaceID, questionID, previousRevisionID, targetHash, payload string
	err = tx.QueryRow(ctx, `
		select w.id::text, q.id::text, q.current_revision_id::text,
			qr.content_hash, qr.normalized_payload::text
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.stable_key = $2
		join content.question_revision qr on qr.question_id = q.id and qr.id = $3::uuid
		where w.stable_key = $1
		for update of q
	`, workspaceKey, stableKey, revisionID).Scan(&workspaceID, &questionID, &previousRevisionID, &targetHash, &payload)
	if err != nil {
		return StoredRevision{}, fmt.Errorf("resolve rollback revision: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		update content.question
		set current_revision_id = $1::uuid, status = 'published', updated_at = now()
		where id = $2::uuid
	`, revisionID, questionID); err != nil {
		return StoredRevision{}, fmt.Errorf("set rollback revision: %w", err)
	}

	metadata, err := json.Marshal(map[string]string{
		"actor":                actor,
		"stable_key":           stableKey,
		"previous_revision_id": previousRevisionID,
		"revision_id":          revisionID,
	})
	if err != nil {
		return StoredRevision{}, fmt.Errorf("encode rollback metadata: %w", err)
	}
	var auditID string
	if err = tx.QueryRow(ctx, `
		insert into content.audit_event
		  (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		values ($1::uuid, 'question', $2::uuid, 'question.revision.rolled_back', $3, $4::jsonb)
		returning id::text
	`, workspaceID, questionID, actor, metadata).Scan(&auditID); err != nil {
		return StoredRevision{}, fmt.Errorf("write rollback audit event: %w", err)
	}
	outboxPayload, err := json.Marshal(map[string]string{
		"reason":               "rollback",
		"previous_revision_id": previousRevisionID,
		"revision_id":          revisionID,
	})
	if err != nil {
		return StoredRevision{}, fmt.Errorf("encode rollback outbox payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into content.outbox_event
		  (aggregate_type, aggregate_id, event_type, idempotency_key, payload)
		values ('question_revision', $1::uuid, 'question.revision.rolled_back', $2, $3::jsonb)
		on conflict (idempotency_key) do nothing
	`, revisionID, fmt.Sprintf("question-rollback:%s:%s:%s", questionID, previousRevisionID, revisionID), outboxPayload)
	if err != nil {
		return StoredRevision{}, fmt.Errorf("write rollback outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return StoredRevision{}, fmt.Errorf("commit rollback transaction: %w", err)
	}

	return StoredRevision{
		QuestionID:      questionID,
		RevisionID:      revisionID,
		Hash:            targetHash,
		Payload:         []byte(payload),
		Action:          map[bool]string{true: "unchanged", false: "rolled_back"}[previousRevisionID == revisionID],
		RevisionCreated: false,
	}, nil
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

func (p *Postgres) ClaimOutbox(ctx context.Context, limit int) ([]OutboxItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := p.pool.Query(ctx, `
		with picked as (
			select id
			from content.outbox_event
			where published_at is null
			  and available_at <= now()
			  and (claimed_at is null or claimed_at < now() - interval '5 minutes')
			order by created_at
			limit $1
			for update skip locked
		)
		update content.outbox_event event
		set claimed_at = now(), attempts = event.attempts + 1
		from picked
		where event.id = picked.id
		returning event.id::text, event.aggregate_type, event.aggregate_id::text,
			event.event_type, event.payload, event.attempts
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim outbox: %w", err)
	}
	defer rows.Close()
	items := make([]OutboxItem, 0, limit)
	for rows.Next() {
		var item OutboxItem
		if err := rows.Scan(&item.ID, &item.AggregateType, &item.AggregateID,
			&item.EventType, &item.Payload, &item.Attempts); err != nil {
			return nil, fmt.Errorf("scan claimed outbox: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed outbox: %w", err)
	}
	return items, nil
}

// EmbeddingProfileActive reports whether the requested embedding profile
// exists and is currently active. The indexer refuses to write vectors under
// an inactive profile so search-time and write-time generations cannot drift.
func (p *Postgres) EmbeddingProfileActive(ctx context.Context, profileKey string) (bool, error) {
	var active bool
	err := p.pool.QueryRow(ctx, `
		select active
		from content.embedding_profile
		where profile_key = $1
	`, profileKey).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query embedding profile: %w", err)
	}
	return active, nil
}

// LocalesMissingEmbedding returns every current-revision locale that has no
// stored vector under the given profile, with the exact text the indexer must
// embed.
func (p *Postgres) LocalesMissingEmbedding(ctx context.Context, profileKey string) ([]LocaleText, error) {
	rows, err := p.pool.Query(ctx, `
		select ql.id::text, qr.content_hash,
			concat_ws(E'\n', ql.prompt, ql.short_answer, ql.explanation, ql.body::text)
		from content.question_locale ql
		join content.question_revision qr on qr.id = ql.revision_id
		join content.question q on q.current_revision_id = ql.revision_id
		left join content.question_embedding e
			on e.locale_id = ql.id and e.profile_key = $1
		where e.id is null
		order by ql.id
	`, profileKey)
	if err != nil {
		return nil, fmt.Errorf("list locales missing embedding: %w", err)
	}
	defer rows.Close()
	locales := make([]LocaleText, 0)
	for rows.Next() {
		var locale LocaleText
		if err := rows.Scan(&locale.ID, &locale.ContentHash, &locale.Text); err != nil {
			return nil, fmt.Errorf("scan locale missing embedding: %w", err)
		}
		locales = append(locales, locale)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locales missing embedding: %w", err)
	}
	return locales, nil
}

func (p *Postgres) RevisionLocales(ctx context.Context, revisionID string) ([]LocaleText, error) {
	rows, err := p.pool.Query(ctx, `
		select ql.id::text, qr.content_hash,
			concat_ws(E'\n', ql.prompt, ql.short_answer, ql.explanation, ql.body::text)
		from content.question_locale ql
		join content.question_revision qr on qr.id = ql.revision_id
		where ql.revision_id = $1::uuid
		order by ql.locale
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load revision locales: %w", err)
	}
	defer rows.Close()
	locales := make([]LocaleText, 0, 2)
	for rows.Next() {
		var locale LocaleText
		if err := rows.Scan(&locale.ID, &locale.ContentHash, &locale.Text); err != nil {
			return nil, fmt.Errorf("scan revision locale: %w", err)
		}
		locales = append(locales, locale)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate revision locales: %w", err)
	}
	return locales, nil
}

func (p *Postgres) UpsertEmbedding(ctx context.Context, localeID, profileKey, contentHash, vector string) error {
	_, err := p.pool.Exec(ctx, `
		insert into content.question_embedding (locale_id, profile_key, content_hash, embedding)
		values ($1::uuid, $2, $3, $4::vector)
		on conflict (locale_id, profile_key, content_hash) do nothing
	`, localeID, profileKey, contentHash, vector)
	if err != nil {
		return fmt.Errorf("upsert embedding: %w", err)
	}
	return nil
}

func (p *Postgres) MarkOutboxPublished(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `
		update content.outbox_event
		set published_at = now(), claimed_at = null, last_error = null
		where id = $1::uuid
	`, id)
	if err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

func (p *Postgres) MarkOutboxFailed(ctx context.Context, id, message string, attempts int) error {
	backoffSeconds := attempts * attempts
	if backoffSeconds > 300 {
		backoffSeconds = 300
	}
	_, err := p.pool.Exec(ctx, `
		update content.outbox_event
		set claimed_at = null,
			available_at = now() + make_interval(secs => $2),
			last_error = $3
		where id = $1::uuid
	`, id, backoffSeconds, message)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}

func (p *Postgres) Search(ctx context.Context, request search.Request) ([]search.Result, error) {
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	locale := strings.TrimSpace(request.Locale)
	if locale == "" {
		locale = "en"
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	queryEmbedding, err := p.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed search query: %w", err)
	}
	queryVector := embedding.VectorLiteral(queryEmbedding)
	rows, err := p.pool.Query(ctx, `
		with candidate as (
			select
				q.id::text as question_id,
				qr.id::text as revision_id,
				q.stable_key,
				q.slug,
				coalesce(q.company, '') as company,
			ql.locale,
			ql.prompt,
			coalesce(ql.short_answer, '') as short_answer,
			coalesce(ql.explanation, '') as explanation,
			qr.normalized_payload as payload,
			coalesce(topic.stable_key, '') as topic_key,
			coalesce(topic.title, '') as topic_title,
			case when lower(q.stable_key) = lower($1) or lower(q.slug) = lower($1) then 1.0 else 0.0 end as exact_score,
				coalesce(ts_rank_cd(ql.search_document, websearch_to_tsquery('simple', unaccent($1))), 0)::double precision as fts_score,
				coalesce(similarity(ql.prompt, $1), 0)::double precision as trigram_score,
				coalesce(semantic.semantic_score, 0)::double precision as semantic_score
			from content.workspace w
			join content.question q on q.workspace_id = w.id and q.status = 'published'
			join content.question_revision qr on qr.id = q.current_revision_id
			join lateral (
				select ql.*
				from content.question_locale ql
				where ql.revision_id = qr.id and ql.locale = $2
				limit 1
			) ql on true
			left join lateral (
				select t.stable_key, t.title
				from content.question_topic qt
				join content.topic t on t.id = qt.topic_id
				where qt.question_id = q.id
				order by case when qt.relation = 'primary' then 0 else 1 end, t.stable_key
				limit 1
			) topic on true
			left join lateral (
				select 1 - (qe.embedding <=> $4::vector) as semantic_score
				from content.question_embedding qe
				where qe.locale_id = ql.id and qe.profile_key = $5
				order by qe.embedding <=> $4::vector
				limit 1
			) semantic on true
			where w.stable_key = $3
			  and ($6 = '' or exists (
				select 1
				from content.question_topic qt_filter
				join content.topic t_filter on t_filter.id = qt_filter.topic_id
				where qt_filter.question_id = q.id and t_filter.stable_key = $6
			  ))
			  and ($10 = '' or lower(q.level) = lower($10))
			  and ($11 = '' or lower(q.company) = lower($11))
		)
		,
		ranked as (
			select candidate.*,
				case when exact_score > 0 then row_number() over (order by exact_score desc, stable_key) end as exact_rank,
				case when fts_score > 0 then row_number() over (order by fts_score desc, stable_key) end as fts_rank,
				case when trigram_score >= 0.15 then row_number() over (order by trigram_score desc, stable_key) end as trigram_rank,
				case when semantic_score >= 0.50 then row_number() over (order by semantic_score desc, stable_key) end as semantic_rank
			from candidate
		),
		fused as (
			select ranked.*,
				coalesce(1.0 / (60 + exact_rank), 0) +
				coalesce(1.0 / (60 + fts_rank), 0) +
				coalesce(1.0 / (60 + trigram_rank), 0) +
				coalesce(1.0 / (60 + semantic_rank), 0) as rank_score
			from ranked
		)
		select question_id, revision_id, stable_key, slug, company, locale, prompt,
			short_answer, explanation, topic_key, topic_title, exact_score,
			fts_score, trigram_score, semantic_score,
			array_remove(array[
				case when exact_score > 0 then 'exact'::text end,
				case when fts_score > 0 then 'fts'::text end,
				case when trigram_score >= 0.15 then 'trigram'::text end,
				case when semantic_score >= 0.50 then 'semantic'::text end
			], null) as match_stages,
			rank_score,
			coalesce(payload->'task', 'null'::jsonb) as task,
			coalesce(payload->'rubric', 'null'::jsonb) as rubric,
			coalesce(payload->'choices', 'null'::jsonb) as choices
		from fused
		where exact_score > 0
			or rank_score >= $8
			or semantic_score >= $9
		order by rank_score desc, stable_key
		limit $7
	`, query, locale, request.WorkspaceKey, queryVector, p.embeddingProfile, strings.TrimSpace(request.TopicKey), limit, p.searchMinRankScore, p.searchMinSemanticScore, strings.TrimSpace(request.Level), strings.TrimSpace(request.Company))
	if err != nil {
		return nil, fmt.Errorf("search candidates: %w", err)
	}
	defer rows.Close()
	results := make([]search.Result, 0, limit)
	for rows.Next() {
		var result search.Result
		var task, rubric, choices json.RawMessage
		if err := rows.Scan(
			&result.QuestionID, &result.RevisionID, &result.StableKey, &result.Slug,
			&result.Company, &result.Locale, &result.Prompt, &result.ShortAnswer, &result.Explanation,
			&result.TopicKey, &result.TopicTitle, &result.ExactScore, &result.FTSScore,
			&result.TrigramScore, &result.SemanticScore, &result.MatchStages, &result.RankScore,
			&task, &rubric, &choices,
		); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		result.Task = jsonRawOrNull(task)
		result.Rubric = jsonRawOrNull(rubric)
		result.Choices = jsonRawOrNull(choices)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}
	return results, nil
}

func (p *Postgres) GetQuestion(ctx context.Context, stableKey, workspaceKey, locale string) (search.Question, error) {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		locale = "en"
	}
	var result search.Question
	var body []byte
	var task, rubric, choices json.RawMessage
	err := p.pool.QueryRow(ctx, `
		select q.id::text, qr.id::text, q.stable_key, q.slug, q.status,
			qr.content_hash, ql.locale, ql.prompt,
			coalesce(ql.short_answer, '') as short_answer,
			coalesce(ql.explanation, '') as explanation, ql.body,
			coalesce(qr.normalized_payload->'task', 'null'::jsonb),
			coalesce(qr.normalized_payload->'rubric', 'null'::jsonb),
			coalesce(qr.normalized_payload->'choices', 'null'::jsonb)
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.stable_key = $1 and q.status = 'published'
		join content.question_revision qr on qr.id = q.current_revision_id
		join lateral (
			select ql.*
			from content.question_locale ql
			where ql.revision_id = qr.id and ql.locale = $3
			limit 1
		) ql on true
		where w.stable_key = $2
	`, stableKey, workspaceKey, locale).Scan(
		&result.QuestionID, &result.RevisionID, &result.StableKey, &result.Slug,
		&result.Status, &result.ContentHash, &result.Locale, &result.Prompt,
		&result.ShortAnswer, &result.Explanation, &body,
		&task, &rubric, &choices,
	)
	if err != nil {
		return search.Question{}, fmt.Errorf("get question: %w", err)
	}
	result.Task = jsonRawOrNull(task)
	result.Rubric = jsonRawOrNull(rubric)
	result.Choices = jsonRawOrNull(choices)
	if err := json.Unmarshal(body, &result.Body); err != nil {
		return search.Question{}, fmt.Errorf("decode question body: %w", err)
	}
	rows, err := p.pool.Query(ctx, `
		select t.stable_key, t.title, qt.relation
		from content.question q
		join content.question_topic qt on qt.question_id = q.id
		join content.topic t on t.id = qt.topic_id
		where q.id = $1::uuid
		order by case when qt.relation = 'primary' then 0 else 1 end, t.stable_key
	`, result.QuestionID)
	if err != nil {
		return search.Question{}, fmt.Errorf("get question topics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var topic search.Topic
		if err := rows.Scan(&topic.StableKey, &topic.Title, &topic.Relation); err != nil {
			return search.Question{}, fmt.Errorf("scan question topic: %w", err)
		}
		result.Topics = append(result.Topics, topic)
	}
	if err := rows.Err(); err != nil {
		return search.Question{}, fmt.Errorf("iterate question topics: %w", err)
	}
	return result, nil
}

// Catalog returns the current published index for one workspace. It is a
// bounded learner read: answer bodies stay behind GetQuestion, while stable
// identity, locale coverage, normalized coordinates, and topic relations are
// available for a release-aware projection.
func (p *Postgres) Catalog(ctx context.Context, request search.CatalogRequest) (search.CatalogResponse, error) {
	workspaceKey := strings.TrimSpace(request.WorkspaceKey)
	if workspaceKey == "" {
		return search.CatalogResponse{}, fmt.Errorf("workspace key is required")
	}
	locale := strings.TrimSpace(request.Locale)
	if locale == "" {
		locale = "en"
	}
	offset := request.Offset
	if offset < 0 {
		offset = 0
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 2_000 {
		limit = 2_000
	}
	topicKey := strings.TrimSpace(request.TopicKey)
	track := strings.TrimSpace(request.Track)
	level := strings.TrimSpace(request.Level)
	company := strings.TrimSpace(request.Company)
	includeFixtures := request.IncludeFixtures

	var total int
	if err := p.pool.QueryRow(ctx, `
		select count(*)
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.status = 'published'
		join content.question_revision qr on qr.id = q.current_revision_id
		where w.stable_key = $1
		  and ($3 or q.content_kind = 'production')
		  and ($2 = '' or exists (
			select 1
			from content.question_topic qt
			join content.topic t on t.id = qt.topic_id
			where qt.question_id = q.id and t.stable_key = $2
		  ))
		  and ($4 = '' or lower(coalesce(qr.normalized_payload->>'track', '')) = lower($4))
		  and ($5 = '' or lower(coalesce(q.level, '')) = lower($5))
		  and ($6 = '' or lower(coalesce(q.company, '')) = lower($6))
	`, workspaceKey, topicKey, includeFixtures, track, level, company).Scan(&total); err != nil {
		return search.CatalogResponse{}, fmt.Errorf("count catalog questions: %w", err)
	}
	var excludedFixtures int
	if err := p.pool.QueryRow(ctx, `
		select count(*)
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.status = 'published'
		where w.stable_key = $1 and q.content_kind = 'fixture'
	`, workspaceKey).Scan(&excludedFixtures); err != nil {
		return search.CatalogResponse{}, fmt.Errorf("count excluded catalog fixtures: %w", err)
	}
	var excludedNonProduction int
	if err := p.pool.QueryRow(ctx, `
		select count(*)
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.status = 'published'
		where w.stable_key = $1 and q.content_kind <> 'production'
	`, workspaceKey).Scan(&excludedNonProduction); err != nil {
		return search.CatalogResponse{}, fmt.Errorf("count excluded non-production catalog records: %w", err)
	}
	if includeFixtures {
		excludedFixtures = 0
		excludedNonProduction = 0
	}

	releaseSeed := ""
	if err := p.pool.QueryRow(ctx, `
		select coalesce(string_agg(q.stable_key || ':' || qr.content_hash, '|' order by q.stable_key), '')
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.status = 'published'
		join content.question_revision qr on qr.id = q.current_revision_id
		where w.stable_key = $1
		  and ($2 or q.content_kind = 'production')
	`, workspaceKey, includeFixtures).Scan(&releaseSeed); err != nil {
		return search.CatalogResponse{}, fmt.Errorf("fingerprint catalog release: %w", err)
	}
	releaseHash := sha256.Sum256([]byte(releaseSeed))
	releaseID := fmt.Sprintf("question-release-%x", releaseHash[:8])

	rows, err := p.pool.Query(ctx, `
		with current as (
			select
				q.id::text as question_id,
				qr.id::text as revision_id,
				q.stable_key,
				q.slug,
				q.status,
				qr.content_hash,
			ql.locale,
			ql.prompt,
			coalesce(ql.short_answer, '') as short_answer,
			coalesce(ql.explanation, '') as explanation,
			qr.normalized_payload,
				coalesce((
					select array_agg(available.locale order by available.locale)
					from content.question_locale available
					where available.revision_id = qr.id
				), array[]::text[]) as available_locales
			from content.workspace w
			join content.question q on q.workspace_id = w.id and q.status = 'published'
			join content.question_revision qr on qr.id = q.current_revision_id
			join lateral (
				select ql.*
				from content.question_locale ql
				where ql.revision_id = qr.id and ql.locale = $2
				limit 1
			) ql on true
			where w.stable_key = $1
			  and ($4 or q.content_kind = 'production')
			  and ($3 = '' or exists (
				select 1
				from content.question_topic qt_filter
				join content.topic t_filter on t_filter.id = qt_filter.topic_id
				where qt_filter.question_id = q.id and t_filter.stable_key = $3
			  ))
			  and ($7 = '' or lower(coalesce(qr.normalized_payload->>'track', '')) = lower($7))
			  and ($8 = '' or lower(coalesce(q.level, '')) = lower($8))
			  and ($9 = '' or lower(coalesce(q.company, '')) = lower($9))
		)
		select
			current.question_id,
			current.revision_id,
			current.stable_key,
			current.slug,
			current.status,
			current.content_hash,
			current.locale,
			current.available_locales,
			current.prompt,
			current.short_answer,
			current.explanation,
			current.normalized_payload,
			coalesce(jsonb_agg(
				jsonb_build_object('stable_key', topic.stable_key, 'title', topic.title, 'relation', qt.relation)
				order by case when qt.relation = 'primary' then 0 else 1 end, topic.stable_key
			) filter (where topic.stable_key is not null), '[]'::jsonb) as topics
		from current
		left join content.question_topic qt on qt.question_id = current.question_id::uuid
		left join content.topic topic on topic.id = qt.topic_id
		group by current.question_id, current.revision_id, current.stable_key, current.slug,
			current.status, current.content_hash, current.locale, current.available_locales,
			current.prompt, current.short_answer, current.explanation, current.normalized_payload
		order by current.stable_key
		offset $5 limit $6
	`, workspaceKey, locale, topicKey, includeFixtures, offset, limit, track, level, company)
	if err != nil {
		return search.CatalogResponse{}, fmt.Errorf("query catalog questions: %w", err)
	}
	defer rows.Close()
	questions := make([]search.CatalogItem, 0, limit)
	for rows.Next() {
		var item search.CatalogItem
		var payload []byte
		var topicsJSON []byte
		if err := rows.Scan(
			&item.QuestionID, &item.RevisionID, &item.StableKey, &item.Slug,
			&item.Status, &item.ContentHash, &item.Locale, &item.AvailableLocales,
			&item.Prompt, &item.ShortAnswer, &item.Explanation, &payload, &topicsJSON,
		); err != nil {
			return search.CatalogResponse{}, fmt.Errorf("scan catalog question: %w", err)
		}
		var normalized map[string]any
		if err := json.Unmarshal(payload, &normalized); err != nil {
			return search.CatalogResponse{}, fmt.Errorf("decode catalog metadata: %w", err)
		}
		item.Metadata = catalogMetadata(normalized)
		if err := json.Unmarshal(topicsJSON, &item.Topics); err != nil {
			return search.CatalogResponse{}, fmt.Errorf("decode catalog topics: %w", err)
		}
		questions = append(questions, item)
	}
	if err := rows.Err(); err != nil {
		return search.CatalogResponse{}, fmt.Errorf("iterate catalog questions: %w", err)
	}
	response := search.CatalogResponse{
		ContractVersion:  "question-brain.catalog.v1",
		WorkspaceKey:     workspaceKey,
		Locale:           locale,
		ReleaseID:        releaseID,
		GeneratedAt:      time.Now().UTC(),
		Total:            total,
		Offset:           offset,
		Limit:            limit,
		IncludeFixtures:  includeFixtures,
		ExcludedFixtures: excludedFixtures,
		ExcludedNonProd:  excludedNonProduction,
		Questions:        questions,
	}
	response.Provenance.Explainable = true
	response.Provenance.Source = "content.question.current_revision"
	response.Provenance.Pipeline = []string{"published-current-revision", "exact-locale", "topic-relations"}
	return response, nil
}

// Release returns the complete identity manifest for a pinned content release.
// It intentionally contains no answer bodies: clients use the revision id and
// stable key to reason about provenance, then follow the normal question read
// boundary when a learner selects a card.
func (p *Postgres) Release(ctx context.Context, request search.ReleaseRequest) (search.ReleaseResponse, error) {
	workspaceKey := strings.TrimSpace(request.WorkspaceKey)
	if workspaceKey == "" {
		return search.ReleaseResponse{}, fmt.Errorf("workspace key is required")
	}
	includeFixtures := request.IncludeFixtures

	var releaseSeed string
	if err := p.pool.QueryRow(ctx, `
		select coalesce(string_agg(
			q.stable_key || ':' || qr.content_hash,
			'|' order by q.stable_key
		), '')
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.status = 'published'
		join content.question_revision qr on qr.id = q.current_revision_id
		where w.stable_key = $1
		  and ($2 or q.content_kind = 'production')
	`, workspaceKey, includeFixtures).Scan(&releaseSeed); err != nil {
		return search.ReleaseResponse{}, fmt.Errorf("fingerprint question release: %w", err)
	}
	releaseHash := sha256.Sum256([]byte(releaseSeed))
	releaseID := fmt.Sprintf("question-release-%x", releaseHash[:8])
	var excludedFixtures int
	if err := p.pool.QueryRow(ctx, `
		select count(*)
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.status = 'published'
		where w.stable_key = $1 and q.content_kind = 'fixture'
	`, workspaceKey).Scan(&excludedFixtures); err != nil {
		return search.ReleaseResponse{}, fmt.Errorf("count excluded question release fixtures: %w", err)
	}
	var excludedNonProduction int
	if err := p.pool.QueryRow(ctx, `
		select count(*)
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.status = 'published'
		where w.stable_key = $1 and q.content_kind <> 'production'
	`, workspaceKey).Scan(&excludedNonProduction); err != nil {
		return search.ReleaseResponse{}, fmt.Errorf("count excluded non-production question release records: %w", err)
	}
	if includeFixtures {
		excludedFixtures = 0
		excludedNonProduction = 0
	}

	rows, err := p.pool.Query(ctx, `
		select
			q.id::text,
			qr.id::text,
			q.stable_key,
			qr.content_hash,
			q.status,
			q.content_kind,
			coalesce(array_agg(distinct ql.locale order by ql.locale)
				filter (where ql.locale is not null), '{}'::text[]),
			qr.source_system,
			case
				when exists (
					select 1 from content.question_topic qt
					where qt.question_id = q.id
				) then 'released'
				when exists (
					select 1 from content.placement_decision pd
					where pd.revision_id = qr.id and pd.decision = 'accepted'
				) then 'accepted-pending'
				when exists (
					select 1 from content.placement_decision pd
					where pd.revision_id = qr.id and pd.decision = 'proposed'
				) then 'proposed'
				else 'unplaced'
			end
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.status = 'published'
		join content.question_revision qr on qr.id = q.current_revision_id
		left join content.question_locale ql on ql.revision_id = qr.id
		where w.stable_key = $1
		  and ($2 or q.content_kind = 'production')
		group by q.id, qr.id, q.stable_key, qr.content_hash, q.status,
			q.content_kind, qr.source_system
		order by q.stable_key
	`, workspaceKey, includeFixtures)
	if err != nil {
		return search.ReleaseResponse{}, fmt.Errorf("query question release: %w", err)
	}
	defer rows.Close()

	items := make([]search.ReleaseItem, 0)
	checks := search.ReleaseChecks{}
	for rows.Next() {
		var item search.ReleaseItem
		if err := rows.Scan(
			&item.QuestionID,
			&item.RevisionID,
			&item.StableKey,
			&item.ContentHash,
			&item.Status,
			&item.ContentKind,
			&item.AvailableLocales,
			&item.SourceSystem,
			&item.GraphState,
		); err != nil {
			return search.ReleaseResponse{}, fmt.Errorf("scan question release: %w", err)
		}
		if item.ContentKind == "fixture" {
			item.QualityState = "fixture"
			checks.Fixtures++
		} else {
			item.QualityState = "published"
			checks.Published++
		}
		switch item.GraphState {
		case "released":
			checks.GraphReleased++
		case "accepted-pending":
			checks.GraphAcceptedPending++
		case "proposed":
			checks.GraphProposed++
		default:
			checks.GraphUnplaced++
		}
		if !containsLocale(item.AvailableLocales, "en") {
			checks.MissingEnglish++
		}
		if !containsLocale(item.AvailableLocales, "ru") {
			checks.MissingRussian++
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return search.ReleaseResponse{}, fmt.Errorf("iterate question release: %w", err)
	}

	response := search.ReleaseResponse{
		ContractVersion:  "question-brain.release.v1",
		WorkspaceKey:     workspaceKey,
		ReleaseID:        releaseID,
		SourceSnapshotID: releaseID,
		GeneratedAt:      time.Now().UTC(),
		Total:            len(items),
		IncludeFixtures:  includeFixtures,
		ExcludedFixtures: excludedFixtures,
		ExcludedNonProd:  excludedNonProduction,
		Items:            items,
		Checks:           checks,
	}
	response.Provenance.Explainable = true
	response.Provenance.Source = "content.question.current_revision"
	response.Provenance.Pipeline = []string{
		"published-current-revision",
		"locale-availability",
		"graph-placement-state",
		"fixture-boundary",
	}
	return response, nil
}

func containsLocale(locales []string, wanted string) bool {
	for _, locale := range locales {
		if locale == wanted {
			return true
		}
	}
	return false
}

// Quality returns release-scoped aggregate evidence for operators and future
// clients. It deliberately omits prompts, answers, normalized payloads, and
// source paths: the endpoint is for measuring coverage/review debt, not for
// creating a second content read boundary.
func (p *Postgres) Quality(ctx context.Context, request search.QualityRequest) (search.QualityResponse, error) {
	release, err := p.Release(ctx, search.ReleaseRequest{
		WorkspaceKey:    request.WorkspaceKey,
		IncludeFixtures: request.IncludeFixtures,
	})
	if err != nil {
		return search.QualityResponse{}, fmt.Errorf("read quality release: %w", err)
	}

	// Index-freshness audit: an outbox backlog or missing embeddings used to
	// be invisible here while search silently degraded (QB-BUG-9).
	var outboxPending int
	if err := p.pool.QueryRow(ctx, `
		select count(*)
		from content.outbox_event
		where published_at is null
	`).Scan(&outboxPending); err != nil {
		return search.QualityResponse{}, fmt.Errorf("count pending outbox events: %w", err)
	}
	var localesWithoutEmbedding int
	if err := p.pool.QueryRow(ctx, `
		select count(*)
		from content.question_locale ql
		join content.question q on q.current_revision_id = ql.revision_id
		join content.workspace w on w.id = q.workspace_id
		left join content.question_embedding e
			on e.locale_id = ql.id
			and e.profile_key = (
				select profile_key from content.embedding_profile where active limit 1)
		where w.stable_key = $1
		  and e.id is null
	`, strings.TrimSpace(request.WorkspaceKey)).Scan(&localesWithoutEmbedding); err != nil {
		return search.QualityResponse{}, fmt.Errorf("count locales without embedding: %w", err)
	}
	checks := search.QualityChecks{ReleaseChecks: release.Checks}
	checks.OutboxPending = &outboxPending
	checks.LocalesWithoutEmbedding = &localesWithoutEmbedding

	// Degenerate-prompt audit: a locale row can exist while its prompt is
	// not a usable question at all. Equality with the answer is only one
	// failure mode; PDF section headings leaked into prompts ("SQL",
	// "Указатели", ":") pass an equality check while leaving the card
	// unusable, which is how 70 broken cards once passed acceptance.
	rows, err := p.pool.Query(ctx, `
		select
			coalesce(ru.prompt, ''),
			coalesce(ru.short_answer, ''),
			(ru.id is not null),
			coalesce(en.prompt, ''),
			coalesce(en.short_answer, ''),
			(en.id is not null),
			coalesce(qr.normalized_payload->>'title', ''),
			coalesce(qr.normalized_payload->>'topic', ''),
			coalesce(qr.normalized_payload->>'scope', ''),
			qr.normalized_payload
		from content.question q
		join content.workspace w on w.id = q.workspace_id
		join content.question_revision qr on qr.id = q.current_revision_id
		left join content.question_locale ru on ru.revision_id = qr.id and ru.locale = 'ru'
		left join content.question_locale en on en.revision_id = qr.id and en.locale = 'en'
		where w.stable_key = $1
		  and q.status = 'published'
		  and q.content_kind = 'production'
	`, strings.TrimSpace(request.WorkspaceKey))
	if err != nil {
		return search.QualityResponse{}, fmt.Errorf("query prompts for degeneracy audit: %w", err)
	}
	defer rows.Close()
	degenerate := 0
	semanticShape := 0
	for rows.Next() {
		var ruPrompt, ruAnswer, enPrompt, enAnswer, title, topic, scope string
		var ruPresent, enPresent bool
		var payload []byte
		if err := rows.Scan(&ruPrompt, &ruAnswer, &ruPresent, &enPrompt, &enAnswer, &enPresent, &title, &topic, &scope, &payload); err != nil {
			return search.QualityResponse{}, fmt.Errorf("scan prompts for degeneracy audit: %w", err)
		}
		ruIssues := []quality.PromptIssue(nil)
		enIssues := []quality.PromptIssue(nil)
		if ruPresent {
			ruIssues = quality.PromptIssues(ruPrompt, ruAnswer, title, topic)
		}
		if enPresent {
			enIssues = quality.PromptIssues(enPrompt, enAnswer, title, topic)
		}
		titleHasSemanticShapeIssue := quality.IsPDFHeading(title) || quality.IsCodeFragmentTitle(title)
		if len(ruIssues) > 0 || len(enIssues) > 0 || titleHasSemanticShapeIssue || quality.JSONHasPDFArtifact(payload) || quality.JSONHasPDFLayoutArtifact(payload, scope) {
			degenerate++
		}
		semanticIssue := titleHasSemanticShapeIssue
		for _, issue := range append(ruIssues, enIssues...) {
			if quality.IsSemanticShapeIssue(issue.Code) {
				semanticIssue = true
				break
			}
		}
		if semanticIssue {
			semanticShape++
		}
		if ruPresent && (strings.TrimSpace(ruPrompt) == "" || strings.TrimSpace(ruPrompt) == strings.TrimSpace(ruAnswer)) {
			checks.RuPromptEqualsAnswer++
		}
	}
	if err := rows.Err(); err != nil {
		return search.QualityResponse{}, fmt.Errorf("iterate prompts for degeneracy audit: %w", err)
	}
	checks.DegeneratePrompts = degenerate
	checks.SemanticShapeIssues = semanticShape

	tracks := make(map[string]int)
	topics := make(map[string]int)
	levels := make(map[string]int)
	companies := make(map[string]int)
	locales := make(map[string]int)
	duplicatePrompts := make(map[string][]string)
	rows, err = p.pool.Query(ctx, `
		select
			q.stable_key,
			qr.normalized_payload,
			q.level,
			q.company,
			coalesce(array_agg(distinct ql.locale order by ql.locale)
				filter (where ql.locale is not null), '{}'::text[])
		from content.workspace w
		join content.question q on q.workspace_id = w.id and q.status = 'published'
		join content.question_revision qr on qr.id = q.current_revision_id
		left join content.question_locale ql on ql.revision_id = qr.id
		where w.stable_key = $1
		  and ($2 or q.content_kind = 'production')
		group by q.stable_key, qr.normalized_payload, q.level, q.company
		order by q.stable_key
	`, strings.TrimSpace(request.WorkspaceKey), request.IncludeFixtures)
	if err != nil {
		return search.QualityResponse{}, fmt.Errorf("query quality metadata: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var stableKey string
		var payload []byte
		var level, company *string
		var availableLocales []string
		if err := rows.Scan(&stableKey, &payload, &level, &company, &availableLocales); err != nil {
			return search.QualityResponse{}, fmt.Errorf("scan quality metadata: %w", err)
		}
		for _, locale := range availableLocales {
			locales[locale]++
		}
		var value map[string]any
		if err := json.Unmarshal(payload, &value); err != nil {
			return search.QualityResponse{}, fmt.Errorf("decode quality metadata for %s: %w", stableKey, err)
		}
		tracks[qualityBucketValue(value, "track")]++
		levels[nullableBucketValue(level)]++
		companies[nullableBucketValue(company)]++
		prompt := normalizeQualityPrompt(qualityBucketValue(value, "question"))
		if prompt != "" {
			duplicatePrompts[prompt] = append(duplicatePrompts[prompt], stableKey)
		}
	}
	if err := rows.Err(); err != nil {
		return search.QualityResponse{}, fmt.Errorf("iterate quality metadata: %w", err)
	}
	// Topic is a legacy content-graph placement.  Use the canonical topic
	// registry and question_topic bindings as the source of truth rather than
	// counting raw payload labels; this collapses reviewed aliases and keeps
	// zero-card registry entries visible for auditability.
	topics = make(map[string]int)
	topicRows, err := p.pool.Query(ctx, `
		select t.title, count(distinct q.id)::int
		from content.workspace w
		join content.topic t on t.workspace_id = w.id
		left join content.question_topic qt on qt.topic_id = t.id
		left join content.question q on q.id = qt.question_id
			and q.workspace_id = w.id
			and q.status = 'published'
			and ($2 or q.content_kind = 'production')
		where w.stable_key = $1
		group by t.title
		order by t.title
	`, strings.TrimSpace(request.WorkspaceKey), request.IncludeFixtures)
	if err != nil {
		return search.QualityResponse{}, fmt.Errorf("query canonical topic registry: %w", err)
	}
	for topicRows.Next() {
		var title string
		var count int
		if err := topicRows.Scan(&title, &count); err != nil {
			topicRows.Close()
			return search.QualityResponse{}, fmt.Errorf("scan canonical topic registry: %w", err)
		}
		topics[title] = count
	}
	if err := topicRows.Err(); err != nil {
		topicRows.Close()
		return search.QualityResponse{}, fmt.Errorf("iterate canonical topic registry: %w", err)
	}
	topicRows.Close()

	duplicateGroups := make([]search.QualityDuplicateGroup, 0)
	for prompt, stableKeys := range duplicatePrompts {
		if len(stableKeys) < 2 {
			continue
		}
		sort.Strings(stableKeys)
		hash := sha256.Sum256([]byte(prompt))
		duplicateGroups = append(duplicateGroups, search.QualityDuplicateGroup{
			Fingerprint: fmt.Sprintf("prompt-%x", hash[:8]),
			StableKeys:  append([]string(nil), stableKeys...),
		})
	}
	sort.Slice(duplicateGroups, func(i, j int) bool {
		return duplicateGroups[i].Fingerprint < duplicateGroups[j].Fingerprint
	})

	// An exact prompt match is only an open quality warning when every pair in
	// the group still lacks an explicit terminal decision. Keep resolved groups
	// in the response for auditability, but do not force operators to review a
	// candidate that was already marked not_duplicate/keep_separate/merge.
	duplicateDecisions := make(map[string]string)
	decisionRows, err := p.pool.Query(ctx, `
		select left_question.stable_key, right_question.stable_key, dc.decision
		from content.duplicate_candidate dc
		join content.question_revision left_revision on left_revision.id = dc.left_revision_id
		join content.question_revision right_revision on right_revision.id = dc.right_revision_id
		join content.question left_question on left_question.id = left_revision.question_id
		join content.question right_question on right_question.id = right_revision.question_id
		join content.workspace w on w.id = left_question.workspace_id
		where w.stable_key = $1
		  and left_question.current_revision_id = left_revision.id
		  and right_question.current_revision_id = right_revision.id
		  and ($2 or (left_question.content_kind = 'production' and right_question.content_kind = 'production'))
	`, strings.TrimSpace(request.WorkspaceKey), request.IncludeFixtures)
	if err != nil {
		return search.QualityResponse{}, fmt.Errorf("query duplicate decisions: %w", err)
	}
	defer decisionRows.Close()
	for decisionRows.Next() {
		var leftStableKey, rightStableKey, decision string
		if err := decisionRows.Scan(&leftStableKey, &rightStableKey, &decision); err != nil {
			return search.QualityResponse{}, fmt.Errorf("scan duplicate decision: %w", err)
		}
		duplicateDecisions[qualityDuplicatePairKey(leftStableKey, rightStableKey)] = decision
	}
	if err := decisionRows.Err(); err != nil {
		return search.QualityResponse{}, fmt.Errorf("iterate duplicate decisions: %w", err)
	}

	openDuplicateGroups := make([]search.QualityDuplicateGroup, 0, len(duplicateGroups))
	resolvedDuplicateGroups := make([]search.QualityResolvedDuplicateGroup, 0)
	for _, group := range duplicateGroups {
		allResolved := true
		decisions := make([]string, 0, len(group.StableKeys))
		for leftIndex := 0; leftIndex < len(group.StableKeys); leftIndex++ {
			for rightIndex := leftIndex + 1; rightIndex < len(group.StableKeys); rightIndex++ {
				decision, ok := duplicateDecisions[qualityDuplicatePairKey(group.StableKeys[leftIndex], group.StableKeys[rightIndex])]
				if !ok || !terminalDuplicateDecision(decision) {
					allResolved = false
					continue
				}
				decisions = append(decisions, decision)
			}
		}
		if !allResolved {
			openDuplicateGroups = append(openDuplicateGroups, group)
			continue
		}
		sort.Strings(decisions)
		resolvedDuplicateGroups = append(resolvedDuplicateGroups, search.QualityResolvedDuplicateGroup{
			Fingerprint: group.Fingerprint,
			StableKeys:  append([]string(nil), group.StableKeys...),
			Decisions:   decisions,
		})
	}

	duplicateStates := make(map[string]int)
	duplicateRows, err := p.pool.Query(ctx, `
		select dc.decision, count(*)
		from content.duplicate_candidate dc
		join content.question_revision left_revision on left_revision.id = dc.left_revision_id
		join content.question left_question on left_question.id = left_revision.question_id
		join content.workspace w on w.id = left_question.workspace_id
		where w.stable_key = $1
		  and ($2 or left_question.content_kind = 'production')
		group by dc.decision
	`, strings.TrimSpace(request.WorkspaceKey), request.IncludeFixtures)
	if err != nil {
		return search.QualityResponse{}, fmt.Errorf("query duplicate states: %w", err)
	}
	defer duplicateRows.Close()
	for duplicateRows.Next() {
		var decision string
		var count int
		if err := duplicateRows.Scan(&decision, &count); err != nil {
			return search.QualityResponse{}, fmt.Errorf("scan duplicate state: %w", err)
		}
		duplicateStates[decision] = count
	}
	if err := duplicateRows.Err(); err != nil {
		return search.QualityResponse{}, fmt.Errorf("iterate duplicate states: %w", err)
	}

	response := search.QualityResponse{
		ContractVersion:         "question-brain.quality.v1",
		WorkspaceKey:            release.WorkspaceKey,
		ReleaseID:               release.ReleaseID,
		GeneratedAt:             time.Now().UTC(),
		Total:                   release.Total,
		IncludeFixtures:         request.IncludeFixtures,
		Checks:                  checks,
		Locales:                 qualityBuckets(locales),
		Tracks:                  qualityBuckets(tracks),
		Topics:                  qualityBuckets(topics),
		Levels:                  qualityBuckets(levels),
		Companies:               qualityBuckets(companies),
		DuplicateGroups:         openDuplicateGroups,
		ResolvedDuplicateGroups: resolvedDuplicateGroups,
		DuplicateStates:         qualityBuckets(duplicateStates),
		Warnings:                qualityWarnings(checks, len(openDuplicateGroups)),
	}
	response.Provenance.Explainable = true
	response.Provenance.Source = "content.question.current_revision"
	response.Provenance.Pipeline = []string{
		"published-current-revision",
		"locale-availability",
		"normalized-taxonomy-metadata",
		"exact-prompt-duplicate-audit",
		"duplicate-review-state",
		"graph-placement-state",
	}
	return response, nil
}

// jsonRawOrNull turns a SQL COALESCE'd 'null' JSON value back into a Go nil
// so the API omits absent task/rubric/choices blocks entirely.
func jsonRawOrNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

// degenerateRussianPrompt reports whether a parsed Russian locale degrades
// the question to the answer text: empty, or identical to the short answer
// because both fell back to Core Idea (RU) in a card without Question (RU).
func degenerateRussianPrompt(prompt, shortAnswer string) bool {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return true
	}
	return prompt == strings.TrimSpace(shortAnswer)
}

func qualityBucketValue(value map[string]any, key string) string {
	if raw, ok := value[key].(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return "unclassified"
}

// nullableBucketValue maps a missing level/company dimension to the same
// "unclassified" bucket used by track/topic; an absent dimension is a legal
// state for legacy content, not an error.
func nullableBucketValue(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "unclassified"
	}
	return strings.TrimSpace(*value)
}

func normalizeQualityPrompt(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func qualityDuplicatePairKey(left, right string) string {
	if left > right {
		left, right = right, left
	}
	return left + "\x00" + right
}

func terminalDuplicateDecision(decision string) bool {
	switch decision {
	case "not_duplicate", "keep_separate", "merge":
		return true
	default:
		return false
	}
}

func qualityBuckets(values map[string]int) []search.QualityBucket {
	buckets := make([]search.QualityBucket, 0, len(values))
	for key, count := range values {
		buckets = append(buckets, search.QualityBucket{Key: key, Count: count})
	}
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].Count != buckets[j].Count {
			return buckets[i].Count > buckets[j].Count
		}
		return buckets[i].Key < buckets[j].Key
	})
	return buckets
}

func qualityWarnings(checks search.QualityChecks, duplicateGroups int) []string {
	warnings := make([]string, 0, 5)
	if checks.GraphReleased < checks.Published {
		warnings = append(warnings, "published questions still have graph placement review debt")
	}
	if checks.MissingRussian > 0 {
		warnings = append(warnings, "some published questions fall back from Russian to English")
	}
	if checks.DegeneratePrompts > 0 {
		warnings = append(warnings, fmt.Sprintf("%d published cards have a degenerate prompt or extracted PDF artefact", checks.DegeneratePrompts))
	}
	if duplicateGroups > 0 {
		warnings = append(warnings, "exact prompt duplicate groups require explicit review")
	}
	if checks.OutboxPending != nil && *checks.OutboxPending > 0 {
		warnings = append(warnings, fmt.Sprintf("embedding index is lagging: %d outbox events are still pending", *checks.OutboxPending))
	}
	if checks.LocalesWithoutEmbedding != nil && *checks.LocalesWithoutEmbedding > 0 {
		warnings = append(warnings, fmt.Sprintf("embedding index is incomplete: %d current-revision locales have no vector under the active profile", *checks.LocalesWithoutEmbedding))
	}
	return warnings
}

func catalogMetadata(payload map[string]any) search.CatalogMetadata {
	metadata := search.CatalogMetadata{
		Group:          stringField(payload, "group"),
		Level:          stringField(payload, "level"),
		Scope:          stringField(payload, "scope"),
		Title:          stringField(payload, "title"),
		Topic:          stringField(payload, "topic"),
		Track:          stringField(payload, "track"),
		Company:        stringField(payload, "company"),
		Priority:       stringField(payload, "priority"),
		Lang:           stringField(payload, "lang"),
		Runtime:        stringField(payload, "runtime"),
		ExecutionMode:  stringField(payload, "execution_mode"),
		OrderKey:       stringField(payload, "order_key"),
		StageKey:       stringField(payload, "stage_key"),
		ProgramKey:     stringField(payload, "program_key"),
		PathKey:        stringField(payload, "path_key"),
		DomainKey:      stringField(payload, "domain_key"),
		CapabilityKey:  stringField(payload, "capability_key"),
		MappingState:   stringField(payload, "mapping_state"),
		MappingVersion: stringField(payload, "mapping_version"),
	}
	// stage_key was the first Lab bridge and is now a deprecated alias for the
	// shared Domain level. Preserve it for existing consumers while making the
	// canonical field explicit for new clients.
	if metadata.StageKey == "" && metadata.DomainKey != "" {
		metadata.StageKey = metadata.DomainKey
	}
	if value, ok := payload["depth"].(float64); ok {
		metadata.Depth = int(value)
	}
	return metadata
}

func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
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
