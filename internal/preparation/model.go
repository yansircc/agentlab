package preparation

import "github.com/yansircc/agentlab/internal/artifact"
import "github.com/yansircc/agentlab/internal/source"

const workerInputContract = "agentlab.worker-input.v1"

type WorkerInput struct {
	Contract        string         `json:"contract"`
	UserIntentRef   artifact.Ref   `json:"user_intent_ref"`
	PublicArtifacts []artifact.Ref `json:"public_artifacts"`
}

type BeginSpec struct {
	UserIntent      []byte
	SourceFiles     []source.InputFile
	PublicArtifacts [][]byte
	Authority       string
}

type DecisionKind string

const (
	DiscoverableFact    DecisionKind = "discoverable_fact"
	HumanDecision       DecisionKind = "human_decision"
	LowRiskAssumption   DecisionKind = "low_risk_assumption"
	BlockedExternalFact DecisionKind = "blocked_external_fact"
)

type DecisionOption struct {
	ID           string         `json:"id"`
	Label        string         `json:"label"`
	Consequences string         `json:"consequences"`
	Reversible   bool           `json:"reversible"`
	Evidence     []artifact.Ref `json:"evidence,omitempty"`
}

type HumanNode struct {
	Question     string           `json:"question"`
	Recommended  DecisionOption   `json:"recommended"`
	Alternatives []DecisionOption `json:"alternatives"`
}

type FactNode struct {
	Query string `json:"query"`
}

type AssumptionNode struct {
	Statement   string `json:"statement"`
	Consequence string `json:"consequence"`
}

type ExternalNode struct {
	Requirement string `json:"requirement"`
}

type DecisionNode struct {
	ID           string          `json:"id"`
	DependsOn    []string        `json:"depends_on,omitempty"`
	MaterialTo   []string        `json:"material_to,omitempty"`
	Human        *HumanNode      `json:"human,omitempty"`
	Fact         *FactNode       `json:"fact,omitempty"`
	Assumption   *AssumptionNode `json:"assumption,omitempty"`
	ExternalFact *ExternalNode   `json:"external_fact,omitempty"`
}

type Resolution struct {
	NodeID    string         `json:"node_id"`
	Answer    string         `json:"answer"`
	OptionID  string         `json:"option_id,omitempty"`
	Authority string         `json:"authority"`
	Evidence  []artifact.Ref `json:"evidence,omitempty"`
}

type RepositoryFact struct {
	ID        string         `json:"id"`
	Statement string         `json:"statement"`
	Evidence  []artifact.Ref `json:"evidence"`
}

type ChallengeGap struct {
	ID        string         `json:"id"`
	Statement string         `json:"statement"`
	Evidence  []artifact.Ref `json:"evidence,omitempty"`
}

type Challenge struct {
	Basis artifact.Ref   `json:"basis"`
	Gaps  []ChallengeGap `json:"gaps"`
}

type LeakageVerdict string

const (
	LeakageClean    LeakageVerdict = "clean"
	LeakageDetected LeakageVerdict = "detected"
)

type LeakageAssay struct {
	WorkerInput    artifact.Ref   `json:"worker_input"`
	SourceSnapshot artifact.Ref   `json:"source_snapshot"`
	Reviewer       string         `json:"reviewer"`
	Authority      string         `json:"authority"`
	Method         string         `json:"method"`
	Verdict        LeakageVerdict `json:"verdict"`
	Evidence       []artifact.Ref `json:"evidence"`
}

type Phase string

const (
	PhaseExploring     Phase = "exploring"
	PhaseNeedsDecision Phase = "needs_decision"
	PhaseBlocked       Phase = "blocked"
	PhaseChallenged    Phase = "challenged"
	PhaseReady         Phase = "ready"
	PhaseSealed        Phase = "sealed"
)

type Status struct {
	PreparationID string         `json:"preparation_id"`
	Phase         Phase          `json:"phase"`
	WorkerInput   artifact.Ref   `json:"worker_input"`
	Source        artifact.Ref   `json:"source_snapshot"`
	NextNode      *DecisionNode  `json:"next_node,omitempty"`
	OpenGaps      []ChallengeGap `json:"open_gaps,omitempty"`
	LeakageAssay  *LeakageAssay  `json:"leakage_assay,omitempty"`
	EventCount    uint64         `json:"event_count"`
}
