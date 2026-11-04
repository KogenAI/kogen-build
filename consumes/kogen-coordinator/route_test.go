package consumes

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/KogenAI/kogen-coordinator/route"
)

// evidence and approval are this suite's own type arguments of route.Steps: kogen-build
// instantiates it with a verification pass and an approved review.
type evidence struct{ pass int }
type approval struct{ of evidence }

// stepsFx is a route.Steps that records the order its steps are driven in. Each Verify answers
// from verifies, each restartable Commit from commits.
type stepsFx struct {
	trace     []string
	verifies  []verifyAnswer
	commits   []commitAnswer
	continues bool
	failAt    string // the step that returns errFx
	freshSeen []string
	passes    []bool // the builderPass of each Verify
}

type verifyAnswer struct {
	green    bool
	feedback string
}

type commitAnswer struct{ drifted bool }

var errFx = errors.New("step failed")

func (s *stepsFx) step(name string) error {
	s.trace = append(s.trace, name)
	if s.failAt == name {
		return errFx
	}
	return nil
}

func (s *stepsFx) Verify(_ context.Context, builderPass bool) (evidence, bool, string, error) {
	s.passes = append(s.passes, builderPass)
	if err := s.step("verify"); err != nil {
		return evidence{}, false, "", err
	}
	v := s.verifies[0]
	s.verifies = s.verifies[1:]
	return evidence{pass: len(s.passes)}, v.green, v.feedback, nil
}

func (s *stepsFx) Review(_ context.Context, e evidence) (approval, error) {
	if err := s.step("review"); err != nil {
		return approval{}, err
	}
	return approval{of: e}, nil
}

func (s *stepsFx) Continue(context.Context) (approval, error) {
	return approval{of: evidence{pass: 100}}, s.step("continue")
}

func (s *stepsFx) Continuable() bool { return s.continues }

func (s *stepsFx) Fresh(_ context.Context, stage string) (evidence, error) {
	s.freshSeen = append(s.freshSeen, stage)
	return evidence{pass: 200}, s.step("fresh")
}

func (s *stepsFx) Approved(context.Context) (approval, error) {
	return approval{of: evidence{pass: 300}}, s.step("approved")
}

func (s *stepsFx) Commit(_ context.Context, a approval, restart bool) (bool, error) {
	if !restart {
		return false, s.step("commit once")
	}
	if err := s.step("commit"); err != nil {
		return false, err
	}
	c := s.commits[0]
	s.commits = s.commits[1:]
	return c.drifted, nil
}

// build/builder.go routes the whole-Intent pass through route.Steps; build/route.go implements
// it. The complete method set is the contract: a method added, removed or changed breaks
// build's `steps`.
func TestStepsMethodSet(t *testing.T) {
	var _ route.Steps[evidence, approval] = &stepsFx{}
	var _ func(route.Steps[evidence, approval], context.Context, bool) (evidence, bool, string, error) = route.Steps[evidence, approval].Verify
	var _ func(route.Steps[evidence, approval], context.Context, evidence) (approval, error) = route.Steps[evidence, approval].Review
	var _ func(route.Steps[evidence, approval], context.Context) (approval, error) = route.Steps[evidence, approval].Continue
	var _ func(route.Steps[evidence, approval]) bool = route.Steps[evidence, approval].Continuable
	var _ func(route.Steps[evidence, approval], context.Context, string) (evidence, error) = route.Steps[evidence, approval].Fresh
	var _ func(route.Steps[evidence, approval], context.Context) (approval, error) = route.Steps[evidence, approval].Approved
	var _ func(route.Steps[evidence, approval], context.Context, approval, bool) (bool, error) = route.Steps[evidence, approval].Commit

	typ := reflect.TypeFor[route.Steps[evidence, approval]]()
	var names []string
	for i := range typ.NumMethod() {
		names = append(names, typ.Method(i).Name)
	}
	equalList(t, "route.Steps methods", names, []string{"Approved", "Commit", "Continuable", "Continue", "Fresh", "Review", "Verify"})
}

// build/core_state.go persists these stage names in its checkpoints: they are bytes of
// persisted state and never change.
func TestStageNamesAreThePersistedOnes(t *testing.T) {
	var _ string = route.StageReview
	var _ string = route.StageReviewVerify
	var _ string = route.StageReviewRetry
	var _ string = route.StageReviewOnly
	var _ string = route.StageCommit
	var _ string = route.StageCommitFailed
	var _ string = route.StageLanded

	equal(t, "route.StageReview", route.StageReview, "review")
	equal(t, "route.StageReviewVerify", route.StageReviewVerify, "review_verify")
	equal(t, "route.StageReviewRetry", route.StageReviewRetry, "review_retry_verify")
	equal(t, "route.StageReviewOnly", route.StageReviewOnly, "review_only")
	equal(t, "route.StageCommit", route.StageCommit, "commit")
	equal(t, "route.StageCommitFailed", route.StageCommitFailed, "commit_failed")
	equal(t, "route.StageLanded", route.StageLanded, "landed")
}

