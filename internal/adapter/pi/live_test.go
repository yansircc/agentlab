package pi

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/run"
)

func TestRealPiSessionUnderExperimentManifestScope(t *testing.T) {
	session := os.Getenv("AGENTLAB_REAL_PI_SESSION")
	pidText := os.Getenv("AGENTLAB_REAL_PI_PID")
	if session == "" || pidText == "" {
		t.Skip("set AGENTLAB_REAL_PI_SESSION and AGENTLAB_REAL_PI_PID to a disposable live Pi v3 session")
	}
	pid, err := strconv.Atoi(pidText)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := processidentity.CaptureProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	bindAdapterTestManifest(t, root, "real-pi-experiment", "real-pi-run")
	operation, _ := run.Open(root, "real-pi-experiment", "real-pi-run")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	begun, err := Begin(operation, session, policy, &identity)
	if err != nil || begun.SessionID == "" || begun.Offset <= 0 {
		t.Fatalf("real Pi begin = %#v, %v", begun, err)
	}
	polled, err := Poll(operation, session)
	if err != nil || polled.SessionID != begun.SessionID || polled.Offset != begun.Offset || polled.BatchCount != 0 {
		t.Fatalf("real Pi stable poll = %#v, %v", polled, err)
	}
	status, err := operation.Status(processidentity.SystemProber{})
	if err != nil || status.ProcessLiveness != run.ProcessAlive || status.Health != run.HealthAliveSilent {
		t.Fatalf("real Pi liveness = %#v, %v", status, err)
	}
	if os.Getenv("AGENTLAB_REAL_PI_EXPECT_APPEND") == "1" {
		assertRealPiAppend(t, operation, session)
	}
}

func assertRealPiAppend(t *testing.T, operation *run.Operation, session string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		result, err := Poll(operation, session)
		if err != nil {
			t.Fatal(err)
		}
		if result.EventCount > 0 {
			kinds, correlations, lastPublic := admittedKinds(t, operation)
			if kinds[run.EvidenceAssistantMessage] && kinds[run.EvidenceToolCall] && kinds[run.EvidenceToolResult] && kinds[run.EvidenceTerminal] && kinds[run.EvidenceExcluded] && lastPublic == run.EvidenceTerminal && correlations["tool_call"] != "" && correlations["tool_call"] == correlations["tool_result"] {
				stable, err := Poll(operation, session)
				if err != nil || stable.EventCount != 0 {
					t.Fatalf("durable cursor did not stabilize: %#v, %v", stable, err)
				}
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("real Pi append did not expose correlated public tool evidence and terminal observation")
}

func admittedKinds(t *testing.T, operation *run.Operation) (map[run.EvidenceKind]bool, map[string]string, run.EvidenceKind) {
	t.Helper()
	kinds := map[run.EvidenceKind]bool{}
	correlations := map[string]string{}
	var lastPublic run.EvidenceKind
	records, err := operation.Inspect(1, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		for item := 0; ; item++ {
			evidence, err := operation.EvidenceAt(run.EvidenceRef{ExperimentID: "real-pi-experiment", RunID: "real-pi-run", Sequence: record.Sequence, Item: item})
			if err != nil {
				break
			}
			kinds[evidence.Kind] = true
			if evidence.Kind != run.EvidenceExcluded && evidence.Kind != run.EvidenceProcess {
				lastPublic = evidence.Kind
			}
			if evidence.Kind == run.EvidenceToolCall {
				correlations["tool_call"] = evidence.CorrelationID
			}
			if evidence.Kind == run.EvidenceToolResult {
				correlations["tool_result"] = evidence.CorrelationID
			}
		}
	}
	return kinds, correlations, lastPublic
}
