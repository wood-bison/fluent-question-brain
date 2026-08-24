#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
api_port="${QB_HTTP_PORT:-48127}"
workspace="${G6_WORKSPACE_KEY:-g6-batch-smoke-20260825}"

"${compose[@]}" up -d --build api postgres >/dev/null
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${api_port}/health/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

docker run --rm --network host \
  -v "${repo_root}:/src" -w /src \
  -e DATABASE_URL="postgres://question_brain:question_brain@127.0.0.1:${QB_PG_PORT:-55437}/question_brain?sslmode=disable" \
  golang:1.24-bookworm sh -lc \
  'export PATH=/usr/local/go/bin:$PATH; go run ./cmd/qb-g6-batch --database-url "$DATABASE_URL" --api-url "http://127.0.0.1:'"${api_port}"'" --workspace-key "'"${workspace}"'" --count 500'

echo "question-brain G6 batch smoke: 500-card exact/semantic/malformed/idempotent fixture passed"
