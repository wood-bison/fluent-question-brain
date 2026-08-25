# G9 alias/supersession review boundary — verification evidence

Date: 2026-08-25  
Repository: `fluent-question-brain`  
Contract: `question-brain.capability-alias-supersession-review.v1`

## Scope

The canonical capability registry is the only write authority for renamed or
retired capability keys. A proposal is staged first; accepting it transactionally
materializes either `content.taxonomy_capability_alias` or
`content.taxonomy_capability_supersedes`. Rejecting it records the decision but
does not mutate the canonical graph. The decision endpoint is authenticated,
actor/rationale audited, compare-and-set protected, and idempotent on replay.

## Live verification

After applying migration `0019_capability_alias_supersession_review.sql` and
rebuilding the API, the following checks were run against the local Compose
stack (`api :48127`, Postgres `:55437`):

```bash
curl -fsS \
  'http://127.0.0.1:48127/v1/capability-aliases/review?workspace=fluent-interview&status=proposed'
# contract_version: question-brain.capability-alias-supersession-review.v1
# proposals: []

docker run --rm -v "$PWD:/src" -w /src fluent-question-brain-go-check:local \
  go test ./...
make contract
git diff --check
```

A disposable alias proposal was staged through the authenticated API, accepted
from the real Lab Studio browser surface, and then checked again:

```text
capability.test.g9-ui-proposal
  -> capability.nodejs.event-loop-ordering
```

The queue returned to an explicit empty result after the browser decision. The
same accepted decision was replayed successfully (idempotent); a conflicting
decision returned HTTP 409 with `review proposal changed concurrently` rather
than overwriting the first decision. The materialized relation was verified in
Postgres with the read-only query:

```sql
select alias_key, canonical_key
from content.taxonomy_capability_alias
where alias_key = 'capability.test.g9-ui-proposal';
```

## Safety properties

- Source and canonical keys must be non-empty, distinct and in the same
  workspace; canonical capabilities must be active.
- Supersession proposals require an existing source key and reject cycles.
- Repeated identical decisions are safe; a different decision for the same
  proposal is a conflict.
- The review projection is answer-free and never grants learner mastery or
  task pass.
- The browser cannot connect to Postgres and cannot call the writer without
  the internal Question Brain token.

## Remaining G9 acceptance

This evidence closes the previously missing canonical-key migration contract.
It does not claim the whole G9 gate is complete: the production plan still
requires one live duplicate, rejected edge, accepted prerequisite,
multi-capability binding, stale-revision conflict, and the later Payload
decision before G9 can be marked accepted.
