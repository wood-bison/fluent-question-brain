# G6 import review evidence — 2026-08-25

Status: **implementation and mandatory +500 acceptance verified**.

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
- `cmd/qb-g6-batch` and `scripts/g6-batch-smoke.sh` provide an isolated,
  deterministic acceptance fixture. The harness uses a local test-only
  embedding server, never the learner API or a hosted model.

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

## Mandatory +500 result

The local machine currently has no Ollama listener on `127.0.0.1:11434`; that
outage was intentionally tested as a fail-closed staged state. The acceptance
fixture uses only a deterministic test provider.

Command: `G6_WORKSPACE_KEY=g6-batch-smoke-20260825-final make g6-batch-smoke`

```text
valid=500 malformed=10 stages=551 exact=1 semantic=499 open=500
retry_unchanged=551 precision=1.000 recall=1.000 embed_requests=551
```

The fixture publishes 51 isolated anchors, imports 500 valid cards, and adds
10 malformed files. The exact copy is blocked, 499 semantic neighbors enter
review, and every generated neighbor remains an unaccepted `related` proposal.
The second import reports all 551 valid source cards as `unchanged`, proving no
duplicate revisions or proposals were created by retry. The fixture workspace
is archived as `content_kind=fixture` after the run and is never part of the
`fluent-interview` learner release.

Precision/recall are measured against the harness's explicit labels (one
semantic anchor per non-exact valid card). This is a deterministic pipeline
acceptance check, not a claim about real-world model quality; a separately
reviewed calibration set remains required before changing production
thresholds.

## Reviewed calibration measurement

`make calibration-smoke` evaluates all 12 identifier-only pairs with the active
`semantic-v1` thresholds and records no skipped cases:

```text
evaluated=12 skipped=0 true_positive=3 false_positive=0
false_negative=0 true_negative=9 precision=1.000 recall=1.000
```

The set includes exact duplicates, two reviewed paraphrases, related
non-duplicates, locale/translation variants, generic questions, and
technology-specific variants. The thresholds remain profile-owned and the
measurement is reproducible; any future calibration revision must add a new
versioned set rather than silently rewriting this result.
