#!/usr/bin/env bash
set -euo pipefail

# Compose initdb only evaluates migrations for a new PostgreSQL volume. This
# idempotent command is the explicit upgrade boundary for an existing volume.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")

"${compose[@]}" up -d postgres >/dev/null
"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U question_brain -d question_brain \
  < "${repo_root}/db/migrations/0017_import_review_staging.sql"

echo "question-brain import review migration: applied 0017_import_review_staging.sql"
