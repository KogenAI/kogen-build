package consumes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

func TestToolsRequire(t *testing.T) {
	var _ func(verify.Tools) error = verify.Tools.Require
	tools := verify.Tools{Node: regular(t, "node"), Quint: regular(t, "cli.js"), Cucumber: t.TempDir()}
	var _ string = tools.Node
	var _ string = tools.Quint
	var _ string = tools.Cucumber

	if err := tools.Require(); err != nil {
		t.Errorf("verify.Tools.Require: both tools present: %v", err)
	}
	if err := (verify.Tools{Node: tools.Node, Quint: tools.Quint}).Require(); err != nil {
		t.Errorf("verify.Tools.Require: the Cucumber folder is not required: %v", err)
	}

	missing := filepath.Join(t.TempDir(), "absent")
	err := verify.Tools{Node: missing, Quint: tools.Quint}.Require()
	e := failWith(t, "verify.Tools.Require", err)
	if e.Class != "environment" || e.Reason != "tool_missing" || e.Exit != 3 || e.Detail != "node is not available at "+missing+"; reinstall Kogen" {
		t.Errorf("verify.Tools.Require: missing node = %+v, want environment/tool_missing, exit 3 and the node path", e)
	}
	err = verify.Tools{Node: tools.Node, Quint: missing}.Require()
	e = failWith(t, "verify.Tools.Require", err)
	if e.Class != "environment" || e.Reason != "tool_missing" || e.Exit != 3 || e.Detail != "quint is not available at "+missing+"; reinstall Kogen" {
		t.Errorf("verify.Tools.Require: missing quint = %+v, want environment/tool_missing, exit 3 and the quint path", e)
	}
	// Node is judged first, and a folder is not a tool.
	err = verify.Tools{Node: t.TempDir(), Quint: missing}.Require()
	if e := failWith(t, "verify.Tools.Require", err); !strings.HasPrefix(e.Detail, "node is not available at ") {
		t.Errorf("verify.Tools.Require: a folder as node = %+v, want node refused first", e)
	}
	if err := (verify.Tools{}).Require(); err == nil {
		t.Errorf("verify.Tools.Require: no tools at all passed")
	}
}

func TestRequireTools(t *testing.T) {
	var _ func(*sys.Session, verify.Tools, string, *project.Config, bool) error = verify.RequireTools

	p := newProj(t)
	s, _ := p.session()
	none := verify.Tools{}

	if err := verify.RequireTools(s, none, p.root, &project.Config{}, false); err != nil {
		t.Errorf("verify.RequireTools: only git required: %v", err)
	}
	// Not configured: commands of the configuration are not looked at.
	missingCmd := &project.Config{Checks: []project.Check{{Name: "lint", Run: "kogen-no-such-tool-xyz --flag"}}}
	if err := verify.RequireTools(s, none, p.root, missingCmd, false); err != nil {
		t.Errorf("verify.RequireTools: configured=false looked at the configuration: %v", err)
	}
	err := verify.RequireTools(s, none, p.root, missingCmd, true)
	e := failWith(t, "verify.RequireTools", err)
	if e.Class != "environment" || e.Reason != "tool_missing" || e.Exit != 3 || e.Detail != "kogen-no-such-tool-xyz is not available" {
		t.Errorf("verify.RequireTools: missing check tool = %+v, want environment/tool_missing, exit 3, %q", e, "kogen-no-such-tool-xyz is not available")
	}

	// Setup, fix, check and acceptance commands all count; assignments and grouping are skipped.
	for name, cfg := range map[string]*project.Config{
		"setup":      {Setup: []string{"kogen-no-such-tool-xyz"}},
		"fix":        {Fix: []string{"kogen-no-such-tool-xyz"}},
		"acceptance": {Acceptance: &project.Acceptance{Run: "kogen-no-such-tool-xyz"}},
		"assignment": {Checks: []project.Check{{Name: "a", Run: "A=1 B='x y' kogen-no-such-tool-xyz"}}},
		"grouping":   {Checks: []project.Check{{Name: "a", Run: "(kogen-no-such-tool-xyz --flag)"}}},
	} {
		err := verify.RequireTools(s, none, p.root, cfg, true)
		if e, ok := sys.AsError(err); !ok || e.Detail != "kogen-no-such-tool-xyz is not available" {
			t.Errorf("verify.RequireTools: %s command: %v, want the missing tool named", name, err)
		}
	}
	present := &project.Config{
		Setup:      []string{"sh -c true"},
		Checks:     []project.Check{{Name: "a", Run: "FOO=1 sh -c true"}, {Name: "b", Run: "(sh -c true)"}},
		Acceptance: &project.Acceptance{Run: "sh ./x"},
	}
	if err := verify.RequireTools(s, none, p.root, present, true); err != nil {
		t.Errorf("verify.RequireTools: commands of present tools: %v", err)
	}

	// A command that starts with $KOGEN_NODE needs the Node Kogen records, unless the caller's
	// own KOGEN_NODE names it.
	nodeCfg := &project.Config{Acceptance: &project.Acceptance{Run: `$KOGEN_NODE features/run.js`}}
	node := script(t, "node", "exit 0\n")
	if err := verify.RequireTools(s, verify.Tools{Node: node}, p.root, nodeCfg, true); err != nil {
		t.Errorf("verify.RequireTools: recorded node: %v", err)
	}
	absent := filepath.Join(t.TempDir(), "absent-node")
	err = verify.RequireTools(s, verify.Tools{Node: absent}, p.root, nodeCfg, true)
	if e, ok := sys.AsError(err); !ok || e.Class != "environment" || e.Reason != "tool_missing" || e.Detail != "$KOGEN_NODE is not available" {
		t.Errorf("verify.RequireTools: absent recorded node: %v, want $KOGEN_NODE named", err)
	}
	callers := sys.NewWith(append(append([]string{}, p.env...), "KOGEN_NODE="+node), p.root)
	if err := verify.RequireTools(callers, verify.Tools{Node: absent}, p.root, nodeCfg, true); err != nil {
		t.Errorf("verify.RequireTools: the caller's KOGEN_NODE wins over the recorded node: %v", err)
	}

	// No git on the session's PATH.
	noGit := sys.NewWith([]string{"PATH=" + t.TempDir(), "HOME=" + t.TempDir()}, p.root)
	err = verify.RequireTools(noGit, none, p.root, &project.Config{}, false)
	if e, ok := sys.AsError(err); !ok || e.Class != "environment" || e.Reason != "tool_missing" || e.Detail != "git is not available" || e.Exit != 3 {
		t.Errorf("verify.RequireTools: no git: %v, want environment/tool_missing: git is not available, exit 3", err)
	}
}

