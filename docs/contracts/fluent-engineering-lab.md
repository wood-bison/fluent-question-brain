# Fluent Engineering Lab integration contract (G4)

Fluent Engineering Lab remains the learner-facing product. Question Brain is
the independent source for canonical question revisions, bilingual content,
graph placement, and retrieval. The Lab must not connect to the `content` or
`cms` schemas directly.

## Read boundary

The Lab talks to the versioned HTTP API only:

```text
POST /v1/search
GET  /v1/questions/{stable_key}?locale=en|ru
```

Every search response carries `provenance.explainable=true`, the active
pipeline (`exact`, `fts`, `trigram`, and the semantic profile), per-result
match stages, and a stable `revision_id`/`content_hash`. The Lab can safely
cache a response by `(workspace, locale, query, topic_key, revision_id)` and
invalidate it when the graph release changes.

## Opt-in feature flag

The first Lab adapter should be disabled by default while parity is measured:

```text
QUESTION_BRAIN_READS=0
QUESTION_BRAIN_BASE_URL=http://127.0.0.1:48127
QUESTION_BRAIN_WORKSPACE=fluent-interview
QUESTION_BRAIN_TIMEOUT_MS=1200
```

The dependency-free reference adapter is committed at
`integrations/fluent-engineering-lab/question-brain-client.ts`; the Lab can
vendor it or copy the same contract into its own workspace without coupling
its learner UI to this repository's Go implementation details.

When the flag is `1`, only the read projection changes. The existing learner
contract remains the response shape; missing graph metadata is treated as a
closed, reviewable projection error rather than guessed in the browser. Write
and authoring operations stay in Payload → Go API and are never proxied from
the learner UI.

## Locale contract

The Lab sends the active UI locale explicitly (`en` or `ru`) on every card
read. If a locale is absent, the Go API returns the configured fallback and
still reports the actual `locale` in the response. UI copy and question content
are therefore switched independently without duplicating source records.

## Rollout and rollback

1. Run the parity report for the same stable-key slice against the current Lab
   archive and Question Brain.
2. Enable `QUESTION_BRAIN_READS=1` only in a disposable Lab process and compare
   counts, locale coverage, `content_hash`, and graph placement.
3. Promote the flag for one learner profile at a time; keep the old archive
   read path available as a read-only fallback for the rollout window.
4. Roll back by setting the flag to `0`; no data migration or schema rollback
   is required.

This is deliberately a contract and adapter seam, not a second copy of the
question registry. The Lab integration is the final G4 consumer change and is
closed only after the parity evidence is committed in the Lab repository.
