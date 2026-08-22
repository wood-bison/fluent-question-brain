package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wood-bison/fluent-question-brain/internal/search"
)

// GraphPlacementRequest is the explicit operator boundary for turning the
// deterministic source-topic proposals into a released question graph. A
// normal import never calls this path, so a source snapshot can be reconciled
// repeatedly without silently publishing relationships.
type GraphPlacementRequest struct {
	WorkspaceKey string
	Actor        string
	Approve      bool
}

// GraphPlacementReport is intentionally answer-free. It is safe to save in a
// release log and contains enough counts to prove what the command inspected
// and whether it actually changed the graph.
type GraphPlacementReport struct {
	ContractVersion    string    `json:"contract_version"`
	WorkspaceKey       string    `json:"workspace_key"`
	QuestionReleaseID  string    `json:"question_release_id"`
	GraphReleaseID     string    `json:"graph_release_id"`
	GeneratedAt        time.Time `json:"generated_at"`
	Approved           bool      `json:"approved"`
	Blocked            bool      `json:"blocked"`
	EligibleQuestions  int       `json:"eligible_questions"`
	ProposedPlacements int       `json:"proposed_placements"`
	AcceptedPlacements int       `json:"accepted_placements"`
	ReleasedTopics     int       `json:"released_topics"`
	MaterializedTopics int       `json:"materialized_topics"`
	InvalidPlacements  int       `json:"invalid_placements"`
	MissingPlacements  int       `json:"missing_placements"`
	BlockedReasons     []string  `json:"blocked_reasons,omitempty"`
}

type graphPlacementRow struct {
	QuestionID string
	RevisionID string
	StableKey  string
	TopicID    string
	TopicKey   string
	Decision   string
}

