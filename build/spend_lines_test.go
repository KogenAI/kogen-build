package build

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/executor"
)

func spendHas(t *testing.T, out, re string) {
	t.Helper()
	if !regexp.MustCompile("(?m)" + re).MatchString(out) {
		t.Errorf("no line matches %q in:\n%s", re, out)
	}
}

func spendHasNot(t *testing.T, out, sub string) {
	t.Helper()
	if strings.Contains(out, sub) {
		t.Errorf("output contains %q:\n%s", sub, out)
	}
}

func TestSpendLinesLandedExternal(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useLineup("")
	var out string
	synctest.Test(t, func(t *testing.T) {
		f := newLineupExec(t)
		f.build = func(n int, root string, in executor.Input) (executor.RoundResult, error) {
			time.Sleep(205 * time.Second)
			return externalWritesApp("candidate\n", executor.Usage{})(n, context.Background(), root, in)
		}
		f.review = func(n int, root string, in executor.Input, task executor.Task) (executor.RoundResult, error) {
			return executor.RoundResult{Structured: verdictJSON("approve")}, nil
		}
		a := e.build(withLineup(f))
		if a.err != nil || a.exit != 0 {
			t.Fatalf("exit=%d err=%v\n%s\n%s", a.exit, a.err, a.out, a.errs)
		}
		out = a.out.String()
	})
	spendHas(t, out, `^  repairs: 0 · time: 3m25s$`)
	spendHasNot(t, out, "tokens: 0")
	spendHas(t, out, `^  executor: claude-code `)
	spendHas(t, out, `^  review executor: claude-code `)
}

func TestSpendLinesLandedNative(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.prov.then(landScript(e.prov)...)
	a := e.build()
	if a.err != nil || a.exit != 0 {
		t.Fatalf("exit=%d err=%v\n%s\n%s", a.exit, a.err, a.out, a.errs)
	}
	out := a.out.String()
	spendHas(t, out, `^  repairs: 0 · tokens: 2 · time: [0-9]+s$`)
	spendHasNot(t, out, "executor:")
}

func TestSpendLinesRoundLimit(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	f := newFakeExecutor(t, externalWritesApp("red\n", harnessUsage(100, 0, 40, 0.25)))
	f.desc.Capabilities.MaxRounds = 2
	a := e.build(withExecutor(f, nil))
	if a.exit != 1 {
		t.Fatalf("exit=%d\n%s", a.exit, a.shown())
	}
	spendHas(t, a.out.String(), `^  edited: app\.txt\n`+
		`  repairs: \d+ · time: [0-9hms]+\n`+
		`  executor: claude-code \(harness-reported tokens input=200 cached=0 output=80 reasoning=unknown, estimate \$0\.5000\)\n`+
		`  run log: `)
}

func TestSpendLinesReviewBlocked(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useLineup("")
	f := newLineupExec(t)
	f.build = func(n int, root string, in executor.Input) (executor.RoundResult, error) {
		return externalWritesApp("candidate\n", harnessUsage(100, 0, 40, 0.5))(n, context.Background(), root, in)
	}
	f.review = func(n int, root string, in executor.Input, task executor.Task) (executor.RoundResult, error) {
		return executor.RoundResult{Structured: verdictJSON("changes", blockingFinding), Usage: harnessUsage(50, 10, 5, 0.25)}, nil
	}
	a := e.build(withLineup(f))
	if a.exit == 0 || a.err != nil {
		t.Fatalf("exit=%d err=%v\n%s", a.exit, a.err, a.shown())
	}
	out := a.out.String()
	spendHas(t, out, `^  executor: claude-code \(harness-reported tokens input=100 cached=0 output=40 reasoning=unknown, estimate \$0\.5000\)\n`+
		`  review executor: claude-code \(harness-reported tokens input=50 cached=10 output=5 reasoning=unknown, estimate \$0\.2500\)\n`)
	spendHas(t, out, `^candidate/review_blocked: `)
}

func TestSpendLinesNativeStop(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *project.Config) { c.Checks = []project.Check{{Name: "test", Run: "true"}} })
	e.prov.then(refused())
	a := e.build()
	if a.exit != 4 {
		t.Fatalf("exit=%d\n%s", a.exit, a.shown())
	}
	out := a.out.String()
	spendHasNot(t, out, "executor:")
	spendHas(t, out, `^  repairs: [0-9]+ · tokens: `)
}

func TestSpendLinesStartupStop(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.hook("commit-msg", "printf changed >> "+stepsFile+"\n")
	a := e.build()
	if a.exit != 1 {
		t.Fatalf("exit=%d\n%s", a.exit, a.shown())
	}
	out := a.out.String()
	spendHasNot(t, out, "  repairs: ")
	spendHasNot(t, out, "executor:")
	spendHas(t, out, `stopped in startup: baseline_changed`)
}
