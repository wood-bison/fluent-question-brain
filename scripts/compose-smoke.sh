#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")

"${compose[@]}" up -d --build >/dev/null
for _ in $(seq 1 30); do
  if "${compose[@]}" ps --format json | grep -q '"Health":"healthy"'; then
    break
  fi
  sleep 2
done

"${compose[@]}" exec -T postgres psql -U question_brain -d question_brain -v ON_ERROR_STOP=1 \
  -c "select 1 from pg_extension where extname = 'vector';" \
  -c "select count(*) from information_schema.tables where table_schema = 'content' and table_name in ('question', 'question_revision', 'question_locale', 'question_embedding', 'outbox_event');" \
  -c "select 1 from pg_trigger where tgname = 'question_locale_search_document';" >/dev/null
"${compose[@]}" exec -T api /question-brain --healthcheck

echo "question-brain compose smoke: ok"

