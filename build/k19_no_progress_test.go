package build

import (
	"context"
	"testing"

	"github.com/KogenAI/kogen-executor/executor"
)

func countEvents(evs []map[string]any, name string) int {
	n := 0
	for _, ev := range evs {
		if ev["event"] == name {
			n++
		}
	}
	return n
}

// An external round that reports a Write call is verified even when it writes the same bytes, so
// the third identical red pass stops the Build.
func TestExternalNoProgressStopsOnThirdIdenticalRed(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	reviews := 0
	f := newFakeExecutor(t, externalWritesApp("red\n", harnessUsage(1, 0, 1, 0)))
	f.reportsWrites = true
	a := e.build(withExecutor(f, &reviews))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stdout", a.out.String(), "candidate/executor_round_limit: the executor used its 3 rounds without a verified, reviewed candidate; the candidate is left in place\n")
	contains(t, "stdout", a.out.String(), "  edited: app.txt\n")
	eq(t, "rounds", len(f.seen()), 3)
	eq(t, "reviews", reviews, 0)
	id := e.onlyRun()
	eq(t, "events", countEvents(e.events(id), "executor_no_progress"), 1)
	eq(t, "pass events", passEvents(t, e, id), 3)
	eq(t, "outcome", e.result(id)["outcome"], any("executor_round_limit"))
}

func TestExternalRoundsWithoutEditsGetANoticeThenStopAsNoProgress(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	reviews := 0
	f := newFakeExecutor(t, externalWritesApp("red\n", harnessUsage(1, 0, 1, 0)))
	a := e.build(withExecutor(f, &reviews))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stdout", a.out.String(), "candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none;")
	seen := f.seen()
	eq(t, "rounds", len(seen), 3)
	eq(t, "round 3 input", seen[2].Feedback, noEditNoticeText)
	eq(t, "reviews", reviews, 0)
	id := e.onlyRun()
	eq(t, "pass events", passEvents(t, e, id), 1)
	eq(t, "outcome", e.result(id)["outcome"], any("no_progress"))
}

func TestAnInvalidExternalBlockerClaimWithoutEditsGetsTheNoteThenTheNotice(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		switch n {
		case 1:
			return externalWritesApp("red\n", executor.Usage{})(n, ctx, root, in)
		case 2:
			return executor.RoundResult{Ending: executor.EndBlocker, Claim: "blocked",
				Blocker: &executor.BlockerClaim{Paths: []string{"app.txt"}, Reason: "r", Evidence: "e"}}, nil
		}
		return executor.RoundResult{Ending: executor.EndNoCall, Claim: "nothing"}, nil
	})
	f.desc.Capabilities.Blocker = true
	a := e.build(withExecutor(f, nil))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	seen := f.seen()
	eq(t, "rounds", len(seen), 3)
	eq(t, "round 3 input", seen[2].Feedback, invalidBlockerMessage+"\n\n"+noEditNoticeText)
	contains(t, "stdout", a.out.String(), "candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none;")
	eq(t, "pass events", passEvents(t, e, e.onlyRun()), 1)
}

func TestExternalNoProgressResetsWhenTheTreeChanges(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	reviews := 0
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		content := "red\n"
		if n%2 == 0 {
			content = "red\nextra\n"
		}
		return externalWritesApp(content, harnessUsage(1, 0, 1, 0))(n, ctx, root, in)
	})
	f.reportsWrites = true
	f.desc.Capabilities.MaxRounds = 4
	a := e.build(withExecutor(f, &reviews))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	eq(t, "rounds", len(f.seen()), 4)
	eq(t, "events", countEvents(e.events(e.onlyRun()), "executor_no_progress"), 0)
	eq(t, "outcome", e.result(e.onlyRun())["outcome"], any("executor_round_limit"))
}
