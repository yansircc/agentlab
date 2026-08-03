package tool

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type Invocation struct {
	Args []string `json:"args"`
}

func Decode(name string, input []byte) (Invocation, error) {
	switch name {
	case PrepareTool:
		var value prepareInput
		if err := strictDecode(input, &value); err != nil {
			return Invocation{}, err
		}
		return value.invocation()
	case RunTool:
		var value runInput
		if err := strictDecode(input, &value); err != nil {
			return Invocation{}, err
		}
		return value.invocation()
	case InspectTool:
		var value inspectInput
		if err := strictDecode(input, &value); err != nil {
			return Invocation{}, err
		}
		return value.invocation()
	case CompareTool:
		var value compareInput
		if err := strictDecode(input, &value); err != nil {
			return Invocation{}, err
		}
		return value.invocation()
	default:
		return Invocation{}, errors.New("unknown AgentLab tool")
	}
}

func strictDecode(input []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("tool input has trailing data")
	}
	return nil
}

func rootArgs(root string) []string {
	if root == "" {
		return nil
	}
	return []string{"-root", root}
}
