#!/usr/bin/env bash
set -euo pipefail

# Compose initdb only evaluates migrations for a fresh PostgreSQL volume. This
# explicit, idempotent upgrade applies the W07 graph evidence guards to an
# existing Question Brain volume without rewriting graph or learner data.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")

"${compose[@]}" up -d postgres >/dev/null
"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U question_brain -d question_brain \
  < "${repo_root}/db/migrations/0021_question_graph_evidence_guards.sql"

echo "question-brain graph evidence migration: applied 0021_question_graph_evidence_guards.sql"
