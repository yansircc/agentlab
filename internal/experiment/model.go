package experiment

import (
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/gate"
)

const (
	eventBegun                = "experiment_begun"
	eventFinding              = "finding_recorded"
	eventDisposition          = "finding_dispositioned"
	eventDiagnosis            = "diagnosis_recorded"
	eventCandidate            = "repair_candidate_bound"
	eventRunBound             = "run_manifest_bound"
	eventComparison           = "comparison_observed"
	eventGate                 = "candidate_gate_recorded"
	eventDecisionEffect       = "decision_effect_intended"
	eventDecisionFinding      = "decision_finding_recorded"
	eventDecisionHandoff      = "decision_handoff_recorded"
	eventDecisionDiagnosis    = "decision_diagnosis_recorded"
	eventDecisionCandidate    = "decision_candidate_bound"
	eventDecisionIntervention = "decision_intervention_recorded"
	eventDecisionRun          = "decision_run_bound"
	eventDecisionComparison   = "decision_comparison_recorded"
	eventDecisionGate         = "decision_gate_recorded"
	eventDecisionContinue     = "decision_continue_recorded"
)

type begun struct {
	PreparationID string       `json:"preparation_id"`
	WorkerInput   artifact.Ref `json:"worker_input"`
	Source        artifact.Ref `json:"source_snapshot"`
}

type Status struct {
	ExperimentID     string         `json:"experiment_id"`
	PreparationID    string         `json:"preparation_id,omitempty"`
	WorkerInput      artifact.Ref   `json:"worker_input"`
	FindingIDs       []string       `json:"finding_ids"`
	DiagnosisIDs     []string       `json:"diagnosis_ids"`
	CandidateIDs     []string       `json:"candidate_ids"`
	InterventionRefs []artifact.Ref `json:"intervention_refs"`
	RunIDs           []string       `json:"run_ids"`
	ComparisonIDs    []string       `json:"comparison_ids"`
	GateIDs          []string       `json:"gate_ids"`
	DecisionIDs      []string       `json:"decision_ids"`
	DispositionCount int            `json:"disposition_count"`
	EventCount       uint64         `json:"event_count"`
}

type state struct {
	begun             *begun
	findings          map[string]finding.Finding
	order             []string
	dispositions      map[string]finding.Disposition
	diagnoses         map[string]diagnosis.Diagnosis
	diagnosisOrder    []string
	candidates        map[string]diagnosis.RepairCandidate
	candidateOrder    []string
	interventions     map[artifact.Ref]Intervention
	interventionOwner map[artifact.Ref]string
	interventionOrder []artifact.Ref
	runs              map[string]runBinding
	runOrder          []string
	comparisons       map[string]comparison.Observation
	comparisonOrder   []string
	gates             map[string]gate.Receipt
	gateDecisions     map[string]SupervisorDecision
	gateOrder         []string
	effects           map[string]DecisionBoundEffect
	effectOrder       []string
	decisions         map[string]SupervisorDecision
	decisionOrder     []string
	handoffs          map[artifact.Ref]HandoffRecord
	handoffOwner      map[artifact.Ref]string
	eventCount        uint64
}
