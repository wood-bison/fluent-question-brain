.PHONY: check test contract smoke g4-smoke compose-up compose-down

check: contract test

smoke:
	bash scripts/compose-smoke.sh

g4-smoke:
	bash scripts/g4-smoke.sh

contract:
	bash scripts/check-contract.sh

test:
	@if command -v go >/dev/null 2>&1; then go test ./...; else echo "go toolchain not installed locally; Docker CI will run go test ./..."; fi

compose-up:
	docker compose -p fluent-question-brain -f deploy/compose/compose.yaml up --build

compose-down:
	docker compose -p fluent-question-brain -f deploy/compose/compose.yaml down
