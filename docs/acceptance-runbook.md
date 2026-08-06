# AgentLab Live Acceptance Runbook

This runbook turns the remaining live acceptance into a mechanical procedure.
It is the **only** source of the final acceptance claims. Static or fixture
tests are never described as live acceptance.

## Prerequisites

1. **A Linux host that can create unprivileged user + mount namespaces.**
   - Local: any Linux with `kernel.unprivileged_userns_clone=1` and no
     seccomp filter (`unshare --user --map-root-user --mount --pid --fork
     -- sh -c 'exit 0'` must return 0).
   - GitHub Actions `ubuntu-latest`: the sandbox works **only when the job
     runs as root** (the runner user cannot write `/proc/self/uid_map`).
     Run the namespace tests and the live flow with `sudo -n env PATH=...`.
   - Containers with a seccomp filter (this devcontainer included) block
     namespaces even as root; they are not acceptance hosts.
2. **Node** and the pinned Pi SDK `@earendil-works/pi-coding-agent@0.83.0`
   installed (`npm install -g @earendil-works/pi-coding-agent@0.83.0`).
3. **Three distinct Host credential-handle environment variables**, for the
   Worker, Coder, and Supervisor provider credentials. Values come from the
   authorized provider (e.g. `pi auth print-api-key --model <model>`).
   Example: `AGENTLAB_WORKER_TOKEN`, `AGENTLAB_CODER_TOKEN`,
   `AGENTLAB_SUPERVISOR_TOKEN` set in the Host environment where each Pi
   role runs; all three must be distinct and non-empty, and the provider
   credential env key must match the provider's convention (DeepSeek uses
   `DEEPSEEK_API_KEY`). Provider spend authorization must be obtained first.
4. **Exact provider/model/policy values** (provider, model, thinking,
   compaction) chosen and frozen before `preflight`.

## Step 0 — Deterministic gates (any host)

```sh
go test -race -count=1 ./...
go vet ./...
make skill
git diff --check
```

These must be green before any live step. They are necessary, not sufficient.

## Step 1 — Build and locate the pinned runtime

```sh
make skill
node --version                       # must satisfy the pinned Pi SDK
npm ls -g @earendil-works/pi-coding-agent   # must be 0.83.0
readlink -f "$(which pi)"            # SDK root = dirname(dirname(entry))
readlink -f "$(which node)"
```

## Step 2 — Provision and preflight (Host-only)

```sh
export HOST_ROOT=/srv/agentlab-host            # new, private
export EVAL_ROOT=/srv/agentlab-evaluated       # new
export AUDIT_ROOT=/srv/agentlab-audit          # new

agentlab acceptance provision \
  --evaluated-root "$EVAL_ROOT" --audit-root "$AUDIT_ROOT"

agentlab acceptance preflight \
  --evaluated-root "$EVAL_ROOT" --audit-root "$AUDIT_ROOT" \
  --host-root "$HOST_ROOT" \
  --skill-root "$PWD/dist/skill" \
  --sdk-root "$SDK_ROOT" \
  --node "$(readlink -f "$(which node)")" \
  --provider "$PROVIDER" --model "$MODEL" \
  --thinking "$THINKING" --compaction "$COMPACTION" \
  --provider-credential-env DEEPSEEK_API_KEY \
  --worker-credential-handle AGENTLAB_WORKER_TOKEN \
  --coder-credential-handle AGENTLAB_CODER_TOKEN \
  --supervisor-credential-handle AGENTLAB_SUPERVISOR_TOKEN
```

The Host environment must export the credential at the provider's env key
(`DEEPSEEK_API_KEY` for DeepSeek) and at the three handles; the canary step
invokes the exact selected model before any Worker or Coder trial.

Record the projection: `preparation_id`, `experiment_id`, `baseline_run_id`,
`audit_id`, `worker_input`, `candidate`, `candidate_executable`.

## Step 3 — Baseline Worker trial (live Pi)

