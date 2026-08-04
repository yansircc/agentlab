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
	Thinking  json.RawMessage `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func translate(line []byte) (Batch, error) { return translateWithSession("", line) }

func translateWithSession(sessionID string, line []byte) (Batch, error) {
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
		return admitMessage("user_message", value.ID, publicSourceLocator(sessionID, value.ID), msg.Content)
	case "assistant":
		return admitAssistant(value.ID, publicSourceLocator(sessionID, value.ID), msg.Content, msg.StopReason)
	case "toolResult":
		return admitToolResult(publicSourceLocator(sessionID, value.ID), msg)
	default:
		return Batch{Exclusions: []Exclusion{{Category: "pi_message_role_" + msg.Role, Size: len(line)}}}, nil
	}
}

func admitMessage(kind, correlationID, sourceLocator string, content json.RawMessage) (Batch, error) {
	if !json.Valid(content) {
		return Batch{}, ErrInvalidSession
	}
	return Batch{Events: []Event{{Kind: kind, CorrelationID: correlationID, SourceLocator: sourceLocator, Label: kind, CompactText: compact(content), Raw: content}}}, nil
}

func admitToolResult(sourceLocator string, msg message) (Batch, error) {
	if msg.ToolCallID == "" || msg.ToolName == "" {
		return Batch{}, ErrInvalidSession
	}
	if !json.Valid(msg.Content) {
		return Batch{}, ErrInvalidSession
	}
	return Batch{Events: []Event{{Kind: "tool_result", CorrelationID: msg.ToolCallID, SourceLocator: sourceLocator, Label: msg.ToolName, CompactText: compact(msg.Content), Raw: msg.Content}}}, nil
}

func publicSourceLocator(sessionID, entryID string) string {
	if sessionID == "" || entryID == "" {
		return ""
	}
	return opaqueLocator("public-entry", entryID)
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
