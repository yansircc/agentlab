//go:build linux

package coder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinuxSandboxRunsAllowedNodeAndExcludesSiblingRoot(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not installed")
	}
	requireLinuxNamespace(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
	publicRoot, auditRoot := filepath.Join(root, "public"), filepath.Join(root, "audit")
	for _, path := range []string{workspace, runtimeRoot, publicRoot, auditRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, "input.txt"), []byte("workspace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicRoot, "public.txt"), []byte("public"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(auditRoot, "private.txt"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{publicRoot}, Executables: []string{node, shell}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sandbox.Wrap([]string{"/bin/cat", filepath.Join(auditRoot, "private.txt")}); err == nil {
		t.Fatal("Linux sandbox accepted an undeclared command")
	}
	profile, err := os.ReadFile(filepath.Join(runtimeRoot, "agentlab-coder-linux-sandbox.sh"))
	if err != nil || !strings.Contains(string(profile), publicRoot) || strings.Contains(string(profile), auditRoot) {
		t.Fatalf("Linux sandbox profile = %q, %v", profile, err)
	}
	probe := `const fs=require("fs"),child=require("child_process");const [workspace,publicRoot,auditRoot]=process.argv.slice(1);if(fs.readFileSync(workspace+"/input.txt","utf8")!=="workspace")process.exit(10);if(fs.readFileSync(publicRoot+"/public.txt","utf8")!=="public")process.exit(11);const status=fs.readFileSync("/proc/self/status","utf8");if(!["CapPrm","CapEff","CapBnd"].every(key=>new RegExp("^"+key+":\\s+0+$","m").test(status)))process.exit(17);fs.writeFileSync(workspace+"/output.txt","written");child.execFileSync("/bin/sh",["-c","printf shell > \"$1/shell.txt\"","sh",workspace]);if(fs.readFileSync(workspace+"/shell.txt","utf8")!=="shell")process.exit(12);try{fs.writeFileSync(publicRoot+"/must-stay-read-only","x");process.exit(13)}catch(error){if(error.code!=="EROFS"&&error.code!=="EACCES"&&error.code!=="EPERM")process.exit(14)}try{fs.readFileSync(auditRoot+"/private.txt");process.exit(15)}catch(error){if(error.code!=="ENOENT"&&error.code!=="EACCES"&&error.code!=="EPERM")process.exit(16)}console.log("sandbox-ok")`
	wrapped, err := sandbox.Wrap([]string{node, "-e", probe, workspace, publicRoot, auditRoot})
	if err != nil || wrapped[0] != linuxUnshare || !containsString(wrapped, "--net") {
		t.Fatalf("Linux sandbox command = %#v, %v", wrapped, err)
	}
	command := exec.Command(wrapped[0], wrapped[1:]...)
	command.Dir = workspace
	command.Env = []string{"HOME=" + runtimeRoot, "PATH=/usr/bin:/bin"}
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "sandbox-ok" {
		t.Fatalf("Linux sandbox probe = %q, %v", output, err)
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "output.txt")); err != nil || string(data) != "written" {
		t.Fatalf("sandbox workspace write = %q, %v", data, err)
	}
}

func TestLinuxSandboxRejectsTemporaryOrOverlappingAuthority(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
	for _, path := range []string{workspace, runtimeRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	if _, err := NewSandbox(SandboxSpec{Workspace: "/tmp/workspace", RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{root}, Executables: []string{node}}); err == nil {
		t.Fatal("Linux sandbox accepted a temporary workspace")
	}
	if _, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: workspace, ReadOnlyRoots: []string{root}, Executables: []string{node}}); err == nil {
		t.Fatal("Linux sandbox accepted overlapping writable roots")
	}
}

