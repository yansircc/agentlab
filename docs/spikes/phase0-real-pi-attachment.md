# Phase 0 Spike B: Real Pi Attachment

## Assumption

A Pi v3 JSONL session can be attached under an exact run manifest, translated incrementally into the closed core evidence algebra, and resumed from a durable cursor without persisting private thinking.

Stable axis: session ID, live process identity, manifest, admitted event policy, and cursor artifact.

Change axis: appended Pi records and provider payload bytes.

Invariant: only newly appended public records cross the adapter boundary. Pi payloads remain adapter artifacts; the kernel receives only core evidence kinds.

## Live Run

Date: 2026-08-03. Pi CLI: `0.83.0`. Session schema: v3.

The run used a temporary copy of a real Pi session and a live Pi RPC process. AgentLab attached at EOF before the RPC prompt was sent. The prompt caused Pi to read the repository `go.mod`, then emit a final public answer.

Observed and asserted by `TestRealPiSessionUnderExperimentManifestScope`:

- effective session ID came from the Pi header;
- the live PID was captured as a full process identity and projected `alive_silent` before append;
- only records after the attached byte cursor were admitted;
- assistant message, tool call, tool result, and terminal observation were present;
- tool call and result shared the exact correlation ID;
- private thinking appeared only as an `excluded_event` size/category fact;
- the last admitted public event was terminal;
- the next poll returned zero events at the same durable cursor.

The exact live command passed:

```text
AGENTLAB_REAL_PI_SESSION=/tmp/agentlab-pi-live.hSO2Jq/session.jsonl \
AGENTLAB_REAL_PI_PID=46251 \
AGENTLAB_REAL_PI_EXPECT_APPEND=1 \
go test -count=1 ./internal/adapter/pi \
  -run TestRealPiSessionUnderExperimentManifestScope -v
```

The Pi process was stopped and the temporary session copy was deleted after the run.

## Adversarial Corpus

The live case is paired with deterministic adapter tests for partial final lines, session rewind, invalid session identity, oversized lines, sink commit failure, private-thinking absence from the artifact tree, terminal observation without kernel terminal mutation, and restart without duplicate admission.

## Result

PASS for the real Pi attachment assumption. Pi-specific parsing and labels remain confined to `internal/adapter/pi` and the CLI composition root.
