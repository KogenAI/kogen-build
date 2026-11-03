package build

import (
	"context"
	"slices"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/agent"
	"github.com/KogenAI/kogen-review/review"
	"github.com/KogenAI/kogen-verification/verify"
	"kogen-loop/provider"
	"kogen-loop/tools"
)

// The independent review runs in kogen-review's review over a narrow input: the saved candidate,
// the exact approved Intent, the verification evidence, the permitted tools and, to resume, the
// review's checkpoint. The Build adapts its state to that input (reviewInput) and serves what the
// review asks of it while it runs (reviewEnv); the review returns its verdict, findings and
// repairs, which the Build records. A reviewer of kogen-review's review is a Reviewer of this
// package through delegate.

// delegate is a kogen-review reviewer as the Build's Reviewer.
type delegate struct{ reviewer review.Reviewer }

// nativeReviewer is the independent review and repair of chapter 06 (kogen-review's review).
func nativeReviewer() Reviewer { return delegate{review.Independent{}} }

// Review numbers a new review of pass and runs it.
func (d delegate) Review(ctx context.Context, b *Attempt, pass verify.Pass) (Approved, error) {
	b.Review++
	if err := b.Track(); err != nil {
		return Approved{}, err
	}
	res, err := d.reviewer.Review(ctx, b.reviewInput(pass), reviewEnv{b})
	return b.reviewed(res, err)
}

// Resume continues the review the checkpoint stopped in.
func (d delegate) Resume(ctx context.Context, b *Attempt) (Approved, error) {
	c := b.Continuation
	var pass verify.Pass
	if c.Pass != nil {
		pass = *c.Pass
	}
	in := b.reviewInput(pass)
	in.Checkpoint = &review.Checkpoint{Stage: c.Stage, Pass: c.Pass, Base: c.Base, Session: c.Review}
	res, err := d.reviewer.Resume(ctx, in, reviewEnv{b})
	return b.reviewed(res, err)
}

// reviewed records what a review returned: a review that ended without a valid verdict is the
// latest review, as invalid, and is kept for the retry handling (reviewWithRetry).
func (b *Attempt) reviewed(res review.Result, err error) (Approved, error) {
	if res.Invalid != nil {
		record := review.InvalidRecord(res.Invalid.Review)
		b.LastReview = &record
		b.invalid = res.Invalid
	}
	if err != nil {
		return Approved{}, err
	}
	return Approved{Pass: res.Pass, Review: res.Record}, nil
}

// reviewInput is the Build's state as the review of pass is given it.
func (b *Attempt) reviewInput(pass verify.Pass) review.Input {
	approval, err := decodeBase64(b.ApprovalBytes)
	if err != nil {
		approval = nil
	}
	builder := ""
	if b.Executor != nil {
		builder = b.Executor.SessionID
	}
	return review.Input{
		Candidate: review.Candidate{Root: b.Root, Start: b.Start, Tree: pass.Tree, Protected: b.Baseline, Excluded: b.Excluded, Builder: builder},
		Intent:    review.Intent{Approval: approval, Source: b.SourceProvenance, Role: b.Config.Models["review"], Plan: b.NavigationIn(phaseReview)},
		Evidence: review.Evidence{Pass: pass, Diagnostic: slices.Concat(b.diagnosticMessages(), b.reviewBaselineMessage(pass)),
			BaselineFailing: len(b.baselineFailures()) > 0},
		Tools: review.Tools{
			Native: func(enabled []string) *tools.Tools {
				return &tools.Tools{Root: b.Root, Session: b.s, Repo: agent.Git(b.repo), Enabled: enabled, Protected: b.Baseline, Excluded: b.Excluded}
			},
			Record: func(call tools.Call, result tools.Output) map[string]any {
				return b.toolRecordFields(call, result, b.reviewPartID)
			},
			Round: b.runGuardedRound,
			Home:  b.s.Getenv("HOME"),
		},
		Number: b.Review, Last: b.LastReview, Executor: b.ReviewExecutor,
		Run: review.Run{ID: b.Run.ID, Dir: b.Run.Dir},
	}
}

// reviewEnv is what the Build serves its independent review while it runs (review.Env).
type reviewEnv struct{ b *Attempt }

var _ review.Env = reviewEnv{}

func (e reviewEnv) Request(ctx context.Context, r review.Request) (*provider.Response, []any, error) {
	return e.b.Request(ctx, RequestSpec{
		Owner: "review", Role: r.Role, Instructions: r.Instructions, Input: r.Input, Tools: r.Tools,
		Retain: func(c *Continuation, input []any) {
			if raw, err := r.Retain(input); err == nil {
				c.Review = raw
			}
		},
		Prefill: r.Prefill, ForceTool: r.ForceTool,
	})
}

