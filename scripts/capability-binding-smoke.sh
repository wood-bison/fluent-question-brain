#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
pg_port="${QB_PG_PORT:-55437}"
manifest_rel="docs/verification/G7-capability-binding-manifest-2026-08-25.json"
report_rel="docs/verification/G7-capability-binding-report-2026-08-25.json"
manifest="${repo_root}/${manifest_rel}"
report="${repo_root}/${report_rel}"

"${repo_root}/scripts/apply-capability-binding-migration.sh" >/dev/null
"${compose[@]}" up -d postgres >/dev/null

docker run --rm --network host \
  -v "${repo_root}:/src" -w /src \
  golang:1.24-bookworm go run ./cmd/qb-capability-release \
  --database-url "postgres://question_brain:question_brain@127.0.0.1:${pg_port}/question_brain?sslmode=disable" \
  --workspace-key fluent-interview --registry-release capability-registry-2026-08-24-v2 \
  --generate "/src/${manifest_rel}"

dry_run="$(docker run --rm --network host \
  -v "${repo_root}:/src" -w /src \
  golang:1.24-bookworm go run ./cmd/qb-capability-release \
  --database-url "postgres://question_brain:question_brain@127.0.0.1:${pg_port}/question_brain?sslmode=disable" \
  --workspace-key fluent-interview --manifest "/src/${manifest_rel}")"
printf '%s\n' "${dry_run}" | python3 -c '
import json, sys
r=json.load(sys.stdin)
assert r["blocked"] is False, r
assert r["manifest_entries"] >= 500, r
assert r["bound"] > 0, r
assert r["theory_only"] > 0, r
assert r["bindings"] == r["bound"], r
print("g7 dry-run:", r["manifest_entries"], "entries", r["bound"], "bound", r["theory_only"], "theory_only")
'

docker run --rm --network host \
  -v "${repo_root}:/src" -w /src \
  golang:1.24-bookworm go run ./cmd/qb-capability-release \
  --database-url "postgres://question_brain:question_brain@127.0.0.1:${pg_port}/question_brain?sslmode=disable" \
  --workspace-key fluent-interview --manifest "/src/${manifest_rel}" --approve --report "/src/${report_rel}"

python3 - "${report}" <<'PY'
import json, sys
r=json.load(open(sys.argv[1]))
assert r["blocked"] is False, r
assert r["approved"] is True, r
assert r["binding_release_id"].startswith("question-capability-release-"), r
print("g7 approved:", r["binding_release_id"], "bindings", r["bindings"])
PY

docker run --rm --network host \
  -v "${repo_root}:/src" -w /src \
  golang:1.24-bookworm go run ./cmd/qb-capability-release \
  --database-url "postgres://question_brain:question_brain@127.0.0.1:${pg_port}/question_brain?sslmode=disable" \
  --workspace-key fluent-interview --manifest "/src/${manifest_rel}" --approve

db_counts="$(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml" exec -T postgres psql -U question_brain -d question_brain -Atc \
  "select count(*) from content.question_capability where binding_release_id is not null; select count(*) from content.question_capability_review; select count(*) from content.question_capability_binding_release_item; select count(*) from content.question_capability_binding_release where workspace_id=(select id from content.workspace where stable_key='fluent-interview') and status='active';")"
printf '%s\n' "${db_counts}" | python3 -c '
import sys
values=[int(x) for x in sys.stdin.read().split()]
assert len(values) == 4 and values[0] > 0 and values[1] >= 500 and values[2] == values[0] and values[3] == 1, values
print("g7 projection:", "bindings", values[0], "reviews", values[1], "release_items", values[2])
'

echo "question-brain G7 capability binding smoke: complete reviewed disposition release passed"
