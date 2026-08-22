#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test -s "${repo_root}/db/migrations/0001_question_brain.sql"
test -s "${repo_root}/deploy/compose/compose.yaml"
test -s "${repo_root}/docs/contracts/question-revision.md"
grep -q "create extension if not exists vector" "${repo_root}/db/migrations/0001_question_brain.sql"
grep -q "create table if not exists content.outbox_event" "${repo_root}/db/migrations/0001_question_brain.sql"
grep -q "create trigger question_locale_search_document" "${repo_root}/db/migrations/0001_question_brain.sql"
grep -q "name: fluent-question-brain" "${repo_root}/deploy/compose/compose.yaml"

if command -v docker >/dev/null 2>&1; then
  docker compose -f "${repo_root}/deploy/compose/compose.yaml" config >/dev/null
fi

echo "question-brain contract: ok"