func TestLinuxSandboxRunsPinnedPiFromReadOnlySDK(t *testing.T) {
	pi, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("Pi is not installed")
	}
	entry, err := filepath.EvalSymlinks(pi)
	if err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not installed")
	}
	requireLinuxNamespace(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
	for _, path := range []string{workspace, runtimeRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	sdkRoot := filepath.Dir(filepath.Dir(entry))
	sandbox, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{sdkRoot}, Executables: []string{node, shell}})
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := sandbox.Wrap([]string{node, entry, "--version"})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(wrapped[0], wrapped[1:]...)
	command.Dir = workspace
	command.Env = []string{"HOME=" + runtimeRoot, "PATH=/usr/bin:/bin"}
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "0.83.0" {
		t.Fatalf("sandboxed Pi = %q, %v", output, err)
	}
	bashTool := filepath.Join(sdkRoot, "dist", "core", "tools", "bash.js")
	probe := `const {createLocalBashOperations}=await import(process.argv[1]);let output="";const result=await createLocalBashOperations().exec("printf pi-shell-ok",process.cwd(),{onData:chunk=>output+=chunk});if(result.exitCode!==0||output!=="pi-shell-ok")process.exit(21);console.log("pi-bash-ok")`
	wrapped, err = sandbox.Wrap([]string{node, "--input-type=module", "-e", probe, bashTool})
	if err != nil {
		t.Fatal(err)
	}
	command = exec.Command(wrapped[0], wrapped[1:]...)
	command.Dir = workspace
	command.Env = []string{"HOME=" + runtimeRoot, "TMPDIR=" + runtimeRoot, "PATH=/bin"}
	output, err = command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "pi-bash-ok" {
		t.Fatalf("sandboxed Pi bash = %q, %v", output, err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestLinuxSandboxScriptMountModesAndNetwork(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
	publicRoot := filepath.Join(root, "public")
	for _, path := range []string{workspace, runtimeRoot, publicRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not installed")
	}
	runtimeOffline, runtimeOnline := filepath.Join(root, "runtime-offline"), filepath.Join(root, "runtime-online")
	for _, path := range []string{runtimeOffline, runtimeOnline} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	offline, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeOffline, ReadOnlyRoots: []string{publicRoot}, Executables: []string{node, shell}})
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(runtimeOffline, "agentlab-coder-linux-sandbox.sh"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(script), "\n")
	chrootRoot := filepath.Join(runtimeOffline, ".agentlab-coder-linux-root")
	remount := func(path string) string {
		quoted := "'" + chrootRoot + path + "'"
		for _, line := range lines {
			if strings.Contains(line, "remount,bind") && strings.Contains(line, quoted) {
				return line
			}
		}
		return ""
	}
	if line := remount("/dev/null"); !strings.Contains(line, "rw,nosuid") || strings.Contains(line, "nodev") {
		t.Fatalf("/dev/null remount = %q", line)
	}
	copied := false
	for _, line := range lines {
		if strings.Contains(line, "cp -a") && strings.Contains(line, publicRoot) {
			copied = true
		}
	}
	if !copied {
		t.Fatalf("read-only root was not copied into the chroot: %s", script)
	}
	if !strings.Contains(string(script), "--bounding-set=-all --inh-caps=-all --ambient-caps=-all --nnp") {
		t.Fatalf("sandbox script dropped capabilities: %s", script)
	}
	if strings.Contains(string(script), "/etc/hosts") {
		t.Fatalf("offline sandbox exposed network files: %s", script)
	}
	wrapped, err := offline.Wrap([]string{node, "-e", "0"})
	if err != nil || !containsString(wrapped, "--net") {
		t.Fatalf("offline sandbox command = %#v, %v", wrapped, err)
	}
	online, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeOnline, ReadOnlyRoots: []string{publicRoot}, Executables: []string{node, shell}, AllowNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	script, err = os.ReadFile(filepath.Join(runtimeOnline, "agentlab-coder-linux-sandbox.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/etc/hosts", "/etc/resolv.conf", "/etc/nsswitch.conf", "/etc/ssl/certs"} {
		if (regular(path) || directory(path)) && !strings.Contains(string(script), path) {
			t.Fatalf("network sandbox omitted %s: %s", path, script)
		}
	}
	wrapped, err = online.Wrap([]string{node, "-e", "0"})
	if err != nil || containsString(wrapped, "--net") {
		t.Fatalf("network sandbox command = %#v, %v", wrapped, err)
	}
}

func TestLinuxSandboxAcceptsStaticGoAndRejectsMissingDynamicDependency(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
	publicRoot := filepath.Join(root, "public")
	for _, path := range []string{workspace, runtimeRoot, publicRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	probeDir := filepath.Join(root, "probe")
	if err := os.Mkdir(probeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(probeDir, "go.mod"), []byte("module probe\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(probeDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	static := filepath.Join(probeDir, "probe")
	build := exec.Command("go", "build", "-o", static, ".")
	build.Dir = probeDir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if err := build.Run(); err != nil {
		t.Skipf("Go static build is unavailable: %v", err)
	}
	if _, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{publicRoot}, Executables: []string{node, static}}); err != nil {
		t.Fatalf("static Go executable was rejected: %v", err)
	}
	lsPath, err := exec.LookPath("ls")
	if err != nil {
		t.Skip("ls is not installed")
	}
	data, err := os.ReadFile(lsPath)
	if err != nil {
		t.Fatal(err)
	}
	patched := false
	for _, candidate := range []string{"libselinux.so.1", "libpcre2-8.so.0", "libc.so.6"} {
		index := bytes.Index(data, []byte(candidate))
		if index < 0 {
			continue
		}
		replacement := []byte("libnope.so.99\x00")
		if len(replacement) > len(candidate) {
			replacement = replacement[:len(candidate)]
		}
		copy(data[index:index+len(candidate)], replacement)
		patched = true
		break
	}
	if !patched {
		t.Skip("no dynamic dependency candidate found in ls")
	}
	broken := filepath.Join(root, "broken")
	if err := os.WriteFile(broken, data, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{publicRoot}, Executables: []string{node, broken}}); err == nil {
		t.Fatal("executable with a missing dynamic dependency was accepted")
	}
}

func TestLinuxSandboxCoversDeployctlCoderToolAuthority(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
	publicRoot := filepath.Join(root, "public")
	workerRuntime, supervisorRuntime := filepath.Join(root, "worker-runtime"), filepath.Join(root, "supervisor-runtime")
	hostPlan := filepath.Join(root, "pi-runtime-plan.json")
	for _, path := range []string{workspace, runtimeRoot, publicRoot, workerRuntime, supervisorRuntime} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(hostPlan, []byte("host-plan"), 0o600); err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	// Mirror internal/deployctlfixture runtimeProfiles, which calls
	// executablePaths("go", "sh", "grep", "find", "ls") for the Coder.
	var tools []string
	for _, name := range []string{"go", "sh", "grep", "find", "ls"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is not installed", name)
		}
		tools = append(tools, path)
	}
	sandbox, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{publicRoot}, Executables: append([]string{node}, tools...)})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range tools {
		if _, err := sandbox.Wrap([]string{path, "--help"}); err != nil {
			t.Fatalf("declared tool %s was rejected: %v", path, err)
		}
	}
	if _, err := sandbox.Wrap([]string{node, "-e", "0"}); err != nil {
		t.Fatalf("declared node was rejected: %v", err)
	}
	if _, err := sandbox.Wrap([]string{"/usr/bin/env", "-i"}); err == nil {
		t.Fatal("undeclared command was accepted")
	}
	script, err := os.ReadFile(filepath.Join(runtimeRoot, "agentlab-coder-linux-sandbox.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{workerRuntime, supervisorRuntime, hostPlan} {
		if strings.Contains(string(script), forbidden) {
			t.Fatalf("sandbox profile exposed %s: %s", forbidden, script)
		}
	}
}

