package consumes

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-review/review"
	"github.com/KogenAI/kogen-verification/verify"
	"kogen-loop/provider"
	"kogen-loop/tools"
)

// lastContent is the content of the last message of a conversation.
func lastContent(input []any) string {
	if len(input) == 0 {
		return ""
	}
	m, _ := input[len(input)-1].(map[string]any)
	s, _ := m["content"].(string)
	return s
}

// toolNames are the names of the tool definitions of a request.
func toolNames(defs []any) []string {
	var names []string
	for _, d := range defs {
		if m, ok := d.(map[string]any); ok {
			names = append(names, m["name"].(string))
		}
	}
	return names
}

// TestIndependentReviewApprovesTheGreenPass pins the native review build runs by default
// (build/host.go nativeReviewer, review.Independent{} through its delegate): the Result's record
// and pass, the one request of the review model with its fields, the checkpoints and records
// the review asks of its Env, and the review record file.
func TestIndependentReviewApprovesTheGreenPass(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	env := newEnv(t, f)
	env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[{"path":"app.txt","line":1,"severity":"note","message":"fine"}]}`)}
	var reviewer review.Reviewer = review.Independent{}
	res, err := reviewer.Review(context.Background(), in, env)
	if err != nil {
		t.Fatal(err)
	}

	var _ core.ReviewRecord = res.Record
	var _ verify.Pass = res.Pass
	var _ *review.Invalid = res.Invalid
	eq(t, "review.Result.Record verdict", res.Record.Verdict, "approve")
	eq(t, "review.Result.Record n", res.Record.N, 1)
	eq(t, "review.Result.Record tree", res.Record.Tree, f.green)
	findings, _ := json.Marshal(res.Record.Findings)
	eq(t, "review.Result.Record findings", string(findings), `[{"line":1,"message":"fine","path":"app.txt","severity":"note"}]`)
	deepEq(t, "review.Result.Pass", res.Pass, f.greenPass())
	eq(t, "review.Result.Invalid", res.Invalid == nil, true)

	// The one request of the review model.
	eq(t, "review.Request count", len(env.requests), 1)
	rq := env.requests[0]
	var _ project.Role = rq.Role
	var _ string = rq.Instructions
	var _ []any = rq.Input
	var _ []any = rq.Tools
	var _ func([]any) (json.RawMessage, error) = rq.Retain
	var _ func([]any) ([]any, error) = rq.Prefill
	var _ string = rq.ForceTool
	eq(t, "review.Request.Role", rq.Role, reviewRole)
	pre := env.prepares[0]
	if rq.Instructions == "" || rq.Instructions != pre.instructions {
		t.Errorf("review.Request.Instructions: %q, want the instructions the conversation was prepared with: %q", rq.Instructions, pre.instructions)
	}
	deepEq(t, "review.Request.Input", rq.Input, []any{map[string]any{"role": "user", "content": pre.material.Authority + "\n" + pre.material.Source}})
	deepEq(t, "review.Request.Tools", rq.Tools, pre.defs)
	deepEq(t, "review.Request.Tools names", toolNames(rq.Tools),
		[]string{"read_file", "view_image", "list_files", "search_text", "write_file", "shell", "submit_review"})
	eq(t, "review.Request.ForceTool", rq.ForceTool, "")

	// Retain is the session a checkpoint keeps with the input actually sent.
	raw, err := rq.Retain([]any{"sent"})
	if err != nil {
		t.Fatal(err)
	}
	sess, err := review.DecodeSession(raw)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.Request.Retain role", sess.Role, reviewRole)
	deepEq(t, "review.Request.Retain input", sess.Input, []any{"sent"})
	// Prefill appends the pinned source through the caller (Env.Prefill).
	out, err := rq.Prefill([]any{"in"})
	if err != nil {
		t.Fatal(err)
	}
	deepEq(t, "review.Request.Prefill result", out, []any{"in", map[string]any{"role": "user", "content": "prefilled"}})
	deepEq(t, "review.Request.Prefill env call", env.prefills, [][]any{{reviewRole, rq.Instructions, pre.defs, []any{"in"}}})

	// What the review asks of its caller, in the stages build records.
	deepEq(t, "review checkpoint stages", env.stages(), []string{"review", "tool", "review_verify"})
	deepEq(t, "review Env.Facts", env.facts, []string{"reviewing"})
	deepEq(t, "review Env.Progress", env.progress, []string{"kogen: reviewing (review 1)", "kogen: review 1: approve, 0 blocking"})
	eq(t, "review Env.Delivered", env.delivered, 1)
	eq(t, "review Env.Prepare tree", pre.tree, f.green)
	eq(t, "review Env.Prepare role", pre.role, reviewRole)
	deepEq(t, "review Env.Reviewed", env.reviewed, []core.ReviewRecord{res.Record, res.Record})
	if env.tracks == 0 || env.called("Err") == 0 || env.called("Custody") == 0 || env.called("BaselineIntact") == 0 || env.called("Unread") == 0 {
		t.Errorf("review Env: Track, Err, Custody, BaselineIntact and Unread are asked of the caller: %v", env.calls)
	}
	events := env.eventsNamed("review")
	eq(t, "review event count", len(events), 1)
	eq(t, "review event verdict", events[0]["verdict"], any("approve"))
	eq(t, "review event pass", events[0]["pass"], any(1))
	eq(t, "review event tree", events[0]["tree"], any(f.green))

	// The review record file of the run folder.
	rec := readJSON(t, filepath.Join(f.dir, review.File(1)))
	for _, key := range []string{"model", "effort", "usage", "verdict", "findings", "tree", "attempts"} {
		if _, ok := rec[key]; !ok {
			t.Errorf("review.File(1): the record has no %q: %v", key, rec)
		}
	}
	eq(t, "review.File(1) model", rec["model"], any(reviewRole.Model))
	eq(t, "review.File(1) effort", rec["effort"], any(reviewRole.Effort))
	eq(t, "review.File(1) verdict", rec["verdict"], any("approve"))
	eq(t, "review.File(1) tree", rec["tree"], any(f.green))
}

// TestInputFieldsReachTheReview pins what each field of review.Input does for a native review:
// Number is the review's number (its progress line, record and file), Run, Evidence, Intent and
// Candidate are read, and Last, Executor and Checkpoint are pinned by their types (they act
// in the resume and external tests).
func TestInputFieldsReachTheReview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	in.Number = 3
	in.Run = review.Run{ID: "run-3", Dir: f.dir}
	var _ review.Candidate = in.Candidate
	var _ review.Intent = in.Intent
	var _ review.Evidence = in.Evidence
	var _ review.Tools = in.Tools
	var _ *review.Checkpoint = in.Checkpoint
	var _ int = in.Number
	var _ *core.ReviewRecord = in.Last
	var _ *core.ExecutorState = in.Executor
	var _ review.Run = in.Run
	var _ string = in.Run.ID
	var _ string = in.Run.Dir
	eq(t, "review.Run.ID", in.Run.ID, "run-3")
	eq(t, "review.Run.Dir", in.Run.Dir, f.dir)

	env := newEnv(t, f)
	env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[]}`)}
	res, err := review.Independent{}.Review(context.Background(), in, env)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.Input.Number record", res.Record.N, 3)
	deepEq(t, "review.Input.Number progress", env.progress, []string{"kogen: reviewing (review 3)", "kogen: review 3: approve, 0 blocking"})
	eq(t, "review.Input.Number event", env.eventsNamed("review")[0]["review"], any(3))
	rec := readJSON(t, filepath.Join(f.dir, review.File(3)))
	eq(t, "review.Input.Number file", rec["verdict"], any("approve"))
	eq(t, "review.Input.Evidence pass", res.Pass.N, in.Evidence.Pass.N)
}

