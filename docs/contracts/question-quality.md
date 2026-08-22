# Question quality audit contract

`GET /v1/quality?workspace=fluent-interview` returns
`question-brain.quality.v1`, an aggregate audit for the same pinned release
served by `/v1/release` and `/v1/catalog`.

The response is deliberately answer-free. It contains:

- locale, track, and source-topic counts;
- the release graph-placement checks (`released`, `accepted-pending`,
  `proposed`, and `unplaced`);
- exact prompt duplicate groups as stable keys plus an opaque fingerprint;
- duplicate-review decision counts from the audited candidate table; and
- explicit warnings for graph review debt, missing Russian coverage, or exact
  duplicate groups.

The endpoint is an audit surface, not a publisher. A proposed placement or a
duplicate group never becomes accepted because it was counted here. Review
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
