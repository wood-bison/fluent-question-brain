# G5 retirement checklist — 2026-08-22

This checklist protects the active Fluent Engineering Lab while the canonical
Question Brain becomes reusable by other products. “Retire” means remove the
old question-registry/runtime writer only after parity; it does not mean
archiving or deleting Lab.

## Required evidence

- [x] Question Brain G0–G5 smoke evidence is committed.
- [x] Backups and restore are repeatable (`docs/verification/g5-hardening-2026-08-22.md`).
- [x] Rollback is token-protected, immutable, audited, and outbox-backed.
- [x] Fluent Engineering Lab parity report covers the complete stable-key
      slice, both locales, body/section semantics, graph placement, and content
      hashes (`docs/verification/g4-lab-parity-2026-08-22.json` in Lab).
- [x] Lab runs with `QUESTION_BRAIN_READS=1` and shadow parity enabled with
      zero projection errors (`1146/1146` matches, zero fallback-causing
      mismatches).
- [x] Product-owner release sign-off recorded on 2026-08-22: the approved
      source-vault snapshot is canonical; Lab stays active and keeps its
      immutable archive as recovery. The old Lab Studio registry endpoints are
      compatibility-only and cannot publish source-vault content.

## Current decision

**Source-vault cutover complete.** Question Brain owns the published source
of truth and the `qb-release` command is the only approved vault publication
boundary. Fluent Engineering Lab remains an active supported product. Its
immutable archive stays available for outage recovery, and any legacy Studio
registry endpoints are compatibility-only diagnostics rather than a second
canonical writer. The Question Brain rollback path remains token-protected,
immutable, audited, and outbox-backed.
