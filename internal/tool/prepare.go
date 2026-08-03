package tool

import "errors"

type prepareInput struct {
	Action        string `json:"action"`
	Root          string `json:"root,omitempty"`
	PreparationID string `json:"preparation_id,omitempty"`
	RequestPath   string `json:"request_path,omitempty"`
}

func (input prepareInput) invocation() (Invocation, error) {
	requestActions := map[string]string{
		"begin": "begin", "record_fact": "record-fact", "propose_decision": "propose-decision",
		"resolve": "resolve", "assay": "assay", "challenge": "challenge",
	}
	if command, ok := requestActions[input.Action]; ok {
		if input.RequestPath == "" || input.RequestPath == "-" || input.PreparationID != "" {
			return Invocation{}, errors.New("prepare mutation requires only a request path")
		}
		args := append([]string{"prepare", command}, rootArgs(input.Root)...)
		return Invocation{Args: append(args, "-request", input.RequestPath)}, nil
	}
	commands := map[string]string{"challenge_basis": "challenge-basis", "seal": "seal", "status": "status"}
	command, ok := commands[input.Action]
	if !ok || input.PreparationID == "" || input.RequestPath != "" {
		return Invocation{}, errors.New("prepare action or target is invalid")
	}
	args := append([]string{"prepare", command}, rootArgs(input.Root)...)
	return Invocation{Args: append(args, "-preparation", input.PreparationID)}, nil
}
