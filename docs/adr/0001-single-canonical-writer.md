# ADR 0001: one canonical writer

## Status

Accepted for G1.

## Decision

The Go Question Brain command/API boundary is the only writer of published
question revisions, graph edges, placements, and embedding jobs in the
`content` schema. Payload may write editorial drafts and versions in the
separate `cms` schema. Publishing is an explicit command that validates,
normalizes, hashes, persists, and emits an outbox event in one transaction.

## Consequences

- We get one definition of identity, locale, status, and graph edges.
- Payload remains valuable for editors, versions, drafts, localization, and
  review without becoming a second database of truth.
- The integration needs a promote endpoint and an audit trail.
- Payload schema migrations must never mutate the canonical schema.

