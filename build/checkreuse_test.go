package build

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
	"kogen-loop/sys"
)

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// reuseEnv is a repository whose checks append their names to an execution log outside it.
func reuseEnv(t *testing.T, checks ...project.Check) (*env, string) {
	t.Helper()
	e := newEnv(t, func(c *project.Config) { c.Checks = checks })
	log := filepath.Join(physical(t, t.TempDir()), "executions.log")
	e.extraEnv = []string{"KBO_LOG=" + log, "KBO_OPEN=" + filepath.Join(filepath.Dir(log), "gate-open")}
	return e, log
}

func executionsOf(t *testing.T, log string) []string {
	t.Helper()
	b, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return []string{}
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(b))
}

func reuseEvents(t *testing.T, e *env, id string) []map[string]any {
	t.Helper()
	events := eventsNamed(e.events(id), "check_reuse")
	for _, ev := range events {
		name, _ := ev["name"].(string)
		key, _ := ev["key"].(string)
		if !hex64.MatchString(key) || ev["message"] != "baseline reused: "+name+" "+key {
			t.Fatalf("malformed reuse event: %v", ev)
		}
	}
	return events
}

func TestCheckReuseWithinABuild(t *testing.T) {
	t.Parallel()
	lint := project.Check{Name: "lint", Run: `echo lint >> "$KBO_LOG"`}
	t.Run("an unchanged tree reuses the passed baseline", func(t *testing.T) {
		t.Parallel()
		e, log := reuseEnv(t, lint)
		e.prov.then(e.prov.calls("finish", map[string]any{"summary": "done"}))
		start := e.head()
		e.build()
		id := e.onlyRun()
		eq(t, "outcome", e.result(id)["outcome"], any("landed"))
		eq(t, "executions", executionsOf(t, log), []string{"lint"})
		events := reuseEvents(t, e, id)
		if len(events) != 1 || events[0]["name"] != "lint" || events[0]["command"] != lint.Run || events[0]["pass"] != float64(1) {
			t.Fatalf("reuse events: %v", events)
		}
		_, state := e.checkpoint(id)
		pass := state["last_pass"].(map[string]any)
		eq(t, "green", pass["green"], any(true))
		eq(t, "commits", e.gitOut("rev-list", "--count", start+"..HEAD"), "1")
	})
	t.Run("a changed tree runs the check again", func(t *testing.T) {
		t.Parallel()
		e, log := reuseEnv(t, lint)
		e.prov.then(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}, "finish", map[string]any{"summary": "done"}))
		e.build()
		id := e.onlyRun()
		eq(t, "outcome", e.result(id)["outcome"], any("landed"))
		eq(t, "executions", executionsOf(t, log), []string{"lint", "lint"})
		eq(t, "reuse events", len(reuseEvents(t, e, id)), 0)
	})
}

func TestCheckLedgerIsSavedAndReusedAfterResume(t *testing.T) {
	t.Parallel()
	e, log := reuseEnv(t,
		project.Check{Name: "lint", Run: `echo lint >> "$KBO_LOG"`},
		project.Check{Name: "gate", Run: `echo gate >> "$KBO_LOG"; test -f "$KBO_OPEN"`})
	id, _ := e.interruptedAtRequest(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}, "finish", map[string]any{"summary": "done"}))
	_, state := e.checkpoint(id)
	records, _ := state["check_results"].([]any)
	if len(records) != 1 {
		t.Fatalf("check_results: %v", records)
	}
	record := records[0].(map[string]any)
	eq(t, "record name", record["name"], any("lint"))
	eq(t, "record command", record["command"], any(`echo lint >> "$KBO_LOG"`))
	for _, field := range []string{"key", "environment"} {
		if s, _ := record[field].(string); !hex64.MatchString(s) {
			t.Fatalf("record %s = %v", field, record[field])
		}
	}
	if s, _ := record["tree"].(string); s == "" {
		t.Fatalf("record tree: %v", record)
	}
	eq(t, "executions before the resume", executionsOf(t, log), []string{"lint", "gate", "lint", "gate"})
	writeFile(t, filepath.Join(filepath.Dir(log), "gate-open"), "open\n", 0o644)
	e.prov.replace(e.prov.calls("finish", map[string]any{"summary": "done"}))
	e.resume(id)
	eq(t, "executions", executionsOf(t, log), []string{"lint", "gate", "lint", "gate", "gate"})
	for _, run := range e.runIDs() {
		if run == id {
			eq(t, "first run reuse", len(reuseEvents(t, e, run)), 0)
			continue
		}
		eq(t, "outcome", e.result(run)["outcome"], any("landed"))
		events := reuseEvents(t, e, run)
		if len(events) != 1 || events[0]["name"] != "lint" {
			t.Fatalf("resumed run reuse events: %v", events)
		}
	}
}