func TestFix(t *testing.T) {
	var _ func(context.Context, *sys.Session, verify.Input) error = verify.Fix

	p := newProj(t)
	s, _ := p.session()
	trace := filepath.Join(p.aux, "fix.txt")
	p.cfg.Fix = []string{
		"pwd >> " + quote(trace) + "; echo a >> fixed.txt",
		"echo b >> fixed.txt; printf '%s|%s|%s\\n' \"$KOGEN_NODE\" \"$KOGEN_QUINT\" \"$KOGEN_CUCUMBER\" >> " + quote(trace),
		"exit 4",
	}
	var log memLog
	intacts := 0
	in := p.input(1)
	in.Tools = verify.Tools{Node: "/n/node", Quint: "/q/cli.js", Cucumber: "/c/modules"}
	in.Log = &log
	in.Intact = func() error { intacts++; return nil }

	if err := verify.Fix(context.Background(), s, in); err != nil {
		t.Fatalf("verify.Fix: %v (a fixer's own nonzero status is not an error)", err)
	}
	if b, _ := os.ReadFile(filepath.Join(p.root, "fixed.txt")); string(b) != "a\nb\n" {
		t.Errorf("verify.Fix: fixers in the checkout, in order, gave %q, want a then b", b)
	}
	b, _ := os.ReadFile(trace)
	if lines := strings.Split(strings.TrimSpace(string(b)), "\n"); len(lines) != 2 || lines[1] != "/n/node|/q/cli.js|/c/modules" {
		t.Errorf("verify.Fix: trace %q, want the Kogen tool variables passed to the fixer", b)
	}
	if intacts != 3 {
		t.Errorf("verify.Fix: Intact ran %d times, want once after each of 3 fixers", intacts)
	}
	if got := log.names("command", "command_result"); strings.Join(got, ",") != "command fix-1,command_result fix-1,command fix-2,command_result fix-2,command fix-3,command_result fix-3" {
		t.Errorf("verify.Fix: run log %v", got)
	}
	if res := log.named("command_result"); res[2]["status"] != 4 {
		t.Errorf("verify.Fix: the third result's status = %v, want 4", res[2]["status"])
	}

	// The refusal of the baseline guard is returned and stops the next fixers.
	p2 := newProj(t)
	s2, _ := p2.session()
	p2.cfg.Fix = []string{"echo one >> fixed.txt", "echo two >> fixed.txt"}
	refusal := sys.Fail("baseline", "changed", "the approved baseline changed", 1)
	in2 := p2.input(1)
	in2.Intact = func() error { return refusal }
	if err := verify.Fix(context.Background(), s2, in2); err != error(refusal) {
		t.Errorf("verify.Fix: Intact's error = %v, want it returned as is", err)
	}
	if b, _ := os.ReadFile(filepath.Join(p2.root, "fixed.txt")); string(b) != "one\n" {
		t.Errorf("verify.Fix: after the refusal the checkout holds %q, want only the first fixer's edit", b)
	}
	// No fixers, no Intact needed.
	p2.cfg.Fix = nil
	in2.Intact = nil
	if err := verify.Fix(context.Background(), s2, in2); err != nil {
		t.Errorf("verify.Fix: no fixers: %v", err)
	}
}

