-- W05-010/W05-011: keep problem-solving and behavioral practice out of
-- unrelated Runtime/Testing buckets.
--
-- This is an additive, idempotent migration.  It never rewrites question
-- payloads or historical releases; it only updates the current
-- revision-scoped curriculum mapping rows and records the new editorial
-- source.  The previous mapping release remains immutable and is the
-- rollback reference.

insert into content.taxonomy_domain (stable_key, title, shared)
values
  ('domain.algorithms', 'Algorithms', true),
  ('domain.behavioral', 'Behavioral/English', true)
on conflict (stable_key) do update
set title = excluded.title,
    shared = excluded.shared;

update content.question_curriculum_mapping mapping
set domain_key = 'domain.algorithms',
    source = 'question-brain-editorial-topic-registry-v1/domain-separated-2026-08-27',
    updated_at = now()
from content.question_revision revision
join content.question question on question.current_revision_id = revision.id
where mapping.revision_id = revision.id
  and question.status = 'published'
  and mapping.path_key = 'path.algorithms'
  and mapping.domain_key is distinct from 'domain.algorithms';

update content.question_curriculum_mapping mapping
set domain_key = 'domain.behavioral',
    source = 'question-brain-editorial-topic-registry-v1/domain-separated-2026-08-27',
    updated_at = now()
from content.question_revision revision
join content.question question on question.current_revision_id = revision.id
where mapping.revision_id = revision.id
  and question.status = 'published'
  and mapping.path_key = 'path.behavioral'
  and mapping.domain_key is distinct from 'domain.behavioral';

-- Fail closed if a future schema edit makes the domain separation partial.
do $verify$
begin
  if exists (
    select 1
    from content.question_curriculum_mapping mapping
    join content.question_revision revision on revision.id = mapping.revision_id
    join content.question question on question.current_revision_id = revision.id
    where question.status = 'published'
      and mapping.path_key = 'path.algorithms'
      and mapping.domain_key <> 'domain.algorithms'
  ) then
    raise exception 'algorithms curriculum mapping still points outside domain.algorithms';
  end if;
  if exists (
    select 1
    from content.question_curriculum_mapping mapping
    join content.question_revision revision on revision.id = mapping.revision_id
    join content.question question on question.current_revision_id = revision.id
    where question.status = 'published'
      and mapping.path_key = 'path.behavioral'
      and mapping.domain_key <> 'domain.behavioral'
  ) then
    raise exception 'behavioral curriculum mapping still points outside domain.behavioral';
  end if;
end
$verify$;
