package consumes

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-review/review"
)

func completeUsage(input, cached, output int) executor.Usage {
	return executor.Usage{Input: core.Count(input), Cached: core.Count(cached), Output: core.Count(output), Reasoning: core.Count(0),
		State: executor.UsageComplete, Source: "reported"}
}

// TestExternalReviewApprovesInItsOwnSession pins the review build runs for an external review
// role (build/executor_external.go: delegate{review.NewExternal(ex, desc, max)}): one fresh
// executor session of the review role with the task build's Input gives it (the candidate's
// checkout, protected and excluded paths, the review model, the approval's hash, the start, the
// run, the home and the round cap), the rounds run through Tools.Round with the caller's guards,
// the review's executor state goes to the caller with the usage added to the earlier state
// (Input.Executor), and the approval of the pass.
func TestExternalReviewApprovesInItsOwnSession(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	in.Candidate.Protected = []string{".kogen/project.yaml"}
	in.Candidate.Excluded = []string{"skip.txt"}
	in.Intent.Approval = []byte("approved bytes")
	in.Intent.Text = "# The approved intent text\n"
	in.Intent.Plan = "\nStage 2 of 3."
	in.Evidence.Diagnostic = []any{map[string]any{"role": "user", "content": "frozen caller diagnostic"}}
	in.Tools.Home = filepath.Join(f.root, "home")
	in.Executor = &core.ExecutorState{Name: "fixture-executor", Version: "0.9.0", SessionID: "earlier-session", Rounds: 2,
		Usage: core.ExecutorUsage{Input: core.Count(5), Cached: core.Count(0), Output: core.Count(1), Reasoning: core.Count(0), State: executor.UsageComplete, Source: "reported"}}
	var rounds []string
	in.Tools.Round = func(ctx context.Context, sess executor.Session, ei executor.Input) (executor.RoundResult, error) {
		rounds = append(rounds, sess.Handle().SessionID+"|"+ei.Feedback)
		return sess.Round(ctx, ei)
	}
	round := structured(`{"verdict":"approve","findings":[]}`)
	round.Usage = completeUsage(3, 1, 2)
	x := &fakeExecutor{t: t, results: []executor.RoundResult{round}}
	env := newEnv(t, f)
	res, err := review.NewExternal(x, externalDescriptor(), 4).Review(context.Background(), in, env)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.NewExternal record verdict", res.Record.Verdict, "approve")
	eq(t, "review.NewExternal record tree", res.Record.Tree, f.green)
	deepEq(t, "review.NewExternal pass", res.Pass, f.greenPass())
	eq(t, "review.NewExternal sessions", x.starts, 1)
	deepEq(t, "review.Tools.Round calls", rounds, []string{"review-session-1|"})

	task := x.tasks[0]
	eq(t, "review.NewExternal task role", task.Role, executor.RoleReview)
	eq(t, "review.Candidate.Root task", task.Root, f.root)
	deepEq(t, "review.Candidate.Protected task", task.Protected, []string{".kogen/project.yaml"})
	deepEq(t, "review.Candidate.Excluded task", task.Excluded, []string{"skip.txt"})
	eq(t, "review.Intent.Role task model", task.Model, reviewRole.Model)
	eq(t, "review.Intent.Role task effort", task.Effort, reviewRole.Effort)
	eq(t, "review.Intent.Approval task", task.Approval, core.HashHex([]byte("approved bytes")))
	eq(t, "review.Candidate.Start task", task.StartTree, f.start)
	eq(t, "review.Run.ID task", task.RunID, "fixture-run")
	eq(t, "review.Tools.Home task", task.Home, in.Tools.Home)
	eq(t, "review.NewExternal max rounds", task.MaxRounds, 4)
	eq(t, "review.Run.Dir task", task.Dir, filepath.Join(f.dir, "executor-review", "session"))
	var schema map[string]any
	if err := json.Unmarshal(task.Schema, &schema); err != nil || schema["type"] != "object" {
		t.Errorf("review.NewExternal task schema: %s", task.Schema)
	}
	for _, want := range []string{"# The approved intent text\n", "frozen caller diagnostic", "Staged plan context for this independent review.\nStage 2 of 3."} {
		if !strings.Contains(task.Brief.Stable, want) {
			t.Errorf("review.NewExternal brief stable lacks %q", want)
		}
	}
	for _, want := range []string{"# Check results of the final pass", "diff --git a/app.txt b/app.txt", "--- app.txt (changed; hunks in diff)"} {
		if !strings.Contains(task.Brief.Dynamic, want) {
			t.Errorf("review.NewExternal brief dynamic lacks %q", want)
		}
	}
	if st, err := os.Stat(filepath.Join(f.dir, "executor-review")); err != nil || st.Mode().Perm() != 0o700 {
		t.Errorf("review.Run.Dir: the review's private evidence folder: %v %v", st, err)
	}

	// The reviewer's executor state, as the caller keeps it: its own session, this round, and the
	// usage added to the earlier state's.
	if len(env.executor) == 0 {
		t.Fatal("review.Input.Executor: the caller was never given the executor state")
	}
	got := env.executor[len(env.executor)-1]
	eq(t, "review executor name", got.Name, "fixture-executor")
	eq(t, "review executor version", got.Version, "1.0.0")
	eq(t, "review executor session", got.SessionID, "review-session-1")
	eq(t, "review executor rounds", got.Rounds, 1)
	eq(t, "review executor usage state", got.Usage.State, executor.UsageComplete)
	eq(t, "review.Input.Executor usage input", *got.Usage.Input, 8)
	eq(t, "review.Input.Executor usage cached", *got.Usage.Cached, 1)
	eq(t, "review.Input.Executor usage output", *got.Usage.Output, 3)
	deepEq(t, "review external stages", env.stages(), []string{"review", "review_verify"})
	eq(t, "review external round events", len(env.eventsNamed("executor_round")), 1)
}

