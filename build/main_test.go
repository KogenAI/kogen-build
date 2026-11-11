package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/durable"
	"github.com/KogenAI/kogen-coordinator/project"
)

// Every Build runs hundreds of git commands, and on macOS /usr/bin/git is the xcode-select
// shim, which resolves the developer directory on each call and costs about twice what the
// real binary does. Each verification pass also starts every toolchain its PATH finds to record
// its version, and every fixture project used to write a new acceptance script, whose first run
// waits for the operating system's check of a new executable.
//
// The suite therefore keeps one private directory for the whole run of the test binary. Its bin
// directory holds only wrappers for the tools the tests use (git, go and node, each found once
// here) and goes first on PATH, followed by the system directories only: every session, git
// command and standalone binary of the tests then sees that PATH, and a Build's check identity
// records only those tools. A tool not found here gets no wrapper. The wrappers exec the real path
// rather than being symlinks to it: git reached through a symlink cannot compute its runtime
// prefix under the Command Line Tools and falls back to a compiled-in Xcode.app path, which
// breaks templates and info/exclude. Its accept directory holds the one read-only acceptance
// script, with its two reports, that every fixture project links to. The directory is removed
// after the run; a test binary re-executed by a test finds the PATH of its parent and shares
// that directory.
func TestMain(m *testing.M) {
	// No global or system Git configuration (hooks, signing, templates, identity rules) reaches
	// the fixture repositories; a re-executed child inherits this through its environment.
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
	if inheritPrivate() {
		// A test re-executed this binary (the baseline tests do) under its PATH: the child
		// shares the parent's private directory, and its environment stays the parent's.
		durable.SkipSyncInTests()
		os.Exit(m.Run())
	}
	private, err := os.MkdirTemp("", "kogen-build-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := setupPrivate(private); err != nil {
		os.RemoveAll(private)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	durable.SkipSyncInTests()
	code := m.Run()
	os.RemoveAll(private)
	os.Exit(code)
}

// sharedAccept is the acceptance script every fixture project links to; set by TestMain.
var sharedAccept string

const systemPath = "/usr/bin:/bin:/usr/sbin:/sbin"

// inheritPrivate reports whether PATH is already exactly what a parent run of this binary set up
// (its private bin directory, then the system directories) and, if so, takes that run's shared
// acceptance script.
func inheritPrivate() bool {
	bin, rest, ok := strings.Cut(os.Getenv("PATH"), string(os.PathListSeparator))
	if !ok || rest != systemPath || filepath.Base(bin) != "bin" || !strings.HasPrefix(filepath.Base(filepath.Dir(bin)), "kogen-build-test-") {
		return false
	}
	script := filepath.Join(filepath.Dir(bin), "accept", "accept.sh")
	if st, err := os.Stat(script); err != nil || st.IsDir() {
		return false
	}
	sharedAccept = script
	return true
}

func setupPrivate(private string) error {
	bin := filepath.Join(private, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		return err
	}
	for name, target := range wrapperTargets() {
		if err := writeWrapper(bin, name, target); err != nil {
			return err
		}
	}
	os.Setenv("PATH", bin+string(os.PathListSeparator)+systemPath)

	dir := filepath.Join(private, "accept")
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	feature := project.FeaturePath(testSlug)
	green, red := filepath.Join(dir, "green.ndjson"), filepath.Join(dir, "red.ndjson")
	for path, status := range map[string]string{green: "PASSED", red: "FAILED"} {
		text, err := reportText(feature, testFeature, status)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
	}
	script := "#!/bin/sh\nif grep -qx red app.txt; then cp '" + red + "' \"$KOGEN_ACCEPTANCE_REPORT\"; echo actual-case-red; exit 1; fi\ncp '" + green + "' \"$KOGEN_ACCEPTANCE_REPORT\"\n"
	sharedAccept = filepath.Join(dir, "accept.sh")
	return os.WriteFile(sharedAccept, []byte(script), 0o555)
}

// wrapperTargets maps each tool the tests use to the real executable found on the host's
// PATH, before it is replaced.
func wrapperTargets() map[string]string {
	targets := map[string]string{}
	if dir := realGitDir(); dir != "" {
		targets["git"] = filepath.Join(dir, "git")
	} else if p, err := exec.LookPath("git"); err == nil {
		targets["git"] = p
	}
	if _, err := exec.LookPath("go"); err == nil {
		if out, err := exec.Command("go", "env", "GOROOT").Output(); err == nil {
			if root := strings.TrimSpace(string(out)); root != "" {
				targets["go"] = filepath.Join(root, "bin", "go")
			}
		}
	}
	if _, err := exec.LookPath("node"); err == nil {
		if out, err := exec.Command("node", "-p", "process.execPath").Output(); err == nil {
			if p := strings.TrimSpace(string(out)); p != "" {
				targets["node"] = p
			}
		}
	}
	return targets
}

func writeWrapper(dir, name, target string) error {
	quoted := "'" + strings.ReplaceAll(target, "'", `'\''`) + "'"
	return os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexec "+quoted+" \"$@\"\n"), 0o700)
}

func realGitDir() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	if p, err := exec.LookPath("git"); err != nil || p != "/usr/bin/git" {
		return ""
	}
	out, err := exec.Command("xcode-select", "-p").Output()
	if err != nil {
		return ""
	}
	dir := filepath.Join(strings.TrimSpace(string(out)), "usr", "bin")
	if st, err := os.Stat(filepath.Join(dir, "git")); err != nil || st.IsDir() {
		return ""
	}
	return dir
}
