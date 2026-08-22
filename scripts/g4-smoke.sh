#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
api_port="${QB_HTTP_PORT:-48127}"
cms_port="${QB_CMS_PORT:-48128}"
token="${QUESTION_BRAIN_INTERNAL_TOKEN:-local-promote-token}"
stable_key="g4.compose-smoke"

"${compose[@]}" up -d --build api cms >/dev/null
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${cms_port}/admin" >/dev/null 2>&1 &&
    curl -fsS "http://127.0.0.1:${api_port}/health/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

"${compose[@]}" exec -T postgres psql -U question_brain -d question_brain -v ON_ERROR_STOP=1 \
  -c "select 1 from information_schema.schemata where schema_name = 'cms';" \
  -c "select 1 from information_schema.tables where table_schema = 'cms' and table_name = 'questions';" >/dev/null

payload='{"workspace_key":"fluent-interview","workspace_name":"Fluent Interview","source_ref":"payload://question/g4.compose-smoke","stable_key":"g4.compose-smoke","slug":"g4-compose-smoke","title":"Compose promote smoke","question":"What is the publish boundary?","sections":[{"title":"Core Idea","body":"Payload drafts become Go-owned revisions."},{"title":"Question (RU)","body":"Где проходит граница публикации?"},{"title":"Core Idea (RU)","body":"Черновик Payload становится ревизией Go API."}]}'
curl -fsS -X POST "http://127.0.0.1:${api_port}/v1/promote" \
  -H 'content-type: application/json' \
  -H "x-question-brain-token: ${token}" \
  -H 'x-question-brain-actor: g4-smoke' \
  -d "${payload}" | jq -e '.status == "published" and .source == "payload-cms"' >/dev/null

curl -fsS "http://127.0.0.1:${api_port}/v1/questions/${stable_key}?locale=en" |
  jq -e '.status == "published" and .locale == "en" and .prompt == "What is the publish boundary?"' >/dev/null
curl -fsS "http://127.0.0.1:${api_port}/v1/questions/${stable_key}?locale=ru" |
  jq -e '.status == "published" and .locale == "ru" and .prompt == "Где проходит граница публикации?"' >/dev/null

echo "question-brain G4 Payload promote smoke: ok"
