package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	commandoracle "github.com/yansircc/agentlab/internal/oracle/command"
	filegitoracle "github.com/yansircc/agentlab/internal/oracle/filegit"
	httporacle "github.com/yansircc/agentlab/internal/oracle/http"
)

func TestOracleCLITransportsExactRequests(t *testing.T) {
	t.Run("command", func(t *testing.T) {
		root, files := t.TempDir(), t.TempDir()
		request := writeJSONFile(t, files, "command.json", map[string]any{
			"command": []string{"/bin/sh", "-c", "printf cli-command"}, "directory": files,
			"timeout": "1s", "max_output_bytes": 1024, "side_effects": []string{"none"},
		})
		value, err := dispatch([]string{"oracle", "command", "-root", root, "-request", request})
		if err != nil {
			t.Fatal(err)
		}
		result := value.(commandoracle.Result)
		if result.Output.ExitCode != 0 || result.Receipt.Kind != "command" {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("http", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			_, _ = response.Write([]byte("cli-http"))
		}))
		defer server.Close()
		root, files := t.TempDir(), t.TempDir()
		request := writeJSONFile(t, files, "http.json", map[string]any{
			"method": "GET", "url": server.URL, "timeout": "1s", "max_body_bytes": 1024,
			"side_effects": []string{"network:read"},
		})
		value, err := dispatch([]string{"oracle", "http", "-root", root, "-request", request})
		if err != nil || value.(httporacle.Result).Output.StatusCode != http.StatusOK {
			t.Fatalf("result = %#v, %v", value, err)
		}
	})

	t.Run("file-git", func(t *testing.T) {
		root, files := t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(files, "fact.txt"), []byte("fact"), 0o600); err != nil {
			t.Fatal(err)
		}
		request := writeJSONFile(t, t.TempDir(), "file.json", map[string]any{
			"root": files, "paths": []string{"fact.txt"}, "max_file_bytes": 1024,
			"capture_git": false, "side_effects": []string{"filesystem:read"},
		})
		value, err := dispatch([]string{"oracle", "file-git", "-root", root, "-request", request})
		if err != nil {
			t.Fatal(err)
		}
		result := value.(filegitoracle.Result)
		if len(result.Output.Files) != 1 || result.Output.Files[0].Content == nil {
			t.Fatalf("result = %#v", result)
		}
	})
}
