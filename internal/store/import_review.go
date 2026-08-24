package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wood-bison/fluent-question-brain/internal/embedding"
	"github.com/wood-bison/fluent-question-brain/internal/normalize"
)

const ImportReviewContractVersion = "question-brain.import-review.v1"

type ImportReviewCandidate struct {
	ID                string     `json:"id"`
	StageID           string     `json:"stage_id"`
	RelatedStableKey  string     `json:"related_stable_key"`
	RelatedRevisionID string     `json:"related_revision_id"`
	CandidateType     string     `json:"candidate_type"`
	ExactScore        *float64   `json:"exact_score,omitempty"`
	LexicalScore      *float64   `json:"lexical_score,omitempty"`
	SemanticScore     *float64   `json:"semantic_score,omitempty"`
	EmbeddingProfile  string     `json:"embedding_profile,omitempty"`
	Decision          string     `json:"decision"`
	DecidedBy         string     `json:"decided_by,omitempty"`
	DecidedAt         *time.Time `json:"decided_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type ImportReviewStage struct {
	ID                 string                  `json:"id"`
	ContractVersion    string                  `json:"contract_version"`
	WorkspaceKey       string                  `json:"workspace_key"`
	RunID              string                  `json:"run_id,omitempty"`
	SourceSystem       string                  `json:"source_system"`
	SourceRef          string                  `json:"source_ref"`
	StableKey          string                  `json:"stable_key"`
	ContentHash        string                  `json:"content_hash"`
	Status             string                  `json:"status"`
	CandidateCount     int                     `json:"candidate_count"`
	OpenCandidateCount int                     `json:"open_candidate_count"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
	Candidates         []ImportReviewCandidate `json:"candidates,omitempty"`
}

type ImportReviewReport struct {
	Stage              ImportReviewStage `json:"stage"`
	Ready              bool              `json:"ready"`
	SemanticReady      bool              `json:"semantic_ready"`
	SemanticProfile    string            `json:"semantic_profile,omitempty"`
	Calibration        string            `json:"calibration_revision,omitempty"`
	ExactCandidates    int               `json:"exact_candidates"`
	LexicalCandidates  int               `json:"lexical_candidates"`
	SemanticCandidates int               `json:"semantic_candidates"`
}

