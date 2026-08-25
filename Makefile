.PHONY: check test go-check contract smoke g4-smoke g5-smoke graph-smoke import-review-smoke g6-batch-smoke calibration-smoke capability-binding-smoke compose-up compose-down

check: contract test

smoke:
	bash scripts/compose-smoke.sh

g4-smoke:
	bash scripts/g4-smoke.sh

g5-smoke:
	bash scripts/g5-smoke.sh

graph-smoke:
	bash scripts/graph-smoke.sh

import-review-smoke:
	bash scripts/import-review-smoke.sh

g6-batch-smoke:
	bash scripts/g6-batch-smoke.sh

calibration-smoke:
	bash scripts/calibration-smoke.sh

capability-binding-smoke:
	bash scripts/capability-binding-smoke.sh

contract:
	bash scripts/check-contract.sh

test:
	bash scripts/go-check.sh

go-check: test

compose-up:
	docker compose -p fluent-question-brain -f deploy/compose/compose.yaml up --build

compose-down:
	docker compose -p fluent-question-brain -f deploy/compose/compose.yaml down
