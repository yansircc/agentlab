# Phase 0 Spike A: Recorder and Liveness

## Assumption

An append-only run ledger plus exact process identity is sufficient to reconstruct liveness, activity, progress, and terminal truth without file-notification state.

Stable axis: run manifest, process identity, admitted evidence, and terminal contract.

Change axis: process behavior, event timing, stream state, recorder interruption, and ownership.

Invariant: ledger replay owns semantic status. Process probes contribute only current liveness observations; notifications never own state.

## Evidence Matrix

| Required case | Direct evidence |
| --- | --- |
| delayed first event | `TestOwnedRunnerTerminalAlgebra/clean` records first-event timeout before later valid evidence and terminal result |
| continuous events | `TestContinuousWorkerProjectsActiveBeforeTerminal` observes `alive_active` before terminal completion |
| alive but silent | `TestSilentWorkerCreatesDurableDeadlines` records first, soft-idle, and hard-idle facts before owned termination |
| dead without terminal | `TestStatusIsReplayableAndIdentityAware` projects identity mismatch as `abandoned` |
| clean exit with one result | `TestOwnedRunnerTerminalAlgebra/clean` |
| zero exit without result | `TestOwnedRunnerTerminalAlgebra/missing` rejects terminal truth |
| duplicate terminal results | `TestOwnedRunnerTerminalAlgebra/duplicate` rejects terminal truth |
| malformed or partial JSONL | `TestReplayFailsClosedOnPartialOrCorruptRecord` |
| child process group | `TestHardIdleCleansOwnedProcessGroup` proves the child cannot survive group cleanup |
| PID reuse | `TestSystemProberRequiresFullIdentity` rejects start-token and command mismatch |
| attached unknown process | `TestOperationPollPersistsCursorWithSanitizedBatch` projects `unverifiable` |
| recorder interruption | launch-attempt tests preserve allocation, captured identity, and termination proof; allocated-only ambiguity blocks retry |
| notification loss | `TestReplayConvergesWithoutFileNotification` opens a fresh reader with no wakeup and reconstructs the same status |
| disposable projection | `TestStatusProjectionIsDisposableAndRebuildable` rebuilds identical bytes after cache corruption |

## Commands

```text
go test -count=1 ./internal/run ./internal/processidentity ./internal/ledger
go test -race -count=1 ./...
```

## Result

PASS for the recorder/liveness assumption.

The spike does not claim that an allocated-only process attempt is safe to retry after recorder death. Without parent-death containment, the surviving-process question is unknowable; retry remains fail-closed under `ErrAttemptUnresolved`.
