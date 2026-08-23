# Question quality audit contract

`GET /v1/quality?workspace=fluent-interview` returns
`question-brain.quality.v1`, an aggregate audit for the same pinned release
served by `/v1/release` and `/v1/catalog`.

The response is deliberately answer-free. It contains:

- locale, track, and source-topic counts;
- the release graph-placement checks (`released`, `accepted-pending`,
  `proposed`, and `unplaced`);
- index-freshness counters: `checks.outbox_pending` (unpublished outbox
  events) and `checks.locales_without_embedding` (current-revision locales
  with no vector under the active embedding profile). Both are part of the
  quality audit only — `/v1/release` omits them — and either value above zero
  also produces an entry in `warnings`, because a lagging indexer silently
  degrades semantic search;
- open exact prompt duplicate groups as stable keys plus an opaque fingerprint;
- resolved exact prompt groups, including their terminal review decisions, so
  an audit remains explainable without re-opening settled candidates;
- duplicate-review decision counts from the audited candidate table; and
- explicit warnings for graph review debt, missing Russian coverage, exact
  duplicate groups, or a lagging embedding index (`outbox_pending` /
  `locales_without_embedding` above zero).

The endpoint is an audit surface, not a publisher. A proposed placement or a
duplicate group never becomes accepted because it was counted here. A group is
reported under `resolved_duplicate_groups` only after every pair has an
explicit terminal decision (`not_duplicate`, `keep_separate`, or `merge`);
otherwise it remains in `duplicate_groups` and keeps the warning. Review
actions remain explicit and auditable, and answer bodies are fetched only from
the normal stable-key question read boundary.

Include smoke/fixture records only for diagnostics:

```sh
curl -sS 'http://localhost:48127/v1/quality?workspace=fluent-interview' | jq
curl -sS 'http://localhost:48127/v1/quality?workspace=fluent-interview&include_fixtures=true' | jq
```

`release_id` is the join key for the report, Lab's task provenance, and any
future coverage dashboard. A consumer must not turn `total` into a completion
percentage without also showing the graph and locale checks.
