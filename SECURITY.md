# Security boundary

- Do not commit `.env`, database credentials, embedding-provider keys, or raw
  interview answers copied from a private source.
- Logs and traces carry ids, hashes, and redacted metadata; raw content is not
  a default observability field.
- Payload admin access is separate from public Question Brain reads.
- Import and promote operations are authenticated commands and must be
  idempotent.

