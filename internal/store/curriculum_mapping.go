package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wood-bison/fluent-question-brain/internal/mapping"
	"github.com/wood-bison/fluent-question-brain/internal/search"
	"github.com/wood-bison/fluent-question-brain/internal/taxonomy"
)

// CurriculumMappingReleaseRequest is the write boundary for the explicit Lab
// crosswalk. Manifest rows are the only source of curriculum keys. When
// UnmappedCurrent is set, the command records an explicit no-inference audit
// row for every current production revision instead of manufacturing keys.
type CurriculumMappingReleaseRequest struct {
	WorkspaceKey    string
	Manifest        *mapping.Manifest
	UnmappedCurrent bool
	Actor           string
	Approve         bool
}

type CurriculumMappingReleaseReport struct {
	ContractVersion    string    `json:"contract_version"`
	TaxonomyVersion    string    `json:"taxonomy_version"`
	WorkspaceKey       string    `json:"workspace_key"`
	QuestionReleaseID  string    `json:"question_release_id"`
	MappingReleaseID   string    `json:"mapping_release_id"`
	GeneratedAt        time.Time `json:"generated_at"`
	Approved           bool      `json:"approved"`
	Blocked            bool      `json:"blocked"`
	Published          int       `json:"published"`
	ManifestEntries    int       `json:"manifest_entries"`
	Covered            int       `json:"covered"`
	Mapped             int       `json:"mapped"`
	Unmapped           int       `json:"unmapped"`
	Proposed           int       `json:"proposed"`
	Accepted           int       `json:"accepted"`
	Rejected           int       `json:"rejected"`
	CapabilityMappings int       `json:"capability_mappings"`
	Invalid            int       `json:"invalid"`
	MissingManifest    int       `json:"missing_manifest"`
	ExtraManifest      int       `json:"extra_manifest"`
	Changed            int       `json:"changed"`
	Unchanged          int       `json:"unchanged"`
	BlockedReasons     []string  `json:"blocked_reasons,omitempty"`
}

type currentCurriculumRevision struct {
	StableKey  string
	RevisionID string
	Hash       string
}

