package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func TestPiRuntimeTreeIsBoundedPublicOnlyAndReturnsHostRef(t *testing.T) {
	root := t.TempDir()
	launch := testWorkerLaunch(t)
	if err := os.MkdirAll(launch.Launch.RuntimeRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	session := filepath.Join(launch.Launch.RuntimeRoot, "session.jsonl")
	long := strings.Repeat("long-public-", 1024)
	writePiTree(t, session, []string{
		`{"type":"session","version":3,"id":"worker-session"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"` + long + `"}}`,
		`{"type":"message","id":"assistant","parentId":"user","message":{"role":"assistant","content":[{"type":"thinking","thinking":"PRIVATE_THINKING_MUST_NOT_LEAK"},{"type":"text","text":"public observation"}]}}`,
	})
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	host, err := NewPiRuntimeHost([]PiRuntimeProfile{{
		Ref: "worker-profile", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart,
		SessionPath: session, Identity: testIdentity(t), Policy: policy, WorkerLaunch: launch,
	}})
	if err != nil {
		t.Fatal(err)
	}
	binding := Binding{Root: root, ExperimentID: "exp", Runtime: host}
	admitRuntimeTreeEvidence(t, binding, session)
	first, err := host.RuntimeTree(binding, "worker", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	page, ok := first.(PiRuntimeTreePage)
	if !ok || page.Contract != runtimeTreePageContract || page.RuntimeRef != "worker-profile" || len(page.Entries) != 1 || page.NextAfter == nil || page.Entries[0].Evidence.ExperimentID != "exp" || !page.Entries[0].PublicTextTruncated || page.Entries[0].StructurallyForkable == false {
		t.Fatalf("first runtime-tree page = %#v", first)
	}
	data, err := json.Marshal(page)
	if err != nil || strings.Contains(string(data), launch.Launch.RuntimeRoot) || strings.Contains(string(data), "PRIVATE_THINKING_MUST_NOT_LEAK") {
		t.Fatalf("runtime-tree projection leaked Host/private data: %s, %v", data, err)
	}
	second, err := host.RuntimeTree(binding, "worker", *page.NextAfter, 100)
	if err != nil {
		t.Fatal(err)
	}
	next := second.(PiRuntimeTreePage)
	if len(next.Entries) != 1 || next.NextAfter != nil || !strings.Contains(next.Entries[0].PublicText, "public observation") || strings.Contains(next.Entries[0].PublicText, "PRIVATE_THINKING_MUST_NOT_LEAK") || !next.Entries[0].StructurallyForkable {
		t.Fatalf("second runtime-tree page = %#v", next)
	}
	if _, err := host.RuntimeTree(binding, "worker", 3, 1); err == nil {
		t.Fatal("runtime-tree cursor beyond public entries was accepted")
	}
	if _, err := host.RuntimeTree(binding, "worker", 0, maxRuntimeTreePage+1); err == nil {
		t.Fatal("unbounded runtime-tree page was accepted")
	}
}

func TestInspectRuntimeTreeAcceptsNoProviderRuntimeLocator(t *testing.T) {
	root := t.TempDir()
	launch := testWorkerLaunch(t)
	if err := os.MkdirAll(launch.Launch.RuntimeRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	session := filepath.Join(launch.Launch.RuntimeRoot, "session.jsonl")
	writePiTree(t, session, []string{
		`{"type":"session","version":3,"id":"worker-session"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"public"}}`,
	})
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	host, err := NewPiRuntimeHost([]PiRuntimeProfile{{Ref: "worker-profile", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart, SessionPath: session, Identity: testIdentity(t), Policy: policy, WorkerLaunch: launch}})
	if err != nil {
		t.Fatal(err)
	}
	binding := Binding{Root: root, ExperimentID: "exp", Runtime: host}
	admitRuntimeTreeEvidence(t, binding, session)
	value, err := Decode(InspectTool, []byte(`{"scope":"runtime_tree","run_id":"worker","after":0,"limit":1}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(binding, value)
	if err != nil {
		t.Fatal(err)
	}
	if page, ok := result.(PiRuntimeTreePage); !ok || page.RuntimeRef != "worker-profile" {
		t.Fatalf("runtime-tree inspect = %#v", result)
	}
	if _, err := Decode(InspectTool, []byte(`{"scope":"runtime_tree","run_id":"worker","runtime_ref":"provider-selected","after":0,"limit":1}`)); err == nil {
		t.Fatal("runtime-tree inspect accepted a provider runtime ref")
	}
}

func TestPiRuntimeTreeRejectsUnadmittedPublicEntries(t *testing.T) {
	root := t.TempDir()
	launch := testWorkerLaunch(t)
	if err := os.MkdirAll(launch.Launch.RuntimeRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	session := filepath.Join(launch.Launch.RuntimeRoot, "session.jsonl")
	writePiTree(t, session, []string{
		`{"type":"session","version":3,"id":"worker-session"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"public"}}`,
	})
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	host, err := NewPiRuntimeHost([]PiRuntimeProfile{{Ref: "worker-profile", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart, SessionPath: session, Identity: testIdentity(t), Policy: policy, WorkerLaunch: launch}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.RuntimeTree(Binding{Root: root, ExperimentID: "exp", Runtime: host}, "worker", 0, 1); err == nil {
		t.Fatal("runtime tree exposed a public entry without durable evidence")
	}
}

func admitRuntimeTreeEvidence(t *testing.T, binding Binding, session string) {
	t.Helper()
	_ = renderOwnedHandoff(t, binding)
	tree, err := piadapter.ReadPublicTree(session)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := run.Open(binding.Root, binding.ExperimentID, "worker")
	if err != nil {
		t.Fatal(err)
	}
	writer, _, err := operation.AcquireAdapterWriter("test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	batch := run.AdapterBatch{Events: make([]run.AdapterEvent, 0, len(tree.Entries))}
	for _, entry := range tree.Entries {
		source, err := entry.EvidenceSource()
		if err != nil {
			t.Fatal(err)
		}
		batch.Events = append(batch.Events, run.AdapterEvent{Kind: run.EvidenceAssistantMessage, SourceLocator: source, Label: "runtime-tree-entry", Raw: []byte(entry.PublicText)})
	}
	if err := writer.Commit([]byte("runtime-tree-cursor"), batch); err != nil {
		t.Fatal(err)
	}
}

func writePiTree(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
