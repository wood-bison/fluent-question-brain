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

Normalization is deterministic: trim Unicode whitespace, normalize line
endings to LF, canonicalize JSON key order, normalize locale tags, and remove
editor-only metadata. The SHA-256 of that normalized payload is the
`content_hash`. It is never derived from a rendered UI.

The contract is intentionally independent of Payload blocks and of any one
embedding vendor. A renderer may add fields, but a published revision cannot
change without a new revision number and a new hash.

