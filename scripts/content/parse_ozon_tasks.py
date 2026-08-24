#!/usr/bin/env python3
"""Parse Ozon_Go_17-20grade.pdf (pdftotext -layout output) into JSON tasks.

Usage: pdftotext -layout <pdf> /tmp/qb/ozon_go.txt && \
       python3 scripts/content/parse_ozon_tasks.py /tmp/qb/ozon_go.txt out.json
"""
import json
import re
import sys

SECTIONS = ["ОС, сети и эксплуатация", "Базы данных", "Архитектура",
            "Структуры данных", "Go (теория)", "Go (практика)", "Скрининг"]
SECTION_RE = re.compile(r"GO\s+(" + "|".join(re.escape(s) for s in SECTIONS) + r")\s*$")
GRADES_RE = re.compile(r"((?:\d{2}\s*)+)грейд\s+(\d+)\s*мин\s+(\d+)\s*(?:раз|раза)?")
SCREENING_RE = re.compile(r"Скрининг\s+(\d+)\s*мин\s+(\d+)\s*(?:раз|раза)?")
UNKNOWN_RE = re.compile(r"Грейд неизвестен\s+(\d+)\s*(?:раз|раза)?")
GRADE_HEAD_RE = re.compile(r"^(\d{2})\s*грейд\s*$")
NUM_RE = re.compile(r"^\d+[.)]\s+")


def clean(line: str) -> str:
    return re.sub(r"\s+", " ", line.strip())


def main(src: str, dst: str) -> None:
    lines = open(src, encoding="utf-8").read().split("\n")
    tasks = []
    i = 0
    while i < len(lines):
        raw = lines[i]
        line = clean(raw)
        m = GRADES_RE.search(line)
        kind = "grades"
        if not m:
            m = SCREENING_RE.search(line)
            kind = "screening"
        if not m:
            m = UNKNOWN_RE.search(line)
            kind = "unknown"
        if not m:
            i += 1
            continue
        sec = None
        for j in range(i - 1, max(-1, i - 14), -1):
            sm = SECTION_RE.search(lines[j])
            if sm:
                sec = sm.group(1)
                break
        if kind == "grades":
            grades = line.split()[0].split() if False else re.findall(r"\d{2}", line.split(" грейд")[0])
            timing = f"{m.group(2)} мин"
            usage_num, usage_word = int(m.group(3)), False
            usage = f"{m.group(3)} " + ("раз" if not line.rstrip().endswith(("раза",)) else "раза")
        elif kind == "screening":
            grades = []
            timing = f"{m.group(1)} мин"
            usage = f"{m.group(2)} раз"
        else:
            grades = []
            timing = ""
            usage = f"{m.group(1)} раз"
        # collect body until next task header
        body: list[str] = []
        j = i + 1
        while j < len(lines):
            probe = clean(lines[j])
            if (GRADES_RE.search(probe) or SCREENING_RE.search(probe)
                    or UNKNOWN_RE.search(probe)):
                break
            body.append(lines[j])
            j += 1
        tasks.append({"grades": grades, "kind": kind, "timing": timing,
                      "usage": usage, "section": sec, "body": body})
        i = j
    # split body into condition (before Ответ) and graded answers
    for t in tasks:
        body = t.pop("body")
        cond_lines: list[str] = []
        answers: list[str] = []
        dest = cond_lines
        seen_answer_head = False
        for ln in body:
            stripped = clean(ln)
            if not seen_answer_head and stripped == "Ответ":
                seen_answer_head = True
                dest = answers
                continue
            dest.append(ln.rstrip())
        t["condition"] = clean_blocks(cond_lines)
        t["answers_raw"] = answers
    data = [finalize(t) for t in tasks]
    json.dump(data, open(dst, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print(f"tasks: {len(data)}")


def clean_blocks(lines):
    out, buf = [], []
    for ln in lines:
        if ln.strip() == "":
            if buf:
                out.append("\n".join(buf).strip())
                buf = []
        else:
            buf.append(ln.strip())
    if buf:
        out.append("\n".join(buf).strip())
    return out


def finalize(t):
    """Split answers_raw into per-grade rubric entries."""
    rubric, current = [], None
    extra_q = None
    for ln in t["answers_raw"]:
        s = clean(ln)
        gh = GRADE_HEAD_RE.match(s)
        if gh:
            current = {"label": f"{gh.group(1)} грейд", "text": []}
            rubric.append(current)
            extra_q = None
            continue
        if s == "Дополнительные вопросы:":
            extra_q = "questions"
            if current is not None:
                current["text"].append("Дополнительные вопросы:")
            continue
        if s == "Ответы:":
            extra_q = "answers"
            if current is not None:
                current["text"].append("Ответы:")
            continue
        target = current["text"] if current is not None else None
        if target is None:
            # answer text before any grade head: treat as common note
            if s:
                rubric.append({"label": "note", "text": [s]})
            continue
        if NUM_RE.match(s):
            target.append(s)
        elif s:
            if target and (target[-1].endswith((":",)) or NUM_RE.match(target[-1] or "")):
                target[-1] += " " + s
            else:
                target.append(s)
    return {
        "grades": t["grades"],
        "kind": t["kind"],
        "timing": t["timing"],
        "usage": t["usage"],
        "section": t["section"],
        "condition": t["condition"],
        "rubric": [{"label": r["label"], "text": "\n".join(r["text"]).strip()}
                   for r in rubric],
    }


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
