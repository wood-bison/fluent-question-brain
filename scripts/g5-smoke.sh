#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")

"${compose[@]}" up -d --build >/dev/null
api_port="${QB_HTTP_PORT:-48127}"
for _ in $(seq 1 30); do
	if curl -fsS "http://127.0.0.1:${api_port}/health/ready" >/dev/null 2>&1; then break; fi
	sleep 2
done
bash "${repo_root}/scripts/migration-smoke.sh"
bash "${repo_root}/scripts/load-smoke.sh"
bash "${repo_root}/scripts/backup-restore-smoke.sh"
bash "${repo_root}/scripts/failure-injection-smoke.sh"
bash "${repo_root}/scripts/rollback-smoke.sh"

echo "question-brain G5 hardening smoke: ok"
