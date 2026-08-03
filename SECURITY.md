# Security Policy

## Reporting a vulnerability

Do not file public issues for suspected credential disclosure, arbitrary command execution outside a declared oracle boundary, unauthorized Worker termination, ledger corruption, or private-thinking persistence.

Use GitHub private vulnerability reporting for this repository. If that channel is unavailable, contact the repository owner through the email address on the GitHub profile and include a minimal reproduction, affected version or commit, impact, and any required cleanup steps.

## Secret handling

AgentLab accepts secret environment handles, not secret values. Resolved secrets must not be written to request receipts, ledgers, artifacts, or reports. Treat the artifact root as sensitive operational data even when its contents are expected to be redacted.
