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

type progressElapsedExecutor struct{ *lineupExec }
type progressElapsedSession struct {
	executor.Session
	emit executor.Sink
	role string
}

func (f progressElapsedExecutor) Start(ctx context.Context, task executor.Task, emit executor.Sink) (executor.Session, error) {
	s, err := f.lineupExec.Start(ctx, task, emit)
	return progressElapsedSession{s, emit, task.Role}, err
}

func (s progressElapsedSession) Round(ctx context.Context, in executor.Input) (executor.RoundResult, error) {
	result, err := s.Session.Round(ctx, in)
	if s.role == executor.RoleBuild {
		s.emit(map[string]any{"event": "executor_tool", "name": "Read", "arguments": map[string]any{"file_path": "input.txt"}, "result_error": false})
		s.emit(map[string]any{"event": "executor_tool", "name": "Bash", "result_error": true})
	}
	return result, err
}

func TestBuildPhaseElapsedProgress(t *testing.T) {
	e := newEnv(t, func(c *project.Config) { c.Checks = []project.Check{{Name: "test", Run: "true"}} })
	e.useLineup("")
	e.credentials()
	e.failingSigner()
	result := map[string]string{}
	synctest.Test(t, func(t *testing.T) {
		f := newLineupExec(t)
		f.build = func(n int, root string, in executor.Input) (executor.RoundResult, error) {
			time.Sleep(205900 * time.Millisecond)
			return externalWritesApp("candidate\n", executor.Usage{})(n, context.Background(), root, in)
		}
		f.review = func(n int, root string, in executor.Input, task executor.Task) (executor.RoundResult, error) {
			time.Sleep(17250 * time.Millisecond)
			return executor.RoundResult{Structured: verdictJSON("approve")}, nil
		}
		a := e.build(func(in *Input) {
			in.External = func(project.Role) (executor.Executor, error) { return progressElapsedExecutor{f}, nil }
			in.Reviewer = nil
		})
		if a.err != nil || a.exit != 3 {
			t.Fatalf("TestBuildPhaseElapsedProgress Build: exit=%d err=%v\n%s\n%s", a.exit, a.err, a.out, a.errs)
		}
		result["fresh"] = a.errs.String()
		id := e.onlyRun()
		time.Sleep(100 * time.Second)
		e.fixSigner()
		r := e.resume(id, func(in *ResumeInput) {
			in.Reviewer = nil
			in.External = func(project.Role) (executor.Executor, error) { return progressElapsedExecutor{f}, nil }
		})
		if r.err != nil || r.exit != 0 {
			t.Fatalf("TestBuildPhaseElapsedProgress Resume: exit=%d err=%v\n%s\n%s", r.exit, r.err, r.out, r.errs)
		}
		result["resumed"] = r.errs.String()
		if e.prov.count() != 0 {
			t.Fatal("TestBuildPhaseElapsedProgress unexpectedly called a model")
		}
		if !strings.Contains(r.out.String(), "landed fixture") {
			t.Fatal("TestBuildPhaseElapsedProgress did not land")
		}
	})
	wantFresh := "kogen: checking test before the Build changes anything (0 s)\n" +
		"kogen: building (round 1)\n" +
		"kogen: building (round 1) — Read input.txt ok\n" +
		"kogen: building (round 1) — Bash failed\n" +
		"kogen: verifying (pass 1) (205 s)\n" +
		"kogen: pass 1: all checks passed (205 s)\n" +
		"kogen: reviewing (review 1) (205 s)\n" +
		"kogen: review 1: approve, 0 blocking (223 s)\n" +
		"kogen: committing (223 s)\n"
	if result["fresh"] != wantFresh {
		t.Fatalf("fresh progress:\ngot %q\nwant %q", result["fresh"], wantFresh)
	}
	if want := "kogen: committing (323 s)\n"; result["resumed"] != want {
		t.Fatalf("resumed progress: got %q want %q", result["resumed"], want)
	}
}

// The red pass and a resumed review exercise boundary paths not reached by the
// successful commit-resume fixture above.
func TestPhaseElapsedFailureAndReviewResume(t *testing.T) {
	t.Run("red pass", func(t *testing.T) {
		e := newEnv(t, func(c *project.Config) { c.Checks = []project.Check{{Name: "test", Run: "false"}} })
		e.useLineup("")
		f := newLineupExec(t)
		f.build = func(n int, root string, in executor.Input) (executor.RoundResult, error) {
			if n > 1 {
				return executor.RoundResult{}, executor.Fail(executor.ReasonProtocol, "stop")
			}
			return externalWritesApp("candidate\n", executor.Usage{})(n, context.Background(), root, in)
		}
		a := e.build(withLineup(f))
		if !regexp.MustCompile(`(?m)^kogen: pass 1: test failed \([0-9]+ s\)$`).MatchString(a.errs.String()) {
			t.Fatalf("red pass progress: %s", a.errs.String())
		}
	})
	t.Run("resumed review", func(t *testing.T) {
		e := newEnv(t)
		e.useLineup("")
		f := newLineupExec(t)
		f.reviewMax = intp(2)
		f.review = redReviewer(t)
		wantStop(t, e.build(withLineup(f)), "candidate/executor_round_limit")
		id := e.onlyRun()
		g := newLineupExec(t)
		g.review = func(n int, root string, in executor.Input, task executor.Task) (executor.RoundResult, error) {
			return executor.RoundResult{Structured: verdictJSON("approve")}, nil
		}
		r := e.resume(id, func(in *ResumeInput) {
			in.External = func(project.Role) (executor.Executor, error) { return g, nil }
			in.Reviewer = nil
		})
		if r.exit != 0 {
			t.Fatalf("resume: %s", r.shown())
		}
		if !regexp.MustCompile(`(?m)^kogen: reviewing \(review 2\) \([0-9]+ s\)$`).MatchString(r.errs.String()) {
			t.Fatalf("resume progress: %s", r.errs.String())
		}
	})
	t.Run("blocking review", func(t *testing.T) {
		e := newEnv(t)
		e.useLineup("")
		f := newLineupExec(t)
		f.review = func(n int, root string, in executor.Input, task executor.Task) (executor.RoundResult, error) {
			return executor.RoundResult{Structured: verdictJSON("changes", blockingFinding)}, nil
		}
		a := e.build(withLineup(f))
		wantStop(t, a, "candidate/review_blocked")
		if !regexp.MustCompile(`(?m)^kogen: review 1: changes, 1 blocking \([0-9]+ s\)$`).MatchString(a.errs.String()) {
			t.Fatalf("review progress: %s", a.errs.String())
		}
	})
}
