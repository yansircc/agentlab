package pi

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
)

func ReadPublicTree(path string) (PublicTree, error) {
	f, err := os.Open(path)
	if err != nil {
		return PublicTree{}, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	if !scanner.Scan() {
		return PublicTree{}, ErrInvalidSession
	}
	var session header
	if json.Unmarshal(scanner.Bytes(), &session) != nil || session.Type != "session" || session.Version != 3 || session.ID == "" {
		return PublicTree{}, ErrInvalidSession
	}
	tree := PublicTree{RuntimeLocator: opaqueLocator("session", session.ID), sessionID: session.ID, nodes: map[string]treeNode{}}
	for count := 0; scanner.Scan(); count++ {
		if count >= maxTreeEntries || tree.append(scanner.Bytes()) != nil {
			return PublicTree{}, ErrInvalidSession
		}
	}
	if scanner.Err() != nil {
		return PublicTree{}, ErrInvalidSession
	}
	return tree, nil
}

func (t *PublicTree) append(data []byte) error {
	var record treeRecord
	if json.Unmarshal(data, &record) != nil || record.Type == "" || record.ID == "" || t.nodes[record.ID].id != "" {
		return ErrInvalidSession
	}
	parent, err := t.parent(record.ParentID)
	if err != nil {
		return err
	}
	payload, calls, result, allowed, err := publicProjection(record)
	if err != nil {
		return err
	}
	pending := clonePending(parent.pending)
	for _, call := range calls {
		if pending[call] {
			return ErrInvalidSession
		}
		pending[call] = true
	}
	if result != "" {
		if !pending[result] {
			return ErrInvalidSession
		}
		delete(pending, result)
	}
	node := treeNode{id: record.ID, pending: pending, valid: parent.valid && allowed, prefix: append([]prefixEntry(nil), parent.prefix...)}
	if record.ParentID != nil {
		node.parentID = *record.ParentID
	}
	if payload != nil {
		entry := PublicEntry{Locator: opaqueLocator(t.sessionID, record.ID), RuntimeLocator: t.RuntimeLocator, Role: payload.Role, Kind: payload.Kind, PublicText: payload.Text}
		if node.parentID != "" {
			entry.ParentLocator = opaqueLocator(t.sessionID, node.parentID)
		}
		node.prefix = append(node.prefix, prefixEntry{Role: entry.Role, Kind: entry.Kind, PublicText: entry.PublicText})
		prefix, err := json.Marshal(node.prefix)
		if err != nil || len(prefix) > 1024*1024 {
			return ErrInvalidSession
		}
		entry.PrefixDigest = sha256Digest(prefix)
		entry.StructurallyForkable = node.valid && len(pending) == 0
		node.entry = &entry
		t.Entries = append(t.Entries, entry)
	}
	t.nodes[record.ID] = node
	return nil
}

func (t PublicTree) parent(parentID *string) (treeNode, error) {
	if parentID == nil {
		if len(t.nodes) != 0 {
			return treeNode{}, ErrInvalidSession
		}
		return treeNode{valid: true}, nil
	}
	parent := t.nodes[*parentID]
	if parent.id == "" {
		return treeNode{}, ErrInvalidSession
	}
	return parent, nil
}

func publicProjection(record treeRecord) (*publicPayload, []string, string, bool, error) {
	if record.Type != "message" || len(record.Message) == 0 {
		return nil, nil, "", false, nil
	}
	var value message
	if json.Unmarshal(record.Message, &value) != nil {
		return nil, nil, "", false, ErrInvalidSession
	}
	switch value.Role {
	case "user":
		return &publicPayload{Role: "user", Kind: "message", Text: string(value.Content)}, nil, "", true, nil
	case "toolResult":
		if value.ToolCallID == "" || value.ToolName == "" {
			return nil, nil, "", false, ErrInvalidSession
		}
		return &publicPayload{Role: "tool", Kind: value.ToolName, Text: string(value.Content)}, nil, value.ToolCallID, true, nil
	case "assistant":
		return projectAssistant(value.Content)
	default:
		return nil, nil, "", false, nil
	}
}

func projectAssistant(content json.RawMessage) (*publicPayload, []string, string, bool, error) {
	var blocks []contentBlock
	if json.Unmarshal(content, &blocks) != nil {
		return nil, nil, "", false, ErrInvalidSession
	}
	public, calls := []publicBlock{}, []string{}
	for _, block := range blocks {
		switch block.Type {
		case "thinking":
		case "text":
			public = append(public, publicBlock{Type: "text", Text: block.Text})
		case "toolCall":
			if block.ID == "" || block.Name == "" {
				return nil, nil, "", false, ErrInvalidSession
			}
			public, calls = append(public, publicBlock{Type: "tool_call", ID: block.ID, Name: block.Name, Arguments: block.Arguments}), append(calls, block.ID)
		default:
			return nil, nil, "", false, nil
		}
	}
	data, err := json.Marshal(public)
	if err != nil || len(public) == 0 {
		return nil, nil, "", false, ErrInvalidSession
	}
	return &publicPayload{Role: "assistant", Kind: "message", Text: string(data)}, calls, "", true, nil
}

type publicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func (t PublicTree) Checkpoint(locator string) ([]byte, []byte, string, error) {
	for _, node := range t.nodes {
		if node.entry == nil || node.entry.Locator != locator {
			continue
		}
		if !node.entry.StructurallyForkable {
			return nil, nil, "", ErrEntryNotForkable
		}
		prefix, err := json.Marshal(node.prefix)
		if err != nil || sha256Digest(prefix) != node.entry.PrefixDigest {
			return nil, nil, "", ErrInvalidSession
		}
		state, err := json.Marshal(checkpointState{RuntimeLocator: t.RuntimeLocator, SessionID: t.sessionID, EntryID: node.id, PrefixDigest: node.entry.PrefixDigest})
		return prefix, state, node.entry.PrefixDigest, err
	}
	return nil, nil, "", ErrEntryNotForkable
}

func clonePending(value map[string]bool) map[string]bool {
	result := map[string]bool{}
	for id := range value {
		result[id] = true
	}
	return result
}

func opaqueLocator(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		hash.Write([]byte{0})
		hash.Write([]byte(part))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func sha256Digest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
