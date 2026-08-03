# Phase 0 Spike C: Grill Preparation

## Assumption

A dependency-ordered decision ledger can turn materially different vague intents into sealed public Worker input without moving domain knowledge into the AgentLab kernel.

This spike does not define a public preparation schema. It classifies decisions and measures which facts recur.

## Method

For each task family:

1. start from one vague user sentence;
2. inspect repository-owned documentation and verification surfaces;
3. build a dependency graph of fact and decision nodes;
4. expose at most one unresolved human decision;
5. reject any branch that would put private source observations into Worker input;
6. count node kinds and identify task-specific facts that cannot enter the kernel.

Node kinds are `discoverable_fact`, `human_decision`, `low_risk_assumption`, and `blocked_external_fact`.

## Family A: zeroY Remote Website Task

Vague intent:

> Watch an Agent complete a remote website task in zeroY, find the friction, and verify the repair.

Repository evidence:

- `/Users/yansir/code/52/zeroY/README.md` identifies runner, sandbox, WordPress plugin, plugin UI, contracts, workflow model, kernel, and compiler boundaries.
- `/Users/yansir/code/52/zeroY/.codex/AGENTS.md` declares the root gate surface and the `protocol -> workflow-model -> workflow-compiler -> workflow-kernel -> app` layer order.
- The README declares `candidate-plan.json`, apply/verify separation, per-site write leases, and `baseline_run_id` optimistic concurrency.

Dependency order:

1. Discover runtime and repository boundaries.
2. Discover supported root verification gates.
3. Human chooses the exact target site and task outcome.
4. Human grants or denies remote mutation authority.
5. Resolve fixture reset capability and site baseline externally.
6. Human chooses acceptable retained side effects.
7. Seal public prompt and authorized public site artifacts.

Private facts forbidden from Worker input:

- compiler stage names discovered only during source diagnosis;
- expected action names or friction hypotheses;
- held-out site state and oracle implementation;
- repair boundary in contracts, kernel, runner, sandbox, or plugin.

Task-specific escape hatch: WordPress fixture identity, site credentials, baseline snapshot, mutation lease, and apply/verify receipts remain pack-owned.

## Family B: Ordinary Go Repository Refactor

Vague intent:

> Refactor AgentLab run status so state cannot drift, and prove the failure class is closed.

Repository evidence:

- `internal/run/replay.go` owns event replay validation.
- `internal/run/status.go` projects liveness and health from replay plus a process probe.
- `internal/ledger/scan.go` validates the entire append-only ledger while bounded inspect collects only the requested page.
- `go test -race ./...` and `go vet ./...` are discoverable verification mechanisms.

Dependency order:

1. Discover the current status fact owner and mutation entrances.
2. Discover tests and package boundaries.
3. Human decides whether the requested behavior includes a public wire-contract change.
4. Resolve dirty checkout and candidate identity from Git/filesystem facts.
5. Human grants or denies deletion/migration of existing persisted data if required.
6. Seal the public refactor request without source-derived solution steps.

Task-specific escape hatch: Go packages, race instrumentation, process fixtures, and repository verification commands remain harness-owned.

## Family C: API Debugging

Vague intent:

> Find why some Claude requests through llm-broker return 400 and verify a durable repair.

Repository evidence:

- `/Users/yansir/code/52/llm-broker/AGENTS.md` makes provider the change axis and `driver.Driver` the provider boundary.
- `/Users/yansir/code/52/llm-broker/docs/request-envelope-analysis-2026-03-18.md` distinguishes accepted and rejected request-envelope families using complete request artifacts and hashes.
- `/Users/yansir/code/52/llm-broker/docs/compat-debugging-2026-03-18.md` records replay ablations and shows that a correct local fix may still be absent from production after deployment rollback.

Dependency order:

1. Discover driver/core boundary and replay tooling.
2. Human selects the production request IDs or time window in scope.
3. Resolve exact request/response artifacts and deployment identity externally.
4. Human grants or denies production replay traffic.
5. Resolve secret handles without persisting secret values.
6. Human chooses the accepted risk for provider-visible replay side effects.
7. Seal a public debugging task that contains request symptoms, not the known envelope delta.

Task-specific escape hatch: request envelope, provider/model identity, secret handles, production deployment identity, replay target, and response oracle remain adapter/pack-owned.

## Measurements

| Task family | Discoverable facts | Human decisions | Low-risk assumptions | Blocked external facts | One-at-a-time questions | Incorrect assumptions found |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| zeroY remote website | 12 | 4 | 1 | 2 | 4 | 1 |
| Go repository refactor | 9 | 2 | 1 | 0 | 2 | 0 |
| API debugging | 11 | 3 | 1 | 2 | 3 | 1 |

Incorrect assumptions falsified:

- zeroY: repository support for remote workflows does not prove that a particular site is reachable, resettable, or authorized for mutation.
- API debugging: a locally successful repair does not prove the deployed candidate is serving traffic; deployment rollback identity is an independent fact.

## Shared Minimum

The recurring preparation facts are:

- raw user intent reference;
- source snapshot/reference;
- repository fact plus evidence references;
- dependency edges between unresolved nodes;
- node kind and resolution authority;
- one current human question;
- explicit low-risk assumption;
- sealed Worker input reference;
- preparation challenge gaps;
- external blocked-fact reference.

Not shared:

- WordPress sites, leases, or candidate plans;
- Go packages, race tests, or process fixtures;
- providers, models, request envelopes, deployment slots, or HTTP replay.

## Verdict

PASS for the narrow assumption: one dependency-ordered decision mechanism fits all three families without domain fields in the kernel.

The evidence does not yet justify freezing the broad `PrepareCommand` and `PrepareResult` contracts. The next spike must test whether an independent Coder performs better from bounded recorder evidence or from a structured Finding before Finding becomes stable.
