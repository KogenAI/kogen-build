package consumes

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-review/review"
	"github.com/KogenAI/kogen-verification/verify"
)

// TestEnvMethodSetPinnedThroughTheInterface pins the complete method set of review.Env, which
// kogen-build serves (build/host.go, reviewEnv), with every signature written out and each
// method named through the interface: an added, removed or changed method fails here.
func TestEnvMethodSetPinnedThroughTheInterface(t *testing.T) {
	t.Parallel()
	var _ func(review.Env, context.Context, review.Request) (*provider.Response, []any, error) = review.Env.Request
	var _ func(review.Env, string, review.MaterialSet, project.Role, string, []any, []any) ([]any, error) = review.Env.Prepare
	var _ func(review.Env, []any) ([]string, error) = review.Env.Unread
	var _ func(review.Env, []any) error = review.Env.Delivered
	var _ func(review.Env, project.Role, string, []any, []any) ([]any, error) = review.Env.Prefill
	var _ func(review.Env) error = review.Env.Custody
	var _ func(review.Env, context.Context) (verify.Pass, error) = review.Env.Verify
	var _ func(review.Env, verify.Pass, string) string = review.Env.Feedback
	var _ func(review.Env) error = review.Env.BaselineIntact
	var _ func(review.Env) []string = review.Env.Comparison
	var _ func(review.Env, review.Checkpoint) error = review.Env.Save
	var _ func(review.Env, []any, *review.Done) error = review.Env.Boundary
	var _ func(review.Env, core.ReviewRecord) error = review.Env.Reviewed
	var _ func(review.Env) error = review.Env.Repaired
	var _ func(review.Env, *core.ExecutorState) = review.Env.Executor
	var _ func(review.Env) error = review.Env.Track
	var _ func(review.Env, string) error = review.Env.Facts
	var _ func(review.Env, string) = review.Env.Progress
	var _ func(review.Env, map[string]any) = review.Env.Event
	var _ func(review.Env, string, any) error = review.Env.Write
	var _ func(review.Env) error = review.Env.Err

	want := []string{"BaselineIntact", "Boundary", "Comparison", "Custody", "Delivered", "Err", "Event", "Executor", "Facts",
		"Feedback", "Prefill", "Prepare", "Progress", "Repaired", "Request", "Reviewed", "Save", "Track", "Unread", "Verify", "Write"}
	typ := reflect.TypeOf((*review.Env)(nil)).Elem()
	var got []string
	for i := 0; i < typ.NumMethod(); i++ {
		got = append(got, typ.Method(i).Name)
	}
	slices.Sort(got)
	deepEq(t, "review.Env method set", got, want)

	// A test Env implements it: the reviewer reaches it through the interface only.
	f := newFixture(t)
	var env review.Env = newEnv(t, f)
	if err := env.Err(); err != nil {
		t.Errorf("review.Env.Err: %v", err)
	}
}

// TestReviewerInterfacePinned pins review.Reviewer, which build/host.go's delegate calls (Review
// and Resume), with both signatures written out and named through the interface, and that
// review.Independent and what review.NewExternal returns are Reviewers.
func TestReviewerInterfacePinned(t *testing.T) {
	t.Parallel()
	var _ func(review.Reviewer, context.Context, review.Input, review.Env) (review.Result, error) = review.Reviewer.Review
	var _ func(review.Reviewer, context.Context, review.Input, review.Env) (review.Result, error) = review.Reviewer.Resume
	typ := reflect.TypeOf((*review.Reviewer)(nil)).Elem()
	eq(t, "review.Reviewer method count", typ.NumMethod(), 2)

	var native review.Reviewer = review.Independent{}
	var _ func(executor.Executor, executor.Descriptor, int) review.External = review.NewExternal
	var external review.Reviewer = review.NewExternal(&fakeExecutor{t: t}, externalDescriptor(), 3)

	// The reviewer is driven through the interface value, as build/host.go does.
	f := newFixture(t)
	env := newEnv(t, f)
	env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[]}`)}
	res, err := native.Review(context.Background(), f.input(), env)
	if err != nil {
		t.Fatalf("review.Reviewer.Review: %v", err)
	}
	eq(t, "review.Reviewer.Review verdict", res.Record.Verdict, "approve")

	pass := f.greenPass()
	raw, err := review.Session{Role: reviewRole, Usage: core.ZeroTokens(), Verdict: &review.Verdict{Verdict: "approve"}, Tree: f.green}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	in := f.input()
	in.Checkpoint = &review.Checkpoint{Stage: "review_verify", Pass: &pass, Session: raw}
	env = newEnv(t, f)
	res, err = native.Resume(context.Background(), in, env)
	if err != nil {
		t.Fatalf("review.Reviewer.Resume: %v", err)
	}
	eq(t, "review.Reviewer.Resume verdict", res.Record.Verdict, "approve")

	// An external reviewer cannot be resumed: kogen-build starts a fresh review instead.
	_, err = external.Resume(context.Background(), review.Input{}, nil)
	e := wantError(t, "review.External.Resume", err, "candidate", "resume_refused")
	eq(t, "review.External.Resume detail", e.Detail, "an independent review by an external executor cannot be resumed (resume capability: none)")
}

// TestFileNamesTheReviewRecord pins the name of a review's record in the run folder, which
// build/hooks.go writes the stub review's record under.
func TestFileNamesTheReviewRecord(t *testing.T) {
	t.Parallel()
	var _ func(int) string = review.File
	eq(t, "review.File 1", review.File(1), "review-1.json")
	eq(t, "review.File 12", review.File(12), "review-12.json")
}

// TestInvalidRecordIsTheRecordOfAReviewWithoutAVerdict pins the review record build keeps for a
// review that ended without a valid verdict (build/host.go): the number, the verdict "invalid",
// no tree and an empty, non-nil findings list that encodes as [].
func TestInvalidRecordIsTheRecordOfAReviewWithoutAVerdict(t *testing.T) {
	t.Parallel()
	var _ func(int) core.ReviewRecord = review.InvalidRecord
	rec := review.InvalidRecord(3)
	deepEq(t, "review.InvalidRecord", rec, core.ReviewRecord{N: 3, Verdict: "invalid", Findings: []map[string]any{}})
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.InvalidRecord JSON", string(b), `{"n":3,"verdict":"invalid","findings":[]}`)
}