// build/builder.go verifyAndReview: a red pass returns the feedback for the builder's repair
// (landed false, nothing reviewed); a green one is reviewed and its approval committed (landed
// true); an approval whose candidate drifted starts over with a fresh verification that is not
// a builder pass; an error of any step stops the cycle.
func TestCycleRoutesTheCandidate(t *testing.T) {
	var _ func(context.Context, route.Steps[evidence, approval], bool) (string, bool, error) = route.Cycle[evidence, approval]
	ctx := context.Background()

	s := &stepsFx{verifies: []verifyAnswer{{false, "repair this"}}}
	feedback, landed, err := route.Cycle[evidence, approval](ctx, s, true)
	equal(t, "route.Cycle red feedback", feedback, "repair this")
	equal(t, "route.Cycle red landed", landed, false)
	equal(t, "route.Cycle red error", err, nil)
	equalList(t, "route.Cycle red steps", s.trace, []string{"verify"})
	equal(t, "route.Cycle passes the builderPass on", s.passes[0], true)

	s = &stepsFx{verifies: []verifyAnswer{{true, ""}}, commits: []commitAnswer{{false}}}
	feedback, landed, err = route.Cycle[evidence, approval](ctx, s, false)
	equal(t, "route.Cycle green feedback", feedback, "")
	equal(t, "route.Cycle green landed", landed, true)
	equal(t, "route.Cycle green error", err, nil)
	equalList(t, "route.Cycle green steps", s.trace, []string{"verify", "review", "commit"})
	equal(t, "route.Cycle passes builderPass false on", s.passes[0], false)

	s = &stepsFx{verifies: []verifyAnswer{{true, ""}, {true, ""}}, commits: []commitAnswer{{true}, {false}}}
	_, landed, err = route.Cycle[evidence, approval](ctx, s, true)
	equal(t, "route.Cycle drift landed", landed, true)
	equal(t, "route.Cycle drift error", err, nil)
	equalList(t, "route.Cycle drift steps", s.trace, []string{"verify", "review", "commit", "verify", "review", "commit"})
	if !slices.Equal(s.passes, []bool{true, false}) {
		t.Errorf("route.Cycle builderPass of each verification after a drift = %v, want [true false]", s.passes)
	}

	s = &stepsFx{verifies: []verifyAnswer{{true, ""}, {false, "red again"}}, commits: []commitAnswer{{true}}}
	feedback, landed, err = route.Cycle[evidence, approval](ctx, s, true)
	equal(t, "route.Cycle drift then red feedback", feedback, "red again")
	equal(t, "route.Cycle drift then red landed", landed, false)
	equal(t, "route.Cycle drift then red error", err, nil)

	for _, step := range []string{"verify", "review", "commit"} {
		s := &stepsFx{failAt: step, verifies: []verifyAnswer{{true, "never"}}, commits: []commitAnswer{{false}}}
		feedback, landed, err := route.Cycle[evidence, approval](ctx, s, true)
		equal(t, "route.Cycle error at "+step, err, errFx)
		equal(t, "route.Cycle landed after an error at "+step, landed, false)
		equal(t, "route.Cycle feedback after an error at "+step, feedback, "")
	}
	// The review is of the evidence the verification returned, and the commit is of what the
	// review approved.
	reviewed := &approvalProbe{}
	if _, landed, err := route.Cycle[evidence, approval](ctx, reviewed, true); err != nil || !landed {
		t.Fatalf("route.Cycle with a probe: %v %v", landed, err)
	}
	equal[any](t, "route.Cycle hands the evidence to the review", reviewed.reviewed, evidence{pass: 7})
	equal[any](t, "route.Cycle hands the approval to the commit", reviewed.committed, approval{of: evidence{pass: 7}})
	equal(t, "route.Cycle commits with restart", reviewed.restart, true)
}

// approvalProbe records what Cycle hands from step to step.
type approvalProbe struct {
	stepsFx
	reviewed, committed any
	restart             bool
}

func (p *approvalProbe) Verify(context.Context, bool) (evidence, bool, string, error) {
	return evidence{pass: 7}, true, "", nil
}

func (p *approvalProbe) Review(_ context.Context, e evidence) (approval, error) {
	p.reviewed = e
	return approval{of: e}, nil
}

func (p *approvalProbe) Commit(_ context.Context, a approval, restart bool) (bool, error) {
	p.committed, p.restart = a, restart
	return false, nil
}

