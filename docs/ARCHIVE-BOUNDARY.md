# Legacy and archive boundary

The existing projects remain intact during migration:

| Path | Role now | Role during migration |
| --- | --- | --- |
| `/Users/sergeyzhechko/developer/fluent-engineering-lab` | Interview-learning product | Active product and future Question Brain API client |
| `/Users/sergeyzhechko/developer/fluent-question-vault` | Git history mirror of Obsidian cards | Import snapshot and export verification source |
| `fluent-question-brain` (this repo) | New canonical service | G1 source of truth candidate |

We do not move or delete the 8.7 GB Lab directory before G1/G2 evidence,
because that would break the current dev server and remove the easiest
rollback. “Archive” means freezing only the old question-registry/runtime
path, tagging its last known-good state, keeping the content mirror immutable,
and removing that legacy path only after the release checklist is green. The
Lab itself remains a supported product. Any later move will be a recoverable
rename into a dated `developer/archive/` directory, never a delete.