// ReleaseCurriculumMapping validates and optionally materializes one complete
// mapping batch. It never reads normalized_payload, Track, Group, Topic, or
// graph labels, so a successful release cannot have inferred a placement from
// legacy content metadata.
func (p *Postgres) ReleaseCurriculumMapping(ctx context.Context, request CurriculumMappingReleaseRequest) (CurriculumMappingReleaseReport, error) {
	workspaceKey := strings.TrimSpace(request.WorkspaceKey)
	if workspaceKey == "" {
		return CurriculumMappingReleaseReport{}, fmt.Errorf("workspace key is required")
	}
	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		actor = "question-brain-curriculum-mapping-release"
	}
	release, err := p.Release(ctx, search.ReleaseRequest{WorkspaceKey: workspaceKey})
	if err != nil {
		return CurriculumMappingReleaseReport{}, fmt.Errorf("read question release: %w", err)
	}
	current, workspaceID, err := p.currentCurriculumRevisions(ctx, workspaceKey)
	if err != nil {
		return CurriculumMappingReleaseReport{}, err
	}

	entries, err := curriculumEntries(request, current)
	if err != nil {
		return CurriculumMappingReleaseReport{}, err
	}
	report := CurriculumMappingReleaseReport{
		ContractVersion:   mapping.ContractVersion,
		TaxonomyVersion:   taxonomy.Version,
		WorkspaceKey:      workspaceKey,
		QuestionReleaseID: release.ReleaseID,
		MappingReleaseID:  mapping.Fingerprint(workspaceKey, entries),
		GeneratedAt:       time.Now().UTC(),
		Approved:          request.Approve,
		Published:         len(current),
		ManifestEntries:   len(entries),
		BlockedReasons:    make([]string, 0),
	}

	byStableKey := make(map[string]mapping.Entry, len(entries))
	for _, entry := range entries {
		byStableKey[entry.StableKey] = entry
		if _, ok := current[entry.StableKey]; !ok {
			report.ExtraManifest++
			continue
		}
		report.Covered++
		switch entry.MappingState {
		case "unmapped":
			report.Unmapped++
		case "proposed":
			report.Proposed++
			report.Mapped++
		case "accepted":
			report.Accepted++
			report.Mapped++
		case "rejected":
			report.Rejected++
			report.Mapped++
		default:
			report.Invalid++
			report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("stable key %q has invalid mapping_state %q", entry.StableKey, entry.MappingState))
		}
		if entry.CapabilityKey != "" {
			report.CapabilityMappings++
		}
		row := current[entry.StableKey]
		if entry.RevisionID != "" && entry.RevisionID != row.RevisionID {
			report.Invalid++
			report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("stable key %q pins revision %s, current is %s", entry.StableKey, entry.RevisionID, row.RevisionID))
		}
		if entry.ContentHash != "" && entry.ContentHash != row.Hash {
			report.Invalid++
			report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("stable key %q pins content_hash %s, current is %s", entry.StableKey, entry.ContentHash, row.Hash))
		}
	}
	for stableKey := range current {
		if _, ok := byStableKey[stableKey]; !ok {
			report.MissingManifest++
		}
	}
	if report.MissingManifest > 0 {
		report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("%d current production revisions are missing from the complete mapping manifest", report.MissingManifest))
	}
	if report.ExtraManifest > 0 {
		report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("%d manifest rows are not current published production revisions", report.ExtraManifest))
	}
	if report.Invalid > 0 {
		report.BlockedReasons = append(report.BlockedReasons, "mapping manifest contains invalid revision pins or placement rows")
	}
	if report.Covered != len(current) {
		report.BlockedReasons = append(report.BlockedReasons, "mapping manifest does not cover every current production revision")
	}

	// FK constraints protect program/path/domain values. Capabilities are
	// intentionally a reviewed inventory, so report missing capability rows
	// before a write instead of turning a typo into an opaque SQL failure.
	capabilities := make([]string, 0)
	for _, entry := range entries {
		if entry.CapabilityKey != "" {
			capabilities = append(capabilities, entry.CapabilityKey)
		}
	}
	capabilities = uniqueStrings(capabilities)
	if len(capabilities) > 0 {
		known, err := p.knownCapabilities(ctx, capabilities)
		if err != nil {
			return CurriculumMappingReleaseReport{}, err
		}
		for _, capability := range capabilities {
			if _, ok := known[capability]; !ok {
				report.Invalid++
				report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("capability %q is not in the reviewed taxonomy inventory", capability))
			}
		}
	}
	sort.Strings(report.BlockedReasons)
	report.Blocked = report.MissingManifest > 0 || report.ExtraManifest > 0 || report.Invalid > 0 || report.Covered != len(current)
	if report.Blocked || !request.Approve {
		return report, nil
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CurriculumMappingReleaseReport{}, fmt.Errorf("begin curriculum mapping release transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, entry := range entries {
		row := current[entry.StableKey]
		var programKey, pathKey, domainKey, capabilityKey any
		if entry.ProgramKey != "" {
			programKey = entry.ProgramKey
		}
		if entry.PathKey != "" {
			pathKey = entry.PathKey
		}
		if entry.DomainKey != "" {
			domainKey = entry.DomainKey
		}
		if entry.CapabilityKey != "" {
			capabilityKey = entry.CapabilityKey
		}
		command, err := tx.Exec(ctx, `
			insert into content.question_curriculum_mapping
				(revision_id, program_key, path_key, domain_key, capability_key,
				 mapping_state, mapping_version, source, created_at, updated_at)
			values ($1::uuid, $2, $3, $4, $5, $6, $7, $8, now(), now())
			on conflict (revision_id) do update set
				program_key = excluded.program_key,
				path_key = excluded.path_key,
				domain_key = excluded.domain_key,
				capability_key = excluded.capability_key,
				mapping_state = excluded.mapping_state,
				mapping_version = excluded.mapping_version,
				source = excluded.source,
				updated_at = now()
			where content.question_curriculum_mapping.program_key is distinct from excluded.program_key
			   or content.question_curriculum_mapping.path_key is distinct from excluded.path_key
			   or content.question_curriculum_mapping.domain_key is distinct from excluded.domain_key
			   or content.question_curriculum_mapping.capability_key is distinct from excluded.capability_key
			   or content.question_curriculum_mapping.mapping_state is distinct from excluded.mapping_state
			   or content.question_curriculum_mapping.mapping_version is distinct from excluded.mapping_version
			   or content.question_curriculum_mapping.source is distinct from excluded.source
		`, row.RevisionID, programKey, pathKey, domainKey, capabilityKey,
			entry.MappingState, taxonomy.Version, entry.Source)
		if err != nil {
			return CurriculumMappingReleaseReport{}, fmt.Errorf("materialize mapping for %s: %w", entry.StableKey, err)
		}
		if command.RowsAffected() == 0 {
			report.Unchanged++
		} else {
			report.Changed++
		}
	}
	metadata, err := json.Marshal(map[string]any{
		"actor":              actor,
		"mapping_release_id": report.MappingReleaseID,
		"question_release":   report.QuestionReleaseID,
		"manifest_entries":   report.ManifestEntries,
		"mapped":             report.Mapped,
		"unmapped":           report.Unmapped,
		"proposed":           report.Proposed,
		"accepted":           report.Accepted,
		"rejected":           report.Rejected,
		"method":             "explicit-curriculum-manifest-v1",
	})
	if err != nil {
		return CurriculumMappingReleaseReport{}, fmt.Errorf("encode curriculum mapping audit: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		select $1::uuid, 'question_curriculum_mapping', $1::uuid,
			'question.curriculum.mapping.released', $2, $3::jsonb
		where not exists (
			select 1 from content.audit_event
			where workspace_id = $1::uuid
			  and aggregate_type = 'question_curriculum_mapping'
			  and event_type = 'question.curriculum.mapping.released'
			  and metadata->>'mapping_release_id' = $3::jsonb->>'mapping_release_id'
		)
	`, workspaceID, actor, metadata); err != nil {
		return CurriculumMappingReleaseReport{}, fmt.Errorf("write curriculum mapping audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return CurriculumMappingReleaseReport{}, fmt.Errorf("commit curriculum mapping release: %w", err)
	}
	return report, nil
}

func curriculumEntries(request CurriculumMappingReleaseRequest, current map[string]currentCurriculumRevision) ([]mapping.Entry, error) {
	if request.UnmappedCurrent {
		if request.Manifest != nil {
			return nil, fmt.Errorf("unmapped-current cannot be combined with a mapping manifest")
		}
		entries := make([]mapping.Entry, 0, len(current))
		for stableKey, row := range current {
			entries = append(entries, mapping.Entry{
				StableKey:    stableKey,
				RevisionID:   row.RevisionID,
				ContentHash:  row.Hash,
				MappingState: "unmapped",
				Source:       "question-brain-i1-no-inference-audit",
			})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].StableKey < entries[j].StableKey })
		return entries, nil
	}
	if request.Manifest == nil {
		return nil, fmt.Errorf("mapping manifest is required unless unmapped-current is requested")
	}
	if strings.TrimSpace(request.Manifest.WorkspaceKey) != strings.TrimSpace(request.WorkspaceKey) {
		return nil, fmt.Errorf("manifest workspace_key %q does not match request workspace %q", request.Manifest.WorkspaceKey, request.WorkspaceKey)
	}
	entries, err := request.Manifest.Normalize()
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (p *Postgres) currentCurriculumRevisions(ctx context.Context, workspaceKey string) (map[string]currentCurriculumRevision, string, error) {
	var workspaceID string
	if err := p.pool.QueryRow(ctx, `select id::text from content.workspace where stable_key = $1`, workspaceKey).Scan(&workspaceID); err != nil {
		return nil, "", fmt.Errorf("read curriculum mapping workspace: %w", err)
	}
	rows, err := p.pool.Query(ctx, `
		select q.stable_key, qr.id::text, qr.content_hash
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		where q.workspace_id = $1::uuid
		  and q.status = 'published'
		  and q.content_kind = 'production'
		order by q.stable_key
	`, workspaceID)
	if err != nil {
		return nil, "", fmt.Errorf("query current curriculum revisions: %w", err)
	}
	defer rows.Close()
	result := make(map[string]currentCurriculumRevision)
	for rows.Next() {
		var row currentCurriculumRevision
		if err := rows.Scan(&row.StableKey, &row.RevisionID, &row.Hash); err != nil {
			return nil, "", fmt.Errorf("scan current curriculum revision: %w", err)
		}
		result[row.StableKey] = row
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate current curriculum revisions: %w", err)
	}
	return result, workspaceID, nil
}

func (p *Postgres) knownCapabilities(ctx context.Context, keys []string) (map[string]struct{}, error) {
	rows, err := p.pool.Query(ctx, `
		select stable_key
		from content.taxonomy_capability
		where stable_key = any($1::text[])
	`, keys)
	if err != nil {
		return nil, fmt.Errorf("query reviewed taxonomy capabilities: %w", err)
	}
	defer rows.Close()
	result := make(map[string]struct{}, len(keys))
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan reviewed taxonomy capability: %w", err)
		}
		result[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reviewed taxonomy capabilities: %w", err)
	}
	return result, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
