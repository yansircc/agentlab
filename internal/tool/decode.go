package tool

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func Decode(name string, input []byte) (Operation, error) {
	switch name {
	case ApplyTool:
		return decodeApply(input)
	case RunTool:
		return decodeRun(input)
	case InspectTool:
		return decodeInspect(input)
	case CompareTool:
		return decodeCompare(input)
	default:
		return nil, errors.New("unknown AgentLab tool")
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

type actionHeader struct {
	Action string `json:"action"`
}

func decodeAction(data []byte) (string, error) {
	var value actionHeader
	if err := json.Unmarshal(data, &value); err != nil || value.Action == "" {
		return "", errors.New("tool action is invalid")
	}
	return value.Action, nil
}
