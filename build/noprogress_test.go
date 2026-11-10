package build

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

// A builder round that leaves the candidate unchanged after a red pass is not verified again:
// the first gets a notice and the second stops the Build.

// says is a response of text only: the builder ends its turn without a tool call.
func says(text string) handler {
	return sse(map[string]any{"type": "message", "id": "msg-say", "role": "assistant",
		"content": []any{map[string]any{"type": "output_text", "text": text}}})
}

// sayings are enough text-only answers to show an unbounded loop if the Build kept asking.
func sayings(n int) []handler {
	var h []handler
	for i := 0; i < n; i++ {
		h = append(h, says("Pass remains red; I made no correction and cannot finish."))
	}
	return h
}

func passEvents(t *testing.T, e *env, id string) int {
	t.Helper()
	log, err := os.ReadFile(e.runPath(id, "log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(log), `"event":"pass"`)
}

func TestASecondRoundWithoutEditsIntoTheSameRedStopsWithTheCandidatePreserved(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	head := e.head()
	e.prov.then(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}))
	e.prov.then(sayings(12)...)
	a := e.build()
	if a.exit != 1 || a.err != nil {
		t.Fatalf("exit %d err %v\n%s", a.exit, a.err, a.shown())
	}
	id := e.onlyRun()
	out := a.out.String()
	contains(t, "stdout", out, "failed fixture: no_progress (Build "+id+")\n")
	contains(t, "stdout", out, "candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none; verification pass 1 is still red and was not run again; edits are left in place\n")
	contains(t, "stdout", out, "  failing: acceptance\n  first failure: failed: First (line 2)\n  rounds without edits: 2 of 3\n")
	contains(t, "stdout", out, "  edited: app.txt\n")
	eq(t, "model requests", e.prov.count(), 3)
	eq(t, "pass events", passEvents(t, e, id), 1)
	res := e.result(id)
	eq(t, "outcome", res["outcome"], any("no_progress"))
	eq(t, "passes", res["passes"], any(float64(1)))
	eq(t, "candidate", e.read("app.txt"), "red\n")
	eq(t, "head", e.head(), head)
}

// lastInput is the text the builder received last in a request: a user message or the output
// that answers its finish call.
func lastInput(req map[string]any) string {
	items, _ := req["input"].([]any)
	if len(items) == 0 {
		return ""
	}
	it, _ := items[len(items)-1].(map[string]any)
	if s, ok := it["output"].(string); ok {
		return s
	}
	switch c := it["content"].(type) {
	case string:
		return c
	case []any:
		var parts []string
		for _, p := range c {
			if m, ok := p.(map[string]any); ok {
				if s, ok := m["text"].(string); ok {
					parts = append(parts, s)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

const noEditNoticeText = "Round 2 made no edits: the candidate is byte-identical to the tree verification pass 1 checked, so it was not verified again; pass 1 is still red.\n" +
	"Make an edit toward the first named failure: failed: First (line 2).\n" +
	"If a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\n" +
	"Another round without edits stops the Build as no_progress (no edits)."

func TestARoundWithoutEditsIntoTheSameRedIsNotVerifiedAgainAndGetsOnlyTheNotice(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}),
		says("The round is complete."),
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "green\n"}, "finish", map[string]any{"summary": "the rest"}),
	)
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	id := e.onlyRun()
	reqs := f.requests()
	eq(t, "model requests", len(reqs), 3)
	eq(t, "outcome", e.result(id)["outcome"], any("landed"))
	eq(t, "passes", e.result(id)["passes"], any(float64(2)))
	eq(t, "pass events", passEvents(t, e, id), 2)
	eq(t, "the builder's next message", lastInput(reqs[2]), noEditNoticeText)
	if !strings.HasPrefix(lastInput(reqs[1]), "Verification pass 1 is not green.") {
		t.Fatalf("the first red pass did not get the pass feedback:\n%s", lastInput(reqs[1]))
	}
	contains(t, "stderr", a.errs.String(), "kogen: round 2 made no edits; not verifying the tree of pass 1 again; asked the builder to edit toward failed: First (line 2) or report a protected-file blocker\n")
}

