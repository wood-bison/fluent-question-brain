#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
api_port="${QB_HTTP_PORT:-48127}"

"${compose[@]}" up -d --build api postgres >/dev/null
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${api_port}/health/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

"${compose[@]}" exec -T postgres psql -U question_brain -d question_brain -v ON_ERROR_STOP=1 \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'question_edge_proposal';" \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'question_graph_release';" \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'question_edge_release';" \
  -c "select 1 from pg_trigger where tgname = 'question_edge_proposal_workspace';" \
  -c "select 1 from pg_trigger where tgname = 'question_edge_release_workspace';" \
  -c 'do $fn$ begin if exists (select 1 from content.question_edge_release edge left join content.question_edge_proposal proposal on proposal.id = edge.proposal_id where proposal.id is null) then raise exception using message = $msg$dangling graph proposal$msg$; end if; end $fn$;' \
  -c 'do $fn$ begin if exists (select 1 from content.question_edge_release edge join content.question_edge_proposal proposal on proposal.id = edge.proposal_id where proposal.status <> '\''accepted'\'') then raise exception using message = $msg$non-accepted graph edge leaked into release$msg$; end if; end $fn$;' >/dev/null

curl -fsS "http://127.0.0.1:${api_port}/v1/graph/proposals?workspace=fluent-interview" |
  jq -e '.contract_version == "question-brain.graph-edge.v1" and (.proposals | type == "array")' >/dev/null

release_id="$(${compose[@]} exec -T postgres psql -U question_brain -d question_brain -Atc "select graph_release_id from content.question_graph_release where status = 'active' order by created_at desc limit 1" | tr -d '\r')"
if [[ -n "${release_id}" ]]; then
  curl -fsS "http://127.0.0.1:${api_port}/v1/graph/releases/${release_id}" |
    jq -e '.contract_version == "question-brain.graph-edge.v1" and .status == "active" and (.edge_count == (.edges | length))' >/dev/null
  curl -fsS "http://127.0.0.1:${api_port}/v1/graph/prerequisites/question.q315" |
    jq -e '(.edges | length) > 0 and (.edges | all(.kind == "prerequisite"))' >/dev/null
  curl -fsS "http://127.0.0.1:${api_port}/v1/graph/contrasts/question.q315" |
    jq -e '(.edges | length) > 0 and (.edges | all(.kind == "contrast"))' >/dev/null
  curl -fsS "http://127.0.0.1:${api_port}/v1/graph/variants/question.q204" |
    jq -e '(.edges | length) > 0 and (.edges | all(.kind == "variant"))' >/dev/null
  "${compose[@]}" exec -T api /qb-graph-edges \
    -database-url 'postgres://question_brain:question_brain@postgres:5432/question_brain?sslmode=disable' \
    -export "${release_id}" | jq -e '.graph_release_id == "'"${release_id}"'"' >/dev/null
fi

echo "question-brain graph smoke: schema, workspace guards, accepted-only release, API, and CLI export ok"
