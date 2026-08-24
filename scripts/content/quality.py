"""Small dependency-free content checks shared by card generators.

The Go release gate is the authoritative runtime implementation. Keeping the
same checks here prevents a generator from silently writing a section heading
into Question (RU) or Question (EN) before the Go boundary sees the file.
"""

import unicodedata

PDF_HEADINGS = {"c", "sql", "-", ":", ";", "указатели", "jquery", "deepcopy"}


def normalize(value):
    return " ".join((value or "").strip().split()).casefold()


def has_question_mark(value):
    return "?" in value or "？" in value


def has_punctuation(value):
    return any(unicodedata.category(char).startswith(("P", "S")) for char in value)


def has_pdf_artifact(value):
    for char in value or "":
        if char in "\u00ad\u200b\u200c\u200d\u2060\ufffd":
            return True
        if unicodedata.category(char) == "Cc" and char not in "\n\r\t":
            return True
    return False


def is_pdf_heading(value):
    return normalize(value) in PDF_HEADINGS


def prompt_issues(prompt, answer="", title="", topic=""):
    prompt = (prompt or "").strip()
    if not prompt:
        return ["empty_prompt"]

    issues = []
    comparable = normalize(prompt)
    if normalize(answer) and comparable == normalize(answer):
        issues.append("prompt_equals_answer")
    if normalize(title) and comparable == normalize(title) and is_compact_label(prompt):
        issues.append("prompt_matches_title")
    if normalize(topic) and comparable == normalize(topic) and is_compact_label(prompt):
        issues.append("prompt_matches_topic")
    if is_pdf_heading(prompt):
        issues.append("pdf_heading_prompt")
    if has_pdf_artifact(prompt):
        issues.append("pdf_artifact")

    words = prompt.split()
    if len(words) == 1 and not has_question_mark(prompt) and not has_punctuation(prompt):
        issues.append("single_token_prompt")
    elif len(words) == 2 and len(prompt) < 20 and not has_question_mark(prompt) and not has_punctuation(prompt):
        issues.append("short_label_prompt")
    return issues


def is_compact_label(value):
    return len((value or "").split()) <= 2 and not has_question_mark(value or "")


def require_prompt(prompt, *, source, field, title="", topic="", answer=""):
    issues = prompt_issues(prompt, answer=answer, title=title, topic=topic)
    if issues:
        raise ValueError(
            f"{source}: {field} failed content quality gate ({', '.join(issues)})"
        )


def require_clean_text(value, *, source, field):
    if has_pdf_artifact(value):
        raise ValueError(f"{source}: {field} contains extracted PDF control characters")