// TestIndependentReviewChangesIsTheReviewBlockedStop pins that a changes verdict ends the Build
// with candidate/review_blocked naming each blocking finding (build's error object and exit
// code), returns the verdict's record and leaves it in a review_verify checkpoint.
func TestIndependentReviewChangesIsTheReviewBlockedStop(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	env := newEnv(t, f)
	env.answers = []*provider.Response{submitReview(`{"verdict":"changes","findings":[{"path":"app.txt","line":2,"severity":"blocking","message":"wrong"},{"path":"app.txt","line":3,"severity":"note","message":"minor"}]}`)}
	res, err := review.Independent{}.Review(context.Background(), f.input(), env)
	e := wantError(t, "review.Result.Record blocked", err, "candidate", "review_blocked")
	eq(t, "review blocked exit", e.Exit, 1)
	eq(t, "review blocked detail", e.Detail, "the reviewer could not resolve a substantive blocker within the approved Intent; edits are left in place")
	deepEq(t, "review blocked lines", e.Lines, []string{"review: app.txt:2: wrong"})
	eq(t, "review.Result.Record verdict", res.Record.Verdict, "changes")
	eq(t, "review.Result.Record findings", len(res.Record.Findings), 2)
	eq(t, "review.Result.Record tree", res.Record.Tree, f.green)
	deepEq(t, "review.Result.Pass blocked", res.Pass, f.greenPass())
	eq(t, "review blocked last review", env.reviewed[len(env.reviewed)-1].Verdict, "changes")

	last := env.saves[len(env.saves)-1]
	var _ string = last.Stage
	var _ json.RawMessage = last.Session
	eq(t, "review.Checkpoint.Stage blocked", last.Stage, "review_verify")
	s, err := review.DecodeSession(last.Session)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.Session.Verdict blocked", s.Verdict.Verdict, "changes")
	eq(t, "review.Session.Tree blocked", s.Tree, f.green)
	eq(t, "review.Session.Attempts blocked", len(s.Attempts), 0)
}