func (e reviewEnv) Material(pass verify.Pass) (materialSet, error) { return e.b.materialOf(pass) }
func (e reviewEnv) Blocks(tree string, m materialSet) ([]materialSegment, error) {
	return e.b.reviewBlocks(tree, m)
}

// Prepare opens a fresh review conversation: the unfinished observations of an earlier review
// that ended without a verdict are not carried into it (its archives stay in custody).
func (e reviewEnv) Prepare(tree string, m materialSet, role project.Role, instructions string, defs, diagnostic []any) ([]any, error) {
	if state := e.b.WorkingContext["review"]; state != nil {
		state.Observations = []any{}
	}
	return e.b.materialPrepare(tree, m, role, instructions, defs, diagnostic, false)
}

func (e reviewEnv) Unread(input []any) ([]string, error) { return e.b.materialUnread(input) }
func (e reviewEnv) Delivered(input []any) error          { return e.b.materialEvent(input) }
func (e reviewEnv) Prefill(role project.Role, instructions string, defs, input []any) ([]any, error) {
	return e.b.prefillSource(role, instructions, defs, input)
}
func (e reviewEnv) Custody() error { return e.b.validateWorkingContext() }

func (e reviewEnv) Tree() (string, error) { return e.b.repo.Tree(e.b.Start, e.b.Excluded) }
func (e reviewEnv) Changed(from, to string) ([]string, error) {
	return e.b.repo.Changed(from, to)
}
func (e reviewEnv) Diff(from, to string, paths []string) (string, error) {
	return e.b.repo.Diff(from, to, paths)
}
func (e reviewEnv) InStart(path string) bool {
	_, ok := e.b.repo.Show(e.b.Start, path)
	return ok
}
func (e reviewEnv) Drift(observed, green string) string { return e.b.treeDrift(observed, green) }

func (e reviewEnv) Verify(ctx context.Context) (verify.Pass, error) { return e.b.Verify(ctx) }
func (e reviewEnv) Feedback(pass verify.Pass, completion string) string {
	return e.b.PassFeedback(pass, completion)
}
func (e reviewEnv) BaselineIntact() error { return e.b.BaselineIntact() }
func (e reviewEnv) Comparison() []string  { return e.b.comparisonLines() }

// Save records the review's checkpoint: the Build is in its review phase.
func (e reviewEnv) Save(c review.Checkpoint) error {
	e.b.SetPhase(phaseReview)
	return e.b.Save(Continuation{Stage: c.Stage, Pass: c.Pass, Review: c.Session, Base: c.Base})
}

// Boundary records the tool boundary of a reviewer's tool call.
func (e reviewEnv) Boundary(input []any, done *review.Done) error {
	c := Continuation{Stage: StageTool, Owner: "review", Input: input}
	if done != nil {
		c.Completed = &Completed{Call: done.Call, Result: done.Result}
	}
	return e.b.Save(c)
}

func (e reviewEnv) Reviewed(record ReviewRecord) error {
	e.b.LastReview = &record
	return e.b.Track()
}
func (e reviewEnv) Repaired() error {
	e.b.Repairs++
	return e.b.Track()
}
func (e reviewEnv) Executor(x *ExecutorState)         { e.b.ReviewExecutor = x }
func (e reviewEnv) Track() error                      { return e.b.Track() }
func (e reviewEnv) Facts(phase string) error          { return e.b.facts.update(e.b, phase, "") }
func (e reviewEnv) Progress(line string)              { e.b.s.Progress(line) }
func (e reviewEnv) Event(event map[string]any)        { e.b.Run.Event(event) }
func (e reviewEnv) Write(name string, data any) error { return e.b.Run.Write(name, data) }
func (e reviewEnv) Err() error                        { return e.b.s.Err() }

// reviewBlocks locates each changed path's material in the review source: its block in the diff
// (from its `diff --git` line to the next block or the end of the diff) and its `--- <path>`
// heading with its full text (to the next heading or the end), in source order.
func (b *Attempt) reviewBlocks(tree string, m materialSet) ([]materialSegment, error) {
	_, segments, _, err := b.materialManifestAt(tree, m.Files, m.PartsTexts, m.PartsSource, materialPartsHeading)
	if err != nil {
		return nil, err
	}
	heading := map[string]int{}
	for _, t := range m.PartsTexts {
		heading[t.Path] = len(t.Heading)
	}
	for i, seg := range segments {
		if seg.Kind == "file" {
			segments[i].Start -= heading[seg.Path]
		}
	}
	slices.SortFunc(segments, func(a, b materialSegment) int { return a.Start - b.Start })
	return segments, nil
}
