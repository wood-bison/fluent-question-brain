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

## Live verification

Environment: Compose project `fluent-question-brain`, PostgreSQL `55437`.
Migration `0018` was applied idempotently.

```text
generated manifest: 1,591 entries
dry-run: 1,591 entries · 19 bound · 1,572 theory_only · 19 bindings
approved release: question-capability-release-1e9ce61471213390
projection: 19 bindings · 1,591 reviews · 19 release items · 1 active release
repeat approval: same release ID, no duplicate release items
```

The 19 existing reviewed crosswalk station references were canonicalized into
active registry keys. The remaining 1,572 cards have an explicit
`theory_only` disposition because no reviewed capability station exists yet;
they are not hidden and they are not counted as learner stations. Future
editorial review may change a row to `bound`, `needs_new_capability`, or
`rejected` in a new pinned release.

The reproducible command is `make capability-binding-smoke`; it applies the
idempotent migration, generates the manifest, runs a blocked-free dry-run,
approves it twice, and checks the active release/projection counts.