// StageImportCard creates an auditable review boundary before publication.
// Existing revisions are a no-op only after their import stage is cleared or
// published. A staged row is resumed so a failed candidate provider cannot
// turn into an unchecked publication.
func (p *Postgres) StageImportCard(ctx context.Context, card normalize.Card, workspaceKey, runID, actor string) (ImportReviewReport, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	if workspaceKey == "" {
		workspaceKey = "fluent-interview"
	}
	if strings.TrimSpace(actor) == "" {
		actor = "question-brain-import-review"
	}
	var workspaceID, existingHash string
	if err := p.pool.QueryRow(ctx, `
		insert into content.workspace (stable_key, display_name)
		values ($1, $2)
		on conflict (stable_key) do update set display_name = excluded.display_name
		returning id::text
	`, workspaceKey, workspaceKey).Scan(&workspaceID); err != nil {
		return ImportReviewReport{}, fmt.Errorf("resolve import review workspace: %w", err)
	}
	if err := p.pool.QueryRow(ctx, `
		select qr.content_hash
		from content.question q
		left join content.question_revision qr on qr.id = q.current_revision_id
		where q.workspace_id = $1::uuid and q.stable_key = $2
	`, workspaceID, card.StableKey).Scan(&existingHash); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ImportReviewReport{}, fmt.Errorf("read current import revision: %w", err)
	}

	var stageID string
	var stageStatus string
	if existingHash == card.Hash {
		// A previous attempt may have staged this hash but failed while the
		// candidate provider was unavailable. Resume staged work instead of
		// treating an unchecked row as an unchanged no-op.
		stageErr := p.pool.QueryRow(ctx, `
			select id::text, status from content.import_review_stage
			where workspace_id = $1::uuid and source_ref = $2 and content_hash = $3
		`, workspaceID, card.SourceRef, card.Hash).Scan(&stageID, &stageStatus)
		if stageErr == nil {
			stage, getErr := p.GetImportReviewStage(ctx, stageID)
			if getErr != nil {
				return ImportReviewReport{}, getErr
			}
			_, _, _, _, _, semanticReady, profileErr := p.duplicateProfile(ctx)
			if profileErr != nil {
				return ImportReviewReport{}, profileErr
			}
			if stage.Status == "cleared" || stage.Status == "published" {
				return ImportReviewReport{Stage: stage, Ready: true, SemanticReady: semanticReady}, nil
			}
			if stage.Status == "blocked" {
				return ImportReviewReport{Stage: stage, Ready: false, SemanticReady: semanticReady}, nil
			}
		} else if !errors.Is(stageErr, pgx.ErrNoRows) {
			return ImportReviewReport{}, fmt.Errorf("find import review stage: %w", stageErr)
		}
	}
	if stageID == "" {
		err := p.pool.QueryRow(ctx, `
			insert into content.import_review_stage
			  (workspace_id, run_id, source_system, source_ref, stable_key, content_hash, normalized_payload, status)
			values ($1::uuid, nullif($2, '')::uuid, $3, $4, $5, $6, $7::jsonb, 'staged')
			on conflict (workspace_id, source_ref, content_hash) do nothing
			returning id::text, status
		`, workspaceID, strings.TrimSpace(runID), "fluent-question-vault", card.SourceRef, card.StableKey, card.Hash, card.Payload).Scan(&stageID, &stageStatus)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return ImportReviewReport{}, fmt.Errorf("stage import card: %w", err)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			if err := p.pool.QueryRow(ctx, `
				select id::text, status from content.import_review_stage
				where workspace_id = $1::uuid and source_ref = $2 and content_hash = $3
			`, workspaceID, card.SourceRef, card.Hash).Scan(&stageID, &stageStatus); err != nil {
				return ImportReviewReport{}, fmt.Errorf("find import review stage: %w", err)
			}
		}
	}

	prompt, _, _ := normalize.EnglishFields(card)
	exactCandidates, err := p.stageExactCandidates(ctx, workspaceID, card, stageID)
	if err != nil {
		return ImportReviewReport{}, err
	}
	profileKey, calibration, lexicalThreshold, semanticThreshold, maxCandidates, semanticReady, err := p.duplicateProfile(ctx)
	if err != nil {
		return ImportReviewReport{}, err
	}
	lexicalCandidates, err := p.stageLexicalCandidates(ctx, workspaceID, card, stageID, prompt, lexicalThreshold, maxCandidates)
	if err != nil {
		return ImportReviewReport{}, err
	}
	semanticCandidates := 0
	if semanticReady {
		vector, embedErr := p.embedder.Embed(ctx, prompt)
		if embedErr != nil {
			return ImportReviewReport{}, fmt.Errorf("generate semantic import candidates: %w", embedErr)
		}
		semanticCandidates, err = p.stageSemanticCandidates(ctx, workspaceID, card, stageID, embedding.VectorLiteral(vector), profileKey, semanticThreshold, maxCandidates, calibration)
		if err != nil {
			return ImportReviewReport{}, err
		}
	}
	if err := p.refreshImportStageStatus(ctx, stageID); err != nil {
		return ImportReviewReport{}, err
	}
	stage, err := p.GetImportReviewStage(ctx, stageID)
	if err != nil {
		return ImportReviewReport{}, err
	}
	if _, err := p.pool.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		values ($1::uuid, 'import_review_stage', $2::uuid, 'import.review.staged', $3::text,
		  jsonb_build_object('candidate_count', $4::int, 'semantic_ready', $5::boolean, 'calibration_revision', $6::text))
		on conflict do nothing
	`, workspaceID, stageID, actor, len(stage.Candidates), semanticReady, calibration); err != nil {
		return ImportReviewReport{}, fmt.Errorf("audit import review stage: %w", err)
	}
	return ImportReviewReport{Stage: stage, Ready: stage.Status == "cleared", SemanticReady: semanticReady, SemanticProfile: profileKey, Calibration: calibration, ExactCandidates: exactCandidates, LexicalCandidates: lexicalCandidates, SemanticCandidates: semanticCandidates}, nil
}

func (p *Postgres) stageExactCandidates(ctx context.Context, workspaceID string, card normalize.Card, stageID string) (int, error) {
	rows, err := p.pool.Query(ctx, `
		select qr.id::text
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		where q.workspace_id = $1::uuid and q.status = 'published'
		  and q.content_kind = 'production' and q.stable_key <> $2
		  and (qr.normalized_payload - 'stable_key' - 'slug') = ($3::jsonb - 'stable_key' - 'slug')
	`, workspaceID, card.StableKey, card.Payload)
	if err != nil {
		return 0, fmt.Errorf("query exact import candidates: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var revisionID string
		if err := rows.Scan(&revisionID); err != nil {
			return 0, err
		}
		if err := p.insertImportCandidate(ctx, stageID, revisionID, "exact_duplicate", 1, nil, nil, "", map[string]any{"method": "canonical_payload_without_identity"}); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func (p *Postgres) stageLexicalCandidates(ctx context.Context, workspaceID string, card normalize.Card, stageID, prompt string, threshold float64, maxCandidates int) (int, error) {
	rows, err := p.pool.Query(ctx, `
		select qr.id::text, similarity(ql.prompt, $2)::double precision
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		join content.question_locale ql on ql.revision_id = qr.id and ql.locale = 'en'
		where q.workspace_id = $1::uuid and q.status = 'published'
		  and q.content_kind = 'production' and q.stable_key <> $3
		  and not exists (
		    select 1 from content.import_review_candidate existing
		    where existing.stage_id = $4::uuid
		      and existing.related_revision_id = qr.id
		      and existing.candidate_type = 'exact_duplicate'
		  )
		  and similarity(ql.prompt, $2) >= $5
		order by similarity(ql.prompt, $2) desc, q.stable_key
		limit $6
	`, workspaceID, prompt, card.StableKey, stageID, threshold, maxCandidates)
	if err != nil {
		return 0, fmt.Errorf("query lexical import candidates: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var revisionID string
		var score float64
		if err := rows.Scan(&revisionID, &score); err != nil {
			return 0, err
		}
		if err := p.insertImportCandidate(ctx, stageID, revisionID, "lexical_neighbor", 0, &score, nil, "", map[string]any{"method": "pg_trgm", "threshold": threshold}); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func (p *Postgres) stageSemanticCandidates(ctx context.Context, workspaceID string, card normalize.Card, stageID, vector, profileKey string, threshold float64, maxCandidates int, calibration string) (int, error) {
	rows, err := p.pool.Query(ctx, `
		select qr.id::text, (1 - (embedding.embedding <=> $1::vector))::double precision
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		join content.question_locale ql on ql.revision_id = qr.id and ql.locale = 'en'
		join content.question_embedding embedding on embedding.locale_id = ql.id and embedding.profile_key = $2
		where q.workspace_id = $3::uuid and q.status = 'published'
		  and q.content_kind = 'production' and q.stable_key <> $4
		  and not exists (
		    select 1 from content.import_review_candidate existing
		    where existing.stage_id = $5::uuid
		      and existing.related_revision_id = qr.id
		      and existing.candidate_type in ('exact_duplicate', 'lexical_neighbor')
		  )
		  and (1 - (embedding.embedding <=> $1::vector)) >= $6
		order by embedding.embedding <=> $1::vector, q.stable_key
		limit $7
	`, vector, profileKey, workspaceID, card.StableKey, stageID, threshold, maxCandidates)
	if err != nil {
		return 0, fmt.Errorf("query semantic import candidates: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var revisionID string
		var score float64
		if err := rows.Scan(&revisionID, &score); err != nil {
			return 0, err
		}
		if err := p.insertImportCandidate(ctx, stageID, revisionID, "semantic_neighbor", 0, nil, &score, profileKey, map[string]any{"method": "pgvector", "threshold": threshold, "calibration_revision": calibration}); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func (p *Postgres) insertImportCandidate(ctx context.Context, stageID, revisionID, candidateType string, exact float64, lexical, semantic *float64, profile string, evidence map[string]any) error {
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	var exactValue *float64
	if exact > 0 {
		exactValue = &exact
	}
	_, err = p.pool.Exec(ctx, `
		insert into content.import_review_candidate
		  (stage_id, related_revision_id, candidate_type, exact_score, lexical_score, semantic_score, embedding_profile, evidence)
		values ($1::uuid, $2::uuid, $3, $4, $5, $6, nullif($7, ''), $8::jsonb)
		on conflict (stage_id, related_revision_id, candidate_type) do update set
		  exact_score = coalesce(excluded.exact_score, content.import_review_candidate.exact_score),
		  lexical_score = coalesce(excluded.lexical_score, content.import_review_candidate.lexical_score),
		  semantic_score = coalesce(excluded.semantic_score, content.import_review_candidate.semantic_score),
		  embedding_profile = coalesce(excluded.embedding_profile, content.import_review_candidate.embedding_profile),
		  evidence = excluded.evidence
	`, stageID, revisionID, candidateType, exactValue, lexical, semantic, profile, encoded)
	if err != nil {
		return fmt.Errorf("insert import review candidate: %w", err)
	}
	return nil
}

func (p *Postgres) duplicateProfile(ctx context.Context) (profile, calibration string, lexical, semantic float64, maxCandidates int, ready bool, err error) {
	err = p.pool.QueryRow(ctx, `
		select config.profile_key, config.calibration_revision, config.lexical_threshold::double precision,
		  config.semantic_threshold::double precision, config.max_candidates, embedding.active
		from content.duplicate_profile_config config
		join content.embedding_profile embedding on embedding.profile_key = config.profile_key
		where config.profile_key = $1
	`, p.embeddingProfile).Scan(&profile, &calibration, &lexical, &semantic, &maxCandidates, &ready)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", 0.55, 0.80, 25, false, nil
	}
	if err != nil {
		return "", "", 0, 0, 0, false, fmt.Errorf("read duplicate profile config: %w", err)
	}
	return profile, calibration, lexical, semantic, maxCandidates, ready, nil
}

func (p *Postgres) refreshImportStageStatus(ctx context.Context, stageID string) error {
	_, err := p.pool.Exec(ctx, `
		update content.import_review_stage stage
		set status = case
		  when exists (select 1 from content.import_review_candidate candidate where candidate.stage_id = stage.id and candidate.decision = 'merge') then 'blocked'
		  when exists (select 1 from content.import_review_candidate candidate where candidate.stage_id = stage.id and candidate.decision = 'open') then 'blocked'
		  else 'cleared' end,
		updated_at = now()
		where stage.id = $1::uuid and stage.status not in ('published', 'discarded')
	`, stageID)
	if err != nil {
		return fmt.Errorf("refresh import review stage: %w", err)
	}
	return nil
}

func (p *Postgres) GetImportReviewStage(ctx context.Context, stageID string) (ImportReviewStage, error) {
	var stage ImportReviewStage
	stage.ContractVersion = ImportReviewContractVersion
	var runID *string
	if err := p.pool.QueryRow(ctx, `
		select stage.id::text, workspace.stable_key, stage.run_id::text, stage.source_system,
		  stage.source_ref, stage.stable_key, stage.content_hash, stage.status,
		  stage.created_at, stage.updated_at,
		  count(candidate.id)::int, count(candidate.id) filter (where candidate.decision = 'open')::int
		from content.import_review_stage stage
		join content.workspace workspace on workspace.id = stage.workspace_id
		left join content.import_review_candidate candidate on candidate.stage_id = stage.id
		where stage.id = $1::uuid
		group by stage.id, workspace.stable_key
	`, stageID).Scan(&stage.ID, &stage.WorkspaceKey, &runID, &stage.SourceSystem, &stage.SourceRef, &stage.StableKey, &stage.ContentHash, &stage.Status, &stage.CreatedAt, &stage.UpdatedAt, &stage.CandidateCount, &stage.OpenCandidateCount); err != nil {
		return ImportReviewStage{}, err
	}
	if runID != nil {
		stage.RunID = *runID
	}
	rows, err := p.pool.Query(ctx, `
		select candidate.id::text, candidate.stage_id::text, related_q.stable_key, candidate.related_revision_id::text,
		  candidate.candidate_type, candidate.exact_score::double precision, candidate.lexical_score::double precision,
		  candidate.semantic_score::double precision, coalesce(candidate.embedding_profile, ''), candidate.decision,
		  coalesce(candidate.decided_by, ''), candidate.decided_at, candidate.created_at
		from content.import_review_candidate candidate
		join content.question_revision related_revision on related_revision.id = candidate.related_revision_id
		join content.question related_q on related_q.id = related_revision.question_id
		where candidate.stage_id = $1::uuid
		order by candidate.candidate_type, candidate.related_revision_id
	`, stageID)
	if err != nil {
		return ImportReviewStage{}, err
	}
	defer rows.Close()
	stage.Candidates = make([]ImportReviewCandidate, 0, stage.CandidateCount)
	for rows.Next() {
		var candidate ImportReviewCandidate
		if err := rows.Scan(&candidate.ID, &candidate.StageID, &candidate.RelatedStableKey, &candidate.RelatedRevisionID, &candidate.CandidateType, &candidate.ExactScore, &candidate.LexicalScore, &candidate.SemanticScore, &candidate.EmbeddingProfile, &candidate.Decision, &candidate.DecidedBy, &candidate.DecidedAt, &candidate.CreatedAt); err != nil {
			return ImportReviewStage{}, err
		}
		stage.Candidates = append(stage.Candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return ImportReviewStage{}, err
	}
	return stage, nil
}

func (p *Postgres) ListImportReviewStages(ctx context.Context, workspaceKey, status string) ([]ImportReviewStage, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	if workspaceKey == "" {
		workspaceKey = "fluent-interview"
	}
	rows, err := p.pool.Query(ctx, `
		select stage.id::text
		from content.import_review_stage stage
		join content.workspace workspace on workspace.id = stage.workspace_id
		where workspace.stable_key = $1 and ($2 = '' or stage.status = $2)
		order by stage.updated_at desc, stage.id
	`, workspaceKey, strings.TrimSpace(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]ImportReviewStage, 0, len(ids))
	for _, id := range ids {
		stage, err := p.GetImportReviewStage(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, stage)
	}
	return result, nil
}

func (p *Postgres) DecideImportReviewCandidate(ctx context.Context, candidateID, decision, actor, rationale string) (ImportReviewStage, error) {
	decision = strings.TrimSpace(decision)
	if decision != "not_duplicate" && decision != "keep_separate" && decision != "merge" {
		return ImportReviewStage{}, fmt.Errorf("unsupported import review decision %q", decision)
	}
	if strings.TrimSpace(actor) == "" {
		actor = "question-brain-reviewer"
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ImportReviewStage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var stageID, workspaceID string
	if err := tx.QueryRow(ctx, `
		select candidate.stage_id::text, stage.workspace_id::text
		from content.import_review_candidate candidate
		join content.import_review_stage stage on stage.id = candidate.stage_id
		where candidate.id = $1::uuid for update of candidate
	`, candidateID).Scan(&stageID, &workspaceID); err != nil {
		return ImportReviewStage{}, err
	}
	if _, err := tx.Exec(ctx, `
		update content.import_review_candidate
		set decision = $2, decided_by = $3, decided_at = now()
		where id = $1::uuid
	`, candidateID, decision, actor); err != nil {
		return ImportReviewStage{}, err
	}
	if _, err := tx.Exec(ctx, `
		update content.import_review_stage stage
		set status = case
		  when exists (select 1 from content.import_review_candidate candidate where candidate.stage_id = stage.id and candidate.decision = 'merge') then 'blocked'
		  when exists (select 1 from content.import_review_candidate candidate where candidate.stage_id = stage.id and candidate.decision = 'open') then 'blocked'
		  else 'cleared' end,
		updated_at = now()
		where stage.id = $1::uuid
	`, stageID); err != nil {
		return ImportReviewStage{}, err
	}
	metadata, _ := json.Marshal(map[string]string{"decision": decision, "rationale": strings.TrimSpace(rationale), "candidate_id": candidateID})
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		values ($1::uuid, 'import_review_candidate', $2::uuid, 'import.review.decided', $3::text, $4::jsonb)
	`, workspaceID, candidateID, actor, metadata); err != nil {
		return ImportReviewStage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ImportReviewStage{}, err
	}
	return p.GetImportReviewStage(ctx, stageID)
}

