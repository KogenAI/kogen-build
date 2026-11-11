package consumes

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-loop/sys"
)

const fixtureIdentity = "Fixture <fixture@example.invalid>"

// repoFx is a temporary repository with its own identity, no signing and an empty hooks
// folder of its own. Its Git runs with the process environment TestMain isolated.
type repoFx struct {
	Dir   string
	Hooks string
}

// newBareRepo is a repository with the identity and no commit.
func newBareRepo(t *testing.T) *repoFx {
	t.Helper()
	f := &repoFx{Dir: realTempDir(t), Hooks: t.TempDir()}
	f.git(t, "init", "-q")
	f.git(t, "config", "user.name", "Fixture")
	f.git(t, "config", "user.email", "fixture@example.invalid")
	f.git(t, "config", "core.hooksPath", f.Hooks)
	f.git(t, "config", "commit.gpgsign", "false")
	return f
}

// newRepo is a repository whose one commit holds README.md.
func newRepo(t *testing.T) *repoFx {
	t.Helper()
	f := newBareRepo(t)
	f.write(t, "README.md", "readme\n")
	f.git(t, "add", ".")
	f.git(t, "commit", "-qm", "baseline")
	return f
}

// git runs Git in the fixture directly and fails the test unless it succeeds.
func (f *repoFx) git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.Dir
	cmd.Env = os.Environ()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out)
}

func (f *repoFx) write(t *testing.T, rel, content string) {
	t.Helper()
	writeFile(t, f.Dir, rel, content)
}

func (f *repoFx) hook(t *testing.T, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.Hooks, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (f *repoFx) head(t *testing.T) string { return strings.TrimSpace(f.git(t, "rev-parse", "HEAD")) }

// session is a Session on the isolated environment whose stderr is kept.
func session(dir string) (*sys.Session, *bytes.Buffer) {
	s := sys.NewWith(os.Environ(), dir)
	stderr := &bytes.Buffer{}
	s.Stderr = stderr
	return s, stderr
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func chmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// check fails the test on err.
func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func equalList(t *testing.T, name string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

// wantSysError checks that err is a *sys.Error of the class, reason and exit status, and
// returns it.
func wantSysError(t *testing.T, name string, err error, class, reason string, exit int) *sys.Error {
	t.Helper()
	e, ok := sys.AsError(err)
	if !ok {
		t.Fatalf("%s: error = %v, want *sys.Error %s/%s", name, err, class, reason)
	}
	if e.Class != class || e.Reason != reason || e.Exit != exit {
		t.Fatalf("%s: error = %s (exit %d), want %s/%s (exit %d)", name, e.Error(), e.Exit, class, reason, exit)
	}
	return e
}

// realTempDir is t.TempDir() with symlinks resolved (on macOS /var is /private/var), the
// form Git reports for paths.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