// build/builder.go resumeStage hands a Build stopped at a routed stage to route.Resume: a failed
// commit is finished alone (no review, never a new cycle); a review-only checkpoint, and a review
// whose reviewer cannot continue, get a fresh review of the saved green evidence; any other
// review continues; and a resumed Build must land.
func TestResumeContinuesAStoppedBuild(t *testing.T) {
	var _ func(context.Context, route.Steps[evidence, approval], string) error = route.Resume[evidence, approval]
	ctx := context.Background()

	for _, c := range []struct {
		name      string
		stage     string
		continues bool
		want      []string
		fresh     []string
	}{
		{"review continues", route.StageReview, true, []string{"continue", "commit"}, nil},
		{"review_verify continues", route.StageReviewVerify, true, []string{"continue", "commit"}, nil},
		{"review_retry_verify continues", route.StageReviewRetry, true, []string{"continue", "commit"}, nil},
		{"review_only is fresh even when continuable", route.StageReviewOnly, true, []string{"fresh", "review", "commit"}, []string{"review_only"}},
		{"a review that cannot continue is fresh", route.StageReview, false, []string{"fresh", "review", "commit"}, []string{"review"}},
		{"review_verify that cannot continue is fresh", route.StageReviewVerify, false, []string{"fresh", "review", "commit"}, []string{"review_verify"}},
	} {
		s := &stepsFx{continues: c.continues, commits: []commitAnswer{{false}}}
		err := route.Resume[evidence, approval](ctx, s, c.stage)
		equal(t, "route.Resume error ("+c.name+")", err, nil)
		equalList(t, "route.Resume steps ("+c.name+")", s.trace, c.want)
		equalList(t, "route.Resume Fresh stage ("+c.name+")", s.freshSeen, c.fresh)
	}

	// commit_failed: the saved approval is committed once, without a restart, and the commit's
	// error is the resume's.
	s := &stepsFx{commits: []commitAnswer{{false}}}
	probe := &approvalProbe{stepsFx: *s}
	equal(t, "route.Resume commit_failed error", route.Resume[evidence, approval](ctx, probe, route.StageCommitFailed), nil)
	equal[any](t, "route.Resume commit_failed commits the saved approval", probe.committed, approval{of: evidence{pass: 300}})
	equal(t, "route.Resume commit_failed commits without restart", probe.restart, false)
	equalList(t, "route.Resume commit_failed steps", probe.trace, []string{"approved"})
	s = &stepsFx{failAt: "commit once"}
	equal(t, "route.Resume commit_failed commit error", route.Resume[evidence, approval](ctx, s, route.StageCommitFailed), errFx)
	equalList(t, "route.Resume commit_failed refusal steps", s.trace, []string{"approved", "commit once"})
	s = &stepsFx{failAt: "approved"}
	equal(t, "route.Resume commit_failed approval error", route.Resume[evidence, approval](ctx, s, route.StageCommitFailed), errFx)
	equalList(t, "route.Resume commit_failed approval error steps", s.trace, []string{"approved"})

	// A drifted approval starts the cycle over with a verification that is not a builder pass,
	// and that cycle must land.
	s = &stepsFx{continues: true, commits: []commitAnswer{{true}, {false}}, verifies: []verifyAnswer{{true, ""}}}
	equal(t, "route.Resume drift error", route.Resume[evidence, approval](ctx, s, route.StageReview), nil)
	equalList(t, "route.Resume drift steps", s.trace, []string{"continue", "commit", "verify", "review", "commit"})
	equal(t, "route.Resume drift verifies as a fresh pass", s.passes[0], false)

	s = &stepsFx{continues: true, commits: []commitAnswer{{true}}, verifies: []verifyAnswer{{false, "still red"}}}
	err := route.Resume[evidence, approval](ctx, s, route.StageReview)
	if err == nil || err.Error() != "candidate changed after review in a resumed commit: still red" {
		t.Errorf("route.Resume of a drifted approval that is red now = %v, want the resumed-commit error naming the feedback", err)
	}

	for _, step := range []string{"continue", "fresh", "review", "commit"} {
		s := &stepsFx{failAt: step, continues: step == "continue", commits: []commitAnswer{{false}}}
		stage := route.StageReviewOnly
		if step == "continue" {
			stage = route.StageReview
		}
		equal(t, "route.Resume error at "+step, route.Resume[evidence, approval](ctx, s, stage), errFx)
	}
	s = &stepsFx{continues: true, failAt: "verify", commits: []commitAnswer{{true}}}
	equal(t, "route.Resume error of the restarted cycle", route.Resume[evidence, approval](ctx, s, route.StageReview), errFx)
}
