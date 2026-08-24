#!/usr/bin/env python3
"""Create an explicitly accepted curriculum crosswalk from an exact registry.

This command is an editorial boundary, not a classifier.  It accepts only
proposal rows whose released primary topic title is present in the checked-in
topic registry.  It never reads Track, Group, card titles, breadcrumbs,
embeddings, or task metadata.  A missing registry entry remains ``unmapped``
and is reported for follow-up.

The deliberately noisy ``--accept-exact-topic-registry`` flag is required so
that a release cannot be promoted by accidentally invoking a convenience
script.  The resulting manifest is still revision/content-hash pinned and
must pass ``qb-map-release --approve`` separately.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import urllib.parse
import urllib.request
from collections import Counter
from pathlib import Path
from typing import Any


CONTRACT = "question-brain.curriculum-mapping.v1"
TAXONOMY = "question-brain.taxonomy.v1"
REGISTRY_CONTRACT = "question-brain.curriculum-topic-registry.v1"
REPORT_CONTRACT = "question-brain.curriculum-topic-acceptance.v1"
SOURCE = "question-brain-editorial-topic-registry-v1/reviewed-2026-08-24"


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def fetch_catalog(url: str, workspace: str) -> dict[str, Any]:
    query = urllib.parse.urlencode({"workspace": workspace, "locale": "en", "limit": "2000"})
    request = urllib.request.Request(
        f"{url.rstrip('/')}/v1/catalog?{query}",
        headers={"Accept": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def registry_decisions(path: Path) -> tuple[dict[str, dict[str, str]], str]:
    raw = path.read_bytes()
    payload = json.loads(raw)
    if payload.get("contract_version") != REGISTRY_CONTRACT:
        raise ValueError(f"registry contract_version must be {REGISTRY_CONTRACT}")
    if payload.get("taxonomy_version") != TAXONOMY:
        raise ValueError(f"registry taxonomy_version must be {TAXONOMY}")
    decisions: dict[str, dict[str, str]] = {}
    for index, item in enumerate(payload.get("entries", [])):
        title = str(item.get("topic_title", "")).strip()
        path_key = str(item.get("path_key", "")).strip()
        domain_key = str(item.get("domain_key", "")).strip()
        rationale = str(item.get("rationale", "")).strip()
        state = str(item.get("review_state", "")).strip().lower()
        if not title or not path_key or not domain_key or not rationale:
            raise ValueError(f"registry entry {index} is missing a decision field")
        if state != "proposed":
            raise ValueError(f"registry entry {index} must be proposed before this explicit review: {title}")
        if title in decisions:
            raise ValueError(f"registry contains duplicate topic title: {title}")
        decisions[title] = {"path_key": path_key, "domain_key": domain_key, "rationale": rationale}
    if not decisions:
        raise ValueError("registry must contain at least one topic decision")
    return decisions, hashlib.sha256(raw).hexdigest()


def primary_topic(item: dict[str, Any]) -> str:
    topics = item.get("topics", [])
    primary = next((topic for topic in topics if topic.get("relation") == "primary"), None)
    if primary is None and len(topics) == 1:
        primary = topics[0]
    return str((primary or {}).get("title", "")).strip()


def build(
    proposal: dict[str, Any],
    catalog: dict[str, Any],
    decisions: dict[str, dict[str, str]],
    registry_sha256: str,
    reviewer: str,
) -> tuple[dict[str, Any], dict[str, Any]]:
    if proposal.get("contract_version") != CONTRACT:
        raise ValueError(f"proposal contract_version must be {CONTRACT}")
    if proposal.get("taxonomy_version") != TAXONOMY:
        raise ValueError(f"proposal taxonomy_version must be {TAXONOMY}")
    questions = {item["stable_key"]: item for item in catalog.get("questions", [])}
    entries: list[dict[str, Any]] = []
    accepted_by_path: Counter[str] = Counter()
    accepted_by_domain: Counter[str] = Counter()
    preserved = 0
    accepted = 0
    unmapped = 0
    missing_catalog = 0
    for original in proposal.get("entries", []):
        entry = dict(original)
        stable_key = str(entry.get("stable_key", "")).strip()
        state = str(entry.get("mapping_state", "unmapped")).strip().lower()
        if state == "accepted":
            entries.append(entry)
            preserved += 1
            continue
        if state not in {"proposed", "unmapped"}:
            raise ValueError(f"proposal row {stable_key} has unsupported state {state!r}")
        item = questions.get(stable_key)
        if item is None:
            missing_catalog += 1
            entry["mapping_state"] = "unmapped"
            for key in ("program_key", "path_key", "domain_key", "capability_key", "mapping_version"):
                entry.pop(key, None)
            entries.append(entry)
            unmapped += 1
            continue
        decision = decisions.get(primary_topic(item))
        if decision is None:
            entry["mapping_state"] = "unmapped"
            for key in ("program_key", "path_key", "domain_key", "capability_key", "mapping_version"):
                entry.pop(key, None)
            entries.append(entry)
            unmapped += 1
            continue
        entry.update(
            {
                "program_key": "program.backend-engineer",
                "path_key": decision["path_key"],
                "domain_key": decision["domain_key"],
                "mapping_state": "accepted",
                "mapping_version": TAXONOMY,
                "source": SOURCE,
            }
        )
        entries.append(entry)
        accepted += 1
        accepted_by_path[decision["path_key"]] += 1
        accepted_by_domain[decision["domain_key"]] += 1

    manifest = {
        "contract_version": CONTRACT,
        "taxonomy_version": TAXONOMY,
        "workspace_key": proposal.get("workspace_key", "fluent-interview"),
        "source": SOURCE,
        "entries": sorted(entries, key=lambda entry: entry["stable_key"]),
    }
    report = {
        "contract_version": REPORT_CONTRACT,
        "taxonomy_version": TAXONOMY,
        "workspace_key": manifest["workspace_key"],
        "question_release_id": catalog.get("release_id"),
        "reviewer": reviewer,
        "review_method": "exact-primary-topic-registry-v1",
        "registry_sha256": registry_sha256,
        "proposal_entries": len(proposal.get("entries", [])),
        "catalog_questions": len(questions),
        "preserved_accepted": preserved,
        "accepted_from_registry": accepted,
        "accepted_total": preserved + accepted,
        "unmapped": unmapped,
        "missing_catalog": missing_catalog,
        "accepted_by_path": dict(sorted(accepted_by_path.items())),
        "accepted_by_domain": dict(sorted(accepted_by_domain.items())),
        "unknown_topics": sorted(
            {
                primary_topic(questions[item["stable_key"]])
                for item in proposal.get("entries", [])
                if item.get("stable_key") in questions
                and item.get("mapping_state") != "accepted"
                and primary_topic(questions[item["stable_key"]]) not in decisions
            }
        ),
        "decision_policy": "accept only exact primary-topic registry rows; no fuzzy or legacy-field inference",
    }
    return manifest, report


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--proposal", required=True, type=Path)
    parser.add_argument("--catalog", type=Path)
    parser.add_argument("--question-brain", default="http://127.0.0.1:48127")
    parser.add_argument("--workspace", default="fluent-interview")
    parser.add_argument("--registry", required=True, type=Path)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument("--reviewer", default="codex-editorial-review")
    parser.add_argument(
        "--accept-exact-topic-registry",
        action="store_true",
        help="required explicit operator confirmation for this editorial action",
    )
    args = parser.parse_args()
    if not args.accept_exact_topic_registry:
        parser.error("refusing to accept rows without --accept-exact-topic-registry")
    proposal = load_json(args.proposal)
    catalog = load_json(args.catalog) if args.catalog else fetch_catalog(args.question_brain, args.workspace)
    decisions, registry_sha256 = registry_decisions(args.registry)
    manifest, report = build(proposal, catalog, decisions, registry_sha256, args.reviewer)
    args.manifest.parent.mkdir(parents=True, exist_ok=True)
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.manifest.write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    args.report.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({"manifest": str(args.manifest), "report": str(args.report), **{key: report[key] for key in ("proposal_entries", "accepted_total", "unmapped")}}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