// TestIndependentReviewWithoutAVerdictIsHandedToTheBuild pins the end of a review without a
// valid verdict, which build/review_retry.go handles: a prose-only turn is reminded once (the
// next request forces submit_review where the route supports it), the second ends the review,
// the Result carries its Invalid and the error is candidate/review_invalid.
func TestIndependentReviewWithoutAVerdictIsHandedToTheBuild(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	env := newEnv(t, f)
	env.answers = []*provider.Response{prose("looks fine"), prose("still fine")}
	res, err := review.Independent{}.Review(context.Background(), f.input(), env)
	e := wantError(t, "review.Result.Invalid", err, "candidate", "review_invalid")
	eq(t, "review invalid exit", e.Exit, 1)
	eq(t, "review invalid detail", e.Detail, "the reviewer did not return a valid verdict; edits are left in place")
	eq(t, "review invalid requests", len(env.requests), 2)
	eq(t, "review.Request.ForceTool reminder", env.requests[1].ForceTool, "submit_review")

	inv := res.Invalid
	if inv == nil {
		t.Fatal("review.Result.Invalid: missing")
	}
	var _ int = inv.Review
	var _ string = inv.ForcedRoute
	var _ string = inv.ForcedModel
	var _ string = inv.Reason
	var _ int = inv.RejectedVerdict
	var _ int = inv.NoToolReminder
	var _ verify.Pass = inv.Pass
	var _ *verify.Pass = inv.Base
	var _ bool = inv.Reopened
	var _ bool = inv.Repaired
	eq(t, "review.Invalid.Review", inv.Review, 1)
	eq(t, "review.Invalid.Reason", inv.Reason, "no_tool_reminder_exhausted")
	eq(t, "review.Invalid.NoToolReminder", inv.NoToolReminder, 1)
	eq(t, "review.Invalid.RejectedVerdict", inv.RejectedVerdict, 0)
	deepEq(t, "review.Invalid.Pass", inv.Pass, f.greenPass())
	if inv.Base == nil {
		t.Fatal("review.Invalid.Base: nil for a review of a green pass")
	}
	deepEq(t, "review.Invalid.Base", *inv.Base, f.greenPass())
	eq(t, "review.Invalid.Reopened", inv.Reopened, false)
	eq(t, "review.Invalid.Repaired", inv.Repaired, false)
	// The reminder request was forced, and the provider ignored it: the hand-off names the route.
	eq(t, "review.Invalid.ForcedRoute", inv.ForcedRoute, "chatgpt")
	eq(t, "review.Invalid.ForcedModel", inv.ForcedModel, reviewRole.Model)
	deepEq(t, "review.InvalidRecord of the invalid review", review.InvalidRecord(inv.Review),
		core.ReviewRecord{N: 1, Verdict: "invalid", Findings: []map[string]any{}})
}

// TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict pins the other allowance of a
// review: a rejected submit_review is sent back once, the second ends the review without a
// verdict and counts the rejection (no forced route: the provider was not ignoring a forced call).
func TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	env := newEnv(t, f)
	bad := `{"verdict":"changes","findings":[]}`
	env.answers = []*provider.Response{submitReview(bad), submitReview(bad)}
	res, err := review.Independent{}.Review(context.Background(), f.input(), env)
	wantError(t, "review.Invalid rejected", err, "candidate", "review_invalid")
	eq(t, "review rejected requests", len(env.requests), 2)
	inv := res.Invalid
	if inv == nil {
		t.Fatal("review.Result.Invalid: missing")
	}
	eq(t, "review.Invalid.Reason rejected", inv.Reason, "rejected_verdict_exhausted")
	eq(t, "review.Invalid.RejectedVerdict rejected", inv.RejectedVerdict, 1)
	eq(t, "review.Invalid.NoToolReminder rejected", inv.NoToolReminder, 0)
	eq(t, "review.Invalid.ForcedRoute rejected", inv.ForcedRoute, "")
	eq(t, "review.Invalid.ForcedModel rejected", inv.ForcedModel, "")
	// The rejection went back to the reviewer as the tool's result.
	out := env.requests[1].Input[len(env.requests[1].Input)-1].(map[string]any)
	if s, _ := out["output"].(string); !strings.Contains(s, "a changes verdict needs at least one blocking finding") {
		t.Errorf("review rejected: the second request's last item is %v", out)
	}
}

// TestIndependentResumeJudgesTheSavedVerdict pins Resume of a review_verify checkpoint, which
// build/host.go delegate.Resume builds: the saved approval is judged again on the current tree
// without a request, the checkpoint's pass, base and session are used and re-saved, and a
// checkpoint whose review state cannot be read is refused.
func TestIndependentResumeJudgesTheSavedVerdict(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	pass, base := f.greenPass(), f.greenPass()
	base.N = 7
	raw, err := review.Session{Role: reviewRole, Usage: core.ZeroTokens(), Tree: f.green, Verdict: &review.Verdict{Verdict: "approve"}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	in := f.input()
	in.Checkpoint = &review.Checkpoint{Stage: "review_verify", Pass: &pass, Base: &base, Session: raw}
	cp := in.Checkpoint
	var _ string = cp.Stage
	var _ *verify.Pass = cp.Pass
	var _ *verify.Pass = cp.Base
	var _ json.RawMessage = cp.Session
	env := newEnv(t, f)
	res, err := review.Independent{}.Resume(context.Background(), in, env)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review resume requests", len(env.requests), 0)
	eq(t, "review resume verdict", res.Record.Verdict, "approve")
	eq(t, "review resume number", res.Record.N, 1)
	deepEq(t, "review resume pass", res.Pass, pass)
	deepEq(t, "review resume stages", env.stages(), []string{"review_verify"})
	saved := env.saves[0]
	deepEq(t, "review.Checkpoint.Pass resumed", saved.Pass, &pass)
	deepEq(t, "review.Checkpoint.Base resumed", saved.Base, &base)

	// A checkpoint without a recorded base: a session that has not repaired works from the pass
	// it was given, a repaired one has none.
	for _, repaired := range []bool{false, true} {
		raw, err := review.Session{Role: reviewRole, Usage: core.ZeroTokens(), Tree: f.green, Repaired: repaired, Verdict: &review.Verdict{Verdict: "approve"}}.Encode()
		if err != nil {
			t.Fatal(err)
		}
		in := f.input()
		in.Checkpoint = &review.Checkpoint{Stage: "review_verify", Pass: &pass, Session: raw}
		env := newEnv(t, f)
		if _, err := (review.Independent{}).Resume(context.Background(), in, env); err != nil {
			t.Fatal(err)
		}
		got := env.saves[0].Base
		switch {
		case repaired && got != nil:
			t.Errorf("review.Checkpoint.Base of a repaired session without a base: %+v, want nil", got)
		case !repaired && (got == nil || got.Tree != f.green):
			t.Errorf("review.Checkpoint.Base of an unrepaired session without a base: %+v, want the pass", got)
		}
	}

	// A checkpoint that cannot be resumed is refused: no pass, an unreadable session, or the
	// stage of a verdict without one.
	noVerdict, err := review.Session{Role: reviewRole}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]*review.Checkpoint{
		"no pass":       {Stage: "review", Session: raw},
		"unreadable":    {Stage: "review", Pass: &pass, Session: []byte(`{"attempts":"no"}`)},
		"no verdict":    {Stage: "review_verify", Pass: &pass, Session: noVerdict},
		"no checkpoint": nil,
	} {
		in := f.input()
		in.Checkpoint = c
		_, err := review.Independent{}.Resume(context.Background(), in, newEnv(t, f))
		wantError(t, "review.Input.Checkpoint "+name, err, "candidate", "resume_refused")
	}
}

