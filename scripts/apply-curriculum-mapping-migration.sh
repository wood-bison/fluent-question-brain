#!/usr/bin/env bash
set -euo pipefail

# The compose initdb directory is only evaluated for a fresh PostgreSQL
# volume. Run this explicit, idempotent upgrade path once for an existing
# Question Brain volume before rebuilding the API that reads the new table.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")

"${compose[@]}" up -d postgres >/dev/null
"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U question_brain -d question_brain \
  < "${repo_root}/db/migrations/0012_curriculum_mapping_release.sql"

"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U question_brain -d question_brain \
  < "${repo_root}/db/migrations/0013_runtime_station_capabilities.sql"

"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U question_brain -d question_brain \
  < "${repo_root}/db/migrations/0014_python_path.sql"

echo "question-brain curriculum mapping migration: applied 0012_curriculum_mapping_release.sql"
echo "question-brain curriculum mapping migration: applied 0013_runtime_station_capabilities.sql"
echo "question-brain curriculum mapping migration: applied 0014_python_path.sql"
