//go:build linux

package coder

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	linuxUnshare = "/usr/bin/unshare"
	linuxShell   = "/usr/bin/sh"
	linuxMount   = "/usr/bin/mount"
	linuxMkdir   = "/usr/bin/mkdir"
	linuxChmod   = "/usr/bin/chmod"
	linuxTouch   = "/usr/bin/touch"
	linuxCp      = "/usr/bin/cp"
	linuxTimeout = "/usr/bin/timeout"
	linuxLdd     = "/usr/bin/ldd"
	linuxSetpriv = "/usr/bin/setpriv"
	linuxChroot  = "/usr/sbin/chroot"
)

type linuxSandboxMount struct {
	source   string
	target   string
	readOnly bool
}

// newSandbox uses an unprivileged user and mount namespace. Its root is a
// private tmpfs containing only Host-declared Coder capabilities. A platform
// without the required namespace tools fails closed rather than falling back
// to the Host filesystem.
func newSandbox(spec SandboxSpec) (Sandbox, error) {
	workspace, err := sandboxDirectory(spec.Workspace)
	if err != nil {
		return Sandbox{}, err
	}
	runtimeRoot, err := sandboxDirectory(spec.RuntimeRoot)
	if err != nil || overlaps(workspace, runtimeRoot) {
		return Sandbox{}, errors.New("coder sandbox directories are invalid")
	}
	readOnly, err := sandboxDirectories(spec.ReadOnlyRoots)
	if err != nil || len(readOnly) == 0 {
		return Sandbox{}, errors.New("coder sandbox read roots are invalid")
	}
	for _, root := range readOnly {
		if overlaps(root, workspace) || overlaps(root, runtimeRoot) {
			return Sandbox{}, errors.New("coder sandbox roots overlap")
		}
	}
	executables, aliases, err := linuxExecutables(spec.Executables)
	if err != nil || len(executables) == 0 || !regularExecutable(linuxUnshare) || !regularExecutable(linuxShell) || !regularExecutable(linuxMount) || !regularExecutable(linuxMkdir) || !regularExecutable(linuxChmod) || !regularExecutable(linuxTouch) || !regularExecutable(linuxLdd) || !regularExecutable(linuxSetpriv) || !regularExecutable(linuxChroot) {
		return Sandbox{}, errors.New("coder Linux sandbox is unavailable")
	}
	for _, executable := range executables {
		if overlaps(executable, workspace) || overlaps(executable, runtimeRoot) {
			return Sandbox{}, errors.New("coder sandbox executable overlaps writable root")
		}
	}
	root := filepath.Join(runtimeRoot, ".agentlab-coder-linux-root")
	script := filepath.Join(runtimeRoot, "agentlab-coder-linux-sandbox.sh")
	// Resolve every mount before touching the runtime root so a declared
	// executable with a missing dynamic dependency fails closed without
	// leaving partial state behind. On the rare write failure below, remove
	// only the two narrow files this function created; never a broad root.
	mounts, err := linuxMounts(workspace, runtimeRoot, readOnly, executables, aliases, spec.AllowNetwork)
	if err != nil {
		return Sandbox{}, err
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		return Sandbox{}, err
	}
	if err := os.WriteFile(script, []byte(linuxSandboxScript(root, workspace, mounts)), 0o700); err != nil {
		_ = os.RemoveAll(root)
		_ = os.Remove(script)
		return Sandbox{}, err
	}
	prefix := []string{linuxUnshare, "--user", "--map-root-user", "--mount", "--pid", "--fork"}
	if !spec.AllowNetwork {
		prefix = append(prefix, "--net")
	}
	prefix = append(prefix, "--", linuxShell, script)
	return Sandbox{commandPrefix: prefix, commandPaths: aliases, workspace: workspace, runtimeRoot: runtimeRoot}, nil
}

func linuxExecutables(values []string) ([]string, map[string]string, error) {
	if len(values) == 0 {
		return nil, nil, errors.New("coder sandbox executable is invalid")
	}
	seen := map[string]bool{}
	aliases := map[string]string{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !filepath.IsAbs(value) || temporary(value) || !profilePath(value) {
			return nil, nil, errors.New("coder sandbox executable is invalid")
		}
		resolved, err := filepath.EvalSymlinks(value)
		info, statErr := os.Stat(resolved)
		if err != nil || statErr != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 || !profilePath(resolved) {
			return nil, nil, errors.New("coder sandbox executable is invalid")
		}
		aliases[value], aliases[resolved] = resolved, resolved
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		result = append(result, resolved)
	}
	sort.Strings(result)
	return result, aliases, nil
}

