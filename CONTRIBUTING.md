# Contributing

## Development gate

Use Go 1.26 or newer on macOS or Linux. Before opening a pull request, run:

```sh
go test -race -count=1 ./...
go vet ./...
test -z "$(gofmt -l $(find cmd internal -type f -name '*.go'))"
```

Every Go source file must remain below 200 lines.

## Design boundary

State the stable axis, change axis, and invariant before changing behavior. One fact has one owner:

- ledgers own semantic events;
- artifacts own immutable bytes;
- projections never mutate ledger state;
- adapters translate runtime bytes into core evidence but do not define kernel semantics;
- Findings describe evidence-backed symptoms; Diagnoses own source-backed root cause and repair boundary.

Avoid compatibility shims, shadow state, provider-specific kernel types, and fallbacks that silently widen authority. Add positive and adversarial tests for the failure class, not only the observed instance.

## Pull requests

Keep changes focused, document any public CLI or artifact-contract change, and do not include tokens, session files, artifacts, or local `.agentlab` state. Public APIs currently remain experimental.
