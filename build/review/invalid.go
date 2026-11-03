package review

import (
	"github.com/KogenAI/kogen-build/build/core"
	"kogen-loop/provider"
	"kogen-loop/sys"
)

// A review that ends without a valid verdict is handed to the Build (core.Host.Invalid) with
// the candidate/review_invalid stop: the Build decides what follows (its retry and review-only
// handling), the reviewer only records the facts.

// allowance records a consumption of the review's allowances in the run log.
func (r *run) allowance(why string) {
	r.b.Run.Event(map[string]any{
		"event": "review_allowance", "review": r.b.Review, "why": why,
		"rejected_verdicts": r.sess.Rejected, "no_tool_reminders": r.sess.NoTool,
	})
}

// invalidStop ends the review without a verdict (the second spending of an allowance). It
// records the reason with both counters, notes the review for the retry gate and returns
// candidate/review_invalid. forced: the route was told to call submit_review and did not, which
// the hand-off names.
func (r *run) invalidStop(problems []string, reason string, forced bool) error {
	b := r.b
	b.Run.Event(map[string]any{
		"event": "review", "review": b.Review, "verdict": "invalid", "problems": problems, "reason": reason,
		"rejected_verdicts": r.sess.Rejected, "no_tool_reminders": r.sess.NoTool,
	})
	b.LastReview = &core.ReviewRecord{N: b.Review, Verdict: "invalid", Findings: []map[string]any{}}
	end := &core.InvalidEnd{
		Review: b.Review, Reason: reason, Problems: problems, RejectedVerdict: r.sess.Rejected, NoToolReminder: r.sess.NoTool,
		Pass: r.pass, Base: r.base, Reopened: r.sess.Reopened, Repaired: r.sess.Repaired,
	}
	if forced {
		end.ForcedRoute = provider.Name(core.ProviderRole(r.sess.Role))
		end.ForcedModel = r.sess.Role.Model
	}
	b.Invalid(end)
	return InvalidError()
}

// InvalidError is the stop of a review that ended without a valid verdict.
func InvalidError() *sys.Error {
	return sys.Fail("candidate", "review_invalid", "the reviewer did not return a valid verdict; edits are left in place", 1)
}
