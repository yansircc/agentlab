# AgentLab MVP Completion Evidence

Date: 2026-08-03

## Invariant

Public Worker input, observable run evidence, source-backed diagnosis, and exact-candidate comparison have separate fact owners. All derived status and reports are replayable from immutable artifacts and append-only ledgers.

## Completion Gates

1. **Launch or attach real Pi Worker** — owned runner acceptance tests plus the live Pi RPC append test in `docs/spikes/phase0-real-pi-attachment.md`.
2. **Alive, silent, dead, abandoned, unverifiable** — `TestStatusIsReplayableAndIdentityAware`, `TestSilentWorkerCreatesDurableDeadlines`, and attached-runtime tests.
3. **Restart without duplicate or missing events** — durable Pi cursor tests, commit-failure cursor test, and live stable second poll.
4. **Vague intent to sealed preparation** — CLI end-to-end preparation, dependency-frontier tests, and three-family spike.
5. **No Designer analysis in Worker input** — WorkerInput is sealed before SourceSnapshot; an independent exact-input leakage assay is mandatory and a detected leak permanently blocks seal.
6. **External oracle facts survive Worker claims** — command, HTTP, and file/Git receipts hash-bind engine, configuration, output, and declared side effects.
7. **Every Finding cites durable evidence** — Finding validation plus experiment/run-scoped `EvidenceRef` resolution.
8. **Bounded Coder evidence** — inspect requires cursor and limit; rendered handoff contains evidence identities, not transcript bytes.
9. **Diagnosis after exact source inspection** — typed source manifest membership and exact line-range validation; arbitrary artifact evidence is rejected.
10. **Exact candidate and run inputs hash-bound** — every candidate is a sealed source snapshot (raw bytes are rejected) and each deterministic RunManifest carries that exact snapshot plus a per-run typed fixture reset proof.
11. **One stochastic pair cannot claim improvement** — comparison policy requires at least two repetitions and returns `inconclusive` below threshold.
12. **Three task families share contracts** — remote site, Go refactor, and API debugging execute the same preparation algebra in `TestSharedPreparationAlgebraSealsThreeMateriallyDifferentFamilies`.
13. **No repository-specific kernel type** — production scan finds no Pi, zeroY, or WordPress terms outside `internal/adapter/pi` and the CLI composition root; kernel packages do not import adapters.

## Structural Additions From Final Audit

- Fixture identity alone was insufficient to establish reset. `FixtureResetProof` now binds run ID, fixture, baseline, and immutable evidence; comparison requires one common baseline.
- Canonical JSON now emits decimal integers when the exact value fits `int64`, so canonical domain artifacts remain typed-decodable while arbitrary large numbers retain exact exponent form.
- File notification is not state. A fresh operation reconstructs current status from ledger replay with no wakeup delivery.
- Secret redaction is deterministic and longest-first, closing prefix-overlap leakage in owned workers and command/HTTP oracles.
- Every Go file is below 200 lines; lifecycle, adapter writing, helper processes, and test fixtures have separate modules.

## Controlled Recursive Preflight (Not Final Acceptance)

`agentlab acceptance provision` creates fresh, disjoint evaluated and audit
roots for the controlled `deployctl` task. It seals the public WorkerInput,
baseline source candidate and executable, fixture reset, and audit ground
truth without starting a Worker or mutating the fixture.

`agentlab acceptance preflight` additionally requires the exact `dist/skill`
artifact plus pinned Pi SDK, provider/model, and thinking/compaction identity.
It derives the AdapterIdentity from the bundled binary, writes the Host-private
Worker/Coder runtime plan, and binds those identities into the Fresh baseline
manifest. Its result remains only deterministic host assembly: it does not run
the final-provider public-suffix/private-thinking canary or any Stage 1–7 run.

## Authoritative Verification

```text
go test -race -count=1 ./...
go vet ./...
gofmt -l <all cmd/internal Go files>       # empty
go_file_length_gate=PASS
kernel_specific_type_gate=PASS
adapter_dependency_direction=PASS
```

The live Pi test also passed with `AGENTLAB_REAL_PI_EXPECT_APPEND=1`; the process was stopped and its temporary session copy deleted afterward.

## Serious-Go Delivery Status

`serious-go` is installed, but this repository has no canonical policy, exact candidate record, or typed receipt set. No gate request was fabricated. Assurance verdict is `Unknown`; delivery status is `BLOCKED` until those three canonical inputs exist and the installed gate returns a decoded `PASS` for the exact checkout.