func TestAReadOnlyRoundCountsAsARoundWithoutEdits(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}),
		f.calls("read_file", map[string]any{"path": "app.txt"}, "finish", map[string]any{"summary": "diagnosed"}),
		says("I changed nothing."),
	)
	f.then(sayings(12)...)
	a := e.build()
	eq(t, "exit", a.exit, 1)
	id := e.onlyRun()
	contains(t, "stdout", a.out.String(), "candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none; verification pass 1 is still red")
	eq(t, "model requests", f.count(), 3)
	eq(t, "pass events", passEvents(t, e, id), 1)
}

func TestARoundThatEditsIsVerifiedEvenWhenTheFailingSetIsUnchanged(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}),
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\nagain\n"}, "finish", map[string]any{"summary": "second"}),
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "green\n"}, "finish", map[string]any{"summary": "repaired"}),
	)
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	eq(t, "passes", e.result(e.onlyRun())["passes"], any(float64(3)))
	eq(t, "model requests", f.count(), 3)
}

// Rewrites still run verification, but four identical red passes stop the Build.
func TestRepeatedFailureDespiteRewritesStops(t *testing.T) {
	e := newEnv(t)
	f := e.prov
	for i := 0; i < 5; i++ {
		f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "again"}))
	}
	a := e.build()
	eq(t, "exit", a.exit, 1)
	id := e.onlyRun()
	eq(t, "passes", passEvents(t, e, id), 4)
	eq(t, "model requests", f.count(), 4)
	contains(t, "diagnostic", a.out.String(), "candidate/no_progress")
	contains(t, "scenario", a.out.String(), "First (line 2)")
	events := eventsNamed(e.events(id), "no_progress")
	if len(events) != 1 {
		t.Fatalf("events: %v", events)
	}
	eq(t, "count", events[0]["count"], any(float64(4)))
	if fp, ok := events[0]["fingerprint"].(string); !ok || len(fp) != 64 {
		t.Fatalf("fingerprint %v", events[0]["fingerprint"])
	}
	failing, _ := json.Marshal(events[0]["failing"])
	eq(t, "failing", string(failing), `["acceptance"]`)

}

// A rewrite round is not a round without edits and gives the remembered red pass: the text-only
// rounds after it are measured against pass 2.
func TestARewriteRoundIsNotCountedAsWithoutEditsAndGivesTheRememberedRedPass(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}),
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "again"}),
		says("I changed nothing."),
		says("Still nothing."),
	)
	f.then(sayings(5)...)
	a := e.build()
	eq(t, "exit", a.exit, 1)
	id := e.onlyRun()
	out := a.out.String()
	contains(t, "stdout", out, "candidate/no_progress: no edits: round 4 made no edits after the builder was told round 3 made none; verification pass 2 is still red and was not run again; edits are left in place\n")
	contains(t, "stdout", out, "  rounds without edits: 2 of 4\n")
	eq(t, "model requests", f.count(), 4)
	eq(t, "pass events", passEvents(t, e, id), 2)
}

func TestARoundWithoutAToolCallContinuesWhenTheFixersChangedTheCandidate(t *testing.T) {
	t.Parallel()
	// Each fixer run changes an untracked candidate file, so every pass verifies a new tree.
	e := newEnv(t, func(c *project.Config) { c.Fix = []string{"printf x >> tick.txt"} })
	f := e.prov
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}),
		says("Nothing to add yet."),
		says("Still thinking."),
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "green\n"}, "finish", map[string]any{"summary": "repaired"}),
	)
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	eq(t, "passes", e.result(e.onlyRun())["passes"], any(float64(4)))
	eq(t, "model requests", f.count(), 4)
}

func TestAChangedFileModeIsProgressForARoundWithoutAToolCall(t *testing.T) {
	t.Parallel()
	// The fixer flips the executable bit each round: same content, different candidate tree.
	e := newEnv(t, func(c *project.Config) {
		c.Fix = []string{"if test -x run.sh; then chmod -x run.sh; else chmod +x run.sh; fi"}
	})
	f := e.prov
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"},
			"write_file", map[string]any{"path": "run.sh", "content": "#!/bin/sh\n"}, "finish", map[string]any{"summary": "first"}),
		says("Nothing to add yet."),
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "green\n"}, "finish", map[string]any{"summary": "repaired"}),
	)
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	eq(t, "model requests", f.count(), 3)
}

