# Question → Capability binding release v1

This contract is the station-level review boundary owned by Question Brain. It
does not replace the explicit Program/Path/Domain curriculum mapping and it
does not create a task or a mastery result.

## Identity

Each manifest entry pins one current `QuestionCard` by `stable_key`,
`revision_id`, and `content_hash`. The manifest also pins the current Question
release and a reviewed capability-registry release. A binding carries an
explicit `path_key`, canonical `capability_key`, relationship `role`,
`provenance`, optional confidence, and optional review evidence.

Allowed roles are `primary`, `prerequisite`, `follow_up`, `contrast`,
`recall`, and `supporting_evidence`.

## Disposition is not a hidden gap

Every current production card has exactly one reviewed disposition:

| Disposition | Meaning | Run button |
| --- | --- | --- |
| `bound` | one or more reviewed station bindings exist | only if a released TaskFamily exists elsewhere |
| `theory_only` | released/searchable card with no station claim | no |
| `needs_new_capability` | executable or otherwise important card waiting for a new reviewed station | no until reviewed |
| `rejected` | explicitly excluded with an audit rationale | no |

`theory_only` is deliberate. It prevents a large content bank from being
compressed into fake stations and makes the coverage report explain what has
not been turned into a learner capability yet.

## Persistence and release

- `content.question_capability_review` stores the disposition and rationale.
- `content.question_capability_binding_proposal` stores each auditable
  candidate/decision, including source, confidence, evidence, actor, and
  question/registry pins.
- `content.question_capability_binding_release` is the immutable, workspace-
  scoped release identity. Only one release is active for a workspace; older
  releases remain available for rollback/history.
- `content.question_capability_binding_release_item` stores the exact many-to-
  many bindings in that release.
- `content.question_capability` remains a compatibility projection for current
  clients, but new rows carry `binding_release_id`, both release pins, role,
  provenance, confidence, and source proposal.

Approval is dry-run by default. `qb-capability-release --approve` is the only
writer. It refuses stale revisions/hashes, missing dispositions, inactive or
deprecated capabilities, path mismatches, missing curriculum mapping, and
cross-workspace rows. Re-running the same manifest is idempotent and
reactivates the same release identity without duplicating bindings.

The generator is intentionally conservative: it can promote an existing
reviewed curriculum capability crosswalk into a canonical station binding;
all other current cards receive an explicit `theory_only` or
`needs_new_capability` disposition. It never infers from Topic, Group, title,
breadcrumb, filename, or embeddings.

## Commands

```bash
# Generate a complete review queue; this does not write the database.
go run ./cmd/qb-capability-release \
  --database-url "$DATABASE_URL" \
  --generate docs/verification/G7-capability-binding-manifest-2026-08-25.json

# Validate only (default is dry-run).
go run ./cmd/qb-capability-release \
  --database-url "$DATABASE_URL" \
  --manifest docs/verification/G7-capability-binding-manifest-2026-08-25.json

# Explicitly publish the reviewed release.
go run ./cmd/qb-capability-release \
  --database-url "$DATABASE_URL" \
  --manifest docs/verification/G7-capability-binding-manifest-2026-08-25.json \
  --approve
```

The next release join (G8) consumes only the active binding release and adds
TaskFamily/TaskRevision references; it must not re-infer bindings in Fluent
Engineering Lab.
