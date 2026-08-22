#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
api_port="${QB_HTTP_PORT:-48127}"
jaeger_port="${QB_JAEGER_UI_PORT:-56686}"

"${compose[@]}" stop api >/dev/null
if curl -fsS --max-time 2 "http://127.0.0.1:${api_port}/health/live" >/dev/null 2>&1; then
  echo "question-brain failure smoke: API stayed reachable after stop" >&2
  exit 1
fi

"${compose[@]}" start api >/dev/null
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${api_port}/health/ready" >/dev/null 2>&1; then break; fi
  sleep 2
done
curl -fsS "http://127.0.0.1:${api_port}/health/live" >/dev/null

# Trace export is best-effort: the API remains available while the local sink
# is restarted, and the Compose restart brings the sink back to a known state.
"${compose[@]}" stop jaeger >/dev/null
curl -fsS "http://127.0.0.1:${api_port}/health/live" >/dev/null
"${compose[@]}" start jaeger >/dev/null
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${jaeger_port}/" >/dev/null 2>&1; then break; fi
  sleep 2
done
curl -fsS "http://127.0.0.1:${jaeger_port}/" >/dev/null

echo "question-brain failure injection smoke: api restart and Jaeger outage recovery ok"