func TestAnIgnoredFileChangeIsNotProgressBecauseVerificationSeesOnlyTheCandidateTree(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *project.Config) { c.Fix = []string{"printf x >> .kogen/local/ignored-tick"} })
	e.prov.then(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}))
	e.prov.then(sayings(5)...)
	a := e.build()
	eq(t, "exit", a.exit, 1)
	contains(t, "stdout", a.out.String(), "candidate/no_progress: no edits:")
	eq(t, "model requests", e.prov.count(), 3)
	eq(t, "pass events", passEvents(t, e, e.onlyRun()), 1)
}

func TestARedStageGateRepeatedWithoutEditsStopsAtTheStage(t *testing.T) {
	t.Parallel()
	checks := []string{"if grep -qx red app.txt; then echo scoped-red; exit 23; fi"}
	e, _ := stagedEnv(t, checks)
	f := e.prov
	f.then(f.calls(stageWrite(f, "red")...), f.calls(stageFinish()...))
	f.then(sayings(12)...)
	a := e.build()
	eq(t, "exit", a.exit, 1)
	out := a.out.String()
	contains(t, "stdout", out, "candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none;")
	contains(t, "stdout", out, "  failing: stage-foundation-1\n  first failure: stage-foundation-1\n")
	eq(t, "model requests", f.count(), 4) // write, finish, two answers without edits
	eq(t, "candidate", e.read("app.txt"), "red\n")
	_, _, state := e.stageCheckpoint()
	eq(t, "active", state["stage_progress"].(map[string]any)["active"], any("foundation"))
}

func TestAStoppedNoProgressBuildResumesFromItsCheckpoint(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}), says("No change."), says("Still no change."))
	a := e.build()
	eq(t, "exit", a.exit, 1)
	id := e.onlyRun()
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": "green\n"}, "finish", map[string]any{"summary": "repaired"}))
	r := e.resume(id)
	if r.exit != 0 {
		t.Fatalf("resume exit %d\n%s", r.exit, r.shown())
	}
	eq(t, "candidate", e.read("app.txt"), "green\n")
}

func TestRepeatedFailureStreakResets(t *testing.T) {
	base := verify.Pass{Clean: true, Failing: []string{"check"}, Checks: []verify.CommandResult{{Name: "check", Status: 1, Output: "first"}}}
	for _, tc := range []struct {
		name   string
		stage  string
		change func(*verify.Pass)
		want   int
	}{
		{"green", "s", func(p *verify.Pass) { p.Green = true }, 0},
		{"unclean", "s", func(p *verify.Pass) { p.Clean = false }, 0},
		{"moved", "s", func(p *verify.Pass) { p.Moved = []string{"x"} }, 0},
		{"stage", "other", func(p *verify.Pass) {}, 1},
		{"feedback", "s", func(p *verify.Pass) { p.Checks = []verify.CommandResult{{Name: "check", Status: 1, Output: "changed"}} }, 1},
		{"failing", "s", func(p *verify.Pass) { p.Failing = []string{"other"} }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &Attempt{State: State{Config: &project.Config{}}}
			for i := 0; i < 3; i++ {
				p := base
				p.N = i + 1
				p.Tree = itoa(i)
				if err := b.repeatedFailure("s", p); err != nil {
					t.Fatal(err)
				}
			}
			p := base
			tc.change(&p)
			if err := b.repeatedFailure(tc.stage, p); err != nil {
				t.Fatal(err)
			}
			eq(t, "count", b.noEdit.redCount, tc.want)
			if tc.want == 0 {
				if err := b.repeatedFailure("s", base); err != nil {
					t.Fatal(err)
				}
				eq(t, "restart", b.noEdit.redCount, 1)
			}
		})
	}
}

func TestRepeatedStageFailureDespiteEditsStops(t *testing.T) {
	e, _ := stagedEnv(t, []string{"echo scoped-red; exit 23"})
	f := e.prov
	for i := 0; i < 5; i++ {
		f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": fmt.Sprintf("red %d\n", i)}, "finish", map[string]any{"summary": "verify"}))
	}
	a := e.build()
	eq(t, "exit", a.exit, 1)
	contains(t, "diagnostic", a.out.String(), "candidate/no_progress")
	contains(t, "failing", a.out.String(), "stage-foundation-1")
	events := eventsNamed(e.events(e.onlyRun()), "no_progress")
	if len(events) != 1 {
		t.Fatalf("events: %v", events)
	}
	eq(t, "count", events[0]["count"], any(float64(4)))
}
