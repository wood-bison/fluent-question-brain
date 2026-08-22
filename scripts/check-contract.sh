#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test -s "${repo_root}/db/migrations/0001_question_brain.sql"
test -s "${repo_root}/db/migrations/0002_ingestion_runs.sql"
test -s "${repo_root}/db/migrations/0003_retrieval_profile.sql"
test -s "${repo_root}/db/migrations/0004_retrieval_indexes.sql"
test -s "${repo_root}/deploy/compose/compose.yaml"
test -s "${repo_root}/docs/contracts/question-revision.md"
grep -q "create extension if not exists vector" "${repo_root}/db/migrations/0001_question_brain.sql"
grep -q "create table if not exists content.outbox_event" "${repo_root}/db/migrations/0001_question_brain.sql"
grep -q "create trigger question_locale_search_document" "${repo_root}/db/migrations/0001_question_brain.sql"
grep -q "create table if not exists content.import_run" "${repo_root}/db/migrations/0002_ingestion_runs.sql"
grep -q "create table if not exists content.import_item" "${repo_root}/db/migrations/0002_ingestion_runs.sql"
grep -q "semantic-dev-hash-v1" "${repo_root}/db/migrations/0003_retrieval_profile.sql"
grep -q "using hnsw" "${repo_root}/db/migrations/0004_retrieval_indexes.sql"
grep -q "name: fluent-question-brain" "${repo_root}/deploy/compose/compose.yaml"
grep -q "jaegertracing/jaeger:2.20.0" "${repo_root}/deploy/compose/compose.yaml"
grep -q "OTEL_EXPORTER_OTLP_ENDPOINT: jaeger:4317" "${repo_root}/deploy/compose/compose.yaml"

ports=(
  "${QB_HTTP_PORT:-48127}"
  "${QB_PG_PORT:-55437}"
  "${QB_JAEGER_UI_PORT:-56686}"
  "${QB_OTLP_GRPC_PORT:-54317}"
  "${QB_OTLP_HTTP_PORT:-54318}"
)
unique_port_count="$(printf '%s\n' "${ports[@]}" | sort -u | wc -l | tr -d ' ')"
if [[ "${unique_port_count}" != "${#ports[@]}" ]]; then
  echo "question-brain contract: host port collision in QB_*_PORT values" >&2
  exit 1
fi

if command -v docker >/dev/null 2>&1; then
  docker compose -f "${repo_root}/deploy/compose/compose.yaml" config >/dev/null
fi

echo "question-brain contract: ok"
