# Question translation contract

Russian coverage is an additive locale on the same immutable question
revision. It is not a second question, a new graph node, or a source-vault
rewrite. `cmd/qb-translate-ru` fills only current published production
revisions that have no `question_locale(locale = 'ru')` row.

The command is safe to rehearse and resumable:

```sh
# Generate and validate a local batch without writing content
go run ./cmd/qb-translate-ru \
  --database-url "$QUESTION_BRAIN_DATABASE_URL" \
  --workspace fluent-interview \
  --limit 10 \
  --report translation-dry-run.json

# Store the validated locale rows with audit and outbox provenance
go run ./cmd/qb-translate-ru \
  --database-url "$QUESTION_BRAIN_DATABASE_URL" \
  --workspace fluent-interview \
  --provider google \
  --approve \
  --actor translation-editorial \
  --report translation-run.json
```

The production translation pass is a fast, non-LLM Google Translate text-endpoint
adapter. It is deliberately explicit in the report and keeps field markers so
the same number and order of sections is reconstructed. LLM providers are
disabled in the command and cannot be selected accidentally. The adapter must
return a Russian question, non-empty section bodies, and the same section count
before anything is written. Locale inserts are idempotent and append a
`question.locale.translated` audit event plus an outbox event for downstream
search/embedding workers.

The output is `question-brain.translation-run.v1`. Generated text remains
traceable to the source revision hash and provider; a future human editorial
pass can replace a locale only through an explicit new reviewed write path.
The canonical English revision and graph are never mutated by translation.
