-- Canonical identity cutover. The importer now emits explicit namespaces so
-- an old source label can never leak into graph identity or topic joins.
update content.question
set stable_key = regexp_replace(stable_key, '^legacy[.]', 'question.')
where stable_key like 'legacy.%';

update content.topic
set stable_key = regexp_replace(stable_key, '^legacy[.]', 'topic.')
where stable_key like 'legacy.%';

update content.question
set slug = regexp_replace(slug, '^legacy-', '')
where slug like 'legacy-%';