func TestAFixLoopRunsTheFailedCheckFirstAndTheSuiteOnce(t *testing.T) {
	t.Parallel()
	e, log := reuseEnv(t,
		project.Check{Name: "first", Run: `echo first >> "$KBO_LOG"`},
		project.Check{Name: "second", Run: `echo second >> "$KBO_LOG"; grep -qx fixed app.txt`})
	write := func(content string) handler {
		return e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": content}, "finish", map[string]any{"summary": "done"})
	}
	e.prov.then(write("half\n"), write("fixed\n"))
	e.build()
	id := e.onlyRun()
	eq(t, "outcome", e.result(id)["outcome"], any("landed"))
	// The baseline, pass 1 in configured order, then pass 2: the check pass 1 failed, then the rest.
	eq(t, "executions", executionsOf(t, log), []string{"first", "second", "first", "second", "second", "first"})
}

func TestAPassWhoseFullSuiteFailsAfterItsTargetedCheckPassedIsNotGreen(t *testing.T) {
	t.Parallel()
	e, log := reuseEnv(t,
		project.Check{Name: "first", Run: `echo first >> "$KBO_LOG"; ! grep -qx broken app.txt`},
		project.Check{Name: "second", Run: `echo second >> "$KBO_LOG"; grep -qx broken app.txt`})
	write := func(content string) handler {
		return e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": content}, "finish", map[string]any{"summary": "done"})
	}
	e.prov.then(write("half\n"), write("broken\n"), refused())
	start := e.head()
	e.build()
	id := e.onlyRun()
	eq(t, "commits", e.gitOut("rev-list", "--count", start+"..HEAD"), "0")
	_, state := e.checkpoint(id)
	pass := state["last_pass"].(map[string]any)
	eq(t, "green", pass["green"], any(false))
	eq(t, "failing", pass["failing"], any([]any{"first"}))
	eq(t, "executions", executionsOf(t, log), []string{"first", "second", "first", "second", "second", "first"})
}

// checkAttempt is an Attempt on a temporary checkout with a session of the given environment.
func checkAttempt(t *testing.T, env []string, checks ...project.Check) *Attempt {
	t.Helper()
	root := physical(t, t.TempDir())
	return &Attempt{State: State{Root: root, Config: &project.Config{Checks: checks}}, s: sys.NewWith(env, root)}
}

func TestCheckEnvironmentAndKeyIdentity(t *testing.T) {
	t.Parallel()
	tools := physical(t, t.TempDir())
	script := filepath.Join(physical(t, t.TempDir()), "gate.sh")
	writeFile(t, filepath.Join(tools, "lint"), "#!/bin/sh\n:\n", 0o755)
	writeFile(t, script, "exit 0\n", 0o644)
	check := project.Check{Name: "gate", Run: "sh '" + script + "' --flag"}
	environment := func(env ...string) string {
		a := checkAttempt(t, append([]string{"PATH=" + tools}, env...), check)
		return checkEnvironment(toolchainIdentity(a.commandEnv()), check)
	}
	base := environment("A=1")
	eq(t, "the same inputs", environment("A=1"), base)
	eq(t, "a later duplicate wins", environment("A=0", "A=1"), base)
	if environment("A=2") == base {
		t.Error("a session variable did not change the environment")
	}
	if environment("A=1", "B=1") == base {
		t.Error("a new session variable did not change the environment")
	}
	writeFile(t, filepath.Join(tools, "lint"), "#!/bin/sh\n: version 2\n", 0o755)
	changed := environment("A=1")
	if changed == base {
		t.Error("a PATH entry's file did not change the environment")
	}
	writeFile(t, filepath.Join(tools, "extra"), "", 0o755)
	if environment("A=1") == changed {
		t.Error("a new PATH entry did not change the environment")
	}
	before := environment("A=1")
	writeFile(t, script, "exit 1\n", 0o644)
	if environment("A=1") == before {
		t.Error("a file the command names did not change the environment")
	}
	a := checkAttempt(t, []string{"PATH=" + tools, "KOGEN_NODE=old"})
	a.deps.Tools = verify.Tools{Node: "new"}
	eq(t, "the tool variable overrides the session's", a.commandEnv()["KOGEN_NODE"], "new")
	if _, ok := a.commandEnv()["KOGEN_QUINT"]; ok {
		t.Error("an empty tool variable is part of the environment")
	}

	key := checkKey("gate", "cmd", "tree", "env")
	if !hex64.MatchString(key) {
		t.Fatalf("key %q", key)
	}
	eq(t, "the same key", checkKey("gate", "cmd", "tree", "env"), key)
	for i, other := range []string{checkKey("lint", "cmd", "tree", "env"), checkKey("gate", "cmd2", "tree", "env"),
		checkKey("gate", "cmd", "tree2", "env"), checkKey("gate", "cmd", "tree", "env2"), checkKey("gate", "cmdtree", "", "env")} {
		if other == key {
			t.Errorf("part %d did not change the key", i)
		}
	}
}

