package main

import (
	"errors"
	"flag"
	"os"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/deployctlfixture"
)

// acceptanceCommand is Host-only orchestration. It is intentionally absent
// from the provider tool projection, which admits only the four Supervisor
// effects after this preflight has bound their Host inputs.
func acceptanceCommand(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab acceptance <provision|preflight|prepare-baseline|prepare-run|verify-heldout>")
	}
	switch args[0] {
	case "provision":
		return acceptanceProvision(args[1:])
	case "preflight":
		return acceptancePreflight(args[1:])
	case "prepare-run":
		return acceptancePrepareRun(args[1:])
	case "prepare-baseline":
		return acceptancePrepareBaseline(args[1:])
	case "verify-heldout":
		return acceptanceVerifyHeldout(args[1:])
	default:
		return nil, errors.New("unknown acceptance command")
	}
}

type acceptanceVerifyHeldoutRequest struct {
	Prepared artifact.Ref `json:"prepared"`
}

func acceptanceVerifyHeldout(args []string) (any, error) {
	set := flag.NewFlagSet("acceptance verify-heldout", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	hostRoot := set.String("host-root", "", "existing Host-private runtime root")
	requestPath := set.String("request", "-", "Host JSON request path or - for stdin")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *hostRoot == "" || set.NArg() != 0 {
		return nil, errors.New("acceptance verify-heldout requires Host root and prepared run")
	}
	var request acceptanceVerifyHeldoutRequest
	if err := readRequest(*requestPath, &request); err != nil {
		return nil, err
	}
	preflight, err := deployctlfixture.LoadRuntimePreflight(*hostRoot)
	if err != nil {
		return nil, err
	}
	verification, err := preflight.VerifyHeldoutPreparedRun(request.Prepared)
	if err != nil {
		return nil, err
	}
	return acceptanceHeldoutProjection{Verification: verification}, nil
}

type acceptancePrepareRunRequest struct {
	RunID      string       `json:"run_id"`
	Completion artifact.Ref `json:"completion"`
}

type acceptancePrepareBaselineRequest struct {
	RunID string `json:"run_id"`
}

// acceptancePrepareBaseline is Host-only repetition setup. The request names
// a future run but cannot select a candidate or any manifest input.
func acceptancePrepareBaseline(args []string) (any, error) {
	set := flag.NewFlagSet("acceptance prepare-baseline", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	hostRoot := set.String("host-root", "", "existing Host-private runtime root")
	requestPath := set.String("request", "-", "Host JSON request path or - for stdin")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *hostRoot == "" || set.NArg() != 0 {
		return nil, errors.New("acceptance prepare-baseline requires Host root and run id")
	}
	var request acceptancePrepareBaselineRequest
	if err := readRequest(*requestPath, &request); err != nil {
		return nil, err
	}
	preflight, err := deployctlfixture.LoadRuntimePreflight(*hostRoot)
	if err != nil {
		return nil, err
	}
	prepared, err := preflight.PrepareBaselineRun(request.RunID)
	if err != nil {
		return nil, err
	}
	return acceptancePreparedRunProjection{RunID: request.RunID, Prepared: prepared}, nil
}

// acceptancePrepareRun is Host-only continuation after a terminal Coder run.
// The request carries only the terminal receipt reference and a future run
// identity; all candidate and RunInputs facts remain Host-produced.
func acceptancePrepareRun(args []string) (any, error) {
	set := flag.NewFlagSet("acceptance prepare-run", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	hostRoot := set.String("host-root", "", "existing Host-private runtime root")
	requestPath := set.String("request", "-", "Host JSON request path or - for stdin")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *hostRoot == "" || set.NArg() != 0 {
		return nil, errors.New("acceptance prepare-run requires Host root and terminal Coder completion")
	}
	var request acceptancePrepareRunRequest
	if err := readRequest(*requestPath, &request); err != nil {
		return nil, err
	}
	preflight, err := deployctlfixture.LoadRuntimePreflight(*hostRoot)
	if err != nil {
		return nil, err
	}
	prepared, err := preflight.PrepareRunFromCoderCompletion(request.RunID, request.Completion)
	if err != nil {
		return nil, err
	}
	return acceptancePreparedRunProjection{RunID: request.RunID, Prepared: prepared}, nil
}

func acceptanceProvision(args []string) (any, error) {
	set := flag.NewFlagSet("acceptance provision", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	evaluatedRoot := set.String("evaluated-root", "", "new Host-owned evaluated capability root")
	auditRoot := set.String("audit-root", "", "new Host-owned audit capability root")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *evaluatedRoot == "" || *auditRoot == "" || set.NArg() != 0 {
		return nil, errors.New("acceptance provision requires new evaluated and audit roots")
	}
	value, err := deployctlfixture.ProvisionPreflight(deployctlfixture.PreflightSpec{EvaluatedRoot: *evaluatedRoot, AuditRoot: *auditRoot})
	if err != nil {
		return nil, err
	}
	return acceptanceProvisionProjection{PreparationID: value.PreparationID, ExperimentID: value.ExperimentID, BaselineRunID: value.BaselineRunID, AuditID: value.AuditID, WorkerInput: value.WorkerInput, Candidate: value.Candidate, CandidateExecutable: value.CandidateExecutable}, nil
}

func acceptancePreflight(args []string) (any, error) {
	set := flag.NewFlagSet("acceptance preflight", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	evaluatedRoot := set.String("evaluated-root", "", "new Host-owned evaluated capability root")
	auditRoot := set.String("audit-root", "", "new Host-owned audit capability root")
	hostRoot := set.String("host-root", "", "new Host-private runtime root")
	skillRoot := set.String("skill-root", "", "exact bundled dist/skill root")
	sdkRoot := set.String("sdk-root", "", "exact pinned Pi SDK package root")
	nodePath := set.String("node", "", "Node executable path")
	provider := set.String("provider", "", "final Pi provider")
	model := set.String("model", "", "final Pi model")
	thinking := set.String("thinking", "", "final Pi thinking policy")
	compaction := set.String("compaction", "", "final Pi compaction policy")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *evaluatedRoot == "" || *auditRoot == "" || *hostRoot == "" || *skillRoot == "" || *sdkRoot == "" || *nodePath == "" || *provider == "" || *model == "" || *thinking == "" || *compaction == "" || set.NArg() != 0 {
		return nil, errors.New("acceptance preflight requires fresh roots and exact bundled Pi runtime identity")
	}
	value, err := deployctlfixture.ProvisionPreflight(deployctlfixture.PreflightSpec{EvaluatedRoot: *evaluatedRoot, AuditRoot: *auditRoot})
	if err != nil {
		return nil, err
	}
	value, err = value.BindRuntime(deployctlfixture.RuntimeSpec{HostRoot: *hostRoot, SkillRoot: *skillRoot, SDKRoot: *sdkRoot, NodePath: *nodePath, Provider: *provider, Model: *model, ThinkingPolicy: *thinking, CompactionPolicy: *compaction})
	if err != nil {
		return nil, err
	}
	return acceptancePreflightProjection{acceptanceProvisionProjection: acceptanceProvisionProjection{PreparationID: value.PreparationID, ExperimentID: value.ExperimentID, BaselineRunID: value.BaselineRunID, AuditID: value.AuditID, WorkerInput: value.WorkerInput, Candidate: value.Candidate, CandidateExecutable: value.CandidateExecutable}, FixtureReset: value.FixtureReset, CoderPrepared: value.CoderPrepared}, nil
}

// acceptanceProvisionProjection contains only opaque evaluated-root refs.
// Ground-truth and filesystem locators remain Host-private.
type acceptanceProvisionProjection struct {
	PreparationID       string       `json:"preparation_id"`
	ExperimentID        string       `json:"experiment_id"`
	BaselineRunID       string       `json:"baseline_run_id"`
	AuditID             string       `json:"audit_id"`
	WorkerInput         artifact.Ref `json:"worker_input"`
	Candidate           artifact.Ref `json:"candidate"`
	CandidateExecutable artifact.Ref `json:"candidate_executable"`
}

type acceptancePreflightProjection struct {
	acceptanceProvisionProjection
	FixtureReset  artifact.Ref `json:"fixture_reset"`
	CoderPrepared artifact.Ref `json:"coder_prepared"`
}

type acceptancePreparedRunProjection struct {
	RunID    string       `json:"run_id"`
	Prepared artifact.Ref `json:"prepared"`
}

type acceptanceHeldoutProjection struct {
	Verification artifact.Ref `json:"verification"`
}
