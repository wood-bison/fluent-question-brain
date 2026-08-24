# ADR-0003 — Reviewed relations and capability bindings stay in Question Brain

**Status:** accepted  
**Date:** 2026-08-24  
**Builds on:** ADR-0001 and ADR-0002

## Context

Question Brain currently has source-topic placement and duplicate decisions, but
it does not yet have a durable, reviewed semantic graph or an approved
capability inventory. Automatically turning 1,591 cards into stations would
hide uncertainty and couple editorial data to the Lab learner graph.

## Decision

Question Brain is the sole writer for:

- immutable localized `QuestionCard` revisions and their hashes;
- typed `ContentRelation` edges (`prerequisite`, `related`, `contrast`,
  `follow_up`, `variant`, `duplicate`, `supersedes`);
- reviewed many-to-many `QuestionCapabilityBinding` rows;
- the canonical `Capability` registry and its many-to-many
  `CapabilityDomainBinding` classification;
- semantic proposals, provenance, confidence, reviewer decisions, and the
  immutable content/graph release that exposes accepted facts.

Candidate semantic edges and capability bindings are never learner-visible
until an explicit review decision and release. A title, topic, task key, or
language profile cannot infer a capability. `questionKeys` accept only stable
Question Brain keys; capability keys are separate fields in Task Runtime and
Lab contracts.

The cross-system identity rules are defined by
`question-capability-task.v1`. The workspace schema/fixture is normative; this
repository owns the Question Brain subset and validates all incoming bindings
against its stable revision/hash and provenance rules.

## Consequences

- A new batch can be searched and enriched without editing Lab code.
- A capability may belong to several shared domains; a card may assess several
  capabilities.
- The two existing rate-limiter keys are not merged automatically. They need a
  reviewed keep/split/merge/supersede decision in G2.
- Historical releases remain immutable. Renames use aliases/supersedes and do
  not rewrite learner evidence.

## Rejected alternatives

- A second graph/vector database: it would create competing search and writer
  semantics; Postgres/pgvector remains the canonical storage boundary.
- Lab-side auto-placement: it has no editorial provenance and would make
  discovery counters look like released curriculum.

