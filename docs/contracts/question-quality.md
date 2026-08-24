# Question quality audit contract

`GET /v1/quality?workspace=fluent-interview` returns
`question-brain.quality.v1`, an aggregate audit for the same pinned release
served by `/v1/release` and `/v1/catalog`.

The response is deliberately answer-free. It contains:

- locale, track, and source-topic counts;
- `levels` and `companies` cuts: published-question counts per level and per
  source organization. Cards without a value land in the `unclassified`
  bucket — an absent dimension is legal for legacy content;
- the release graph-placement checks (`released`, `accepted-pending`,
  `proposed`, and `unplaced`);
- index-freshness counters: `checks.outbox_pending` (unpublished outbox
  events) and `checks.locales_without_embedding` (current-revision locales
  with no vector under the active embedding profile). Both are part of the
  quality audit only — `/v1/release` omits them — and either value above zero
  also produces an entry in `warnings`, because a lagging indexer silently
  degrades semantic search;
- `checks.degenerate_prompts`: production cards whose English or Russian
  prompt is not a usable question — empty, equal to the answer/title/topic,
  an unpunctuated short label, a truncated sentence fragment (for example a
  prompt ending in `для` or `у нас`), a known PDF heading such as `C`/`SQL`/`-`,
  or text containing an extracted PDF control/replacement character. For Ozon
  cards the same check also covers learner-facing `sections`, `task`,
  `rubric`, and `choices` payload content, rejecting extracted PDF sidebar or
  footer remnants such as page counters and category labels while ignoring
  valid taxonomy metadata such as `Track: Backend`. A card is counted once
  even when both locales fail. This is the I0 content gate;
- `checks.semantic_shape_issues`: the subset of `degenerate_prompts` caused by
  malformed learner-facing prose or a code fragment promoted to a title. It is
  kept separate from PDF control/layout debris so a clean zero means both
  semantic shape and extraction shape passed; a card is still counted once;
- `checks.ru_prompt_equals_answer`: the legacy Russian-only equality counter,
  retained for compatibility with older operators and dashboards. It is a
  subset of `degenerate_prompts`;
- open exact prompt duplicate groups as stable keys plus an opaque fingerprint;
- resolved exact prompt groups, including their terminal review decisions, so
  an audit remains explainable without re-opening settled candidates;
- duplicate-review decision counts from the audited candidate table; and
- explicit warnings for graph review debt, missing Russian coverage, exact
  duplicate groups, or a lagging embedding index (`outbox_pending` /
  `locales_without_embedding` above zero).

Curriculum mapping audit counters are also revision-scoped and answer-free:
`curriculum_mapped`, `curriculum_unmapped`, `curriculum_proposed`,
`curriculum_accepted`, `curriculum_rejected`, and `curriculum_capabilities`.
`curriculum_unmapped` includes current production revisions without a mapping
row and rows explicitly marked `mapping_state=unmapped`. These counters are
review debt, not an inference signal: the quality endpoint never derives a
Path, Domain, or Capability from Track, Group, Topic, title, or task hints.

The endpoint is an audit surface, not a publisher. A proposed placement or a
duplicate group never becomes accepted because it was counted here. A group is
reported under `resolved_duplicate_groups` only after every pair has an
explicit terminal decision (`not_duplicate`, `keep_separate`, or `merge`);
otherwise it remains in `duplicate_groups` and keeps the warning. Review
actions remain explicit and auditable, and answer bodies are fetched only from
the normal stable-key question read boundary. The same I0 checks run before a
source-vault import/release, so `/v1/quality` is a post-publish guard rather
than the first place malformed content is discovered.

Include smoke/fixture records only for diagnostics:

```sh
curl -sS 'http://localhost:48127/v1/quality?workspace=fluent-interview' | jq
curl -sS 'http://localhost:48127/v1/quality?workspace=fluent-interview&include_fixtures=true' | jq
```

`release_id` is the join key for the report, Lab's task provenance, and any
future coverage dashboard. A consumer must not turn `total` into a completion
percentage without also showing the graph and locale checks.
