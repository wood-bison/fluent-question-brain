#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
api_port="${QB_HTTP_PORT:-48127}"
token="${QUESTION_BRAIN_INTERNAL_TOKEN:-local-promote-token}"

"${compose[@]}" up -d --build api postgres >/dev/null
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${api_port}/health/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
curl -fsS "http://127.0.0.1:${api_port}/health/ready" >/dev/null

"${compose[@]}" exec -T postgres psql -U question_brain -d question_brain -v ON_ERROR_STOP=1 \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'import_review_stage';" \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'import_review_candidate';" \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'duplicate_profile_config';" \
  -c "select 1 from pg_trigger where tgname = 'import_review_candidate_workspace';" \
  -c 'do $fn$ begin if exists (select 1 from content.import_review_stage stage join content.import_review_candidate candidate on candidate.stage_id = stage.id where stage.status in ('"'"'cleared'"'"', '"'"'published'"'"') and candidate.decision = '"'"'open'"'"') then raise exception using message = $msg$open import candidate leaked into a releasable stage$msg$; end if; end $fn$;' >/dev/null

curl -fsS "http://127.0.0.1:${api_port}/v1/import/review?workspace=fluent-interview" |
  jq -e '.contract_version == "question-brain.import-review.v1" and (.stages | type == "array")' >/dev/null

if curl -sS -o /dev/null -w '%{http_code}' -X POST \
  "http://127.0.0.1:${api_port}/v1/import/review/candidates/00000000-0000-0000-0000-000000000000/decision" \
  -H 'content-type: application/json' -d '{"decision":"not_duplicate"}' | grep -qx '401'; then
  :
else
  echo "import review smoke: unauthenticated decision was not rejected" >&2
  exit 1
fi

# Every generated graph proposal remains proposed until an editorial actor
# explicitly decides it; this query is intentionally read-only.
"${compose[@]}" exec -T postgres psql -U question_brain -d question_brain -v ON_ERROR_STOP=1 \
  -c "do \$fn\$ begin if exists (select 1 from content.question_edge_proposal where source like 'import-review:%' and status <> 'proposed') then raise exception using message = \$msg\$import-generated edge was auto-accepted\$msg\$; end if; end \$fn\$;" >/dev/null

echo "question-brain import review smoke: staging, API auth, releasable-stage guard, and proposal lifecycle ok"
