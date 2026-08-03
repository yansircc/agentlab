package tool

import "errors"

type runInput struct {
	Action         string `json:"action"`
	Root           string `json:"root,omitempty"`
	ExperimentID   string `json:"experiment_id"`
	RunID          string `json:"run_id"`
	Adapter        string `json:"adapter,omitempty"`
	StreamPath     string `json:"stream_path,omitempty"`
	RequestPath    string `json:"request_path,omitempty"`
	FirstEvent     string `json:"first_event,omitempty"`
	SoftIdle       string `json:"soft_idle,omitempty"`
	HardIdle       string `json:"hard_idle,omitempty"`
	KillOnHardIdle bool   `json:"kill_on_hard_idle,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

func (input runInput) invocation() (Invocation, error) {
	if input.ExperimentID == "" || input.RunID == "" {
		return Invocation{}, errors.New("run tool requires experiment and run ids")
	}
	base := rootArgs(input.Root)
	base = append(base, "-experiment", input.ExperimentID, "-run", input.RunID)
	switch input.Action {
	case "start":
		if input.RequestPath == "" || input.RequestPath == "-" || !input.hasDeadlines() || input.Adapter != "" || input.StreamPath != "" || input.Reason != "" {
			return Invocation{}, errors.New("owned start input is invalid")
		}
		args := append([]string{"run", "start"}, base...)
		args = append(args, "-request", input.RequestPath)
		args = append(args, input.deadlineArgs()...)
		if input.KillOnHardIdle {
			args = append(args, "-kill-on-hard-idle")
		}
		return Invocation{Args: args}, nil
	case "attach_begin":
		if input.Adapter == "" || input.StreamPath == "" || !input.hasDeadlines() || input.RequestPath != "" || input.KillOnHardIdle || input.Reason != "" {
			return Invocation{}, errors.New("attach begin input is invalid")
		}
		args := append([]string{"run", "attach", "begin"}, base...)
		args = append(args, "-adapter", input.Adapter, "-stream", input.StreamPath)
		return Invocation{Args: append(args, input.deadlineArgs()...)}, nil
	case "attach_poll":
		if input.Adapter == "" || input.StreamPath == "" || input.hasAnyDeadline() || input.RequestPath != "" || input.KillOnHardIdle || input.Reason != "" {
			return Invocation{}, errors.New("attach poll input is invalid")
		}
		args := append([]string{"run", "attach", "poll"}, base...)
		return Invocation{Args: append(args, "-adapter", input.Adapter, "-stream", input.StreamPath)}, nil
	case "stop":
		if input.Adapter != "" || input.StreamPath != "" || input.hasAnyDeadline() || input.RequestPath != "" || input.KillOnHardIdle {
			return Invocation{}, errors.New("stop input is invalid")
		}
		args := append([]string{"run", "stop"}, base...)
		if input.Reason != "" {
			args = append(args, "-reason", input.Reason)
		}
		return Invocation{Args: args}, nil
	case "status":
		if input.Adapter != "" || input.StreamPath != "" || input.hasAnyDeadline() || input.RequestPath != "" || input.KillOnHardIdle || input.Reason != "" {
			return Invocation{}, errors.New("status input is invalid")
		}
		return Invocation{Args: append([]string{"run", "status"}, base...)}, nil
	default:
		return Invocation{}, errors.New("unknown run action")
	}
}

func (input runInput) hasDeadlines() bool {
	return input.FirstEvent != "" && input.SoftIdle != "" && input.HardIdle != ""
}

func (input runInput) hasAnyDeadline() bool {
	return input.FirstEvent != "" || input.SoftIdle != "" || input.HardIdle != ""
}

func (input runInput) deadlineArgs() []string {
	return []string{"-first-event", input.FirstEvent, "-soft-idle", input.SoftIdle, "-hard-idle", input.HardIdle}
}
