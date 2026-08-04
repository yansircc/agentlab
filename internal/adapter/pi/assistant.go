package pi

import "encoding/json"

func admitAssistant(entryID, sourceLocator string, content json.RawMessage, stopReason string) (Batch, error) {
	var blocks []contentBlock
	if err := json.Unmarshal(content, &blocks); err != nil {
		return Batch{}, ErrInvalidSession
	}
	var batch Batch
	public := []publicBlock{}
	toolCalls := []Event{}
	for _, block := range blocks {
		switch block.Type {
		case "thinking":
			batch.Exclusions = append(batch.Exclusions, Exclusion{Category: "pi_thinking", Size: len(block.Thinking)})
		case "text":
			public = append(public, publicBlock{Type: "text", Text: block.Text})
		case "toolCall":
			if block.ID == "" || block.Name == "" {
				return Batch{}, ErrInvalidSession
			}
			raw, err := json.Marshal(struct {
				ID        string          `json:"id"`
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}{block.ID, block.Name, block.Arguments})
			if err != nil {
				return Batch{}, err
			}
			public = append(public, publicBlock{Type: "tool_call", ID: block.ID, Name: block.Name, Arguments: block.Arguments})
			toolCalls = append(toolCalls, Event{Kind: "tool_call", CorrelationID: block.ID, Label: block.Name, CompactText: compact(block.Arguments), Raw: raw})
		default:
			encoded, _ := json.Marshal(block)
			batch.Exclusions = append(batch.Exclusions, Exclusion{Category: "pi_assistant_" + block.Type, Size: len(encoded)})
		}
	}
	if len(public) > 0 {
		raw, err := json.Marshal(public)
		if err != nil {
			return Batch{}, err
		}
		batch.Events = append(batch.Events, Event{Kind: "assistant_message", CorrelationID: entryID, SourceLocator: sourceLocator, Label: "assistant_message", CompactText: compact(raw), Raw: raw})
	}
	batch.Events = append(batch.Events, toolCalls...)
	if stopReason == "stop" {
		raw, _ := json.Marshal(struct {
			StopReason string `json:"stop_reason"`
		}{stopReason})
		batch.Events = append(batch.Events, Event{Kind: "terminal", CorrelationID: entryID, Label: "pi_turn_completed", Raw: raw})
	}
	return batch, nil
}
