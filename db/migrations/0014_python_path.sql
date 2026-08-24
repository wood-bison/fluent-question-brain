-- Add the Python path to the explicit v1 curriculum registry.
--
-- Python cards were intentionally left unmapped by the first editorial
-- proposal because the original eight-path program had no Python route.  The
-- path is additive and does not rewrite revisions or accept any card mapping.

insert into content.taxonomy_path (stable_key, program_key, title)
values ('path.python', 'program.backend-engineer', 'Python')
on conflict (stable_key) do nothing;
