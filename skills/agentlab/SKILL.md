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