// TestIndependentResumeOfABlockedReviewReopensIt pins that an explicit resume of a review that
// ended with changes does not replay the verdict: the caller's last review (Input.Last) becomes
// pending, a retry checkpoint is saved, the candidate is verified afresh and the original finding
// and the actual results go back to the same reviewer; a review that then ends without a verdict
// is invalid and reopened (carrying the repaired flag of its session and its base).
func TestIndependentResumeOfABlockedReviewReopensIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	pass, base := f.greenPass(), f.greenPass()
	base.N = 4
	finding := map[string]any{"path": "app.txt", "line": json.Number("2"), "severity": "blocking", "message": "wrong"}
	raw, err := review.Session{Role: reviewRole, Usage: core.ZeroTokens(), Tree: f.green, Repaired: true,
		Input:   []any{map[string]any{"role": "user", "content": "opening"}},
		Verdict: &review.Verdict{Verdict: "changes", Findings: []map[string]any{finding}}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	in := f.input()
	in.Number = 2
	in.Checkpoint = &review.Checkpoint{Stage: "review_verify", Pass: &pass, Base: &base, Session: raw}
	last := core.ReviewRecord{N: 2, Verdict: "changes", Findings: []map[string]any{finding}, Tree: "last-tree"}
	in.Last = &last
	in.Evidence.BaselineFailing = true
	var _ bool = in.Evidence.BaselineFailing
	env := newEnv(t, f)
	env.verifies = []verify.Pass{pass}
	env.answers = []*provider.Response{prose("a"), prose("b")}
	res, err := review.Independent{}.Resume(context.Background(), in, env)
	wantError(t, "review reopened", err, "candidate", "review_invalid")

	want := last
	want.Verdict = "pending"
	deepEq(t, "review.Input.Last reopened", env.reviewed[0], want)
	eq(t, "review reopened last unchanged", last.Verdict, "changes")
	eq(t, "review reopened verifies", env.called("Verify"), 1)
	eq(t, "review.Evidence.BaselineFailing comparison", env.called("Comparison"), 1)
	if got := env.stages(); len(got) == 0 || got[0] != "review_retry_verify" {
		t.Errorf("review reopened stages: %v, want a review_retry_verify checkpoint first", got)
	}
	first := lastContent(env.requests[0].Input)
	for _, want := range []string{"The caller explicitly resumed this blocked Build", `"message":"wrong"`, "comparison line 1", "comparison line 2"} {
		if !strings.Contains(first, want) {
			t.Errorf("review reopened: the first request's last message lacks %q: %s", want, first)
		}
	}
	inv := res.Invalid
	if inv == nil {
		t.Fatal("review reopened: no Invalid")
	}
	eq(t, "review.Invalid.Reopened", inv.Reopened, true)
	eq(t, "review.Invalid.Repaired", inv.Repaired, true)
	eq(t, "review.Invalid.Review reopened", inv.Review, 2)
	if inv.Base == nil {
		t.Fatal("review.Invalid.Base reopened: nil")
	}
	deepEq(t, "review.Invalid.Base reopened", *inv.Base, base)
	deepEq(t, "review.Invalid.Pass reopened", inv.Pass, pass)

	// Without the caller's last review the pending record is made from the saved verdict, and
	// without BaselineFailing the comparison is not asked for. The retry stage resumes the same.
	in = f.input()
	in.Number = 2
	in.Checkpoint = &review.Checkpoint{Stage: "review_retry_verify", Pass: &pass, Session: raw}
	env = newEnv(t, f)
	env.verifies = []verify.Pass{pass}
	env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[]}`)}
	res, err = review.Independent{}.Resume(context.Background(), in, env)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review retry comparison", env.called("Comparison"), 0)
	eq(t, "review retry verdict", res.Record.Verdict, "approve")
	deepEq(t, "review retry pass", res.Pass, pass)
	eq(t, "review retry verified", env.called("Verify"), 1)
}

// TestIndependentApprovalOfAnUnverifiedTreeIsVerifiedFirst pins the exact-tree rule through the
// Env: an approval of a tree that is not the green pass's is verified by the caller (Env.Verify),
// the changed contents go back to the same reviewer with their diff, a red verification returns
// its Env.Feedback (named with the tool to call), and only an approval of a verified tree binds.
func TestIndependentApprovalOfAnUnverifiedTreeIsVerifiedFirst(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	writeFile(t, filepath.Join(f.root, "app.txt"), otherText) // the candidate changed after the green pass
	env := newEnv(t, f)
	env.answers = []*provider.Response{
		submitReview(`{"verdict":"approve","findings":[]}`),
		submitReview(`{"verdict":"approve","findings":[]}`),
	}
	env.verifies = []verify.Pass{
		{N: 2, Clean: true, Green: false, Tree: f.other},
		{N: 3, Clean: true, Green: true, Tree: f.other},
	}
	res, err := review.Independent{}.Review(context.Background(), f.input(), env)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review Env.Verify count", env.called("Verify"), 2)
	eq(t, "review Env.Repaired count", env.repairs, 1)
	eq(t, "review.Result.Pass verified", res.Pass.N, 3)
	eq(t, "review.Result.Pass verified tree", res.Pass.Tree, f.other)
	eq(t, "review.Result.Record verified tree", res.Record.Tree, f.other)
	eq(t, "review.Result.Record verified verdict", res.Record.Verdict, "approve")
	eq(t, "review requests", len(env.requests), 2)
	// The approval waits as pending while the caller verifies.
	var verdicts []string
	for _, r := range env.reviewed {
		verdicts = append(verdicts, r.Verdict)
	}
	deepEq(t, "review Env.Reviewed verdicts", verdicts, []string{"approve", "pending", "approve", "pending", "approve"})

	delivery := lastContent(env.requests[1].Input)
	if !strings.HasPrefix(delivery, "Your verdict was not applied: the candidate changed after its contents were delivered to you, other than through your own Kogen tool calls: app.txt.\n") ||
		!strings.Contains(delivery, "diff --git a/app.txt b/app.txt") || !strings.Contains(delivery, "-candidate\n+"+otherText) {
		t.Errorf("review delivery section: %q", delivery)
	}
	eq(t, "review Env.Feedback", env.feedbacks[0], "fixture feedback of pass 2; call submit_review")
	if !strings.Contains(delivery, "Final verification of your repairs failed") || !strings.Contains(delivery, env.feedbacks[0]) {
		t.Errorf("review Env.Feedback: the failed verification's feedback did not reach the reviewer: %q", delivery)
	}
	if !slices.Contains(env.progress, "kogen: review 1: the candidate changed after its contents were delivered; returning the changes to the reviewer") {
		t.Errorf("review Env.Progress: %q", env.progress)
	}
}

// TestCandidateExcludedPathsAreOutsideTheCandidateTree pins that the paths of Candidate.Excluded
// do not belong to the candidate the review judges: editing one after the green pass leaves the
// candidate tree the green pass's, so no verification is asked for, while the same edit of a path
// that is not excluded is verified first.
func TestCandidateExcludedPathsAreOutsideTheCandidateTree(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	in.Candidate.Excluded = []string{"skip.txt"}
	var _ []string = in.Candidate.Excluded
	writeFile(t, filepath.Join(f.root, "skip.txt"), "edited by another intent\n")
	env := newEnv(t, f)
	env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[]}`)}
	res, err := review.Independent{}.Review(context.Background(), in, env)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.Candidate.Excluded verify", env.called("Verify"), 0)
	eq(t, "review.Candidate.Excluded tree", res.Record.Tree, f.green)

	moved, err := f.repo.Tree(f.start, nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved == f.green {
		t.Fatal("fixture: the edit of skip.txt did not change the tree")
	}
	in = f.input()
	env = newEnv(t, f)
	env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[]}`), submitReview(`{"verdict":"approve","findings":[]}`)}
	env.verifies = []verify.Pass{{N: 2, Clean: true, Green: true, Tree: moved}}
	res, err = review.Independent{}.Review(context.Background(), in, env)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review without Excluded verify", env.called("Verify"), 1)
	eq(t, "review without Excluded tree", res.Record.Tree, moved)
}

// TestToolsOfAReviewAreTheCallersNativeTools pins review.Tools for a native review: Native is
// asked for exactly the reviewer's six tools and its tools run the reviewer's calls in the
// checkout, Record's fields join the review_tool_call event, and the caller's Boundary gets the
// call that ran and its result (review.Done) once, and the conversation before it.
func TestToolsOfAReviewAreTheCallersNativeTools(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	var enabled [][]string
	var recorded []string
	in.Tools.Native = func(e []string) *tools.Tools {
		enabled = append(enabled, e)
		return &tools.Tools{Root: f.root, Enabled: e}
	}
	in.Tools.Record = func(call tools.Call, result tools.Output) map[string]any {
		recorded = append(recorded, call.Name)
		return map[string]any{"part": "p-1"}
	}
	in.Tools.Home = filepath.Join(f.root, "home")
	var _ func([]string) *tools.Tools = in.Tools.Native
	var _ func(tools.Call, tools.Output) map[string]any = in.Tools.Record
	var _ func(context.Context, executor.Session, executor.Input) (executor.RoundResult, error) = in.Tools.Round
	var _ string = in.Tools.Home

	env := newEnv(t, f)
	env.answers = []*provider.Response{callTool("read_file", `{"path":"app.txt"}`), submitReview(`{"verdict":"approve","findings":[]}`)}
	if _, err := (review.Independent{}).Review(context.Background(), in, env); err != nil {
		t.Fatal(err)
	}
	if len(enabled) == 0 {
		t.Fatal("review.Tools.Native: never asked for")
	}
	deepEq(t, "review.Tools.Native enabled", enabled[0], []string{"read_file", "view_image", "list_files", "search_text", "write_file", "shell"})
	deepEq(t, "review.Tools.Record calls", recorded, []string{"read_file"})
	events := env.eventsNamed("review_tool_call")
	eq(t, "review_tool_call events", len(events), 1)
	eq(t, "review_tool_call name", events[0]["name"], any("read_file"))
	eq(t, "review.Tools.Record fields", events[0]["part"], any("p-1"))

	// The tool ran in the checkout: its result reached the reviewer's next request.
	second := env.requests[1].Input
	out := second[len(second)-1].(map[string]any)
	if s, _ := out["output"].(string); !strings.Contains(s, "candidate") {
		t.Errorf("review.Tools.Native: the read_file result did not reach the reviewer: %v", out)
	}

	// The Boundary of the tool call that ran carries the call and its result.
	var done *review.Done
	for _, b := range env.boundary {
		if b.done != nil {
			done = b.done
		}
	}
	if done == nil {
		t.Fatal("review.Done: no Boundary carried a call that ran")
	}
	var _ map[string]any = done.Call
	var _ any = done.Result
	deepEq(t, "review.Done.Call", done.Call, map[string]any{"type": "function_call", "call_id": "call-read_file", "name": "read_file", "arguments": `{"path":"app.txt"}`})
	if s, _ := done.Result.(string); !strings.Contains(s, "candidate") {
		t.Errorf("review.Done.Result: %#v, want the read_file result", done.Result)
	}
}

// TestIntentFieldsMakeTheReviewersAuthority pins what each field of review.Intent does: Text and
// Slug (its acceptance feature is read from the checkout) are the authority the reviewer reads,
// Source is stated in it and its custody is validated before each tool call, Role is the review
// model of its requests, and Plan is told to the reviewer as the caller's staged-plan message;
// Approval acts for an external review (see the external tests).
func TestIntentFieldsMakeTheReviewersAuthority(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	in.Intent = review.Intent{
		Slug: "fixture", Text: "# The approved intent text\n", Approval: []byte("approved bytes"),
		Source: nil, Role: project.Role{Provider: "chatgpt", Model: "other-model", Effort: "high"}, Plan: "\nStage 2 of 3.",
	}
	var _ string = in.Intent.Slug
	var _ string = in.Intent.Text
	var _ []byte = in.Intent.Approval
	var _ *project.SourceRef = in.Intent.Source
	var _ project.Role = in.Intent.Role
	var _ string = in.Intent.Plan
	in.Evidence.Diagnostic = []any{map[string]any{"role": "user", "content": "frozen caller diagnostic"}}
	var _ []any = in.Evidence.Diagnostic

	env := newEnv(t, f)
	env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[]}`)}
	if _, err := (review.Independent{}).Review(context.Background(), in, env); err != nil {
		t.Fatal(err)
	}
	pre := env.prepares[0]
	eq(t, "review.Intent.Role prepare", pre.role, in.Intent.Role)
	eq(t, "review.Intent.Role request", env.requests[0].Role, in.Intent.Role)
	rec := readJSON(t, filepath.Join(f.dir, review.File(1)))
	eq(t, "review.Intent.Role record", rec["model"], any("other-model"))
	eq(t, "review.Intent.Role record effort", rec["effort"], any("high"))
	for _, want := range []string{"# Intent\n\n# The approved intent text\n", "# acceptance.feature\n\n" + feature, "Source provenance: unavailable (legacy)"} {
		if !strings.Contains(pre.material.Authority, want) {
			t.Errorf("review.Intent authority lacks %q", want)
		}
	}
	// The caller's frozen diagnostic, then the staged plan's position as its own message.
	deepEq(t, "review.Evidence.Diagnostic and review.Intent.Plan", pre.diagnostic, []any{
		map[string]any{"role": "user", "content": "frozen caller diagnostic"},
		map[string]any{"role": "user", "content": "Staged plan context for this independent review.\nStage 2 of 3."},
	})
	deepEq(t, "review.Evidence.Diagnostic untouched", in.Evidence.Diagnostic, []any{map[string]any{"role": "user", "content": "frozen caller diagnostic"}})

	// A slug whose acceptance feature is not in the checkout cannot be reviewed.
	in = f.input()
	in.Intent.Slug = "absent"
	env = newEnv(t, f)
	if _, err := (review.Independent{}).Review(context.Background(), in, env); err == nil {
		t.Error("review.Intent.Slug: a review of an Intent without a feature in the checkout did not fail")
	}
}