func TestReadinessSetup(t *testing.T) {
	var _ func(*verify.Readiness, context.Context, *sys.Session, string, bool) error = (*verify.Readiness).Setup

	p := newProj(t)
	s, stderr := p.session()
	var log memLog
	cfg := &project.Config{Setup: []string{"echo first >> setup.txt; pwd > where.txt", "echo second >> setup.txt; echo \"$KOGEN_NODE\" > node.txt"}}
	r := &verify.Readiness{Root: p.root, Repo: p.repo, Config: cfg, Tools: verify.Tools{Node: "/n/node"}, Log: &log}
	var _ string = r.Root
	var _ = r.Repo
	var _ *project.Config = r.Config
	var _ verify.Tools = r.Tools
	var _ verify.Logger = r.Log

	// A Build always runs setup, even when it last ran for this very state.
	for round := 1; round <= 2; round++ {
		if err := r.Setup(context.Background(), s, p.root, false); err != nil {
			t.Fatalf("verify.Readiness.Setup: round %d: %v", round, err)
		}
		want := strings.Repeat("first\n", round)
		if b, _ := os.ReadFile(filepath.Join(p.root, "setup.txt")); !strings.HasPrefix(string(b), "first\n") || strings.Count(string(b), "first") != round || strings.Count(string(b), "second") != round {
			t.Errorf("verify.Readiness.Setup: round %d: setup.txt = %q, want %d runs of both commands (%q first)", round, b, round, want)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(p.root, "node.txt")); strings.TrimSpace(string(b)) != "/n/node" {
		t.Errorf("verify.Readiness.Setup: KOGEN_NODE seen by setup = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(p.root, "where.txt")); strings.TrimSpace(string(b)) != p.root {
		t.Errorf("verify.Readiness.Setup: ran in %q, want the checkout %q", b, p.root)
	}
	if got := log.names("command", "command_result"); len(got) != 8 || got[0] != "command setup-1" || got[1] != "command_result setup-1" || got[2] != "command setup-2" {
		t.Errorf("verify.Readiness.Setup: run log %v", got)
	}
	if !strings.Contains(stderr.String(), "kogen: running setup-1: echo first") {
		t.Errorf("verify.Readiness.Setup: progress %q", stderr.String())
	}

	// The first failing command refuses with environment/setup_failed and its output's tail.
	p2 := newProj(t)
	s2, _ := p2.session()
	failing := &project.Config{Setup: []string{"echo ran >> setup.txt", "i=1; while [ $i -le 30 ]; do echo line $i; i=$((i+1)); done; exit 3", "echo never >> setup.txt"}}
	err := (&verify.Readiness{Root: p2.root, Repo: p2.repo, Config: failing, Log: &log}).Setup(context.Background(), s2, p2.root, false)
	e := failWith(t, "verify.Readiness.Setup", err)
	if e.Class != "environment" || e.Reason != "setup_failed" || e.Detail != "Setup setup-2 failed (status=3)." || e.Exit != 3 {
		t.Errorf("verify.Readiness.Setup: %+v, want environment/setup_failed: Setup setup-2 failed (status=3)., exit 3", e)
	}
	if len(e.Lines) != 20 || e.Lines[0] != "line 11" || e.Lines[19] != "line 30" {
		t.Errorf("verify.Readiness.Setup: detail lines %q, want the last 20 lines of the output", e.Lines)
	}
	if b, _ := os.ReadFile(filepath.Join(p2.root, "setup.txt")); string(b) != "ran\n" {
		t.Errorf("verify.Readiness.Setup: setup.txt = %q, want the commands after the failure not to run", b)
	}
}
