# ADR-0005: Keep Algorithms and Behavioral as dedicated curriculum domains

- Status: accepted
- Date: 2026-08-27
- Owner: Question Brain taxonomy

## Context

The canonical path/domain crosswalk had 52 Algorithms cards under
`domain.runtime` and 103 Behavioral cards under `domain.testing`. That made
the learner projection report technically valid counts while misrepresenting
the activity: solving an algorithm is not runtime execution, and rehearsing a
behavioral answer is not a testing skill.

## Decision

Add `domain.algorithms` and `domain.behavioral` to taxonomy v1 as shared,
first-class domains. Enforce two shape invariants in the explicit placement
resolver:

1. `path.algorithms` ↔ `domain.algorithms`;
2. `path.behavioral` ↔ `domain.behavioral`.

The mapping is released through a new, revision-pinned manifest rather than a
payload rewrite. The SQL migration is idempotent and has a fail-closed check
for partial updates. The previous canonical manifest remains the rollback
source.

## Consequences

- Route filters and progress counters can distinguish problem solving and
  communication from runtime/testing.
- Existing shared modules remain reusable through explicit manifest joins.
- A future card cannot silently leak into a dedicated lane; it must use a new
  reviewed manifest and the taxonomy shape tests.
- Content metadata (`Track`, `Group`, `Topic`) stays unchanged, so hashes and
  locale parity remain stable.

## Evidence

- `db/migrations/0020_curriculum_domain_separation.sql`
- `releases/curriculum-mapping-2026-08-27-domain-separated.json`
- `scripts/mapping/derive-domain-separated-release.mjs`
- `docs/verification/two-audit-remediation/W05/`