// TestIntentSourceProvenanceIsStatedAndGuarded pins review.Intent.Source: the reviewer is told
// the receipt, and a receipt whose custody changed stops the review at its next tool call with
// candidate/source_provenance_changed.
func TestIntentSourceProvenanceIsStatedAndGuarded(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	dir := filepath.Join(f.root, ".kogen", "local", "provenance", "fixture")
	data := `{"comparison":"c"}`
	path := filepath.Join(dir, "current.json")
	writeFile(t, path, data)
	for _, d := range []string{dir, filepath.Dir(dir)} {
		if err := chmod(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	ref := &project.SourceRef{Path: path, SHA256: sha256Hex(data), ContractSHA256: "contract-hash"}
	in := f.input()
	in.Intent.Source = ref

	env := newEnv(t, f)
	env.answers = []*provider.Response{callTool("read_file", `{"path":"app.txt"}`), submitReview(`{"verdict":"approve","findings":[]}`)}
	if _, err := (review.Independent{}).Review(context.Background(), in, env); err != nil {
		t.Fatal(err)
	}
	if want := project.SourceMessage(ref); !strings.Contains(env.prepares[0].material.Authority, want) {
		t.Errorf("review.Intent.Source: the authority lacks the receipt's message %q", want)
	}

	// The receipt changes while the review runs: the next tool call stops it.
	writeFile(t, path, `{"comparison":"changed"}`)
	env = newEnv(t, f)
	env.answers = []*provider.Response{callTool("read_file", `{"path":"app.txt"}`)}
	_, err := review.Independent{}.Review(context.Background(), in, env)
	wantError(t, "review.Intent.Source custody", err, "candidate", "source_provenance_changed")
}
