-- Promote level and company to first-class indexed dimensions (QB-BUG-4,
-- QB-BUG-5). Level already lives inside normalized_payload; lifting it into a
-- column makes "only Middle+" answerable by the API without a JSON scan.
-- Company is a new optional dimension sourced from the card's Company
-- metadata line; existing cards keep NULL, which is a legal empty value and
-- never falls back to a default employer.

alter table content.question
  add column if not exists level text;

update content.question q
set level = nullif(qr.normalized_payload->>'level', '')
from content.question_revision qr
where qr.id = q.current_revision_id;

create index if not exists question_level_idx on content.question (level);

alter table content.question
  add column if not exists company text;

create index if not exists question_company_idx on content.question (company);
