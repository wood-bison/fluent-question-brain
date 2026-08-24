#!/usr/bin/env python3
"""Generate the reviewed-topic crosswalk proposal for Question Brain.

This is an editorial registry, not a classifier.  Every topic title below is
listed explicitly and joined by its exact canonical topic key from the live
catalogue.  The generator never matches prefixes, titles, tracks, groups,
breadcrumbs, embeddings, or task hints.  A topic which is absent from the
registry stays ``unmapped`` and is reported for review.

The output is deliberately ``proposed``.  An operator may review the report,
change individual rows in the checked-in manifest, and only then run
``qb-map-release --approve``.  This keeps editorial decisions in a revision-
pinned release file and prevents a convenience script from becoming a hidden
fallback taxonomy.
"""

from __future__ import annotations

import argparse
import json
import urllib.parse
import urllib.request
from collections import Counter
from pathlib import Path
from typing import Any


PROGRAM = "program.backend-engineer"
SOURCE = "question-brain-editorial-topic-registry-v1"
CONTRACT = "question-brain.curriculum-mapping.v1"
TAXONOMY = "question-brain.taxonomy.v1"


def fetch_catalog(url: str, workspace: str) -> dict[str, Any]:
    query = urllib.parse.urlencode({"workspace": workspace, "locale": "en", "limit": "2000"})
    request = urllib.request.Request(f"{url.rstrip('/')}/v1/catalog?{query}", headers={"Accept": "application/json"})
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def load_registry(path: Path) -> dict[str, tuple[str, str, str]]:
    payload = load_json(path)
    if payload.get("contract_version") != "question-brain.curriculum-topic-registry.v1":
        raise ValueError("registry contract_version must be question-brain.curriculum-topic-registry.v1")
    if payload.get("taxonomy_version") != TAXONOMY:
        raise ValueError(f"registry taxonomy_version must be {TAXONOMY}")
    decisions: dict[str, tuple[str, str, str]] = {}
    for index, item in enumerate(payload.get("entries", [])):
        title = str(item.get("topic_title", "")).strip()
        path_key = str(item.get("path_key", "")).strip()
        domain_key = str(item.get("domain_key", "")).strip()
        rationale = str(item.get("rationale", "")).strip()
        review_state = str(item.get("review_state", "")).strip().lower()
        if not title or not path_key or not domain_key or not rationale:
            raise ValueError(f"registry entry {index} is missing an explicit decision field")
        if review_state != "proposed":
            raise ValueError(f"registry entry {index} must remain proposed until review: {title}")
        if title in decisions:
            raise ValueError(f"registry contains duplicate topic title: {title}")
        decisions[title] = (path_key, domain_key, rationale)
    if not decisions:
        raise ValueError("registry must contain at least one topic decision")
    return decisions


def generate(catalog: dict[str, Any], preserve: dict[str, Any] | None, decisions: dict[str, tuple[str, str, str]]) -> tuple[dict[str, Any], dict[str, Any]]:
    questions = catalog.get("questions", [])
    preserved = {entry["stable_key"]: entry for entry in (preserve or {}).get("entries", [])}
    entries: list[dict[str, Any]] = []
    unmapped_topics: Counter[str] = Counter()
    proposed_by_path: Counter[str] = Counter()
    proposed_by_domain: Counter[str] = Counter()
    preserved_count = 0

    for item in questions:
        stable_key = item["stable_key"]
        existing = preserved.get(stable_key)
        if existing and existing.get("mapping_state") == "accepted":
            entries.append(existing)
            preserved_count += 1
            continue
        topics = item.get("topics", [])
        primary = next((topic for topic in topics if topic.get("relation") == "primary"), None)
        if primary is None and len(topics) == 1:
            primary = topics[0]
        decision = decisions.get((primary or {}).get("title", ""))
        entry: dict[str, Any] = {
            "stable_key": stable_key,
            "revision_id": item["revision_id"],
            "content_hash": item["content_hash"],
            "mapping_state": "unmapped",
            "source": SOURCE,
        }
        if decision:
            path_key, domain_key, _ = decision
            entry.update({
                "program_key": PROGRAM,
                "path_key": path_key,
                "domain_key": domain_key,
                "mapping_state": "proposed",
                "mapping_version": TAXONOMY,
            })
            proposed_by_path[path_key] += 1
            proposed_by_domain[domain_key] += 1
        else:
            unmapped_topics[(primary or {}).get("title", "<missing topic>")] += 1
        entries.append(entry)

    manifest = {
        "contract_version": CONTRACT,
        "taxonomy_version": TAXONOMY,
        "workspace_key": catalog.get("workspace_key", "fluent-interview"),
        "source": SOURCE,
        "entries": sorted(entries, key=lambda entry: entry["stable_key"]),
    }
    report = {
        "contract_version": "question-brain.curriculum-topic-review.v1",
        "source": SOURCE,
        "workspace_key": manifest["workspace_key"],
        "question_release_id": catalog.get("release_id"),
        "published": len(questions),
        "registry_topics": len(decisions),
        "preserved_accepted": preserved_count,
        "proposed": sum(proposed_by_path.values()),
        "unmapped": sum(unmapped_topics.values()),
        "proposed_by_path": dict(sorted(proposed_by_path.items())),
        "proposed_by_domain": dict(sorted(proposed_by_domain.items())),
        "unmapped_topics": [
            {"title": title, "cards": count}
            for title, count in sorted(unmapped_topics.items(), key=lambda item: (-item[1], item[0]))
        ],
        "decision_policy": "exact-topic-registry-only; proposed rows require editorial review before acceptance",
    }
    return manifest, report


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--question-brain", default="http://127.0.0.1:48127")
    parser.add_argument("--workspace", default="fluent-interview")
    parser.add_argument("--catalog", type=Path, help="saved /v1/catalog response")
    parser.add_argument("--preserve", type=Path, help="existing manifest whose accepted rows must survive")
    parser.add_argument(
        "--registry",
        type=Path,
        default=Path(__file__).resolve().parents[2] / "docs/manifests/curriculum-topic-registry-2026-08-24.json",
        help="machine-readable exact-topic editorial registry",
    )
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--report", required=True, type=Path)
    args = parser.parse_args()

    catalog = load_json(args.catalog) if args.catalog else fetch_catalog(args.question_brain, args.workspace)
    preserve = load_json(args.preserve) if args.preserve else None
    decisions = load_registry(args.registry)
    manifest, report = generate(catalog, preserve, decisions)
    args.manifest.parent.mkdir(parents=True, exist_ok=True)
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.manifest.write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    args.report.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({"manifest": str(args.manifest), "report": str(args.report), **{k: report[k] for k in ("published", "proposed", "unmapped", "preserved_accepted")}}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