func (p *Postgres) AssertImportReviewReady(ctx context.Context, card normalize.Card, workspaceKey string) error {
	workspaceKey = strings.TrimSpace(workspaceKey)
	if workspaceKey == "" {
		workspaceKey = "fluent-interview"
	}
	var status string
	err := p.pool.QueryRow(ctx, `
		select stage.status
		from content.import_review_stage stage
		join content.workspace workspace on workspace.id = stage.workspace_id
		where workspace.stable_key = $1 and stage.source_ref = $2 and stage.content_hash = $3
	`, workspaceKey, card.SourceRef, card.Hash).Scan(&status)
	if err == nil {
		if status == "cleared" || status == "published" {
			return nil
		}
		return fmt.Errorf("import review is %s; open candidates must be resolved before publication", status)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var currentHash string
	err = p.pool.QueryRow(ctx, `
		select qr.content_hash
		from content.workspace workspace
		join content.question q on q.workspace_id = workspace.id and q.stable_key = $2
		join content.question_revision qr on qr.id = q.current_revision_id
		where workspace.stable_key = $1
	`, workspaceKey, card.StableKey).Scan(&currentHash)
	if err == nil && currentHash == card.Hash {
		return nil
	}
	return fmt.Errorf("import card has no review stage; publication is blocked")
}

func (p *Postgres) MarkImportStagePublished(ctx context.Context, card normalize.Card, workspaceKey string) error {
	_, err := p.pool.Exec(ctx, `
		update content.import_review_stage stage
		set status = 'published', updated_at = now()
		from content.workspace workspace
		where stage.workspace_id = workspace.id and workspace.stable_key = $1
		  and stage.source_ref = $2 and stage.content_hash = $3
	`, workspaceKey, card.SourceRef, card.Hash)
	return err
}

// MaterializeImportEdgeProposals turns generated neighbors into explicit,
// reviewable Question Brain graph proposals after the incoming revision has an
// immutable identity. It never accepts an edge and never writes learner data.
func (p *Postgres) MaterializeImportEdgeProposals(ctx context.Context, card normalize.Card, workspaceKey string, revisionID, actor string) error {
	if strings.TrimSpace(revisionID) == "" {
		return nil
	}
	if strings.TrimSpace(actor) == "" {
		actor = "question-brain-import-review"
	}
	_, err := p.pool.Exec(ctx, `
		insert into content.question_edge_proposal
		  (workspace_id, from_revision_id, to_revision_id, kind, status, confidence, rationale, source)
		select stage.workspace_id, $3::uuid, candidate.related_revision_id, 'related', 'proposed',
		       coalesce(candidate.semantic_score, candidate.lexical_score),
		       'Generated neighbor requires editorial review before graph release',
		       'import-review:' || candidate.candidate_type
		from content.import_review_stage stage
		join content.import_review_candidate candidate on candidate.stage_id = stage.id
		join content.workspace workspace on workspace.id = stage.workspace_id
		where workspace.stable_key = $1
		  and stage.source_ref = $2 and stage.content_hash = $4
		  and candidate.candidate_type in ('semantic_neighbor', 'lexical_neighbor')
		  and candidate.related_revision_id <> $3::uuid
		on conflict (workspace_id, from_revision_id, to_revision_id, kind) do nothing
	`, workspaceKey, card.SourceRef, revisionID, card.Hash)
	if err != nil {
		return fmt.Errorf("materialize import edge proposals: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		select stage.workspace_id, 'question_revision', $3::uuid, 'import.review.edges_proposed', $4::text,
		       jsonb_build_object('source_ref', stage.source_ref, 'content_hash', stage.content_hash)
		from content.import_review_stage stage
		join content.workspace workspace on workspace.id = stage.workspace_id
		where workspace.stable_key = $1 and stage.source_ref = $2 and stage.content_hash = $5
	`, workspaceKey, card.SourceRef, revisionID, actor, card.Hash)
	if err != nil {
		return fmt.Errorf("audit import edge proposals: %w", err)
	}
	return nil
}
