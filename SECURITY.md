# Security boundary

- Do not commit `.env`, database credentials, embedding-provider keys, or raw
  interview answers copied from a private source.
- Logs and traces carry ids, hashes, and redacted metadata; raw content is not
  a default observability field.
- Query strings, request bodies, authorization headers, and embedding vectors
  must never be logged. The redaction test in `internal/telemetry` is a release
  gate.
- Payload admin access is separate from public Question Brain reads.
- Import and promote operations are authenticated commands and must be
  idempotent.
- Backups are encrypted and access-controlled in production; local restore
  drills use a disposable database named `question_brain_restore_smoke`.
