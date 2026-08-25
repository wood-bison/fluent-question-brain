# G7 evidence — reviewed Question ↔ Capability bindings

Status: **complete for the current production corpus**.

## Implementation

- Migration `0018_question_capability_bindings.sql` adds dispositions,
  proposals, immutable binding releases, release items, and release-pinned
  compatibility projection fields.
- `qb-capability-release` is the only writer. It generates a complete review
  queue, defaults unbound cards to an explicit disposition, validates every
  revision/hash and registry key, and is dry-run by default.
- Canonical station keys are resolved through the reviewed taxonomy registry;
  deprecated task-shaped keys are never written to a new release.
- A bound card may have multiple capabilities and roles. A theory-only card is
  still released and searchable, but has no fabricated station or Run button.
- A second approval of the same manifest reactivates the same release ID and
  creates no duplicate rows.
- Optional semantic review staging uses the profile-owned
  `semantic-neighbor-v1` configuration (`min_similarity=0.65`, max three
  candidates per card) and writes only `status=proposed` rows. It never changes
  the accepted release.

## Live verification

Environment: Compose project `fluent-question-brain`, PostgreSQL `55437`.
Migration `0018` was applied idempotently.

```text
generated manifest: 1,591 entries
dry-run: 1,591 entries · 19 bound · 1,572 theory_only · 19 bindings
approved release: question-capability-release-3c38b4c8c0fa7f47
projection: 19 bindings · 1,591 reviews · 19 release items · 1 active release
repeat approval: same release ID, no duplicate release items
rollback: restored the previous release ID with 19 bindings after a second
release was published; no immutable release items were rewritten
semantic review staging: 742 target cards · 1,373 candidates generated ·
1,288 open proposals after idempotent conflict coalescing
```

The active capability registry pin for this release is
`capability-registry-2026-08-25-v3`. The registry includes the explicit
execution-boundary capability used by the unreleased project-book family; it
is available for release validation but is not a fabricated QuestionCard
station.

The 19 existing reviewed crosswalk station references were canonicalized into
active registry keys. The remaining 1,572 cards have an explicit
`theory_only` disposition because no reviewed capability station exists yet;
they are not hidden and they are not counted as learner stations. Future
editorial review may change a row to `bound`, `needs_new_capability`, or
`rejected` in a new pinned release.

The reproducible command is `make capability-binding-smoke`; it applies the
idempotent migration, generates the manifest and semantic review candidates,
runs a blocked-free dry-run,
approves it twice, publishes a second isolated registry revision, rolls back
to the first release, and checks the active release/projection counts.