func linuxMounts(workspace, runtimeRoot string, readOnly, executables []string, aliases map[string]string, allowNetwork bool) ([]linuxSandboxMount, error) {
	byTarget := map[string]linuxSandboxMount{}
	add := func(source, target string, readOnly bool) error {
		if !filepath.IsAbs(source) || !filepath.IsAbs(target) || !profilePath(source) || !profilePath(target) {
			return errors.New("coder Linux sandbox mount is invalid")
		}
		if existing, found := byTarget[target]; found {
			if existing.source != source || existing.readOnly != readOnly {
				return errors.New("coder Linux sandbox mount overlaps")
			}
			return nil
		}
		byTarget[target] = linuxSandboxMount{source: source, target: target, readOnly: readOnly}
		return nil
	}
	if err := add(workspace, workspace, false); err != nil {
		return nil, err
	}
	if err := add(runtimeRoot, runtimeRoot, false); err != nil {
		return nil, err
	}
	if err := add(linuxSetpriv, linuxSetpriv, true); err != nil {
		return nil, err
	}
	setprivDependencies, err := linuxDependencies(linuxSetpriv)
	if err != nil {
		return nil, err
	}
	for _, dependency := range setprivDependencies {
		if err := add(dependency, dependency, true); err != nil {
			return nil, err
		}
	}
	for _, root := range readOnly {
		if err := add(root, root, true); err != nil {
			return nil, err
		}
	}
	for _, executable := range executables {
		if err := add(executable, executable, true); err != nil {
			return nil, err
		}
		dependencies, err := linuxDependencies(executable)
		if err != nil {
			return nil, err
		}
		for _, dependency := range dependencies {
			if err := add(dependency, dependency, true); err != nil {
				return nil, err
			}
		}
		if filepath.Base(executable) == "node" {
			library := filepath.Join(filepath.Dir(filepath.Dir(executable)), "lib")
			if directory(library) {
				if err := add(library, library, true); err != nil {
					return nil, err
				}
			}
		}
		if filepath.Base(executable) == "go" {
			goRoot := filepath.Dir(filepath.Dir(executable))
			if directory(goRoot) {
				if err := add(goRoot, goRoot, true); err != nil {
					return nil, err
				}
			}
		}
	}
	for alias, executable := range aliases {
		if alias != executable {
			if err := add(executable, alias, true); err != nil {
				return nil, err
			}
		}
	}
	for _, executable := range executables {
		if filepath.Base(executable) == "dash" || filepath.Base(executable) == "sh" {
			if err := add(executable, "/bin/sh", true); err != nil {
				return nil, err
			}
		}
	}
	for _, device := range []string{"/dev/null", "/dev/zero", "/dev/random", "/dev/urandom"} {
		if regularOrDevice(device) {
			if err := add(device, device, device != "/dev/null"); err != nil {
				return nil, err
			}
		}
	}
	if allowNetwork {
		for _, path := range []string{"/etc/hosts", "/etc/resolv.conf", "/etc/nsswitch.conf", "/etc/ssl/certs", "/etc/ssl/openssl.cnf"} {
			if regular(path) || directory(path) {
				if err := add(path, path, true); err != nil {
					return nil, err
				}
			}
		}
		for _, name := range []string{"libnss_dns.so.2", "libnss_files.so.2", "libnss_compat.so.2"} {
			for _, pattern := range []string{"/lib/*/" + name, "/lib64/" + name, "/usr/lib/*/" + name} {
				paths, _ := filepath.Glob(pattern)
				for _, path := range paths {
					if regular(path) {
						if err := add(path, path, true); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	// A file mount whose target lies inside a directory mount is redundant:
	// the covering directory bind makes the whole tree reachable, and mounting
	// the file after its parent would write the touch onto the read-only bind.
	covered := map[string]bool{}
	for target, value := range byTarget {
		if directory(value.source) {
			continue
		}
		for other, outer := range byTarget {
			if other == target || !directory(outer.source) {
				continue
			}
			if insidePath(other, target) {
				covered[target] = true
				break
			}
		}
	}
	for target := range covered {
		delete(byTarget, target)
	}
	result := make([]linuxSandboxMount, 0, len(byTarget))
	for _, value := range byTarget {
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].target < result[right].target
	})
	return result, nil
}

func linuxDependencies(executable string) ([]string, error) {
	// CombinedOutput keeps the dynamic loader's stderr diagnostics (for
	// example "not a dynamic executable" or a missing library name) visible
	// alongside the trace on stdout, so static Go binaries are accepted and
	// a binary with a missing dependency fails closed.
	output, err := exec.Command(linuxLdd, executable).CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "not a dynamic executable") {
			return nil, nil
		}
		return nil, errors.New("coder Linux executable dependencies are unavailable")
	}
	if strings.Contains(string(output), "not found") {
		return nil, errors.New("coder Linux executable dependency is missing")
	}
	seen := map[string]bool{}
	for scanner := bufio.NewScanner(strings.NewReader(string(output))); scanner.Scan(); {
		for _, field := range strings.Fields(scanner.Text()) {
			if !strings.HasPrefix(field, "/") {
				continue
			}
			path := strings.TrimRight(field, ":")
			if regular(path) {
				seen[path] = true
			}
		}
	}
	result := make([]string, 0, len(seen))
	for path := range seen {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}

func insidePath(root, value string) bool {
	relative, err := filepath.Rel(root, value)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func linuxSandboxScript(root, workspace string, mounts []linuxSandboxMount) string {
	options := "set -eu"
	if os.Getenv("AGENTLAB_SANDBOX_DEBUG") != "" {
		options = "set -eux"
	}
	lines := []string{"#!" + linuxShell, options, linuxMount + " --make-rprivate /", linuxMount + " -t tmpfs -o mode=0755,nosuid,nodev tmpfs " + shellQuote(root), linuxMkdir + " -p " + shellQuote(root+"/dev") + " " + shellQuote(root+"/etc") + " " + shellQuote(root+"/proc") + " " + shellQuote(root+"/tmp"), linuxMount + " -t proc proc " + shellQuote(root+"/proc"), linuxChmod + " 1777 " + shellQuote(root+"/tmp")}
	for _, value := range mounts {
		target := root + value.target
		lines = append(lines, linuxMkdir+" -p "+shellQuote(filepath.Dir(target)))
		if directory(value.source) {
			lines = append(lines, linuxMkdir+" -p "+shellQuote(target))
		} else {
			lines = append(lines, linuxTouch+" "+shellQuote(target))
		}
		lines = append(lines, linuxMount+" --bind "+shellQuote(value.source)+" "+shellQuote(target))
		mode := "rw,nosuid,nodev"
		if value.readOnly {
			mode = "ro,nosuid,nodev"
		}
		// These four explicit device nodes are the only device authority in
		// the chroot. nodev would turn /dev/null into an ordinary inaccessible
		// file, which breaks Pi's detached shell stdin redirection.
		if strings.HasPrefix(value.target, "/dev/") {
			mode = "rw,nosuid"
			if value.readOnly {
				mode = "ro,nosuid"
			}
		}
		lines = append(lines, linuxMount+" -o remount,bind,"+mode+" "+shellQuote(target))
	}
	// unshare grants CAP_SYS_ADMIN so the setup script can build the mount
	// namespace. Do not pass that capability to Pi: otherwise a Coder could
	// remount a read-only bind or add a new mount from within the chroot.
	// Enter the workspace before chroot so the exec'd role starts inside the
	// candidate; chroot preserves the working directory when it resolves.
	lines = append(lines, "cd "+shellQuote(workspace))
	lines = append(lines, "exec "+linuxChroot+" "+shellQuote(root)+" "+linuxSetpriv+" --bounding-set=-all --inh-caps=-all --ambient-caps=-all --nnp \"$@\"")
	return strings.Join(lines, "\n") + "\n"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func regular(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func regularExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

func regularOrDevice(path string) bool {
	info, err := os.Stat(path)
	return err == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeDevice != 0)
}

func directory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
