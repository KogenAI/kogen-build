package consumes

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

const (
	slug    = "fixture"
	feature = "Feature: Fixture\n  Scenario: First\n    Then the candidate succeeds\n  Scenario: Later\n    Then the candidate succeeds\n"
)

// quote single-quotes s for /bin/sh.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// processEnv is the process environment (already isolated by TestMain).
func processEnv() []string { return os.Environ() }

// memLog is a verify.Logger that keeps the events it is given.
type memLog struct {
	mu     sync.Mutex
	events []map[string]any
}

func (l *memLog) Log(event map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

// names lists "<event> <name>" of the events with a name, in order, for the named kinds.
func (l *memLog) names(kinds ...string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, e := range l.events {
		for _, k := range kinds {
			if e["event"] == k {
				out = append(out, fmt.Sprintf("%v %v", k, e["name"]))
			}
		}
	}
	return out
}

func (l *memLog) named(kind string) []map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []map[string]any
	for _, e := range l.events {
		if e["event"] == kind {
			out = append(out, e)
		}
	}
	return out
}

// proj is a Git repository in t.TempDir() holding one committed Intent whose acceptance
// command copies a prepared Cucumber Messages report: green unless app.txt holds `red`.
type proj struct {
	t      *testing.T
	root   string
	env    []string
	repo   *git.Repo
	aux    string // a folder outside the repository
	cfg    *project.Config
	intent *project.Intent
	green  string
	red    string
}

func newProj(t *testing.T) *proj {
	t.Helper()
	p := &proj{t: t, root: t.TempDir(), aux: t.TempDir()}
	p.env = append(processEnv(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	p.repo = git.New(p.root, p.env)
	p.git("init", "-q")
	p.git("config", "user.name", "Fixture")
	p.git("config", "user.email", "fixture@example.invalid")
	p.git("config", "core.hooksPath", t.TempDir())
	p.git("config", "commit.gpgsign", "false")
	p.write(".gitignore", ".kogen/local/\n")
	p.write("app.txt", "green\n")
	p.write(project.FeaturePath(slug), feature)
	expected, err := project.ParseGherkin(feature)
	if err != nil {
		t.Fatal(err)
	}
	p.green = writeReport(t, p.aux, "green.ndjson", project.FeaturePath(slug), expected, "")
	p.red = writeReport(t, p.aux, "red.ndjson", project.FeaturePath(slug), expected, "FAILED")
	run := fmt.Sprintf(`if grep -qx red app.txt; then cp %s "$KOGEN_ACCEPTANCE_REPORT"; echo actual-case-red; exit 1; fi; cp %s "$KOGEN_ACCEPTANCE_REPORT"`, quote(p.red), quote(p.green))
	p.cfg = &project.Config{Acceptance: &project.Acceptance{Run: run}}
	p.intent = &project.Intent{Root: p.root, Slug: slug, Model: &project.ModelSpec{None: true}}
	p.commit("fixture project")
	return p
}

func (p *proj) git(args ...string) string {
	p.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = p.root
	cmd.Env = p.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		p.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (p *proj) write(rel, content string) {
	p.t.Helper()
	path := filepath.Join(p.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func (p *proj) commit(msg string) {
	p.t.Helper()
	p.git("add", "-A")
	p.git("commit", "-qm", msg)
}

func (p *proj) head() string     { return p.git("rev-parse", "HEAD") }
func (p *proj) headTree() string { return p.git("rev-parse", "HEAD^{tree}") }

// session is a sys.Session over the project's isolated environment with stderr captured.
func (p *proj) session() (*sys.Session, *strings.Builder) {
	s := sys.NewWith(p.env, p.root)
	stderr := &strings.Builder{}
	s.Stderr = stderr
	return s, stderr
}

func (p *proj) input(n int) verify.Input {
	return verify.Input{Root: p.root, Repo: p.repo, Config: p.cfg, Intent: p.intent, Start: p.head(), N: n}
}

// script writes an executable shell script in a new temporary folder and returns its path.
func script(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// regular writes a regular file in a new temporary folder and returns its path.
func regular(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("// stand-in\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeReport writes a Cucumber Messages report with one case per expected scenario, every
// step PASSED, or all of them with the given status (FAILED carries a message), and returns
// its path.
func writeReport(t *testing.T, dir, name, uri string, expected []project.Scenario, status string) string {
	t.Helper()
	type obj = map[string]any
	var children, events []any
	for i, s := range expected {
		var steps []any
		for j, st := range s.Steps {
			steps = append(steps, obj{"id": fmt.Sprintf("ast-step-%d-%d", i, j), "location": obj{"line": st.Line}, "keyword": st.Keyword})
		}
		children = append(children, obj{"scenario": obj{"id": fmt.Sprintf("ast-scenario-%d", i), "location": obj{"line": s.Line}, "steps": steps, "examples": []any{}}})
	}
	events = append(events, obj{"gherkinDocument": obj{"uri": uri, "feature": obj{"children": children}}})
	for i, s := range expected {
		pickle, tcase, started := fmt.Sprintf("pickle-%d", i), fmt.Sprintf("case-%d", i), fmt.Sprintf("started-%d", i)
		var pickleSteps, testSteps, finished []any
		for j, st := range s.Steps {
			pid, tid := fmt.Sprintf("pickle-step-%d-%d", i, j), fmt.Sprintf("test-step-%d-%d", i, j)
			pickleSteps = append(pickleSteps, obj{"id": pid, "astNodeIds": []any{fmt.Sprintf("ast-step-%d-%d", i, j)}, "text": st.Text})
			testSteps = append(testSteps, obj{"id": tid, "pickleStepId": pid})
			result := obj{"status": "PASSED"}
			if status != "" {
				result = obj{"status": status}
				if status == "FAILED" {
					result["message"] = "boom: the candidate is red"
				}
			}
			finished = append(finished, obj{"testStepFinished": obj{"testCaseStartedId": started, "testStepId": tid, "testStepResult": result}})
		}
		events = append(events,
			obj{"pickle": obj{"id": pickle, "uri": uri, "name": s.Name, "astNodeIds": []any{fmt.Sprintf("ast-scenario-%d", i)}, "steps": pickleSteps}},
			obj{"testCase": obj{"id": tcase, "pickleId": pickle, "testSteps": testSteps}},
			obj{"testCaseStarted": obj{"id": started, "testCaseId": tcase}})
		events = append(events, finished...)
		events = append(events, obj{"testCaseFinished": obj{"testCaseStartedId": started, "willBeRetried": false}})
	}
	var lines []string
	for _, e := range events {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(b))
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// failWith is the sys.Error err carries, failing the test when it carries none.
func failWith(t *testing.T, item string, err error) *sys.Error {
	t.Helper()
	e, ok := sys.AsError(err)
	if !ok {
		t.Fatalf("%s: error %v is not a sys.Error", item, err)
	}
	return e
}
