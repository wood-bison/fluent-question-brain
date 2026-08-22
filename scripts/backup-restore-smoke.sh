#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
backup="$(mktemp -t question-brain-backup.XXXXXX.dump)"
restore_db="question_brain_restore_smoke"

cleanup() {
  "${compose[@]}" exec -T postgres psql -U question_brain -d postgres -v ON_ERROR_STOP=1 \
    -c "drop database if exists ${restore_db};" >/dev/null 2>&1 || true
  rm -f "${backup}"
}
trap cleanup EXIT

"${compose[@]}" exec -T postgres pg_dump -Fc -U question_brain -d question_brain >"${backup}"
"${compose[@]}" exec -T postgres psql -U question_brain -d postgres -v ON_ERROR_STOP=1 \
  -c "drop database if exists ${restore_db};" \
  -c "create database ${restore_db};" >/dev/null
"${compose[@]}" exec -T postgres pg_restore -U question_brain -d "${restore_db}" \
	--no-owner --no-privileges --exit-on-error <"${backup}" >/dev/null

count="$("${compose[@]}" exec -T postgres psql -U question_brain -d "${restore_db}" -At \
  -c "select count(*) from content.question;" | tr -d '[:space:]')"
if [[ -z "${count}" || "${count}" -lt 1 ]]; then
  echo "question-brain backup restore smoke: restored database has no questions" >&2
  exit 1
fi

echo "question-brain backup restore smoke: dump_bytes=$(wc -c <"${backup}" | tr -d ' ') restored_questions=${count}"
