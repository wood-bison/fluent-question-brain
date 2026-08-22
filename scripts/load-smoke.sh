#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
api_port="${QB_HTTP_PORT:-48127}"
base_url="http://127.0.0.1:${api_port}"
samples=60
latencies="$(mktemp -t question-brain-load.XXXXXX)"
trap 'rm -f "${latencies}"' EXIT

for i in $(seq 1 "${samples}"); do
  if (( i % 2 == 0 )); then
    curl -fsS -o /dev/null -w '%{time_total}\n' "${base_url}/health/live" >>"${latencies}"
  else
    curl -fsS -o /dev/null -w '%{time_total}\n' \
      -X POST "${base_url}/v1/search" \
      -H 'content-type: application/json' \
      -d '{"query":"event loop","locale":"en","limit":5}' >>"${latencies}"
  fi
done

p95_ms="$(sort -n "${latencies}" | awk -v n="${samples}" 'NR == int(n * 0.95 + 0.999999) { printf "%.2f", $1 * 1000; exit }')"
if [[ -z "${p95_ms}" ]]; then
  echo "question-brain load smoke: no latency samples" >&2
  exit 1
fi
awk -v p95="${p95_ms}" 'BEGIN { exit !(p95 < 1000) }' || {
  echo "question-brain load smoke: p95 ${p95_ms}ms exceeds 1000ms" >&2
  exit 1
}

echo "question-brain load smoke: requests=${samples} errors=0 p95_ms=${p95_ms}"
