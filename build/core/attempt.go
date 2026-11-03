// Package core is what package build and package build/review share: the persisted Build state
// and the types it holds, the run record, the review contract (Reviewer, Approved, RequestSpec)
// and the services the independent review uses (Host). It imports no other package of this
// module.
package core

import (
	"context"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-verification/verify"
	"kogen-loop/provider"
	"kogen-loop/tools"
)

// Approved is the result of a review that approves: the green pass whose exact tree was
// reviewed (the final one, after any repairs the reviewer made and verification repeated)
// and the review.
type Approved struct {
	Pass   verify.Pass
	Review ReviewRecord
}

// RequestSpec is one model request of a Build.
type RequestSpec struct {
	// Owner is the conversation: "build" or "review".
	Owner        string
	Role         project.Role
	Instructions string
	Input        []any
	Tools        []any
	// Retain records the input actually sent (after working-context resets) in the
	// continuation the request is checkpointed with. Nil: the builder's Continuation.Input.
	Retain func(c *Continuation, input []any)
	// Prefill, when set, may append native data to the input after the working window is
	// prepared and before the request is checkpointed, so the checkpoint, the archive and the
	// request all hold exactly what is sent.
	Prefill func(input []any) ([]any, error)
	// ForceTool names a tool this request alone must call, where the route supports named
	// tool selection (provider.Request.ToolChoice); the next request is ordinary again.
	ForceTool string
}

// Reviewer is the independent review and repair of a green pass (chapter 06). The Build
// calls it after every green verification pass and commits what it approves.
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

// Attempt is a Build in progress as a reviewer sees it: the Build's own state (the very value
// the Build keeps, not a copy: what a reviewer changes in it is the Build's) and the services
// the Build provides.
type Attempt struct {
	*State
	Host
}

// InvalidEnd is what a review that ended without a valid verdict leaves for the Build: the
// reason, the consumed allowances and what the review did to the checkout.
type InvalidEnd struct {
	Review          int
	ForcedRoute     string
	ForcedModel     string
	Reason          string
	Problems        []string
	RejectedVerdict int
	NoToolReminder  int
	Pass            verify.Pass
	// Base is the green pass the review session was started from (nil: the Build decides from
	// Repaired whether the pass under review is it).
	Base *verify.Pass
	// Reopened: it was the reopened conversation of a blocked review, whose blocker only a
	// new explicit decision of that reviewer can clear. Repaired: it had already judged and
	// sent back a verdict, so its pass may hold its own verified edits.
	Reopened, Repaired bool
}

// Host is the services a Build provides to its independent review. The first six are the
// exported operations of the Build's own Attempt; the others are what the review reads of the
// Build's environment, with the Build's own semantics.
type Host interface {
	// Save makes c the continuation and publishes a complete checkpoint.
	Save(c Continuation) error
	// Track publishes the local facts of the Build's current state.
	Track() error
	// Verify runs one verification pass over the current candidate.
	Verify(ctx context.Context) (verify.Pass, error)
	// BaselineIntact is the baseline guard: the approved files, with the same bytes.
	BaselineIntact() error
	// PassFeedback is what goes back to the repair owner after a red pass; completion names
	// the tool the owner calls when done.
	PassFeedback(pass verify.Pass, completion string) string
	// Request sends one request of a conversation with the shared bookkeeping.
	Request(ctx context.Context, sp RequestSpec) (*provider.Response, []any, error)

	// Progress writes a progress line of the Build; Err is the Build's interrupt, if any;
	// Getenv reads the Build's environment.
	Progress(line string)
	Err() error
	Getenv(name string) string
	// PublishFacts publishes the local facts with the phase the Build is in.
	PublishFacts(phase string) error

	// Tree is the candidate tree of the checkout now; Changed, Diff and InStart query the
	// checkout's Git trees and the start HEAD.
	Tree() (string, error)
	Changed(from, to string) ([]string, error)
	Diff(from, to string, paths []string) (string, error)
	InStart(path string) bool
	// TreeDrift states how the candidate tree differs from the green tree of its pass.
	TreeDrift(observed, green string) string

	// NativeTools are the native file and command tools of the checkout, limited to enabled.
	NativeTools(enabled []string) *tools.Tools
	// ToolRecord are the extra fields of a review_tool_call event for a tool call.
	ToolRecord(call tools.Call, result tools.Output) map[string]any
	// GuardedRound runs one round of an external executor session under the Build's guards.
	GuardedRound(ctx context.Context, sess executor.Session, in executor.Input) (executor.RoundResult, error)

	// DiagnosticMessages are the caller's frozen diagnostic data; BaselineMessage the check
	// baseline for the review of pass; BaselineFailing whether a check failed before the Build
	// and ComparisonLines its account.
	DiagnosticMessages() []any
	BaselineMessage(pass verify.Pass) []any
	BaselineFailing() bool
	ComparisonLines() []string
	// ValidateWorkingContext checks the custody of every archived working-context window.
	ValidateWorkingContext() error

	// OnDemand reports whether the review_material lever is on.
	OnDemand() bool
	// Material gathers the review material of the green pass; MaterialBlocks locates each
	// changed path's material in the source of the parts route.
	Material(pass verify.Pass) (MaterialSet, error)
	MaterialBlocks(tree string, m MaterialSet) ([]MaterialSegment, error)
	// MaterialPrepare makes the reviewer's opening conversation of m in the working window.
	MaterialPrepare(tree string, m MaterialSet, role project.Role, instructions string, defs, diagnostic []any, force bool) ([]any, error)
	// MaterialUnread and MaterialRequiredUnread are the pinned parts a conversation has not
	// read (all, and those covering a changed hunk); MaterialEvent records the delivery.
	MaterialUnread(input []any) ([]string, error)
	MaterialRequiredUnread(input []any) ([]string, error)
	MaterialEvent(input []any) error
	// PrefillSource appends the pinned source to a reopened review conversation.
	PrefillSource(role project.Role, instructions string, defs, input []any) ([]any, error)

	// Invalid hands the Build the review that just ended without a valid verdict.
	Invalid(end *InvalidEnd)
}
