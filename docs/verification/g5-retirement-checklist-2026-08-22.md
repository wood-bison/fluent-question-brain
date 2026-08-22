# G5 retirement checklist — 2026-08-22

This checklist protects the active Fluent Engineering Lab while the canonical
Question Brain becomes reusable by other products. “Retire” means remove the
old question-registry/runtime writer only after parity; it does not mean
archiving or deleting Lab.

## Required evidence

- [x] Question Brain G0–G5 smoke evidence is committed.
- [x] Backups and restore are repeatable (`docs/verification/g5-hardening-2026-08-22.md`).
- [x] Rollback is token-protected, immutable, audited, and outbox-backed.
- [ ] Fluent Engineering Lab parity report covers the agreed stable-key slice,
      both locales, body/section semantics, graph placement, and content hashes.
- [ ] Lab can run with `QUESTION_BRAIN_READS=1` for a disposable profile with
      zero projection errors.
- [ ] Product owner signs the release that removes the old registry/runtime
      writer.

## Current decision

**HOLD legacy removal.** The old registry remains read-only and recoverable;
Fluent Engineering Lab remains an active supported product. Once the Lab
parity items above are signed in the Lab repository, remove only the legacy
writer, keep the archive mirror, and retain the Question Brain rollback path.
