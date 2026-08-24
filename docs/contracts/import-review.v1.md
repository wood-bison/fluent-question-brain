# Question Brain import review contract v1

`question-brain.import-review.v1` is the publication boundary between a vault
card import and the released Question Brain graph. It is intentionally separate
from learner progress and from the Task Runtime.

## Lifecycle

```text
source card
   │
   ▼
staged ── candidate generation ──┬── no candidates ──► cleared
                                └── candidates ─────► blocked
                                                           │
                                     explicit actor + rationale
                                                           │
                                                           ▼
                                                     cleared
                                                           │
                                                           ▼
                                                     published
```

`discarded` is an operator-only terminal state for a staged source that will
not enter the release. A stage with an `open` or `merge` candidate can never be
published. A changed content hash creates a new stage; a resolved pair is not
reopened for the same source reference and hash.

## Candidate generation

Question Brain performs all three checks inside the owning workspace and only
against current published production revisions:

| Candidate | Evidence | Default decision |
| --- | --- | --- |
| `exact_duplicate` | canonical payload equality after removing stable identity fields (`stable_key`, `slug`, and display `title`) | review required |
| `lexical_neighbor` | PostgreSQL `pg_trgm` prompt similarity | review required |
| `semantic_neighbor` | active `pgvector` embedding profile | review required |

The active profile owns its lexical/semantic thresholds, maximum candidate
count, and calibration revision in `content.duplicate_profile_config`. Missing
or inactive embeddings do not silently switch to a different semantic profile;
the stage remains auditable and lexical/exact evidence is still visible.

Candidate rows contain scores and method metadata, never raw prompt text,
learner code, hidden tests, model output, or secrets. An editorial decision must
include an authenticated actor. `not_duplicate` and `keep_separate` clear the
candidate; `merge` keeps the stage blocked until a separate supersession/merge
decision is implemented.

## Graph proposals

Lexical and semantic neighbors are copied only as `related` proposals after the
incoming revision receives its immutable revision ID. The proposal is
`status=proposed` and is never auto-accepted. Graph release can include it only
after the existing Question Brain graph review API accepts it. Exact duplicate
evidence remains in import review and does not create an automatic duplicate
edge.

## API

```text
GET  /v1/import/review?workspace=fluent-interview&status=blocked
GET  /v1/import/review/{stageID}
POST /v1/import/review/candidates/{candidateID}/decision
```

The decision endpoint requires `X-Question-Brain-Token` and records
`X-Question-Brain-Actor` (or the explicit reviewer default) plus the rationale.
The response is the complete stage projection, including candidate scores and
current readiness.

## Ownership and safety

Question Brain owns staging, candidate evidence, duplicate decisions, and graph
proposals. Fluent Lab consumes only released Question Brain revisions and graph
releases. Task Runtime remains the only owner of executable task material.
There is no compatibility fallback that publishes an unreviewed import.
