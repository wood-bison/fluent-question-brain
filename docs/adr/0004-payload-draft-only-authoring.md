# ADR 0004: keep Payload as the draft-only authoring surface

## Status

Accepted for G9 on 2026-08-25.

## Context

The Review Workbench now resolves machine proposals through the Question Brain
API, so leaving a second publishing workflow would make canonical ownership
ambiguous. Payload is already useful for editor drafts, localized fields,
versions and review ergonomics, but its published view must not become another
content store or a direct SQL writer.

## Decision

Keep Payload in the product as a draft/review CMS with one explicit promotion
seam:

```text
Payload `questions` draft/version
        │ publish (authenticated hook)
        ▼
Question Brain POST /v1/promote
        │ validate → normalize → hash → revision/audit/outbox
        ▼
Question Brain `content` release/search projection
```

Payload owns only the `cms` schema and its editorial versions. The Go Question
Brain API remains the sole writer of published revisions, placements, graph
relations, capability bindings and embedding/index work. The
`published-questions` collection is a read-only view over released `content`
rows; it is not a second copy and cannot be written through Payload.

## Evidence and guardrails

- `apps/cms/src/hooks/promotePublishedQuestion.ts` requires the promote URL and
  internal token, sends one answer payload, and fails the editor publish when
  the canonical API rejects it.
- `apps/cms/src/collections/PublishedQuestions.ts` denies create/update/delete
  and reads only `status = 'published'`, `content_kind = 'production'` rows.
- `docs/verification/g4-payload-promote-2026-08-22.md` records the authenticated
  bilingual publish, immutable revision, audit event and outbox path.
- `docs/verification/G9-REVIEW-WORKBENCH-2026-08-25.md` records the live review
  matrix and confirms the Workbench has no second Payload write path.
- Host checks use `npm ci`, `npm run typecheck`, and `npm run build` inside
  `apps/cms`; the Compose CMS image remains the production runtime.

## Rejected alternative

Do not make Payload the canonical content writer and do not keep a Payload
published copy synchronized beside `content`. That would reintroduce split
identity, duplicate release semantics and an unreviewed bypass around the
Question Brain audit/outbox boundary.

## Revisit condition

Revisit only if the full promote contract, release rollback, review parity and
backup/restore guarantees are deliberately replaced in a future ADR. Until
then, a second authoring or publish path is a production defect.
