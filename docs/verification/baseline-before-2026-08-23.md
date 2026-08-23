# Baseline snapshot — before handoff work (2026-08-23)

Snapshot of the stack state taken before executing `HANDOFF-BRIEF.md`.
Stack: Question Brain Compose project, all services healthy. Branch:
`feat/handoff-fix-and-populate` (cut from `main` at `4a5805f`).

Companion raw file: `baseline-before-2026-08-23.quality.json`
(`GET /v1/quality?workspace=fluent-interview`).

## SQL metrics

```sql
select count(*) from content.question;
```

```
 questions 
-----------
      1373
(1 row)
```

Breakdown: 1368 `production` + 5 `fixture`, all `published`.

```sql
select status, content_kind, count(*) from content.question group by 1,2;
```

```
  status   | content_kind | count 
-----------+--------------+-------
 published | fixture      |     5
 published | production   |  1368
(2 rows)
```

```sql
select count(*) from content.question_revision;
```

```
 revisions 
-----------
      1375
(1 row)
```

```sql
select count(*) from content.question_locale;
select locale, count(*) from content.question_locale group by 1;
```

```
 locale_rows 
-------------
        2747
(1 row)

 locale | count 
--------+-------
 en     |  1375
 ru     |  1372
(2 rows)
```

```sql
select count(*) from content.question_topic;
```

```
 topic_bindings 
----------------
           1368
(1 row)
```

```sql
select count(*) from content.question_edge;
```

```
 edges 
-------
     0
(1 row)
```

```sql
select count(*) from content.topic;
```

```
 topics 
--------
     128
(1 row)
```

```sql
select count(*) from content.outbox_event where published_at is null;
```

```
 outbox_pending 
----------------
            349
(1 row)
```

```sql
select count(*) from content.question_embedding;
```

```
 embeddings 
------------
       2399
(1 row)
```

```sql
select profile_key, provider, model, active from content.embedding_profile;
```

```
     profile_key      | provider |      model      | active 
----------------------+----------+-----------------+--------
 semantic-dev-hash-v1 | local    | sha256-token-v1 | t
 semantic-v1          | pending  | pending         | f
(2 rows)
```

## Cross-check against the audit

| metric | audit | snapshot | match |
|---|---|---|---|
| production questions | 1368 | 1368 (+5 fixtures) | ✅ |
| question_locale rows | 2747 | 2747 (en 1375 / ru 1372) | ✅ |
| question_topic bindings | 1368 | 1368 | ✅ |
| question_edge rows | 0 | 0 | ✅ |
| outbox pending events | 349 | 349 | ✅ |
| embeddings | 2399 | 2399 | ✅ |

Notes:

- The audit's "1368" refers to `content_kind = 'production'`; there are also
  5 fixture questions (total 1373). Recorded here to avoid later confusion.
- Locale asymmetry (en 1375 vs ru 1372) corresponds to the 348 untranslated
  current-revision locales named in QB-BUG-9 plus one fixture revision.
- Active embedding profile is still the hash stub (`semantic-dev-hash-v1`);
  `semantic-v1` exists but is `pending/inactive` — QB-BUG-2 confirmed.
