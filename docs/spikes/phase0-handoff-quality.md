# Phase 0 Spike D: Handoff Quality

## Assumption

A structured Finding improves an independent Coder's source-backed diagnosis over a bounded recorder evidence slice without anchoring root cause or repair boundary.

This spike evaluates handoff quality. It does not freeze a public Finding schema.

## Failure Under Review

An owned launch wrote changing process identity to run-scoped, write-once `process.json` before committing `process_started` to the run ledger. If the ledger append failed, the killed attempt left a receipt that made an exact retry fail with `ErrValueExists`.

Stable axis: run identity, exact request receipt, append-only run ledger.

Change axis: launch attempt identity and failure point.

Invariant: run-ledger sequence 1 `process_started` is the sole accepted-process fact. A new attempt is permitted only when every earlier possible process is proven terminated; uncertainty blocks retry.

## Method

Two independent, read-only Coder passes inspected the same implementation boundary:

1. bounded recorder evidence without a normative owner or repair proposal;
2. an evidence-only structured Finding describing the observed retry failure.

The comparison measured owner identification, repair boundary, missing-evidence requests, and premature anchoring. Concurrent candidate edits were treated as evidence, not as an accepted answer.

## Results

| Measure | Bounded recorder evidence | Structured Finding |
| --- | --- | --- |
| Fact owner | Correctly identified ledger seq-1 `process_started` | Correctly identified ledger seq-1 `process_started` |
| Initial repair boundary | Split run request from attempt-scoped lifecycle; keep acceptance only in ledger | Remove precommit `process.json`; rebuild it from ledger projection |
| Retry falsifier | Requested append failure, process-group death, empty ledger, distinct retry identity, ambiguous commit, and concurrent retry | Requested append failure, process-group death, empty ledger, distinct retry identity, and ambiguous commit |
| Recorder-crash window | Found `spawn -> identity fsync` as an untracked-child counterexample | Initially accepted direct `spawn -> ledger append` as the minimal repair |
| Missing mechanism | Named supervisor/parent-death containment as the condition for crash-safe automatic retry | Did not request containment evidence before judging the candidate boundary correct |
| Anchoring | Low; derived the attempt algebra from lifecycle uncertainty | Material; the observed stale-receipt symptom anchored the repair to deleting that receipt |

Both passes found the original fact-owner violation. Only the recorder-first pass immediately generalized from a returned append error to recorder death between spawn and durable identity.

## Counterexample

Deleting the precommit receipt fixes the observed retry but opens this execution:

```text
spawn worker
recorder is killed before durable identity
worker remains alive because Setpgid is not parent-death containment
restart sees an empty ledger
retry spawns a second worker
```

An ordinary attempt journal narrows but does not eliminate the window. A durable `allocated` event written before spawn becomes `spawn_unknown` after a crash until containment proves that no worker could survive.

## Minimum Contract Chosen by the Spike

The evidence supports these internal facts:

- immutable run-scoped request receipt;
- attempt-scoped append-only lifecycle with allocation, captured identity, and termination proof;
- attempt ID bound into ledger `process_started`;
- ledger replay as the only acceptance decision;
- fail-closed unresolved attempt when spawn cannot be excluded;
- exact-identity reconciliation before terminating a live unaccepted orphan.

It does not support putting source owner, root cause, repair boundary, or prohibited patches into Finding. Those remain Diagnosis facts authored only after source inspection.

## Verdict

FAIL for the assumption that the structured Finding improved this handoff. It preserved evidence references but caused earlier repair anchoring and omitted the decisive recorder-crash counterexample.

Phase 0 evidence therefore supports a small evidence-only Finding and a separate source-backed Diagnosis. The Finding schema remains provisional. Automatic retry after an `allocated-only` crash remains prohibited until an attempt-scoped supervisor or equivalent parent-death containment produces a termination guarantee.
