package build

import (
	"context"
	"fmt"

	"github.com/KogenAI/kogen-coordinator/route"
	"github.com/KogenAI/kogen-verification/verify"
)

// The outer routing of the candidate (verify → review → repair → commit, and the resume of a
// Build stopped at one of those steps) is kogen-coordinator's route. The Build keeps its
// implementation loop and feedback policy; steps are its phases as the routing drives them.

// steps is the Build's phases through route's narrow interface: the evidence is a verification
// pass, the approval what a review approved.
type steps struct{ b *Attempt }

var _ route.Steps[verify.Pass, Approved] = steps{}

// Verify is the whole-Intent pass. A builder pass (after a builder round) is also checked for a
// repeated host or base failure; a pass that only restarts verification after a drifted review
// is not. A red pass goes back to the builder as its feedback, with a missing-items notice.
func (s steps) Verify(ctx context.Context, builderPass bool) (verify.Pass, bool, string, error) {
	b := s.b
	pass, err := b.Verify(ctx)
	if err != nil {
		return verify.Pass{}, false, "", err
	}
	if builderPass {
		if err := b.hostOrBaseStop(pass); err != nil {
			return verify.Pass{}, false, "", err
		}
	}
	notice, err := b.stalled("", pass)
	if err != nil {
		return verify.Pass{}, false, "", err
	}
	editedNotice := b.unmovedNotice("", pass)
	if !pass.Green {
		b.Repairs++
		return pass, false, withNotice(editedNotice, withNotice(notice, b.PassFeedback(pass, "finish"))), nil
	}
	return pass, true, "", nil
}

// Review is a fresh review of a green pass, with the Build's handling of a review that ends
// without a valid verdict.
func (s steps) Review(ctx context.Context, pass verify.Pass) (Approved, error) {
	b := s.b
	approved, err := b.reviewWithRetry(ctx, func() (Approved, error) { return b.deps.reviewer().Review(ctx, b, pass) })
	if err != nil {
		return Approved{}, err
	}
	return approved, b.Track()
}

// Continue continues the review of the checkpoint.
func (s steps) Continue(ctx context.Context) (Approved, error) {
	b := s.b
	return b.reviewWithRetry(ctx, func() (Approved, error) { return b.deps.reviewer().Resume(ctx, b) })
}

// Continuable: only the review stage of an external reviewer cannot be continued.
func (s steps) Continuable() bool { return !s.b.externalReviewStage() }

// Fresh prepares the fresh review of a review-only checkpoint, or of the external review stage.
func (s steps) Fresh(ctx context.Context, stage string) (verify.Pass, error) {
	if stage == StageReviewOnly {
		return s.b.reviewOnlyFresh()
	}
	return s.b.externalReviewFresh()
}

// Approved is the saved approval of a commit_failed checkpoint.
func (s steps) Approved(context.Context) (Approved, error) {
	b := s.b
	pass, review := *b.LastPass, *b.LastReview
	b.s.Puts(fmt.Sprintf("finishing the commit of the candidate verified in pass %d and approved in review %d, "+
		"with no builder round, no review and no rerun of its checks", pass.N, review.N))
	return Approved{Pass: pass, Review: review}, nil
}

func (s steps) Commit(ctx context.Context, ap Approved, restart bool) (bool, error) {
	return s.b.commitApproved(ctx, ap, restart)
}