```sh
agentlab acceptance prepare-baseline --host-root "$HOST_ROOT" \
  --request <(echo '{"run_id":"'"$BASELINE_RUN_ID"'"}')

agentlab acceptance supervisor-start --host-root "$HOST_ROOT"
agentlab acceptance supervisor-status --host-root "$HOST_ROOT"   # poll to completion
```

The Supervisor (a bundled-Skill Pi extension with exactly four tools) drives
the Worker against the fixture. On a normal non-stopped Worker exit the Host
records the canonical `acceptance worker-oracle` evidence **before** the
terminal fact.

## Step 4 — Guided Coder repair (live Pi)

The Host renders the decision-bound handoff from the Worker evidence and
starts the bounded Coder (read,bash,edit,write,grep,find,ls inside the Linux
sandbox). The Coder completion is a terminal Coder profile artifact.

```sh
agentlab acceptance prepare-run --host-root "$HOST_ROOT" \
  --request <(echo '{"run_id":"'"$CODER_RUN_ID"'","completion":"'"$COMPLETION_REF"'"}')
```

`prepare-run` creates the Host-prepared manifest (the sole run-input fact
source) from the terminal Coder completion.

## Step 5 — Post-seal held-out verification

```sh
agentlab acceptance verify-heldout --host-root "$HOST_ROOT" \
  --request <(echo '{"prepared":"'"$CODER_PREPARED_REF"'"}')
```

The held-out target is generated only after the exact candidate is sealed.

## Step 6 — Independent audit observation

With a separate Codex/audit process (never an evaluated role):

```sh
agentlab acceptance audit-status --host-root "$HOST_ROOT"
agentlab acceptance audit-review --host-root "$HOST_ROOT" --request <(review.json)
agentlab acceptance audit-finding --host-root "$HOST_ROOT" --request <(finding.json)
agentlab acceptance audit-seal --host-root "$HOST_ROOT"
```

Findings bind to supervisor decisions; sealing makes the audit immutable.

## Step 7 — Recursive gate (19-condition PASS)

```sh
agentlab acceptance recursive-gate --host-root "$HOST_ROOT" \
  --request <(echo '{"candidate_id":"'"$CANDIDATE_ID"'","gate_id":"'"$GATE_ID"'","heldout":"'"$HELDOUT_REF"'"}')
```

`recursive-gate` blocks unless: the audit is sealed and unintervened; every
finding verifies; the experiment admits only verified runtime effects; the
gate passes on the exact candidate; and the audit covers every decision.

## Acceptance criteria mapping

| Handoff item | Runbook step |
| --- | --- |
| Real provider credentials / Host handles | Step 1–2 |
| Exact provider/model/thinking policy + live canary | Step 2 (preflight binds it) |
| Stage 0–7 recursive gate | Step 7 + `gate` records |
| Supervisor → Worker → bounded Coder with four tools | Steps 3–4 |
| Runtime fork receipt restart reconciliation | Step 3/4 (restart Supervisor mid-run once) |
| Independent audit-root/Codex observation | Step 6 |
| Mutation A guided-as-fresh rejection | Steps 3–4 + Step 6 finding, then Step 7 block |
| Mutation B repair-generality rejection | Step 5 (`verify-heldout` oracle) |
| Worker/Supervisor/Meta gates, 19-condition PASS | Step 7 verdict `pass` |

Mutation A and B deterministic oracle machinery is covered by fixture tests
(`TestTargetSpecificRepairFailsPostSealHeldoutTarget`,
`TestClassLevelRepairPassesKnownAndPostSealHeldoutTargets`, and the
decision-bound/origin tests in `internal/experiment`). The runbook steps
above demonstrate the same rejections on a live trial with real evidence.

## Not acceptance

- Tests that skip when namespaces are unavailable are not live acceptance.
- A `go test` pass, `make skill`, or fixture oracle pass alone is not
  acceptance of a live trial.
- The Linux sandbox is verified only when `TestLinuxSandboxRunsPinnedPiFromReadOnlySDK`
  and `TestLinuxSandboxRunsAllowedNodeAndExcludesSiblingRoot` **run** (see
  the `sandbox-ns` CI job, which fails if they skip).
