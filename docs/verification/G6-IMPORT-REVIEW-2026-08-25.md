# G6 import review evidence — 2026-08-25

Status: **implementation slice verified; mandatory +500 acceptance remains open**.

## What is now implemented

- Migration `0017_import_review_staging.sql` adds workspace-safe import stages,
  candidate rows, duplicate-profile configuration, and a workspace trigger.
- `UpsertCard` and `PublishImportedCard` enter the staging boundary before a
  revision is written; publication refuses blocked/staged rows.
- Exact canonical-payload, PostgreSQL trigram, and active-profile pgvector
  candidate generation store scores and method evidence without raw content.
- Existing `staged` rows resume after a provider failure; `cleared` and
  `published` rows are idempotent no-ops for the same source/hash.
- Neighbor candidates materialize only `related` graph proposals with
  `status=proposed` after an immutable revision ID exists. No proposal is
  accepted automatically.
- Authenticated review APIs expose the complete stage and require an explicit
  decision actor/rationale.

## Live verification

Environment: Compose project `fluent-question-brain`, API `48127`, PostgreSQL
`55437`. The 0017 migration was applied idempotently and the API readiness
endpoint returned `{"status":"ready"}`.

```text
make check                         PASS
make import-review-smoke           PASS
Docker Go tests (go test ./...)    PASS
GET /v1/import/review              PASS (v1 contract, stage array)
unauthenticated decision           PASS (401)
open candidate in cleared stage    PASS (database guard: none)
```

The deliberate `g6-rate-limiter-duplicate` fixture produced one lexical
candidate against an existing published card. An authenticated
`not_duplicate` decision cleared its stage and recorded actor/rationale. A
provider-unavailable retry stayed staged and did not publish, demonstrating the
fail-closed boundary. The live local machine currently has no Ollama listener
on `127.0.0.1:11434`, so semantic candidates were intentionally not claimed as
passing in this evidence.

## Remaining acceptance work

The plan's mandatory batch remains unchecked until a reproducible 500-card
fixture runs with a live `semantic-v1` provider and records:

1. exact duplicates blocked;
2. semantic duplicates entering review;
3. related/new cards remaining separate;
4. malformed cards rejected;
5. a retry producing no duplicate revisions or proposals; and
6. candidate precision/recall and bounded resource measurements.
