#!/usr/bin/env python3
"""Build the first reviewed crosswalk from released executable tasks.

The mapping is intentionally small and explicit.  It does not classify cards
from Track, Group, Topic, title, or embeddings.  Every other production card
is emitted as an ``unmapped`` audit row so the manifest remains complete and
revision-pinned while editorial work continues.
"""

from __future__ import annotations

import argparse
import json
import urllib.parse
import urllib.request
from pathlib import Path


QUESTION_MAP = {
    "question.c009": ("path.nodejs-typescript", "domain.runtime", "capability.runtime.node-event-loop-001"),
    "question.q775": ("path.nodejs-typescript", "domain.runtime", "capability.runtime.node-cpu-bound-002"),
    "question.q792": ("path.nodejs-typescript", "domain.runtime", "capability.runtime.node-streams-003"),
    "question.q816": ("path.nodejs-typescript", "domain.runtime", "capability.runtime.node-memory-004"),
    "question.q1062": ("path.nodejs-typescript", "domain.runtime", "capability.runtime.node-concurrency-012"),
    "question.q937": ("path.dotnet-csharp", "domain.runtime", "capability.runtime.dotnet-cancellation-001"),
    "question.q977": ("path.nodejs-typescript", "domain.runtime", "capability.runtime.fluent-calculator"),
    "question.c142": ("path.nodejs-typescript", "domain.runtime", "capability.runtime.deferred"),
    "question.q777": ("path.nodejs-typescript", "domain.runtime", "capability.runtime.deferred"),
    "question.q195": ("path.nodejs-typescript", "domain.http-api", "capability.http-api.node-auth-015"),
    "question.q416": ("path.nodejs-typescript", "domain.http-api", "capability.http-api.node-auth-015"),
    "question.q206": ("path.nodejs-typescript", "domain.delivery-observability", "capability.delivery-observability.node-cache-014"),
    "question.q722": ("path.nodejs-typescript", "domain.distributed-systems", "capability.distributed-systems.node-idempotency-013"),
    "question.q695": ("path.system-design", "domain.data-postgresql", "capability.data-postgresql.pg-indexes-008"),
    "question.q700": ("path.system-design", "domain.data-postgresql", "capability.data-postgresql.pg-indexes-008"),
    "question.q059": ("path.system-design", "domain.data-postgresql", "capability.data-postgresql.pg-locks-016"),
    "question.q315": ("path.system-design", "domain.distributed-systems", "capability.distributed-systems.rate-limiter"),
    "question.q444": ("path.system-design", "domain.distributed-systems", "capability.distributed-systems.rate-limiter"),
    "question.c024": ("path.system-design", "domain.http-api", "capability.http-api.rate-limiter"),
}


def fetch_release(url: str, workspace: str) -> dict:
    query = urllib.parse.urlencode({"workspace": workspace})
    request = urllib.request.Request(
        f"{url.rstrip('/')}/v1/release?{query}",
        headers={"Accept": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--question-brain", default="http://127.0.0.1:48127")
    parser.add_argument("--workspace", default="fluent-interview")
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()

    release = fetch_release(args.question_brain, args.workspace)
    entries = []
    for item in release.get("items", []):
        stable_key = item["stable_key"]
        entry = {
            "stable_key": stable_key,
            "revision_id": item["revision_id"],
            "content_hash": item["content_hash"],
            "mapping_state": "unmapped",
            "source": "question-brain-i2-runtime-crosswalk-2026-08-24",
        }
        placement = QUESTION_MAP.get(stable_key)
        if placement:
            path_key, domain_key, capability_key = placement
            entry.update(
                {
                    "program_key": "program.backend-engineer",
                    "path_key": path_key,
                    "domain_key": domain_key,
                    "capability_key": capability_key,
                    "mapping_state": "accepted",
                    "mapping_version": "question-brain.taxonomy.v1",
                }
            )
        entries.append(entry)

    manifest = {
        "contract_version": "question-brain.curriculum-mapping.v1",
        "taxonomy_version": "question-brain.taxonomy.v1",
        "workspace_key": args.workspace,
        "source": "question-brain-i2-runtime-crosswalk-2026-08-24",
        "entries": sorted(entries, key=lambda entry: entry["stable_key"]),
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    mapped = sum(1 for entry in entries if entry["mapping_state"] == "accepted")
    print(json.dumps({"question_release_id": release["release_id"], "entries": len(entries), "accepted": mapped, "unmapped": len(entries) - mapped, "output": str(args.output)}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
