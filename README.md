# AgentLab

AgentLab is a Go CLI for black-box Agent experiments. It records only admissible public Worker behavior, turns repeated friction into durable evidence, and compares exact repair candidates under a controlled run manifest.

It is an experimental MVP. `internal/` packages and on-disk JSON schemas may change before a stable release.

## What it owns

- immutable content-addressed artifacts and append-only ledgers;
- owned and attached Worker lifecycle, including liveness, idle, stop, and terminal facts;
- Pi v3 session attachment with durable cursor, public-event filtering, tool correlation, and thinking exclusion;
- sealed Worker input, source snapshots, decision preparation, and leakage assays;
- command, HTTP, and file/Git oracle receipts;
- evidence-only Findings, source-backed Diagnoses, exact repair candidates, and repeated-run comparison;
- four provider-safe tool projections for Anthropic and OpenAI Responses.

The source of truth is `events.jsonl` plus immutable artifacts. `result.json`, status, reports, and indexes are disposable projections.

## Requirements

- Go 1.26 or newer;
- macOS or Linux. Worker process-group control uses Unix process semantics;
- Pi is optional and needed only for `run attach` against Pi sessions.

## Install

```sh
go install github.com/yansircc/agentlab/cmd/agentlab@latest
agentlab tool schemas -provider anthropic
```

For development:

```sh
git clone https://github.com/yansircc/agentlab.git
cd agentlab
go test -race ./...
go vet ./...
```

AgentLab stores data in `~/.agentlab` by default. Pass `-root /absolute/path` to commands when an experiment needs an explicit storage location.

## CLI surface

```text
agentlab prepare begin|record-fact|propose-decision|resolve|assay|challenge-basis|challenge|seal|status
agentlab experiment begin|bind-run|status
agentlab run start|attach|status|stop
agentlab oracle command|http|file-git
agentlab review detect-repeated|detect-bypass|handoff
agentlab diagnose record|bind-candidate
agentlab compare record|show
agentlab gate record|show
agentlab inspect
agentlab tool schemas|invoke
agentlab acceptance provision|preflight
```

Mutation commands accept strict JSON requests. Secrets are referenced by environment-variable handles, never accepted as CLI values, and resolved values are redacted before evidence persistence.

## Supervisor tool boundary

Provider integrations expose exactly four host-bound tools:

```text
agentlab_apply
agentlab_run
agentlab_inspect
agentlab_compare
```

Their input is strict JSON on stdin. The Host, not the model, binds the AgentLab root, preparation, experiment, runtime profile, executable, session locator, and capability profile. Tool schemas and their decoder reject root, request-file, stream/session, executable, raw-transcript, and audit-root locators.

## Bundled Supervisor artifact

With Pi `0.83.0` available as `pi`, build the locally reviewable artifact:

```sh
make skill
```

It creates `dist/skill` with only `SKILL.md`, `extension.ts`, and the bundled
`bin/agentlab`. The extension derives its four active tool definitions from that
binary and invokes only the adjacent bundled executable. The Host supplies task
bindings through `AGENTLAB_ROOT`, `AGENTLAB_PREPARATION`,
`AGENTLAB_EXPERIMENT`, and, when needed, `AGENTLAB_PI_RUNTIME_PLAN`; none is a
model tool parameter. This build does not install or register the artifact.

## Verification and scope

The current MVP evidence and Phase 0 spike reports are in [docs/completion-evidence.md](docs/completion-evidence.md) and [docs/spikes](docs/spikes).

AgentLab does not coach Workers, capture private thinking, modify source automatically, create commits, perform releases, or claim causal improvement from a single baseline/candidate pair.

`agentlab acceptance provision` is Host-only: it creates new evaluated and
audit roots for the controlled `deployctl` task, seals the baseline input and
candidate, and returns only evaluated-root opaque refs. `acceptance preflight`
additionally requires the exact bundled artifact and Pi provider/model policy.
Before it creates a Host runtime plan or Fresh manifest, it forks a disposable
Pi session and calls that exact final provider through the bundled extension.
The canary must recover the selected public prefix while excluding its private
thinking and parent suffix. It persists only pass booleans plus an opaque
AdapterIdentity ref; tokens, session paths, and model output are deleted.
Pi 0.83.0's CLI cannot bind a non-off compaction policy, so preflight rejects
one rather than claiming it tested a different configuration. This establishes
only the Stage 0 context-semantic gate, not a Stage 1–7 acceptance result.

## Contributing and security

See [CONTRIBUTING.md](CONTRIBUTING.md), [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md), and [SECURITY.md](SECURITY.md). The project is released under the [MIT License](LICENSE).
