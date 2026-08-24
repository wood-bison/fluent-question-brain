#!/usr/bin/env python3
"""Write served Russian questions back into the vault so the source is complete.

341 cards had their Russian question repaired in the database only. The repair
holds today because upsertCard guards against a degenerate overwrite — but a
guard protects an existing row, not an empty database. Rebuild from the vault
and the parser falls back to Core Idea (RU) again, quietly turning every one of
those questions back into its own answer.

This copies the live Russian prompt into the card file as an explicit
`## Question (RU)` section, for every card that is missing one. After it runs the
vault reproduces what is served instead of diverging from it.

Read-only against the database. Writes only to the vault.
"""
from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path

VAULT = Path.home() / "developer" / "fluent-question-vault"
# Cards are split across four folders; a concept card is no less a card.
CARD_DIRS = ("Question Cards", "Concept Cards", "Best Practice Cards", "Behavioral Cards")
COMPOSE = ["docker", "compose", "-f", "deploy/compose/compose.yaml", "exec", "-T", "postgres"]
SECTION = "## Question (RU)"

QUERY = """
select coalesce(json_agg(row_to_json(t)), '[]') from (
  select q.stable_key, ru.prompt
  from content.question q
  join content.question_revision r on r.id = q.current_revision_id
  join content.question_locale ru on ru.revision_id = r.id and ru.locale = 'ru'
  where q.content_kind = 'production'
    and not (r.normalized_payload->'sections' @> '[{"title":"Question (RU)"}]')
    and length(btrim(ru.prompt)) > 15
) t;
"""


def fetch() -> list[dict[str, str]]:
    out = subprocess.run(
        [*COMPOSE, "psql", "-U", "question_brain", "-d", "question_brain", "-tAc", QUERY],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    return json.loads(out)


def card_path(stable_key: str) -> Path | None:
    # question.q701 -> "Q701 — ...md"; the id keeps the source's own casing.
    ident = stable_key.split(".", 1)[-1].upper()
    for folder in CARD_DIRS:
        for path in (VAULT / folder).glob("*.md"):
            name = path.name
            if name.startswith(ident + " ") or name.startswith(ident + "."):
                return path
    return None


def insert(path: Path, question: str) -> str:
    text = path.read_text(encoding="utf-8")
    if SECTION in text:
        return "already present"
    first = re.search(r"^## ", text, flags=re.M)
    if not first:
        return "no sections"
    block = f"{SECTION}\n\n{question.strip()}\n\n"
    path.write_text(text[: first.start()] + block + text[first.start() :], encoding="utf-8")
    return "inserted"


def main() -> int:
    rows = fetch()
    print(f"cards missing an explicit Russian question: {len(rows)}")

    counts: dict[str, int] = {}
    missing: list[str] = []
    for row in rows:
        path = card_path(row["stable_key"])
        if path is None:
            missing.append(row["stable_key"])
            counts["file not found"] = counts.get("file not found", 0) + 1
            continue
        outcome = insert(path, row["prompt"])
        counts[outcome] = counts.get(outcome, 0) + 1

    for outcome, n in sorted(counts.items()):
        print(f"  {outcome}: {n}")
    if missing:
        print(f"  unmatched keys (first 10): {', '.join(missing[:10])}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
