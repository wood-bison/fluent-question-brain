-- G4 additive migration: strip Runtime-owned solutions from the current
-- learner projection without rewriting any historical question_revision.
-- Run with psql against the intended workspace. It is idempotent: rows whose
-- current task block already carries contract_version v1 are ignored.

\set ON_ERROR_STOP on
begin;

create temp table g4_task_migration on commit drop as
with source as (
  select
    q.id as question_id,
    q.stable_key,
    qr.id as old_revision_id,
    qr.revision_no as old_revision_no,
    qr.source_system,
    qr.source_ref,
    qr.normalized_payload as old_payload,
    jsonb_set(
      jsonb_set(
        qr.normalized_payload,
        '{task}',
        ((qr.normalized_payload->'task') - 'solution') || jsonb_build_object(
          'contract_version', 'question-brain.task-brief.v1',
          'kind', 'historical_content'
        ),
        true
      ),
      '{sections}',
      coalesce((
        select jsonb_agg(section order by ordinal)
        from jsonb_array_elements(coalesce(qr.normalized_payload->'sections', '[]'::jsonb))
          with ordinality as sections(section, ordinal)
        where lower(coalesce(section->>'title', '')) not in
          ('solution', 'решение', 'эталонное решение')
      ), '[]'::jsonb),
      true
    ) as new_payload
  from content.question q
  join content.question_revision qr on qr.id = q.current_revision_id
  where q.status = 'published'
    and q.content_kind = 'production'
    and jsonb_typeof(qr.normalized_payload->'task') = 'object'
    and coalesce(qr.normalized_payload->'task'->>'contract_version', '') <> 'question-brain.task-brief.v1'
)
select *, encode(digest(new_payload::text, 'sha256'), 'hex') as new_hash
from source;

insert into content.question_revision
  (question_id, revision_no, content_hash, normalized_payload, source_system, source_ref, authored_by)
select
  question_id,
  old_revision_no + 1,
  new_hash,
  new_payload,
  'g4-task-boundary-migration',
  source_ref,
  'g4-task-boundary'
from g4_task_migration;

alter table g4_task_migration add column new_revision_id uuid;
update g4_task_migration migration
set new_revision_id = revision.id
from content.question_revision revision
where revision.question_id = migration.question_id
  and revision.content_hash = migration.new_hash;

do $$
begin
  if exists (select 1 from g4_task_migration where new_revision_id is null) then
    raise exception 'G4 migration could not resolve every new revision';
  end if;
end $$;

-- Copy learner locales while removing the legacy Solution section from the
-- rendered body. Prompt/answer/explanation remain unchanged.
insert into content.question_locale
  (revision_id, locale, prompt, short_answer, explanation, body)
select
  migration.new_revision_id,
  locale.locale,
  locale.prompt,
  locale.short_answer,
  locale.explanation,
  jsonb_set(
    locale.body,
    '{sections}',
    coalesce((
      select jsonb_agg(section order by ordinal)
      from jsonb_array_elements(coalesce(locale.body->'sections', '[]'::jsonb))
        with ordinality as sections(section, ordinal)
      where lower(coalesce(section->>'title', '')) not in
        ('solution', 'решение', 'эталонное решение')
    ), '[]'::jsonb),
    true
  )
from g4_task_migration migration
join content.question_locale locale on locale.revision_id = migration.old_revision_id;

insert into content.question_capability
  (revision_id, path_key, capability_key, mapping_state, mapping_version, source)
select migration.new_revision_id, relation.path_key, relation.capability_key,
  relation.mapping_state, relation.mapping_version, relation.source
from g4_task_migration migration
join content.question_capability relation on relation.revision_id = migration.old_revision_id;

insert into content.question_curriculum_mapping
  (revision_id, program_key, path_key, domain_key, capability_key, mapping_state,
   mapping_version, source, created_at, updated_at)
select migration.new_revision_id, mapping.program_key, mapping.path_key,
  mapping.domain_key, mapping.capability_key, mapping.mapping_state,
  mapping.mapping_version, mapping.source, mapping.created_at, mapping.updated_at
from g4_task_migration migration
join content.question_curriculum_mapping mapping on mapping.revision_id = migration.old_revision_id;

insert into content.placement_decision
  (revision_id, topic_id, decision, evidence, decided_by, decided_at)
select migration.new_revision_id, placement.topic_id, placement.decision,
  placement.evidence, placement.decided_by, placement.decided_at
from g4_task_migration migration
join content.placement_decision placement on placement.revision_id = migration.old_revision_id;

update content.question question
set current_revision_id = migration.new_revision_id,
    updated_at = now()
from g4_task_migration migration
where question.id = migration.question_id;

insert into content.audit_event
  (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
select question.workspace_id, 'question_revision', migration.new_revision_id,
  'question.task_brief.migrated', 'g4-task-boundary',
  jsonb_build_object(
    'old_revision_id', migration.old_revision_id::text,
    'new_revision_id', migration.new_revision_id::text,
    'stable_key', migration.stable_key,
    'reason', 'remove Runtime-owned solution from current learner projection'
  )
from g4_task_migration migration
join content.question question on question.id = migration.question_id;

insert into content.outbox_event
  (aggregate_type, aggregate_id, event_type, idempotency_key, payload)
select 'question_revision', migration.new_revision_id, 'question.revision.published',
  'question-publication:' || migration.new_revision_id::text,
  jsonb_build_object('reason', 'g4-task-boundary-migration', 'source', 'question-brain')
from g4_task_migration migration
on conflict (idempotency_key) do nothing;

select count(*) as migrated_cards from g4_task_migration;
commit;
