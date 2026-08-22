# Legacy and archive boundary

The existing projects remain intact during migration:

| Path | Role now | Role during migration |
| --- | --- | --- |
| `/Users/sergeyzhechko/developer/fluent-engineering-lab` | Interview-learning product | Active product and future Question Brain API client |
| `/Users/sergeyzhechko/developer/fluent-question-vault` | Git history mirror of Obsidian cards | Import snapshot and export verification source |
| `fluent-question-brain` (this repo) | Canonical service | Published source of truth and reusable API |

The source-vault cutover is complete: the approved Question Brain release is
the only learner-visible source of truth, while the Lab archive is retained as
an immutable, read-only outage fallback. We do not move or delete the 8.7 GB
Lab directory because that would break the learner product and remove the
easiest rollback. “Archive” means preserving the last known-good projection,
not deleting Fluent Engineering Lab. Studio compatibility endpoints may remain
for local diagnostics, but they are explicitly not a canonical writer.
