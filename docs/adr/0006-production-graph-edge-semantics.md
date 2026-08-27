# ADR-0006 — Production graph edge semantics and evidence guards

**Status:** accepted  
**Date:** 2026-08-27  
**Builds on:** ADR-0003

## Context

The first graph release was created by review smoke tests. Its relations were
useful for exercising the lifecycle, but fixture actors and rationales were
visible in the active production workspace. A learner graph must not turn test
provenance into curriculum truth, and a confidence score must not be mistaken
for review evidence.

## Decision

Question Brain owns one executable edge-kind registry. Each kind has an
explicit learner effect:

| Kind | Meaning | Learner effect |
| --- | --- | --- |
| `prerequisite` | target is understood first | gates recommendation; acyclic |
| `related` | adjacent context | context only |
| `contrast` | alternative or boundary | comparison only |
| `follow_up` | deeper treatment | suggests next |
| `variant` | alternate formulation/implementation | alternative only |
| `duplicate` | equivalent content | deduplicates |
| `supersedes` | historical replacement | historical replacement |

The production workspace rejects new test/fixture/synthetic provenance. Release
dry-runs fail closed if an accepted edge still contains it, points at a
non-published or archived question, is stale, cyclic, or lacks reviewer
evidence. Confidence `1.0` requires rationale and source. The database repeats
these checks so the API and direct operational tooling cannot bypass them.

Test fixtures use dedicated `g6-batch-smoke-*` workspaces and remain auditable;
historical fixture rows in `fluent-interview` are rejected, never deleted.

## Consequences

- The active learner graph can be empty while editorial semantic proposals are
  reviewed; no fake edge is better than a test edge.
- Every future graph release can be audited read-only from API projections with
  stable release evidence and a deterministic digest.
- A human reviewer must intentionally accept production semantics; automated
  enrichment can propose but cannot publish.
