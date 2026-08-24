#!/usr/bin/env python3
"""Produce a deterministic, no-write capability migration report.

The script deliberately reads task manifests and Lab's adapter as consumers,
then queries only aggregate capability keys from Question Brain. It never
rewrites content, releases, or learner evidence.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
from collections import Counter, defaultdict
from pathlib import Path


CAPABILITY_RE = re.compile(r'capability\.[a-z0-9-]+\.[a-z0-9][a-z0-9-]*')


def psql_counts(container: str, sql: str) -> dict[str, int]:
    try:
        output = subprocess.check_output(
            ["docker", "exec", container, "psql", "-U", "question_brain", "-d", "question_brain", "-At", "-F", "\t", "-c", sql],
            text=True,
            stderr=subprocess.DEVNULL,
        )
    except (FileNotFoundError, subprocess.CalledProcessError):
        return {}
    result: dict[str, int] = {}
    for line in output.splitlines():
        key, _, value = line.partition("\t")
        if key and value.isdigit():
            result[key] = int(value)
    return result


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument("--as-of", default="2026-08-24")
    parser.add_argument("--postgres-container", default=os.environ.get("QB_PG_CONTAINER", "fluent-question-brain-postgres-1"))
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[1]
    workspace = repo.parent
    manifest_path = repo / "docs/manifests/capability-registry-2026-08-24.json"
    manifest = json.loads(manifest_path.read_text())
    dispositions = {item["oldKey"]: item for item in manifest["dispositions"]}
    consumers: dict[str, list[str]] = defaultdict(list)
    task_refs: Counter[str] = Counter()
    for task_path in sorted((workspace / "fluent-task-runtime/tasks").glob("*/task.json")):
        task = json.loads(task_path.read_text())
        for key in task.get("capabilityKeys", []):
            task_refs[key] += 1
            consumers[key].append(f"task-runtime:{task_path.parent.name}")
    adapter = workspace / "fluent-engineering-lab/apps/learning-api/src/app/question-brain/question-brain-taxonomy.adapter.ts"
    for key in sorted(set(CAPABILITY_RE.findall(adapter.read_text()))):
        consumers[key].append("lab:question-brain-taxonomy.adapter.ts")
    mapping_refs = psql_counts(args.postgres_container, "select capability_key, count(*) from content.question_curriculum_mapping where capability_key is not null group by capability_key order by capability_key")
    direct_refs = psql_counts(args.postgres_container, "select capability_key, count(*) from content.question_capability group by capability_key order by capability_key")
    for key in sorted(set(task_refs) | set(mapping_refs) | set(direct_refs) | set(consumers)):
        if key not in dispositions:
            consumers[key].append("UNRESOLVED")
    unresolved = sorted(key for key, refs in consumers.items() if "UNRESOLVED" in refs)
    report = {
        "contractVersion": "question-brain.capability-migration-dry-run.v1",
        "asOf": args.as_of,
        "registryReleaseId": manifest["registryReleaseId"],
        "writeMode": "none",
        "source": {"taskRuntimeTaskCount": len(list((workspace / "fluent-task-runtime/tasks").glob("*/task.json"))), "labAdapter": str(adapter.relative_to(workspace))},
        "currentCapabilityKeyCount": len(dispositions),
        "dispositionCounts": {kind: sum(1 for item in dispositions.values() if item["disposition"] == kind) for kind in ["keep", "rename", "split", "merge", "retire"]},
        "references": {
            key: {
                "disposition": dispositions[key]["disposition"],
                "canonicalKey": dispositions[key]["canonicalKey"],
                "taskRuntimeRefs": task_refs.get(key, 0),
                "questionCurriculumMappingRefs": mapping_refs.get(key, 0),
                "questionCapabilityRefs": direct_refs.get(key, 0),
                "consumers": sorted(consumers.get(key, [])),
            }
            for key in sorted(dispositions)
        },
        "unresolved": unresolved,
        "acceptance": {"allCurrentKeysHaveDisposition": not unresolved, "noWritesPerformed": True},
    }
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"report": str(args.report), "unresolved": unresolved, "writeMode": "none"}, ensure_ascii=False))
    return 0 if not unresolved else 2


if __name__ == "__main__":
    raise SystemExit(main())
