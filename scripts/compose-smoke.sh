#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
api_port="${QB_HTTP_PORT:-48127}"
jaeger_ui_port="${QB_JAEGER_UI_PORT:-56686}"

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
curl -fsS "http://127.0.0.1:${api_port}/health/live" >/dev/null
curl -fsSL "http://127.0.0.1:${jaeger_ui_port}/" >/dev/null
curl -fsS -X POST "http://127.0.0.1:${api_port}/v1/search" \
  -H 'content-type: application/json' \
  -d '{"query":"contract smoke","locale":"en","limit":1}' \
  | grep -q '"explainable":true'

# A request creates a server span; Jaeger v2 exports the service catalog over
# its health API after the OTLP batch flushes.
curl -fsS "http://127.0.0.1:${api_port}/" >/dev/null
sleep 7
curl -fsS "http://127.0.0.1:${jaeger_ui_port}/api/services" | grep -q 'question-brain-api'

echo "question-brain compose smoke: ok"
