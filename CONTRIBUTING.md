# Contributing

Question Brain is a gated system. A pull request must state which gate it
advances and include evidence for that gate. A later feature cannot hide an
open earlier contract.

Before opening a pull request:

```sh
make check
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm gofmt -l ./cmd ./internal
```

Schema changes are additive by default. Every migration needs a rollback or a
documented irreversible step, a query-plan check, and a data-migration plan.
Never edit canonical content directly from a UI or a one-off SQL console.

