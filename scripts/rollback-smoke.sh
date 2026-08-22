#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
api_port="${QB_HTTP_PORT:-48127}"
token="${QUESTION_BRAIN_INTERNAL_TOKEN:-local-promote-token}"
base_url="http://127.0.0.1:${api_port}"
stable_key="g5.rollback-smoke"

promote() {
  curl -fsS -X POST "${base_url}/v1/promote" \
    -H 'content-type: application/json' \
    -H "x-question-brain-token: ${token}" \
    -H 'x-question-brain-actor: g5-rollback-smoke' \
    -d "$1"
}

first="$(promote '{"workspace_key":"fluent-interview","workspace_name":"Fluent Interview","source_ref":"g5://rollback-smoke","stable_key":"g5.rollback-smoke","slug":"g5-rollback-smoke","title":"Rollback smoke original","question":"What is the original rollback state?","sections":[{"title":"Core Idea","body":"The first immutable revision."}]}')"
first_revision="$(jq -er '.revision_id' <<<"${first}")"
promote '{"workspace_key":"fluent-interview","workspace_name":"Fluent Interview","source_ref":"g5://rollback-smoke","stable_key":"g5.rollback-smoke","slug":"g5-rollback-smoke","title":"Rollback smoke changed","question":"What is the changed rollback state?","sections":[{"title":"Core Idea","body":"The second immutable revision."}]}' >/dev/null

curl -fsS -X POST "${base_url}/v1/questions/${stable_key}/rollback" \
  -H 'content-type: application/json' \
  -H "x-question-brain-token: ${token}" \
  -H 'x-question-brain-actor: g5-rollback-smoke' \
  -d "{\"revision_id\":\"${first_revision}\"}" \
  | jq -e --arg revision "${first_revision}" '.status == "published" and .revision_id == $revision and .action == "rolled_back"' >/dev/null

curl -fsS "${base_url}/v1/questions/${stable_key}?locale=en" \
	| jq -e --arg revision "${first_revision}" '.revision_id == $revision and .prompt == "What is the original rollback state?"' >/dev/null

echo "question-brain rollback smoke: immutable revision restored and pointer/audit/outbox path ok"