// TestLinuxSandboxStartsPinnedPiCoderSession proves the full Coder pi startup
// path inside the sandbox: the real skill extension (which execs the bundled
// agentlab binary for tool schemas) plus the builtin tool set loads and the
// session file appears. A fake key is deliberate: the session must appear
// before any model call.
func TestLinuxSandboxStartsPinnedPiCoderSession(t *testing.T) {
	pi, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("Pi is not installed")
	}
	entry, err := filepath.EvalSymlinks(pi)
	if err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not installed")
	}
	requireLinuxNamespace(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	sdkRoot := filepath.Dir(filepath.Dir(entry))
	skillRoot := filepath.Join(root, "skill")
	workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
	for _, path := range []string{skillRoot, workspace, runtimeRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// The real extension execs the bundled agentlab binary for tool schemas,
	// exactly what the Coder pi does at startup inside its sandbox.
	repo := os.Getenv("AGENTLAB_REPO")
	if repo == "" {
		repo = "../.."
	}
	extensionData, err := os.ReadFile(filepath.Join(repo, "skills", "agentlab", "extension.ts"))
	if err != nil {
		t.Fatalf("bundled extension source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "extension.ts"), extensionData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(skillRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(skillRoot, "bin", "agentlab")
	build := exec.Command("go", "build", "-o", binary, "./cmd/agentlab")
	build.Dir = repo
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if err := build.Run(); err != nil {
		t.Skipf("bundled binary build is unavailable: %v", err)
	}
	tools := []string{node, shell}
	for _, name := range []string{"go", "grep", "find", "ls"} {
		if path, err := exec.LookPath(name); err == nil {
			tools = append(tools, path)
		}
	}
	sandbox, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{sdkRoot, skillRoot}, Executables: tools, AllowNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	session := filepath.Join(runtimeRoot, "session.jsonl")
	command := []string{node, entry, "--session", session, "--session-dir", runtimeRoot, "--provider", "xai", "--model", "grok-4.3", "--thinking", "high", "--no-extensions", "--extension", filepath.Join(skillRoot, "extension.ts"), "--no-builtin-tools", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--tools", "read,bash,edit,write,grep,find,ls", "--print", "task"}
	wrapped, err := sandbox.Wrap(command)
	if err != nil {
		t.Fatal(err)
	}
	key := os.Getenv("XAI_API_KEY")
	if key == "" {
		key = "fake-key"
	}
	var stderr bytes.Buffer
	process := exec.Command(wrapped[0], wrapped[1:]...)
	process.Dir = workspace
	process.Env = []string{"HOME=" + runtimeRoot, "TMPDIR=" + runtimeRoot, "PATH=/usr/bin:/bin", "XAI_API_KEY=" + key, "PI_CODING_AGENT_DIR=" + runtimeRoot, "PI_CODING_AGENT_SESSION_DIR=" + runtimeRoot}
	process.Stderr = &stderr
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = process.Process.Kill()
		_, _ = process.Process.Wait()
	}()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(session); err == nil && len(data) > 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("Coder pi did not write its session inside the sandbox; stderr:\n%s", stderr.String())
}

// TestLinuxSandboxCoderHangBisect runs the Coder pi startup with each declared
// tool separately so the mount that blocks the boot is isolated.
func TestLinuxSandboxCoderHangBisect(t *testing.T) {
	pi, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("Pi is not installed")
	}
	entry, err := filepath.EvalSymlinks(pi)
	if err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	requireLinuxNamespace(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	sdkRoot := filepath.Dir(filepath.Dir(entry))
	for _, name := range []string{"go", "grep", "find", "ls", "sh"} {
		tool, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		t.Run(name, func(t *testing.T) {
			root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(root) })
			skillRoot := filepath.Join(root, "skill")
			workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
			for _, path := range []string{skillRoot, workspace, runtimeRoot} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(skillRoot, "extension.ts"), []byte("export default async () => {};\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			sandbox, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{sdkRoot, skillRoot}, Executables: []string{node, tool}, AllowNetwork: true})
			if err != nil {
				t.Fatal(err)
			}
			session := filepath.Join(runtimeRoot, "session.jsonl")
			command := []string{node, entry, "--session", session, "--session-dir", runtimeRoot, "--provider", "xai", "--model", "grok-4.3", "--thinking", "high", "--no-extensions", "--extension", filepath.Join(skillRoot, "extension.ts"), "--no-builtin-tools", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--tools", "read,bash,edit,write,grep,find,ls", "--print", "task"}
			wrapped, err := sandbox.Wrap(command)
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			process := exec.Command(wrapped[0], wrapped[1:]...)
			process.Dir = workspace
			process.Env = []string{"HOME=" + runtimeRoot, "TMPDIR=" + runtimeRoot, "PATH=/usr/bin:/bin", "XAI_API_KEY=fake-key", "AGENTLAB_SANDBOX_DEBUG=1"}
			process.Stderr = &stderr
			if err := process.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = process.Process.Kill()
				_, _ = process.Process.Wait()
			}()
			deadline := time.Now().Add(20 * time.Second)
			for time.Now().Before(deadline) {
				if data, err := os.ReadFile(session); err == nil && len(data) > 0 {
					return
				}
				time.Sleep(200 * time.Millisecond)
			}
			out, _ := exec.Command("sh", "-c", "ps -eo pid,ppid,stat,wchan:24,cmd | grep -E 'unshare|cli.js|go |version' | grep -v grep | head -10").Output()
			t.Fatalf("Coder pi hung with tool %s; stderr:\n%s\nprocesses:\n%s", name, stderr.String(), out)
		})
	}
}

func requireLinuxNamespace(t *testing.T) {
	t.Helper()
	if err := exec.Command(linuxUnshare, "--user", "--map-root-user", "--mount", "--pid", "--fork", "--", linuxShell, "-c", "exit 0").Run(); err != nil {
		t.Skipf("Linux user/mount namespace is unavailable: %v", err)
	}
}

// TestLinuxSandboxReachesDeclaredNetworkEndpoint proves the AllowNetwork=true
// sandbox can complete an HTTPS request with only the declared DNS/TLS files.
// A managed Pi role inside the sandbox must reach its provider API; a hang
// here means the closed network surface is missing a resolver or CA fact.
func TestLinuxSandboxReachesDeclaredNetworkEndpoint(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not installed")
	}
	requireLinuxNamespace(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
	publicRoot := filepath.Join(root, "public")
	for _, path := range []string{workspace, runtimeRoot, publicRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	sandbox, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{publicRoot}, Executables: []string{node, shell}, AllowNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	probe := `fetch("https://api.x.ai/v1/models").then(r=>console.log("https-status-"+r.status)).catch(()=>process.exit(2))`
	wrapped, err := sandbox.Wrap([]string{node, "--input-type=module", "-e", probe})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(wrapped[0], wrapped[1:]...)
	command.Dir = workspace
	command.Env = []string{"HOME=" + runtimeRoot, "TMPDIR=" + runtimeRoot, "PATH=/usr/bin:/bin"}
	output, err := command.CombinedOutput()
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(output)), "https-status-") {
		t.Fatalf("sandboxed HTTPS fetch = %q, %v", output, err)
	}
}

// TestLinuxSandboxStartsPinnedPiWorkerSession proves the full Worker pi
// startup path inside the sandbox: node boots the pinned CLI with the context
// filter and worker extensions and writes its session file. A fake provider
// key is deliberate: the session must appear before any model call, so a
// failure here is a sandbox startup defect, not an authentication one.
func TestLinuxSandboxStartsPinnedPiWorkerSession(t *testing.T) {
	pi, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("Pi is not installed")
	}
	entry, err := filepath.EvalSymlinks(pi)
	if err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not installed")
	}
	requireLinuxNamespace(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-linux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	sdkRoot := filepath.Dir(filepath.Dir(entry))
	skillRoot := filepath.Join(root, "skill")
	fixtureRoot, runtimeRoot := filepath.Join(root, "fixture"), filepath.Join(root, "runtime")
	for _, path := range []string{skillRoot, fixtureRoot, runtimeRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "extension.ts"), []byte("export default async () => {};\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workerExtension := filepath.Join(runtimeRoot, "agentlab-worker-tools.ts")
	if err := os.WriteFile(workerExtension, []byte("import { Type } from \"typebox\";\nimport type { ExtensionAPI } from \"@earendil-works/pi-coding-agent\";\nexport default async function (pi: ExtensionAPI) { pi.registerTool({ name: \"deployctl_help\", description: \"help\", parameters: Type.Object({}), execute: async () => ({ content: [{ type: \"text\", text: \"ok\" }], details: {} }) }); pi.on(\"session_start\", () => pi.setActiveTools([\"deployctl_help\"])); }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandbox(SandboxSpec{Workspace: fixtureRoot, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{sdkRoot, skillRoot}, Executables: []string{node, shell}, AllowNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	session := filepath.Join(runtimeRoot, "session.jsonl")
	command := []string{node, entry, "--session", session, "--session-dir", runtimeRoot, "--provider", "xai", "--model", "grok-4.3", "--thinking", "high", "--no-extensions", "--extension", filepath.Join(skillRoot, "extension.ts"), "--extension", workerExtension, "--no-builtin-tools", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--tools", "deployctl_help", "--print", "task"}
	wrapped, err := sandbox.Wrap(command)
	if err != nil {
		t.Fatal(err)
	}
	process := exec.Command(wrapped[0], wrapped[1:]...)
	process.Dir = fixtureRoot
	process.Env = []string{"HOME=" + runtimeRoot, "TMPDIR=" + runtimeRoot, "PATH=/usr/bin:/bin", "XAI_API_KEY=fake-key", "AGENTLAB_CONTEXT_FILTER_ONLY=1", "AGENTLAB_WORKER_FIXTURE=" + fixtureRoot, "AGENTLAB_WORKER_DEPLOYCTL=/bin/sh"}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = process.Process.Kill()
		_, _ = process.Process.Wait()
	}()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(session); err == nil && len(data) > 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("Worker pi did not write its session inside the sandbox")
}
