package run

// TerminalAccepted reports whether this run has a durable successful terminal
// result. A process exit alone, an abandoned run, or a rejected terminal fact
// cannot be used as comparison evidence.
func (o *Operation) TerminalAccepted() (bool, error) {
	state, err := o.currentState()
	if err != nil {
		return false, err
	}
	return state.exit != nil && state.exit.Code == 0 && state.terminalAccepted && !state.terminalRejected, nil
}
