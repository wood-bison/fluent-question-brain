#!/usr/bin/env bash
set -euo pipefail

# Compose initdb only evaluates migrations for a new PostgreSQL volume. This
# explicit upgrade path applies the W05 editorial domain split to an existing
# workspace without touching question payloads or historical releases.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")

"${compose[@]}" up -d postgres >/dev/null
"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U question_brain -d question_brain \
  < "${repo_root}/db/migrations/0020_curriculum_domain_separation.sql"

echo "question-brain domain separation migration: applied 0020_curriculum_domain_separation.sql"
