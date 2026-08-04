package deployctlfixture

import (
	"errors"

	"github.com/yansircc/agentlab/internal/tool"
)

// StartSupervisor is Host-only orchestration. Its fixed plan was issued at
// runtime preflight; no Supervisor-controlled operation or path reaches it.
func (value Preflight) StartSupervisor() (tool.PiSupervisorReceipt, error) {
	if err := value.verifyRuntime(); err != nil {
		return tool.PiSupervisorReceipt{}, errors.New("deployctl Supervisor preflight is unavailable")
	}
	return tool.StartPiSupervisor(value.supervisorPlanPath)
}

// SupervisorStatus reads a Host-private receipt without projecting it into
// evaluated evidence or the four provider tools.
func (value Preflight) SupervisorStatus() (tool.PiSupervisorStatus, error) {
	if err := value.verifyRuntime(); err != nil {
		return tool.PiSupervisorStatus{}, errors.New("deployctl Supervisor preflight is unavailable")
	}
	return tool.SupervisorStatus(value.supervisorPlanPath)
}