// ReleaseGraph validates and, when explicitly approved, publishes the
// deterministic source-topic graph. It only considers current published
// production revisions and never accepts a stale proposal or a cross-workspace
// topic. The transaction is idempotent: a second run reports the existing
// accepted/materialized state and makes no duplicate edges or audit events.
func (p *Postgres) ReleaseGraph(ctx context.Context, request GraphPlacementRequest) (GraphPlacementReport, error) {
	workspaceKey := strings.TrimSpace(request.WorkspaceKey)
	if workspaceKey == "" {
		return GraphPlacementReport{}, fmt.Errorf("workspace key is required")
	}
	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		actor = "question-brain-graph-release"
	}
	release, err := p.Release(ctx, search.ReleaseRequest{WorkspaceKey: workspaceKey})
	if err != nil {
		return GraphPlacementReport{}, fmt.Errorf("read question release: %w", err)
	}

	var workspaceID string
	if err := p.pool.QueryRow(ctx, `
		select id::text from content.workspace where stable_key = $1
	`, workspaceKey).Scan(&workspaceID); err != nil {
		return GraphPlacementReport{}, fmt.Errorf("read graph workspace: %w", err)
	}

	rows, err := p.pool.Query(ctx, `
		select
			q.id::text,
			qr.id::text,
			q.stable_key,
			coalesce(pd.topic_id::text, ''),
			coalesce(t.stable_key, ''),
			coalesce(pd.decision, '')
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		left join content.placement_decision pd on pd.revision_id = qr.id
		left join content.topic t on t.id = pd.topic_id and t.workspace_id = q.workspace_id
		where q.workspace_id = $1::uuid
		  and q.status = 'published'
		  and q.content_kind = 'production'
		order by q.stable_key, pd.created_at, pd.id
	`, workspaceID)
	if err != nil {
		return GraphPlacementReport{}, fmt.Errorf("query graph placement proposals: %w", err)
	}
	defer rows.Close()

	byQuestion := make(map[string][]graphPlacementRow)
	for rows.Next() {
		var row graphPlacementRow
		if err := rows.Scan(&row.QuestionID, &row.RevisionID, &row.StableKey, &row.TopicID, &row.TopicKey, &row.Decision); err != nil {
			return GraphPlacementReport{}, fmt.Errorf("scan graph placement proposal: %w", err)
		}
		byQuestion[row.QuestionID] = append(byQuestion[row.QuestionID], row)
	}
	if err := rows.Err(); err != nil {
		return GraphPlacementReport{}, fmt.Errorf("iterate graph placement proposals: %w", err)
	}

	report := GraphPlacementReport{
		ContractVersion:   "question-brain.graph-placement.v1",
		WorkspaceKey:      workspaceKey,
		QuestionReleaseID: release.ReleaseID,
		GeneratedAt:       time.Now().UTC(),
		Approved:          request.Approve,
		EligibleQuestions: len(byQuestion),
		BlockedReasons:    make([]string, 0),
	}
	pairs := make([]string, 0, len(byQuestion))
	for _, items := range byQuestion {
		if len(items) != 1 {
			report.MissingPlacements++
			if len(items) == 0 {
				report.BlockedReasons = append(report.BlockedReasons, "published production question has no placement proposal")
			} else {
				report.InvalidPlacements++
				report.BlockedReasons = append(report.BlockedReasons, "published production question has multiple placement proposals")
			}
			continue
		}
		row := items[0]
		if row.TopicID == "" || row.TopicKey == "" {
			report.InvalidPlacements++
			report.BlockedReasons = append(report.BlockedReasons, "placement proposal references a missing or cross-workspace topic")
			continue
		}
		switch row.Decision {
		case "proposed":
			report.ProposedPlacements++
		case "accepted":
			report.AcceptedPlacements++
		default:
			report.InvalidPlacements++
			report.BlockedReasons = append(report.BlockedReasons, "placement proposal is not proposed or accepted")
			continue
		}
		pairs = append(pairs, row.StableKey+"="+row.TopicKey)
	}
	sort.Strings(pairs)
	report.GraphReleaseID = graphReleaseID(release.ReleaseID, pairs)
	report.ReleasedTopics = report.AcceptedPlacements + report.ProposedPlacements
	report.Blocked = report.MissingPlacements > 0 || report.InvalidPlacements > 0 || report.EligibleQuestions != release.Checks.Published
	if report.EligibleQuestions != release.Checks.Published {
		report.BlockedReasons = append(report.BlockedReasons, "placement inspection did not cover every published production question")
	}
	sort.Strings(report.BlockedReasons)
	if report.Blocked || !request.Approve {
		return report, nil
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return GraphPlacementReport{}, fmt.Errorf("begin graph release transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `
		with eligible as (
			select distinct on (q.id)
				q.id as question_id,
				pd.topic_id
			from content.question q
			join content.question_revision qr on qr.id = q.current_revision_id
			join content.placement_decision pd on pd.revision_id = qr.id
			join content.topic t on t.id = pd.topic_id and t.workspace_id = q.workspace_id
			where q.workspace_id = $1::uuid
			  and q.status = 'published'
			  and q.content_kind = 'production'
			  and pd.decision in ('proposed', 'accepted')
			order by q.id, pd.created_at, pd.id
		)
		insert into content.question_topic (question_id, topic_id, relation)
		select question_id, topic_id, 'primary' from eligible
		on conflict (question_id, topic_id) do nothing
	`, workspaceID)
	if err != nil {
		return GraphPlacementReport{}, fmt.Errorf("materialize question topics: %w", err)
	}
	// Count the final materialized set for an accurate report; the command tag is
	// useful to callers inspecting a dry-run versus a repeat release.
	_ = result.RowsAffected()
	if err := tx.QueryRow(ctx, `
		select count(*)
		from content.question_topic qt
		join content.question q on q.id = qt.question_id
		where q.workspace_id = $1::uuid and q.status = 'published' and q.content_kind = 'production'
	`, workspaceID).Scan(&report.MaterializedTopics); err != nil {
		return GraphPlacementReport{}, fmt.Errorf("count materialized question topics: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		update content.placement_decision pd
		set decision = 'accepted', decided_by = $2::text, decided_at = now(),
			evidence = pd.evidence || jsonb_build_object(
				'accepted_by', $2::text,
				'method', 'source-topic-v1',
				'graph_release_id', $3::text
			)
		from content.question_revision qr
		join content.question q on q.id = qr.question_id
		where pd.revision_id = qr.id
		  and q.workspace_id = $1::uuid
		  and q.status = 'published'
		  and q.content_kind = 'production'
		  and qr.id = q.current_revision_id
		  and pd.decision = 'proposed'
	`, workspaceID, actor, report.GraphReleaseID); err != nil {
		return GraphPlacementReport{}, fmt.Errorf("accept graph placement decisions: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"actor":            actor,
		"graph_release_id": report.GraphReleaseID,
		"question_release": report.QuestionReleaseID,
		"accepted":         report.ProposedPlacements,
		"materialized":     report.MaterializedTopics,
		"method":           "source-topic-v1",
	})
	if err != nil {
		return GraphPlacementReport{}, fmt.Errorf("encode graph release audit: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		select $1::uuid, 'question_graph', $1::uuid, 'question.graph.placements.accepted', $2, $3::jsonb
		where not exists (
			select 1 from content.audit_event
			where workspace_id = $1::uuid
			  and aggregate_type = 'question_graph'
			  and event_type = 'question.graph.placements.accepted'
			  and metadata->>'graph_release_id' = $3::jsonb->>'graph_release_id'
		)
	`, workspaceID, actor, metadata); err != nil {
		return GraphPlacementReport{}, fmt.Errorf("write graph release audit: %w", err)
	}
	outboxPayload, err := json.Marshal(map[string]string{
		"graph_release_id": report.GraphReleaseID,
		"question_release": report.QuestionReleaseID,
		"workspace":        workspaceKey,
	})
	if err != nil {
		return GraphPlacementReport{}, fmt.Errorf("encode graph release outbox: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.outbox_event (aggregate_type, aggregate_id, event_type, idempotency_key, payload)
		values ('question_graph', $1::uuid, 'question.graph.released', $2, $3::jsonb)
		on conflict (idempotency_key) do nothing
	`, workspaceID, "question-graph-release:"+report.GraphReleaseID, outboxPayload); err != nil {
		return GraphPlacementReport{}, fmt.Errorf("write graph release outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return GraphPlacementReport{}, fmt.Errorf("commit graph release: %w", err)
	}
	report.AcceptedPlacements += report.ProposedPlacements
	report.ProposedPlacements = 0
	report.ReleasedTopics = report.MaterializedTopics
	return report, nil
}

func graphReleaseID(questionReleaseID string, pairs []string) string {
	hash := sha256.Sum256([]byte(questionReleaseID + "|" + strings.Join(pairs, "|")))
	return "question-graph-release-" + hex.EncodeToString(hash[:8])
}
