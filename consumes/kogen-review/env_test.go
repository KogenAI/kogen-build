package consumes

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-review/review"
	"github.com/KogenAI/kogen-verification/verify"
)

// TestEnvErrorsStopTheReviewUnchanged pins how a review treats what its Env returns: the
// caller's interrupt (Env.Err), a custody failure, a failed checkpoint, a failed model request
// and the other failures of the caller stop the review and are returned as they are, so that
// build reports its own error and not a review's.
func TestEnvErrorsStopTheReviewUnchanged(t *testing.T) {
	t.Parallel()
	boom := errors.New("caller's failure")
	for _, method := range []string{"Prepare", "Save", "Facts", "Request", "Track", "Err", "Custody", "Boundary", "Unread", "BaselineIntact", "Reviewed", "Write"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			env := newEnv(t, f)
			env.failOn = map[string]error{method: boom}
			env.answers = []*provider.Response{callTool("read_file", `{"path":"app.txt"}`), submitReview(`{"verdict":"approve","findings":[]}`)}
			_, err := review.Independent{}.Review(context.Background(), f.input(), env)
			if !errors.Is(err, boom) {
				t.Errorf("review.Env.%s: a review whose caller failed returned %v, want the caller's error", method, err)
			}
		})
	}

	// The same for a verification the caller fails while the review checks a repair.
	f := newFixture(t)
	writeFile(t, filepath.Join(f.root, "app.txt"), otherText)
	env := newEnv(t, f)
	env.failOn = map[string]error{"Verify": boom}
	env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[]}`)}
	_, err := review.Independent{}.Review(context.Background(), f.input(), env)
	if !errors.Is(err, boom) {
		t.Errorf("review.Env.Verify: returned %v, want the caller's error", err)
	}
}

// TestEnvUnreadPartsBlockAnApproval pins what the review asks of Env.Unread: an approval while
// the conversation has not read every pinned part is sent back with the unread parts named, and
// the same rejection again ends the review without a verdict.
func TestEnvUnreadPartsBlockAnApproval(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	env := newEnv(t, f)
	env.unread = []string{"part-2-of-3"}
	env.answers = []*provider.Response{
		submitReview(`{"verdict":"approve","findings":[]}`),
		submitReview(`{"verdict":"approve","findings":[]}`),
	}
	res, err := review.Independent{}.Review(context.Background(), f.input(), env)
	wantError(t, "review.Env.Unread", err, "candidate", "review_invalid")
	out := env.requests[1].Input[len(env.requests[1].Input)-1].(map[string]any)
	if s, _ := out["output"].(string); !strings.Contains(s, "Read all pinned source evidence before approving: 1 unread part(s); use read_file on part-2-of-3") {
		t.Errorf("review.Env.Unread: the rejection sent back: %v", out)
	}
	eq(t, "review.Env.Unread reason", res.Invalid.Reason, "rejected_verdict_exhausted")
	eq(t, "review.Env.Delivered unread", env.delivered, 0)
}

// TestRepairKeepsTheEarlierVerdictsAsAttempts pins that a review sent back to its reviewer after
// a verified repair keeps the earlier submitted verdict in its saved session (Attempts) and
// continues the conversation (Input), and that the checkpoint says it was repaired.
func TestRepairKeepsTheEarlierVerdictsAsAttempts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	writeFile(t, filepath.Join(f.root, "app.txt"), otherText)
	env := newEnv(t, f)
	env.answers = []*provider.Response{
		submitReview(`{"verdict":"approve","findings":[{"path":"app.txt","line":1,"severity":"note","message":"n"}]}`),
		submitReview(`{"verdict":"approve","findings":[]}`),
	}
	env.verifies = []verify.Pass{{N: 2, Clean: true, Green: true, Tree: f.other}}
	if _, err := (review.Independent{}).Review(context.Background(), f.input(), env); err != nil {
		t.Fatal(err)
	}
	var repaired *review.Session
	for _, c := range env.saves {
		s, err := review.DecodeSession(c.Session)
		if err != nil {
			t.Fatal(err)
		}
		if len(s.Attempts) > 0 && repaired == nil {
			repaired = &s
			eq(t, "review.Checkpoint.Pass repaired", c.Pass.N, 2)
		}
	}
	if repaired == nil {
		t.Fatal("review.Session.Attempts: no checkpoint kept the earlier verdict")
	}
	if len(repaired.Attempts) != 1 {
		t.Fatalf("review.Session.Attempts: %+v", repaired.Attempts)
	}
	a := repaired.Attempts[0]
	eq(t, "review.Submitted.Pass attempt", a.Pass, 1)
	eq(t, "review.Submitted.Tree attempt", a.Tree, f.other)
	eq(t, "review.Submitted.Verdict attempt", a.Verdict, "approve")
	eq(t, "review.Submitted.Findings attempt", len(a.Findings), 1)
	eq(t, "review.Session.Role repaired", repaired.Role, reviewRole)
	if len(repaired.Input) < 2 {
		t.Errorf("review.Session.Input of a repaired review: %d items, want the conversation and the delivery section", len(repaired.Input))
	}
	rec := readJSON(t, filepath.Join(f.dir, review.File(1)))
	attempts, _ := rec["attempts"].([]any)
	eq(t, "review.File(1) attempts", len(attempts), 2)
}

// TestNewExternalLimitsTheRoundsOfASession pins the round cap NewExternal is given (build passes
// the executor's own cap or its default): a session that needs another round than the cap allows
// stops with the executor's round-limit error.
func TestNewExternalLimitsTheRoundsOfASession(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	x := &fakeExecutor{t: t, results: []executor.RoundResult{structured(`{"verdict":"maybe","findings":[]}`), structured(`{"verdict":"approve","findings":[]}`)}}
	_, err := review.NewExternal(x, externalDescriptor(), 1).Review(context.Background(), f.input(), newEnv(t, f))
	if err == nil {
		t.Fatal("review.NewExternal: a second round beyond a cap of one did not fail")
	}
	eq(t, "review.NewExternal rounds run", len(x.inputs), 1)
	want := executor.RoundLimit(1)
	eq(t, "review.NewExternal round limit", err.Error(), want.Error())
}

// TestResumeOfAReviewContinuesItsSavedConversation pins what a resume takes from the saved
// session (build/legacy.go writes version 3 sessions in this shape): the saved conversation is
// what the next request sends, and the tool definitions the conversation was given are kept (a
// session without any gets the current reviewer tools).
func TestResumeOfAReviewContinuesItsSavedConversation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	pass := f.greenPass()
	saved := []any{map[string]any{"role": "user", "content": "saved opening"}}
	defs := []any{map[string]any{"name": "read_file"}, map[string]any{"name": "submit_review"}}
	for _, kept := range []bool{true, false} {
		s := review.Session{Role: reviewRole, Input: saved}
		if kept {
			s.Tools = defs
		}
		raw, err := s.Encode()
		if err != nil {
			t.Fatal(err)
		}
		in := f.input()
		in.Checkpoint = &review.Checkpoint{Stage: "review", Pass: &pass, Session: raw}
		env := newEnv(t, f)
		env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[]}`)}
		res, err := review.Independent{}.Resume(context.Background(), in, env)
		if err != nil {
			t.Fatal(err)
		}
		eq(t, "review.Session resumed verdict", res.Record.Verdict, "approve")
		eq(t, "review resume prepare", len(env.prepares), 0)
		rq := env.requests[0]
		deepEq(t, "review.Session.Input resumed", rq.Input, saved)
		eq(t, "review.Session.Role resumed", rq.Role, reviewRole)
		if kept {
			deepEq(t, "review.Session.Tools resumed", rq.Tools, defs)
		} else {
			eq(t, "review.Session.Tools default", len(toolNames(rq.Tools)), 7)
		}
	}
}
