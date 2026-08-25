#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
pg_port="${QB_PG_PORT:-55437}"
manifest_rel="docs/verification/G7-capability-binding-manifest-2026-08-25.json"
manifest_v2="/tmp/qb-g7-capability-binding-manifest-v2.json"
report_rel="docs/verification/G7-capability-binding-report-2026-08-25.json"
manifest="${repo_root}/${manifest_rel}"
report="${repo_root}/${report_rel}"

"${repo_root}/scripts/apply-capability-binding-migration.sh" >/dev/null
"${compose[@]}" up -d postgres >/dev/null

docker run --rm --network host \
  -v "${repo_root}:/src" -v /tmp:/tmp -w /src \
  golang:1.24-bookworm go run ./cmd/qb-capability-release \
  --database-url "postgres://question_brain:question_brain@127.0.0.1:${pg_port}/question_brain?sslmode=disable" \
  --workspace-key fluent-interview --registry-release capability-registry-2026-08-25-v3 \
  --generate "/src/${manifest_rel}" --stage-proposals

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

# A reviewed manifest is revision-pinned. Mutating one hash must fail closed
# before approval rather than silently rebinding a newer QuestionCard.
stale_manifest="/tmp/qb-g7-capability-binding-manifest-stale.json"
python3 - "${manifest}" "${stale_manifest}" <<'PY'
import json, sys

source, target = sys.argv[1:]
data = json.load(open(source))
for entry in data["entries"]:
    if entry["disposition"] == "bound":
        entry["content_hash"] = "f" * 64
        break
else:
    raise SystemExit("no bound entry available for stale-pin fixture")
json.dump(data, open(target, "w"), indent=2)
open(target, "a").write("\n")
PY
set +e
stale_output="$(docker run --rm --network host \
  -v "${repo_root}:/src" -v /tmp:/tmp -w /src \
  golang:1.24-bookworm go run ./cmd/qb-capability-release \
  --database-url "postgres://question_brain:question_brain@127.0.0.1:${pg_port}/question_brain?sslmode=disable" \
  --workspace-key fluent-interview --manifest "${stale_manifest}" 2>&1)"
stale_status=$?
set -e
if [[ ${stale_status} -eq 0 ]]; then
  echo "stale capability binding manifest unexpectedly approved" >&2
  exit 1
fi
printf '%s\n' "${stale_output}" | grep -q 'stale pins' || {
  echo "stale capability binding failure did not expose the typed reason" >&2
  printf '%s\n' "${stale_output}" >&2
  exit 1
}
echo "g7 stale-pin guard: blocked as expected"

docker run --rm --network host \
  -v "${repo_root}:/src" -v /tmp:/tmp -w /src \
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

docker run --rm --network host \
  -v "${repo_root}:/src" -v /tmp:/tmp -w /src \
  golang:1.24-bookworm go run ./cmd/qb-capability-release \
  --database-url "postgres://question_brain:question_brain@127.0.0.1:${pg_port}/question_brain?sslmode=disable" \
  --workspace-key fluent-interview --registry-release capability-registry-2026-08-25-v4 \
  --generate "${manifest_v2}"

docker run --rm --network host \
  -v "${repo_root}:/src" -v /tmp:/tmp -w /src \
  golang:1.24-bookworm go run ./cmd/qb-capability-release \
  --database-url "postgres://question_brain:question_brain@127.0.0.1:${pg_port}/question_brain?sslmode=disable" \
  --workspace-key fluent-interview --manifest "${manifest_v2}" --approve >/dev/null

target_release="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["binding_release_id"])' "${report}")"
rollback_json="$(docker run --rm --network host \
  -v "${repo_root}:/src" -w /src \
  golang:1.24-bookworm go run ./cmd/qb-capability-release \
  --database-url "postgres://question_brain:question_brain@127.0.0.1:${pg_port}/question_brain?sslmode=disable" \
  --workspace-key fluent-interview --rollback-release "${target_release}" --approve)"
printf '%s\n' "${rollback_json}" | python3 -c '
import json, sys
r=json.load(sys.stdin)
assert r["blocked"] is False and r["approved"] is True, r
assert r["restored_release_id"] == r["previous_release_id"] or r["restored_bindings"] > 0, r
assert r["restored_bindings"] > 0, r
print("g7 rollback:", r["restored_release_id"], "bindings", r["restored_bindings"])
'

db_counts="$(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml" exec -T postgres psql -U question_brain -d question_brain -Atc \
  "select count(*) from content.question_capability where binding_release_id is not null; select count(*) from content.question_capability_review; select count(*) from content.question_capability_binding_release_item; select count(*) from content.question_capability_binding_release where workspace_id=(select id from content.workspace where stable_key='fluent-interview') and status='active'; select count(*) from content.question_capability_binding_proposal where status='proposed';")"
printf '%s\n' "${db_counts}" | python3 -c '
import sys
values=[int(x) for x in sys.stdin.read().split()]
assert len(values) == 5 and values[0] > 0 and values[1] >= 500 and values[2] >= values[0] and values[3] == 1 and values[4] > 0, values
print("g7 projection:", "bindings", values[0], "reviews", values[1], "release_items", values[2])
print("g7 semantic candidates:", values[4])
'

echo "question-brain G7 capability binding smoke: complete reviewed disposition release passed"
