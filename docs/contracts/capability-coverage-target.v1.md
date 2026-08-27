# Capability coverage target v1

`question-brain.capability-coverage-target.v1` is the reviewed, immutable
coverage policy for one exact Question Brain and Question → Capability binding
release. It answers two questions without inventing another content graph:

1. which pinned cards are `core`, `supplemental`, or `quarantined`;
2. how many primary questions and supporting prompts a capability must contain.

It does **not** own Path, Capability, or placement role. Those relations remain
canonical in `question-brain.capability-binding.v1`; the coverage validator
joins the exact binding release and derives learner presentation:

| Canonical binding role | Learner presentation |
| --- | --- |
| `primary` | `primary_question` |
| `prerequisite`, `follow_up`, `contrast`, `recall`, `supporting_evidence` | `supporting_prompt` |

The derived presentation role is not stored as a second relation.

## Manifest

```json
{
  "contract_version": "question-brain.capability-coverage-target.v1",
  "taxonomy_version": "question-brain.taxonomy.v1",
  "workspace_key": "fluent-interview",
  "question_release_id": "question-release-…",
  "capability_registry_release_id": "capability-registry-…",
  "capability_binding_release_id": "question-capability-release-…",
  "minimum_coverage_score_bps": 9000,
  "source": "reviewed-production-coverage-policy",
  "targets": [
    {
      "path_key": "path.nodejs-typescript",
      "capability_key": "capability.nodejs.event-loop-ordering",
      "mandatory": true,
      "minimum_primary_questions": 7,
      "minimum_supporting_prompts": 3,
      "rationale": "technical core capability bundle"
    }
  ],
  "cards": [
    {
      "stable_key": "question.c009",
      "revision_id": "…",
      "content_hash": "…",
      "disposition": "core",
      "rationale": "reviewed primary mechanism question"
    }
  ]
}
```

The card ledger is complete for the referenced capability-binding release.
Every row pins the same immutable revision and hash. A `core` card must have a
reviewed binding; a `quarantined` card must have none. `supplemental` material
may remain searchable but never satisfies a mandatory target.

## Fail-closed gates

- unknown JSON fields, stale release IDs, stale revision/hash pins, duplicates,
  missing rationale, or an incomplete card ledger are rejected;
- a mandatory capability requires at least one primary question;
- raw card count cannot substitute for the required primary/supporting split;
- a deprecated capability cannot be a new target;
- a bound quarantined card and an unbound core card are contradictions;
- `minimum_coverage_score_bps` is at least `9000`; this manifest records the
  policy threshold, while full layer/depth/practice scoring remains a separate
  release gate and cannot be fabricated from counts.

The first production manifest must classify the current release explicitly.
This contract does not auto-classify 1,591 cards and does not authorize filler
content. `coverage.ValidateAgainstBindings` is the fail-closed validation
boundary: a caller must receive a ready report before persisting or promoting
the policy. Database rows additionally enforce the immutable release identity.
