#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
api_port="${QB_HTTP_PORT:-48127}"

"${compose[@]}" config >/dev/null
"${compose[@]}" exec -T api /question-brain --healthcheck
"${compose[@]}" exec -T postgres psql -U question_brain -d question_brain -v ON_ERROR_STOP=1 \
  -c "select 1 from information_schema.schemata where schema_name = 'content';" \
  -c "select 1 from information_schema.schemata where schema_name = 'cms';" \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'question_curriculum_mapping';" \
  -c "select 1 from content.taxonomy_path where stable_key = 'path.python';" \
  -c "select 1 from cms.payload_migrations where name = '20260822_101040_initial';" >/dev/null
curl -fsS "http://127.0.0.1:${api_port}/health/ready" | grep -q '"migration":"compose-init"'

echo "question-brain migration smoke: compose config, Go boundary, content/cms migrations ok"
