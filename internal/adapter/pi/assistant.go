package pi

import "encoding/json"

func admitAssistant(entryID string, content json.RawMessage, stopReason string) (Batch, error) {
	var blocks []contentBlock
	if err := json.Unmarshal(content, &blocks); err != nil {
		return Batch{}, ErrInvalidSession
	}
	var batch Batch
	for _, block := range blocks {
		switch block.Type {
		case "thinking":
			batch.Exclusions = append(batch.Exclusions, Exclusion{Category: "pi_thinking", Size: len([]byte(block.Thinking))})
		case "text":
			raw, err := json.Marshal(struct {
				Text string `json:"text"`
			}{block.Text})
			if err != nil {
				return Batch{}, err
			}
			batch.Events = append(batch.Events, Event{Kind: "assistant_message", CorrelationID: entryID, Label: "assistant_text", CompactText: compact([]byte(block.Text)), Raw: raw})
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
			batch.Events = append(batch.Events, Event{Kind: "tool_call", CorrelationID: block.ID, Label: block.Name, CompactText: compact(block.Arguments), Raw: raw})
		default:
			encoded, _ := json.Marshal(block)
			batch.Exclusions = append(batch.Exclusions, Exclusion{Category: "pi_assistant_" + block.Type, Size: len(encoded)})
		}
	}
	if stopReason == "stop" {
		raw, _ := json.Marshal(struct {
			StopReason string `json:"stop_reason"`
		}{stopReason})
		batch.Events = append(batch.Events, Event{Kind: "terminal", CorrelationID: entryID, Label: "pi_turn_completed", Raw: raw})
	}
	return batch, nil
}