// TestExternalReviewWithoutAVerdictIsHandedToTheBuild pins the external verdict rules build's
// retry relies on: an invalid structured verdict goes back once as text (the next round's
// Input.Feedback), a second ends the review without a verdict and a round that ended without a
// call does at once; the Result carries the Invalid and the error is candidate/review_invalid.
func TestExternalReviewWithoutAVerdictIsHandedToTheBuild(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	var feedbacks []string
	in.Tools.Round = func(ctx context.Context, sess executor.Session, ei executor.Input) (executor.RoundResult, error) {
		feedbacks = append(feedbacks, ei.Feedback)
		return sess.Round(ctx, ei)
	}
	x := &fakeExecutor{t: t, results: []executor.RoundResult{structured(`{"verdict":"maybe","findings":[]}`), structured(`{"verdict":"maybe","findings":[]}`)}}
	res, err := review.NewExternal(x, externalDescriptor(), 4).Review(context.Background(), in, newEnv(t, f))
	wantError(t, "review.NewExternal invalid", err, "candidate", "review_invalid")
	deepEq(t, "review.Tools.Round feedback", feedbacks, []string{"",
		"Not a valid verdict: submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was a string with an unsupported value. Submit a corrected structured verdict."})
	inv := res.Invalid
	if inv == nil {
		t.Fatal("review.Result.Invalid external: missing")
	}
	eq(t, "review.Invalid.Reason external", inv.Reason, "rejected_verdict_exhausted")
	eq(t, "review.Invalid.RejectedVerdict external", inv.RejectedVerdict, 1)
	eq(t, "review.Invalid.Review external", inv.Review, 1)
	deepEq(t, "review.Invalid.Pass external", inv.Pass, f.greenPass())
	if inv.Base == nil || inv.Base.Tree != f.green {
		t.Errorf("review.Invalid.Base external: %+v, want the green pass", inv.Base)
	}

	x = &fakeExecutor{t: t, results: []executor.RoundResult{{Ending: executor.EndNoCall}}}
	res, err = review.NewExternal(x, externalDescriptor(), 4).Review(context.Background(), f.input(), newEnv(t, f))
	wantError(t, "review.NewExternal no call", err, "candidate", "review_invalid")
	if res.Invalid == nil {
		t.Fatal("review.Result.Invalid external no call: missing")
	}
	eq(t, "review.Invalid.Reason no call", res.Invalid.Reason, "executor_no_call")
}

// TestExternalReviewIsNeverTheBuildersSession pins review.Candidate.Builder: a reviewer session
// that is the implementer's executor session is refused, a distinct one reviews.
func TestExternalReviewIsNeverTheBuildersSession(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	in.Candidate.Builder = "review-session-1"
	x := &fakeExecutor{t: t, results: []executor.RoundResult{structured(`{"verdict":"approve","findings":[]}`)}}
	_, err := review.NewExternal(x, externalDescriptor(), 4).Review(context.Background(), in, newEnv(t, f))
	if err == nil || !strings.Contains(err.Error(), "the reviewer session is not distinct from the builder's") {
		t.Errorf("review.Candidate.Builder: error %v, want the refusal of the builder's session", err)
	}
	eq(t, "review.Candidate.Builder rounds", len(x.inputs), 0)

	in.Candidate.Builder = "another-session"
	x = &fakeExecutor{t: t, results: []executor.RoundResult{structured(`{"verdict":"approve","findings":[]}`)}}
	res, err := review.NewExternal(x, externalDescriptor(), 4).Review(context.Background(), in, newEnv(t, f))
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.Candidate.Builder distinct", res.Record.Verdict, "approve")
}

// TestExternalReviewChangesIsTheReviewBlockedStop pins that an external reviewer's changes verdict
// ends the Build as the native one does: candidate/review_blocked with the blocking findings.
func TestExternalReviewChangesIsTheReviewBlockedStop(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	x := &fakeExecutor{t: t, results: []executor.RoundResult{structured(`{"verdict":"changes","findings":[{"path":"app.txt","line":2,"severity":"blocking","message":"wrong"}]}`)}}
	env := newEnv(t, f)
	res, err := review.NewExternal(x, externalDescriptor(), 4).Review(context.Background(), f.input(), env)
	e := wantError(t, "review.NewExternal blocked", err, "candidate", "review_blocked")
	deepEq(t, "review.NewExternal blocked lines", e.Lines, []string{"review: app.txt:2: wrong"})
	eq(t, "review.NewExternal blocked verdict", res.Record.Verdict, "changes")
	eq(t, "review.NewExternal blocked last review", env.reviewed[len(env.reviewed)-1].Verdict, "changes")
}
