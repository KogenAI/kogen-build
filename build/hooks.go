package build

import (
	"context"
	"encoding/json"
	"time"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-review/review"
	"github.com/KogenAI/kogen-verification/verify"
)

// Deps are what a Build needs from outside: the provider client, Kogen's own tools and the
// hook a later change fills in (the independent review of a green pass).
type Outcome struct {
	Tree   string
	Checks []verify.CommandResult
	Error  *sys.Error
}

type Deps struct {
	// Outcome, when non-nil, receives evidence from this invocation.
	Outcome *Outcome
	// shellAfter is a test seam that production leaves nil. It is passed as the native tools'
	// tools.Tools.ShellAfter, so a test controls when the loop's shell wait elapses. It is no CLI
	// flag, project.yaml key, environment variable or checkpoint field; being unexported, only
	// tests in this package set it.
	shellAfter func(wait time.Duration) <-chan time.Time

	// Client sends the model requests; its prompt-cache is not used, the Build keeps its own
	// (restored from a checkpoint).
	Client *provider.Client
	// Tools are Kogen's own Node, Quint and Cucumber.
	Tools verify.Tools
	// Reviewer runs the independent review of a green pass. Nil: see newReviewer.
	Reviewer Reviewer
	// External returns the executor of a build role that selects an external executor
	// (`executor: claude-code`). Nil: such a role stops with executor/unavailable ("not
	// available in this program"). The native builder never uses it.
	External func(role project.Role) (executor.Executor, error)
}

// Reviewer is the independent review and repair of a green pass (chapter 06). The Build
// calls it after every green verification pass and commits what it approves. kogen-review's
// reviewers are Reviewers through delegate (host.go), which hands them only the review's
// narrow input and Env; this seam is for a program's or a test's own reviewer.
//
// An implementation works on the *Attempt it is given, in the Build's own state:
//
//   - b.Review counts reviews; it sets b.LastReview (verdict "pending" while a repair is
//     being verified) and keeps b.Continuation current with b.Save, using the review stages;
//   - b.Request sends a conversation request with the shared bookkeeping (working context,
//     checkpoint, usage, provider attempts, image delivery);
//   - b.Verify runs a verification pass over the current candidate (fixers are the Build's
//     round end, not part of it) and b.BaselineIntact guards the approved baseline;
//   - a stop (`candidate/review_blocked`, `candidate/review_invalid`, ...) is a *sys.Error:
//     return it and the Build ends as it ends for any other stop.
//
// An approval returns the pass and review it rests on; anything else is an error.
type Reviewer interface {
	// Review reviews pass, a green pass.
	Review(ctx context.Context, b *Attempt, pass verify.Pass) (Approved, error)
	// Resume continues a Build whose checkpoint is in a review stage (b.Continuation.Stage is
	// StageReview, StageReviewVerify or StageReviewRetry).
	// A review-only checkpoint (StageReviewOnly) is not resumed through it: the Build starts a
	// fresh Review of the saved green pass instead.
	Resume(ctx context.Context, b *Attempt) (Approved, error)
}

// Checkpoint is the envelope of a checkpoint file: the identity of the checkout, the
// candidate tree, the user's index and the approval as they were saved (index and approval
// are base64), and the state.
type Checkpoint struct {
	Version   int             `json:"version"`
	Identity  string          `json:"identity"`
	Candidate string          `json:"candidate"`
	Index     string          `json:"index"`
	Approval  string          `json:"approval"`
	State     json.RawMessage `json:"state"`
	// StageProgress repeats the staged progress of State (absent without a plan).
	StageProgress *StageProgress `json:"stage_progress,omitempty"`
	// RecoveryRaw is the raw identity of a builder-reported blocker's candidate (see
	// Recover), null while there is no blocker.
	RecoveryRaw map[string]*RawMeta `json:"recovery_raw"`
}

// Loaded is a decoded checkpoint.
type Loaded struct {
	Doc   Checkpoint
	State *State
}

// newReviewer is the reviewer a Build uses when Deps.Reviewer is nil: the independent
// review and repair of chapter 06 (kogen-review's review).
func newReviewer(Deps) Reviewer { return nativeReviewer() }

func (d Deps) reviewer() Reviewer {
	if d.Reviewer != nil {
		return d.Reviewer
	}
	return newReviewer(d)
}

// approveWithoutReview is the STUB reviewer: it approves the green pass as it is. It records
// the review the way a real one does (the number, the run log event, `review-<n>.json`) so
// that everything after it can be exercised, and marks itself in both.
type approveWithoutReview struct{}

func (approveWithoutReview) Review(_ context.Context, b *Attempt, pass verify.Pass) (Approved, error) {
	b.Review++
	record := ReviewRecord{N: b.Review, Verdict: "approve", Findings: []map[string]any{}, Tree: pass.Tree}
	b.LastReview = &record
	b.Track()
	b.Run.Write(review.File(record.N), map[string]any{
		"verdict": "approve", "findings": []any{}, "tree": pass.Tree, "stub": "no independent review ran",
	})
	b.Run.Event(map[string]any{"event": "review", "review": record.N, "pass": pass.N, "tree": pass.Tree,
		"verdict": "approve", "findings": []any{}, "stub": "no independent review ran"})
	return Approved{Pass: pass, Review: record}, nil
}

func (approveWithoutReview) Resume(context.Context, *Attempt) (Approved, error) {
	return Approved{}, core.ResumeRefused("no reviewer is available to continue this review")
}
