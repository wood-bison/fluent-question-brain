#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test -s "${repo_root}/db/migrations/0001_question_brain.sql"
test -s "${repo_root}/db/migrations/0002_ingestion_runs.sql"
test -s "${repo_root}/db/migrations/0003_retrieval_profile.sql"
test -s "${repo_root}/db/migrations/0004_retrieval_indexes.sql"
test -s "${repo_root}/db/migrations/0005_cms_schema.sql"
test -s "${repo_root}/db/migrations/0011_path_domain_capability_taxonomy.sql"
test -s "${repo_root}/db/migrations/0012_curriculum_mapping_release.sql"
test -s "${repo_root}/db/migrations/0013_runtime_station_capabilities.sql"
test -s "${repo_root}/deploy/compose/compose.yaml"
test -s "${repo_root}/docs/contracts/question-revision.md"
test -s "${repo_root}/internal/quality/quality.go"
test -s "${repo_root}/internal/mapping/manifest.go"
test -s "${repo_root}/cmd/qb-map-release/main.go"
test -s "${repo_root}/docs/contracts/fluent-engineering-lab.md"
test -s "${repo_root}/docs/contracts/question-capability-task.v1.md"
test -s "${repo_root}/docs/adr/0003-reviewed-content-relations-and-capability-bindings.md"
test -s "${repo_root}/apps/cms/src/migrations/20260824_path_taxonomy.ts"
test -s "${repo_root}/apps/cms/package-lock.json"
for script in load-smoke migration-smoke backup-restore-smoke failure-injection-smoke rollback-smoke g5-smoke apply-curriculum-mapping-migration; do
  test -x "${repo_root}/scripts/${script}.sh"
done
grep -q "create extension if not exists vector" "${repo_root}/db/migrations/0001_question_brain.sql"
grep -q "create table if not exists content.outbox_event" "${repo_root}/db/migrations/0001_question_brain.sql"
grep -q "create trigger question_locale_search_document" "${repo_root}/db/migrations/0001_question_brain.sql"
grep -q "create table if not exists content.import_run" "${repo_root}/db/migrations/0002_ingestion_runs.sql"
grep -q "create table if not exists content.import_item" "${repo_root}/db/migrations/0002_ingestion_runs.sql"
grep -q "semantic-dev-hash-v1" "${repo_root}/db/migrations/0003_retrieval_profile.sql"
grep -q "using hnsw" "${repo_root}/db/migrations/0004_retrieval_indexes.sql"
grep -q "create schema if not exists cms" "${repo_root}/db/migrations/0005_cms_schema.sql"
grep -q "create table if not exists content.taxonomy_program" "${repo_root}/db/migrations/0011_path_domain_capability_taxonomy.sql"
grep -q "create table if not exists content.question_capability" "${repo_root}/db/migrations/0011_path_domain_capability_taxonomy.sql"
grep -q "create table if not exists content.question_curriculum_mapping" "${repo_root}/db/migrations/0012_curriculum_mapping_release.sql"
grep -q "capability.runtime.node-event-loop-001" "${repo_root}/db/migrations/0013_runtime_station_capabilities.sql"
grep -q "program.backend-engineer" "${repo_root}/db/migrations/0011_path_domain_capability_taxonomy.sql"
grep -q "migration_20260824_path_taxonomy" "${repo_root}/apps/cms/src/migrations/index.ts"
grep -q "name: fluent-question-brain" "${repo_root}/deploy/compose/compose.yaml"
grep -q "jaegertracing/jaeger:2.20.0" "${repo_root}/deploy/compose/compose.yaml"
grep -q "OTEL_EXPORTER_OTLP_ENDPOINT: jaeger:4317" "${repo_root}/deploy/compose/compose.yaml"
grep -q "013_runtime_station_capabilities.sql" "${repo_root}/deploy/compose/compose.yaml"
grep -q "QB_CMS_PORT" "${repo_root}/deploy/compose/compose.yaml"
grep -q "schemaName: 'cms'" "${repo_root}/apps/cms/payload.config.ts"
grep -q 'GET /metrics' "${repo_root}/cmd/question-brain/main.go"
grep -q 'question.revision.rolled_back' "${repo_root}/internal/store/postgres.go"
grep -q 'degenerate_prompts' "${repo_root}/internal/search/types.go"
grep -q 'quality.CardIssues' "${repo_root}/cmd/qb-release/main.go"
grep -q 'quality.CardIssues' "${repo_root}/internal/ingest/importer.go"

ports=(
  "${QB_HTTP_PORT:-48127}"
  "${QB_PG_PORT:-55437}"
  "${QB_JAEGER_UI_PORT:-56686}"
  "${QB_OTLP_GRPC_PORT:-54317}"
  "${QB_OTLP_HTTP_PORT:-54318}"
  "${QB_CMS_PORT:-48128}"
)
unique_port_count="$(printf '%s\n' "${ports[@]}" | sort -u | wc -l | tr -d ' ')"
if [[ "${unique_port_count}" != "${#ports[@]}" ]]; then
  echo "question-brain contract: host port collision in QB_*_PORT values" >&2
  exit 1
fi

if command -v docker >/dev/null 2>&1; then
  docker compose -f "${repo_root}/deploy/compose/compose.yaml" config >/dev/null
fi

if command -v go >/dev/null 2>&1; then
  go test ./internal/contractmodel >/dev/null
fi

echo "question-brain contract: ok"
