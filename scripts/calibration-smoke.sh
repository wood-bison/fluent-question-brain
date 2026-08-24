#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
database_url="postgres://question_brain:question_brain@127.0.0.1:${QB_PG_PORT:-55437}/question_brain?sslmode=disable"

result="$(docker run --rm --network host \
  -v "${repo_root}:/src" -w /src \
  golang:1.24-bookworm sh -lc \
  'export PATH=/usr/local/go/bin:$PATH; go run ./cmd/qb-calibrate --database-url "$0" --calibration docs/verification/G6-calibration-set-2026-08-25.json --workspace-key fluent-interview' \
  "${database_url}")"
printf '%s\n' "${result}"
printf '%s\n' "${result}" | tail -1 | jq -e '.evaluated == .cases and .skipped == 0 and (.precision >= 0) and (.recall >= 0)' >/dev/null
echo "question-brain calibration smoke: all reviewed pairs evaluated with profile-owned thresholds"
