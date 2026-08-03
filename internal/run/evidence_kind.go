package run

type EvidenceKind string

const (
	EvidenceUserMessage      EvidenceKind = "user_message"
	EvidenceAssistantMessage EvidenceKind = "assistant_message"
	EvidenceToolCall         EvidenceKind = "tool_call"
	EvidenceToolResult       EvidenceKind = "tool_result"
	EvidenceProcess          EvidenceKind = "process"
	EvidenceArtifact         EvidenceKind = "artifact"
	EvidenceOracle           EvidenceKind = "oracle"
	EvidenceTerminal         EvidenceKind = "terminal"
	EvidenceExcluded         EvidenceKind = "excluded_event"
)

func (kind EvidenceKind) valid() bool {
	switch kind {
	case EvidenceUserMessage, EvidenceAssistantMessage, EvidenceToolCall, EvidenceToolResult,
		EvidenceProcess, EvidenceArtifact, EvidenceOracle, EvidenceTerminal, EvidenceExcluded:
		return true
	default:
		return false
	}
}
