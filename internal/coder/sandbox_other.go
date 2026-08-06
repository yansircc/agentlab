//go:build !darwin && !linux

package coder

import "errors"

func newSandbox(SandboxSpec) (Sandbox, error) {
	return Sandbox{}, errors.New("coder sandbox is unavailable on this platform")
}
