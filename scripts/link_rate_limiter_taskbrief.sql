-- G4 reference join: attach the reviewed rate-limiter TaskFamily to the
-- existing conceptual question without copying executable source. The old
-- question revision remains immutable and the operation is idempotent.

\set ON_ERROR_STOP on
begin;

create temp table g4_rate_limiter_link on commit drop as
select
  q.id as question_id,
  q.workspace_id,
  qr.id as old_revision_id,
  qr.revision_no as old_revision_no,
  qr.source_ref,
  jsonb_set(
    qr.normalized_payload,
    '{task}',
    jsonb_build_object(
      'contract_version', 'question-brain.task-brief.v1',
      'kind', 'runtime_task_reference',
      'task_family_key', 'task-family.rate-limiter',
      'condition', 'Implement a token bucket rate limiter for the stated request contract.',
      'starter', 'allow(clientId, timestamp)',
      'walkthrough', 'Explain the invariant, burst boundary, reset behaviour, and complexity.',
      'difficulty', 'MEDIUM'
    ),
    true
  ) as new_payload
from content.question q
join content.question_revision qr on qr.id = q.current_revision_id
where q.stable_key = 'question.q315'
  and q.status = 'published'
  and q.content_kind = 'production'
  and coalesce(qr.normalized_payload->'task'->>'contract_version', '') <> 'question-brain.task-brief.v1';

alter table g4_rate_limiter_link add column new_hash text;
update g4_rate_limiter_link
set new_hash = encode(digest(new_payload::text, 'sha256'), 'hex');

insert into content.question_revision
  (question_id, revision_no, content_hash, normalized_payload, source_system, source_ref, authored_by)
select question_id, old_revision_no + 1, new_hash, new_payload,
  'g4-task-boundary-migration', 'task-family.rate-limiter', 'g4-task-boundary'
from g4_rate_limiter_link;

alter table g4_rate_limiter_link add column new_revision_id uuid;
update g4_rate_limiter_link migration
set new_revision_id = revision.id
from content.question_revision revision
where revision.question_id = migration.question_id
  and revision.content_hash = migration.new_hash;

insert into content.question_locale
  (revision_id, locale, prompt, short_answer, explanation, body)
select migration.new_revision_id, locale.locale, locale.prompt,
  locale.short_answer, locale.explanation, locale.body
from g4_rate_limiter_link migration
join content.question_locale locale on locale.revision_id = migration.old_revision_id;

insert into content.question_capability
  (revision_id, path_key, capability_key, mapping_state, mapping_version, source)
select migration.new_revision_id, relation.path_key, relation.capability_key,
  relation.mapping_state, relation.mapping_version, relation.source
from g4_rate_limiter_link migration
join content.question_capability relation on relation.revision_id = migration.old_revision_id;

insert into content.question_curriculum_mapping
  (revision_id, program_key, path_key, domain_key, capability_key, mapping_state,
   mapping_version, source, created_at, updated_at)
select migration.new_revision_id, mapping.program_key, mapping.path_key,
  mapping.domain_key, mapping.capability_key, mapping.mapping_state,
  mapping.mapping_version, mapping.source, mapping.created_at, mapping.updated_at
from g4_rate_limiter_link migration
join content.question_curriculum_mapping mapping on mapping.revision_id = migration.old_revision_id;

insert into content.placement_decision
  (revision_id, topic_id, decision, evidence, decided_by, decided_at)
select migration.new_revision_id, placement.topic_id, placement.decision,
  placement.evidence, placement.decided_by, placement.decided_at
from g4_rate_limiter_link migration
join content.placement_decision placement on placement.revision_id = migration.old_revision_id;

update content.question question
set current_revision_id = migration.new_revision_id, updated_at = now()
from g4_rate_limiter_link migration
where question.id = migration.question_id;

insert into content.audit_event
  (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
select migration.workspace_id, 'question_revision', migration.new_revision_id,
  'question.task_family.linked', 'g4-task-boundary',
  jsonb_build_object('old_revision_id', migration.old_revision_id::text,
    'new_revision_id', migration.new_revision_id::text,
    'task_family_key', 'task-family.rate-limiter')
from g4_rate_limiter_link migration;

insert into content.outbox_event
  (aggregate_type, aggregate_id, event_type, idempotency_key, payload)
select 'question_revision', migration.new_revision_id, 'question.revision.published',
  'question-publication:' || migration.new_revision_id::text,
  jsonb_build_object('reason', 'g4-task-family-link', 'source', 'question-brain')
from g4_rate_limiter_link migration
on conflict (idempotency_key) do nothing;

select count(*) as linked_cards from g4_rate_limiter_link;
commit;
