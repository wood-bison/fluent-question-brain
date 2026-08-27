-- W07 production graph evidence guards.
-- Confidence is an editorial signal, not a substitute for provenance or a
-- reviewer. Accepted edges must have a human/auditable decision record.

begin;

alter table content.question_edge_proposal
  drop constraint if exists question_edge_proposal_confidence_evidence;
alter table content.question_edge_proposal
  add constraint question_edge_proposal_confidence_evidence
  check (
    confidence is distinct from 1
    or (length(trim(rationale)) > 0 and length(trim(source)) > 0)
  );

alter table content.question_edge_proposal
  drop constraint if exists question_edge_proposal_accepted_review;
alter table content.question_edge_proposal
  add constraint question_edge_proposal_accepted_review
  check (
    status <> 'accepted'
    or (
      decided_at is not null
      and length(trim(coalesce(decided_by, ''))) > 0
      and length(trim(rationale)) > 0
    )
  );

alter table content.question_edge_release
  drop constraint if exists question_edge_release_confidence_evidence;
alter table content.question_edge_release
  add constraint question_edge_release_confidence_evidence
  check (
    confidence is distinct from 1
    or length(trim(rationale)) > 0
  );

commit;
