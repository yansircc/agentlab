package pi

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

type entry struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Message json.RawMessage `json:"message"`
}

type message struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCallID string          `json:"toolCallId"`
	ToolName   string          `json:"toolName"`
	IsError    bool            `json:"isError"`
	StopReason string          `json:"stopReason"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func translate(line []byte) (Batch, error) {
	var value entry
	if json.Unmarshal(line, &value) != nil || value.Type == "" {
		return Batch{}, fmt.Errorf("%w: malformed appended record", ErrInvalidSession)
	}
	if value.Type != "message" {
		return Batch{Exclusions: []Exclusion{{Category: "pi_" + value.Type, Size: len(line)}}}, nil
	}
	if value.ID == "" || len(value.Message) == 0 {
		return Batch{}, ErrInvalidSession
	}
	var msg message
	if json.Unmarshal(value.Message, &msg) != nil {
		return Batch{}, ErrInvalidSession
	}
	switch msg.Role {
	case "user":
		return admitMessage("user_message", value.ID, msg.Content)
	case "assistant":
		return admitAssistant(value.ID, msg.Content, msg.StopReason)
	case "toolResult":
		return admitToolResult(msg)
	default:
		return Batch{Exclusions: []Exclusion{{Category: "pi_message_role_" + msg.Role, Size: len(line)}}}, nil
	}
}

func admitMessage(kind, correlationID string, content json.RawMessage) (Batch, error) {
	raw, err := json.Marshal(struct {
		Content json.RawMessage `json:"content"`
	}{content})
	if err != nil {
		return Batch{}, err
	}
	return Batch{Events: []Event{{Kind: kind, CorrelationID: correlationID, Label: kind, CompactText: compact(content), Raw: raw}}}, nil
}

func admitToolResult(msg message) (Batch, error) {
	if msg.ToolCallID == "" || msg.ToolName == "" {
		return Batch{}, ErrInvalidSession
	}
	raw, err := json.Marshal(struct {
		ToolCallID string          `json:"tool_call_id"`
		ToolName   string          `json:"tool_name"`
		IsError    bool            `json:"is_error"`
		Content    json.RawMessage `json:"content"`
	}{msg.ToolCallID, msg.ToolName, msg.IsError, msg.Content})
	if err != nil {
		return Batch{}, err
	}
	return Batch{Events: []Event{{Kind: "tool_result", CorrelationID: msg.ToolCallID, Label: msg.ToolName, CompactText: compact(msg.Content), Raw: raw}}}, nil
}

func compact(value []byte) string {
	const maxRunes = 160
	if !utf8.Valid(value) {
		return ""
	}
	runes := []rune(string(value))
	if len(runes) > maxRunes {
		runes = append(runes[:maxRunes], '…')
	}
	return string(runes)
}
