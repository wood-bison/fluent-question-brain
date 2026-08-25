# G9 Payload decision — verification evidence

Date: 2026-08-25

The G9 Workbench acceptance condition is met for the review matrix, so the
Payload future decision is recorded explicitly: **keep Payload as a draft-only
authoring surface; keep Question Brain as the sole canonical published writer**.

## Checks

```bash
cd /Users/sergeyzhechko/developer/fluent-interview/fluent-question-brain/apps/cms
npm ci --ignore-scripts
npm run typecheck
npm run build
```

Observed: typecheck passed; Next production build passed with routes `/`,
`/admin/[[...segments]]`, and `/api/[...slug]`.

The live Compose checks also passed:

```bash
docker compose -p fluent-question-brain -f \
  /Users/sergeyzhechko/developer/fluent-interview/fluent-question-brain/deploy/compose/compose.yaml ps
curl -fsS http://127.0.0.1:48127/health/ready
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:48128/admin
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:56686
```

The API and CMS containers were healthy, Question Brain readiness returned
`status=ready`, Payload `/admin` returned `200`, and Jaeger returned `200`.

## Operational rule

Editors may create, localize, version and review drafts in Payload. A publish
must call the authenticated `/v1/promote` boundary; a failed promotion stays
visible as an editor error and cannot create a learner-visible revision. The
read-only `published-questions` view is for reconciliation only. The decision
is encoded in `docs/adr/0004-payload-draft-only-authoring.md`.