func TestComparisonFindsAChecksResultByName(t *testing.T) {
	t.Parallel()
	checks := []project.Check{{Name: "a", Run: "a"}, {Name: "b", Run: "b"}, {Name: "c", Run: "c"}}
	failed := verify.CommandResult{Name: "b", Command: "b", Status: 1, Output: "b broke"}
	passed := verify.CommandResult{Name: "c", Command: "c"}
	// The pass has results for b and c only: they are not a prefix of the configuration.
	pass := verify.Pass{N: 2, Checks: []verify.CommandResult{failed, passed}}
	a := &Attempt{State: State{Config: &project.Config{Checks: checks}, LastPass: &pass, CheckBaseline: []verify.CommandResult{
		{Name: "a", Command: "a"}, {Name: "b", Command: "b", Status: 1, Output: "b baseline"}, {Name: "c", Command: "c"}}}}
	rows := a.comparison()
	eq(t, "rows", len(rows), 3)
	eq(t, "a after", rows[0].After.State, checkNotRun)
	eq(t, "b after", rows[1].After.State, checkFailed)
	eq(t, "c after", rows[2].After.State, checkPassed)
	eq(t, "c before", rows[2].Before.State, checkPassed)
	msg := a.reviewBaselineMessage(pass)
	text := msg[0].(map[string]any)["content"].(string)
	contains(t, "review message", text, "1. a (a): baseline passed → final not run")
	contains(t, "review message", text, "2. b (b): baseline failed (status=1) → final failed (status=1)")
	contains(t, "review message", text, "3. c (c): baseline passed → final passed")
}

func TestAccountKeepsTheWholeConfiguredSuiteInOrder(t *testing.T) {
	t.Parallel()
	checks := []project.Check{{Name: "a", Run: "a"}, {Name: "b", Run: "b"}, {Name: "c", Run: "c"}}
	a := &Attempt{State: State{Config: &project.Config{Checks: checks}}}
	d := reuseDecision{point: checkPoint{tree: "T"}, reused: map[int]checkResult{0: {Name: "a", Output: "recorded"}}}
	// verify ran c then b; c failed, so b did not run.
	pass := verify.Pass{Tree: "T", Checks: []verify.CommandResult{{Name: "c", Command: "c", Status: 1}},
		NotRun: []string{"b"}, Failing: []string{"acceptance", "c"}}
	got := a.account(d, pass)
	eq(t, "checks", got.Checks, []verify.CommandResult{{Name: "a", Command: "a", Output: "recorded"}, {Name: "c", Command: "c", Status: 1}})
	eq(t, "not run", got.NotRun, []string{"b"})
	eq(t, "failing", got.Failing, []string{"acceptance", "c"})
	eq(t, "green", got.Green, false)

	pass = verify.Pass{Tree: "T", Green: true, Checks: []verify.CommandResult{{Name: "b", Command: "b"}, {Name: "c", Command: "c"}}, NotRun: []string{}, Failing: []string{}}
	got = a.account(d, pass)
	eq(t, "all checks", len(got.Checks), 3)
	eq(t, "green with reuse", got.Green, true)

	// A pass on another tree than the reuse was decided on does not count its reused checks.
	pass.Tree = "U"
	got = a.account(d, pass)
	eq(t, "other tree: checks", len(got.Checks), 2)
	eq(t, "other tree: not run", got.NotRun, []string{"a"})
	eq(t, "other tree: green", got.Green, false)
}
