---
name: agentlab
description: Supervise black-box agent execution, turn observed friction into repairs, preserve useful public context through runtime forks, and close with fresh unassisted verification.
executables:
  agentlab: bin/agentlab
---

# AgentLab Supervisor

## Preparation boundary

Seal one WorkerInput before a Worker starts. It contains the user task and
public task material only. Keep source diagnosis, repair hypotheses, oracle
implementation, private ground truth, and previous-run private reasoning out
of WorkerInput.

For the first start of an unstarted FreshOrigin run, record only the closed
bootstrap decision to launch that sealed input: it has no Worker evidence.
Do not use bootstrap for a guided child or any later decision; those must cite
their admissible public evidence prefix.

Use only the active AgentLab tools and their bounded public projections. The
Host owns runtime location, task binding, and role capabilities; never infer or
request a filesystem, shell, session, or executable locator.

Run ids are short Host names (for example `baseline-worker`), never artifact
digests. Before starting any run, inspect the experiment ledger to discover the
public run ids and prepared runs. If a tool rejects an id, correct the id; do
not repeat the same call unchanged.

## Starting state

The Host has already sealed the WorkerInput, begun the experiment, and bound
the first FreshOrigin Worker run. Do not call `begin_preparation`,
`seal_preparation`, `begin_experiment`, `record_fact`, `challenge`, or any
preparation/experiment setup action. Start by inspecting the experiment ledger,
then issue the first baseline `start` (below).

## Supervision loop (mandatory order)

After the baseline `start`, you MUST remain active until the Worker has exited
and you have observed its evidence. The loop is:

1. Poll the worker (`poll` with run id and runtime ref) and read its status
   until the process exits.
2. Read the worker's evidence (inspect the run ledger or use `status`); the
   baseline is expected to fail its objective oracle.
3. When the failure is no longer recoverable, issue a `stop` with a
   decision citing that evidence, then `record_finding`, then
   `render_handoff`.
4. Bind and start the Coder with the handoff, poll it to completion, read
   the Coder completion, then `record_diagnosis` and `bind_candidate`.
5. Continue until the comparison and gate are recorded.

Never finish after a `start`: starting a Worker is the beginning of an
observation cycle, not the end of the task. If you have no pending tool call,
continue polling rather than writing a final answer.

## First baseline start (exact call)

For the first, unstarted FreshOrigin Worker run, issue exactly one `agentlab_run`
`start` with a bootstrap decision: `evidence_through` is `0`, `evidence` is
empty, and `action` is `worker_start`. A plain `start` never carries fork fields
(`child_run`, `entry_locator`, `checkpoint`). The `decision` carries only the
fields below, named exactly like this:

```json
{"action":"start","effect_id":"start-baseline","run_id":"baseline-worker","runtime_ref":"baseline-worker","decision":{"id":"start-baseline","worker_run":"baseline-worker","evidence_through":0,"claim":"the fresh baseline Worker starts from its sealed input","action":"worker_start","falsifier":"the baseline Worker did not start from its sealed input"}}
```

The Host runtime profiles are named exactly like their runs: the Worker profile
ref is `baseline-worker` and the Coder profile ref is `coder-repair` (the same
short name as the run id).

Later Worker or Coder starts must cite admissible public evidence: set
`evidence_through` to the observed evidence sequence and `evidence` to exactly
that one `EvidenceRef` with `experiment_id`, `run_id`, `sequence`, and `item`.
`coder_start` starts additionally require the `handoff` ref returned by the
`render_handoff` apply operation.

The experiment id is `deployctl-supervision` (the ledger's experiment id, not
the preparation id). Every non-bootstrap decision — `stop`, `finding`,
`coder_handoff`, `diagnosis`, `candidate`, `run_binding`, `comparison` — MUST
carry the `evidence` array with the cited ref. Exact stop example:

```json
{"action":"stop","reason":"objective failure, non-recoverable","run_id":"baseline-worker","runtime_ref":"baseline-worker","decision":{"id":"stop-baseline","worker_run":"baseline-worker","evidence_through":8,"claim":"baseline-worker failed the objective oracle","action":"stop","evidence":[{"experiment_id":"deployctl-supervision","run_id":"baseline-worker","sequence":8,"item":0}],"falsifier":"baseline-worker passed the objective oracle"}}
```

Polling needs the runtime profile ref too: `{"action":"poll","run_id":"baseline-worker","runtime_ref":"baseline-worker"}`. Every `poll` and `start` carries the matching runtime profile ref; `status` takes only the run id.

## Observe, stop, diagnose, repair, splice

Observe public Worker evidence and objective oracle facts. Distinguish alive,
active, and progressing. Do not invent progress without an oracle owner.

When a material failure is no longer recoverable, record an evidence-bounded
decision and stop the run before additional unsupported mutation. Create a
Finding from public evidence only. Hand the bounded Finding to Coder; establish
Diagnosis only after source inspection, then bind an exact repaired candidate.

For development, select a public checkpoint before the failed understanding,
bind any new information as an Intervention, fork, and continue in the repaired
world. Preserve the parent branch. Do not present a splice as resume or as
autonomous evidence.

## Intervention absorption

Every Intervention is temporary scaffolding. Identify the public product,
tool, schema, error, help, or status boundary that should make the same fact
available to a fresh Worker. If that owner is absent, return to diagnosis and
repair; do not add another supervisory workaround.

## Fresh acceptance

Close only with fresh, no-parent, no-Intervention Workers using the original
sealed WorkerInput, clean fixture, exact candidate, normal public tools, and
objective oracles. Guided results may support development evidence but never
count as autonomous repetitions or supported improvement. Keep held-out trials
and exact-candidate gates separate from the guided branch.
