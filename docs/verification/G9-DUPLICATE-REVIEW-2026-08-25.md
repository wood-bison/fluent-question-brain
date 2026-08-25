# G9 durable duplicate review — verification evidence

Date: 2026-08-25  
Scope: Question Brain duplicate-review contract and Fluent Lab operator queue

## Contract

Question Brain publishes the answer-free `question-brain.duplicate-review.v1`
projection from the durable duplicate candidate table. The projection is
workspace-scoped, exposes current production revisions only, and maps
`status=proposed` to the durable `open` decision state. It includes stable and
revision IDs, exact/semantic scores, locales and provenance, but never answer
bodies or secrets.

The endpoint is:

```bash
curl -fsS \
  'http://127.0.0.1:48127/v1/duplicates/review?workspace=fluent-interview&status=proposed'
```

The response is versioned with `contractVersion` equal to
`question-brain.duplicate-review.v1` and a deterministic `candidates` array.
The write path remains `POST /v1/duplicates/decision`; it requires the
internal token, actor, rationale and compare-and-set `expectedDecision`.

## Live result

The packaged Fluent Lab review workbench consumed the durable candidate for
`q204 ↔ q315`. The operator saw both RU and EN prompts, stable/revision IDs and
the exact/semantic scores, supplied an actor and rationale, and selected
`Это не дубликат`.

Observed outcomes:

- the Lab announcement confirmed the saved decision;
- the next queue read showed zero proposed duplicate candidates;
- no learner progress or active graph release changed;
- a conflicting repeat decision returned HTTP `409` rather than overwriting
  the first decision;
- the browser rendered no answer content or secret markers.

## Verification commands

Run from `/Users/sergeyzhechko/developer/fluent-interview/fluent-question-brain`:

```bash
docker run --rm -v "$PWD":/src -w /src fluent-question-brain-go-check:local \
  go test ./internal/httpapi ./internal/store
```

The HTTP tests cover missing token, missing rationale, an audited duplicate
decision, and the versioned duplicate-review response. The Lab adapter test
also verifies that the durable projection is preferred; the legacy quality
group projection is retained only as a unit-test fixture compatibility path and
is not used when the production response is present.
