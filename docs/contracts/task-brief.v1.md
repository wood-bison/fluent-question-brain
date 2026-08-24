# Question Brain TaskBrief v1

`TaskBrief` is the learner-facing statement of an exercise. It answers what
the candidate must demonstrate and how the interviewer evaluates it. It is
not an execution manifest.

## Ownership

| Concern | Owner |
| --- | --- |
| condition, input/schema/signature, walkthrough, difficulty, rubric | Question Brain |
| TaskFamily identity and language/profile revisions | Task Runtime |
| starter workspace, solution, hidden tests, harness, image, limits, policy | Task Runtime |
| learner attempt, run, evidence, progress | Fluent Lab |

## Versioned payload

```json
{
  "task": {
    "contract_version": "question-brain.task-brief.v1",
    "kind": "runtime_task_reference",
    "task_family_key": "task-family.rate-limiter",
    "condition": "Implement a token bucket.",
    "starter": "function allow(clientId, timestamp)",
    "walkthrough": "Explain the invariant, boundary, and complexity.",
    "difficulty": "MEDIUM",
    "constraints": "No reference implementation is stored here."
  }
}
```

`kind` is one of:

- `discussion_prompt` — an interview prompt with no executable task;
- `design_exercise` — a system-design or behavioural exercise with no sandbox;
- `runtime_task_reference` — a runnable brief joined to exactly one released
  TaskFamily;
- `historical_content` — an immutable pre-v1 block kept for provenance only.

The v1 validator rejects a missing/invalid kind, a runtime brief without a
TaskFamily key, an invalid family key, a family key on a non-runtime kind, and
any non-empty `solution`. The strict import/release flags are
`--strict-task-boundary` on `qb-import` and `qb-release`. Legacy unversioned
blocks remain readable and are counted by the quality audit; they are not
silently rewritten.

## Retrieval and API boundary

Question Brain may return the brief and family key through its normal question
projection. It never returns Task Runtime solutions, hidden tests, harness
commands, images, limits, or sandbox policy. Fluent Lab resolves the family
key against the Task Runtime release and then asks the learner to choose a
language revision before opening a workspace.
