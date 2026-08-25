# Payload authoring surface (G4)

Payload is the editorial surface for the bank. It owns drafts, review history,
localization, and promotion requests; the Go Question Brain service remains the
single writer of canonical `content` rows and graph releases.

## Two deliberately different collections

- `questions` is the editable authoring collection. Its `afterChange` hook
  promotes a reviewed document through the Go API; it never writes canonical
  tables directly.
- `published-questions` is a read-only projection of the released bank. A
  migration creates `cms.published_questions` as a view over the Go-owned
  `content.question` and `content.question_revision` tables. It is useful for
  editorial inspection and reconciliation without creating a second copy of
  1,392 cards. Access rules deny all writes and the database view is not
  automatically updatable as a second safety boundary.

The projection is intentionally limited to `status = 'published'` and
`content_kind = 'production'`. Fixtures used by smoke tests therefore do not
appear in the editorial bank. The view carries the stable graph key, both
locales, taxonomy facets, revision number, and content hash so a reviewer can
compare what the CMS shows with the Question Brain release.

## Local operation

The Compose stack runs the CMS on port `48128` and applies committed migrations
before starting Next:

```sh
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml up -d --build cms
open http://localhost:48128/admin
```

For a type/build check without Docker:

```sh
npm ci
npm run typecheck
npm run generate:types
npm run build
```

This app intentionally uses its own `package-lock.json`: run these commands
with `npm` from `apps/cms`, not `pnpm --dir apps/cms`. The Lab monorepo uses
pnpm, but installing its workspace dependencies into this Payload app can
move or replace the CMS dependency tree and make a healthy Compose container
look broken on the host.

Do not add a second local catalogue, fallback data source, or direct SQL write
from Payload. If the published projection and the API disagree, investigate
the release/migration path rather than editing the view by hand.
