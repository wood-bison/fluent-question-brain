# Canonical question revision contract

```json
{
  "workspace_key": "fluent-interview",
  "stable_key": "node.event-loop.ordering",
  "revision": 1,
  "status": "published",
  "locales": {
    "en": {
      "prompt": "Why does this callback run before the timer?",
      "short_answer": "…",
      "explanation": "…",
      "body": {"blocks": []}
    },
    "ru": {
      "prompt": "Почему этот callback выполняется раньше таймера?",
      "short_answer": "…",
      "explanation": "…",
      "body": {"blocks": []}
    }
  },
  "topics": ["nodejs.runtime.event-loop"],
  "edges": [
    {"to": "nodejs.runtime.timers", "kind": "prerequisite"}
  ],
  "source": {
    "system": "fluent-question-vault",
    "path": "Question Cards/…"
  }
}
```

## Optional typed payload blocks

The normalized payload may carry three additional optional blocks. All are
absent (`omitempty`) unless the source card explicitly contains them, so
adding them never changes existing `content_hash` values.

### `task` — practical exercise

Stored as structured data instead of flattened prose:

```json
{
  "task": {
    "condition": "…",
    "starter": "DDL or function signature or input data",
    "solution": "reference solution",
    "walkthrough": "why this solution works",
    "difficulty": "MEDIUM",
    "constraints": "time/memory limits where applicable"
  }
}
```

A block is created only when a recognized condition section coexists with
evidence the card is solvable (`solution`, `walkthrough`, or `starter`); a
narrative section merely titled “Task” stays prose.

### `rubric` — ordered assessment levels

```json
{
  "rubric": [
    {"label": "развёрнуто", "text": "…"},
    {"label": "приемлемо", "text": "…"},
    {"label": "достаточно для джуна", "text": "…"}
  ]
}
```

Source order is kept; each entry says what a candidate must demonstrate.

### `choices` — screening question with an answer key

```json
{
  "choices": {
    "options": [{"label": "А", "text": "…"}, {"label": "Б", "text": "…"}],
    "answer_key": "А",
    "rationale": "why the remaining options are wrong"
  }
}
```

## Dimensions

`level` and `company` are metadata fields in the payload *and* indexed columns
on `content.question` (`level` backfilled from the current revision;
`company` from the card's `Company:` line). An absent value is legal — legacy
cards have no company and must not inherit one. Both are exposed as search
filters and quality-audit cuts.

## Optional Fluent Lab curriculum mapping

The legacy `track`, `group`, and `topic` fields remain in the normalized
payload for content compatibility. They must not be interpreted as a Lab
curriculum assignment. A reviewed cross-system mapping may add this optional
metadata:

```json
{
  "program_key": "program.backend-engineer",
  "path_key": "path.nodejs-typescript",
  "domain_key": "domain.runtime",
  "capability_key": "capability.runtime.event-loop",
  "mapping_state": "proposed",
  "mapping_version": "question-brain.taxonomy.v1"
}
```

`path_key` and `domain_key` are explicit stable keys. `capability_key` is a
reviewed Lab station and requires both. `mapping_state` is never inferred from
the old `Topic`, `Group`, `Track`, task concepts, or a UI breadcrumb. The
catalog exposes a deprecated `stage_key` compatibility projection of
`domain_key` for older Lab clients; new clients use `path_key` and
`domain_key`. The revision-scoped `content.question_capability` relation is
many-to-many and release-aware; no legacy card is backfilled by this contract.
The additive `content.question_curriculum_mapping` table is the one-row-per-
revision release seam for Program/Path/Domain decisions. Its explicit
`mapping_state=unmapped` row records review coverage without adding keys. The
`qb-map-release` command requires a complete manifest pinned to current
`revision_id` and `content_hash`; it never infers v1 fields from legacy
metadata.

Normalization is deterministic: trim Unicode whitespace, normalize line
endings to LF, canonicalize JSON key order, normalize locale tags, and remove
editor-only metadata. The SHA-256 of that normalized payload is the
`content_hash`. It is never derived from a rendered UI.

The contract is intentionally independent of Payload blocks and of any one
embedding vendor. A renderer may add fields, but a published revision cannot
change without a new revision number and a new hash.
