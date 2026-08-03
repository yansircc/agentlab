package tool

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerExtensionHasExactlyFourPublicTools(t *testing.T) {
	pi, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("Pi is not installed")
	}
	entry, err := filepath.EvalSymlinks(pi)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(entry))
	loader := filepath.Join(root, "dist", "core", "extensions", "loader.js")
	if _, err := os.Stat(loader); err != nil {
		t.Skip("Pi extension loader is unavailable")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	extension := filepath.Join(t.TempDir(), "worker-tools.ts")
	if err := os.WriteFile(extension, []byte(workerExtensionSource), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "deployctl")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s|%s|%s|%s|%s' \"$DEPLOYCTL_ROOT\" \"$1\" \"$2\" \"$3\" \"$4\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(node, "--input-type=module", "-e", workerExtensionProbe, loader, extension, executable)
	command.Env = append(os.Environ(), "AGENTLAB_WORKER_FIXTURE=/fixture", "AGENTLAB_WORKER_DEPLOYCTL="+executable)
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "deployctl_deploy,deployctl_help,deployctl_receipt,deployctl_status" {
		t.Fatalf("worker extension = %q, %v", output, err)
	}
}

const workerExtensionProbe = `import assert from "node:assert/strict";
import { pathToFileURL } from "node:url";
const loader = await import(pathToFileURL(process.argv[1]).href);
const loaded = await loader.loadExtensions([process.argv[2]], process.cwd());
assert.deepEqual(loaded.errors, []);
assert.equal(loaded.extensions.length, 1);
const extension = loaded.extensions[0];
const names = [...extension.tools.values()].map((tool) => tool.definition.name).sort();
assert.deepEqual(names, ["deployctl_deploy", "deployctl_help", "deployctl_receipt", "deployctl_status"]);
const deploy = [...extension.tools.values()].find((tool) => tool.definition.name === "deployctl_deploy");
const response = await deploy.definition.execute("probe", { target: "staging", release: "release-a" });
const output = JSON.parse(response.content[0].text);
assert.deepEqual(output, { exit_code: 0, stdout: "[fixture]|deploy|--target|staging|--release", stderr: "" });
let active;
loaded.runtime.setActiveTools = (value) => { active = value.sort(); };
for (const handler of extension.handlers.get("session_start") ?? []) await handler({ type: "session_start" }, {});
assert.deepEqual(active, names);
console.log(names.join(","));`
