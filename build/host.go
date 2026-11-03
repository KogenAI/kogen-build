package build

import (
	"context"
	"slices"

	"github.com/KogenAI/kogen-build/build/core"
	"github.com/KogenAI/kogen-build/build/review"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/agent"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-verification/verify"
	"kogen-loop/provider"
	"kogen-loop/tools"
)

// The independent review runs in package review over a core.Attempt: the Build's own state and
// the services of the Build in progress (host). A reviewer of build/review is a Reviewer of this
// package through delegate, which hands it that core.Attempt.

// delegate is a core.Reviewer as the Build's Reviewer, on the Build's own state.
type delegate struct{ reviewer core.Reviewer }

// nativeReviewer is the independent review and repair of chapter 06 (build/review).
func nativeReviewer() Reviewer { return delegate{review.Independent{}} }

func (d delegate) Review(ctx context.Context, b *Attempt, pass verify.Pass) (Approved, error) {
	return d.reviewer.Review(ctx, b.reviewing(), pass)
}

func (d delegate) Resume(ctx context.Context, b *Attempt) (Approved, error) {
	return d.reviewer.Resume(ctx, b.reviewing())
}

// reviewing is the Build as a reviewer of build/review sees it: this very state, not a copy.
func (b *Attempt) reviewing() *core.Attempt {
	return &core.Attempt{State: &b.State, Host: host{b}}
}

// host is the services the Build provides to its independent review (core.Host).
type host struct{ b *Attempt }

var _ core.Host = host{}

func (h host) Save(c Continuation) error                       { return h.b.Save(c) }
func (h host) Track() error                                    { return h.b.Track() }
func (h host) Verify(ctx context.Context) (verify.Pass, error) { return h.b.Verify(ctx) }
func (h host) BaselineIntact() error                           { return h.b.BaselineIntact() }
func (h host) PassFeedback(pass verify.Pass, completion string) string {
	return h.b.PassFeedback(pass, completion)
}
func (h host) Request(ctx context.Context, sp RequestSpec) (*provider.Response, []any, error) {
	return h.b.Request(ctx, sp)
}

func (h host) Progress(line string)            { h.b.s.Progress(line) }
func (h host) Err() error                      { return h.b.s.Err() }
func (h host) Getenv(name string) string       { return h.b.s.Getenv(name) }
func (h host) PublishFacts(phase string) error { return h.b.facts.update(h.b, phase, "") }

func (h host) Tree() (string, error) { return h.b.repo.Tree(h.b.Start, h.b.Excluded) }
func (h host) Changed(from, to string) ([]string, error) {
	return h.b.repo.Changed(from, to)
}
func (h host) Diff(from, to string, paths []string) (string, error) {
	return h.b.repo.Diff(from, to, paths)
}
func (h host) InStart(path string) bool {
	_, ok := h.b.repo.Show(h.b.Start, path)
	return ok
}
func (h host) TreeDrift(observed, green string) string { return h.b.treeDrift(observed, green) }

func (h host) NativeTools(enabled []string) *tools.Tools {
	b := h.b
	return &tools.Tools{Root: b.Root, Session: b.s, Repo: agent.Git(b.repo), Enabled: enabled, Protected: b.Baseline, Excluded: b.Excluded}
}
func (h host) ToolRecord(call tools.Call, result tools.Output) map[string]any {
	return h.b.toolRecordFields(call, result, h.b.reviewPartID)
}
func (h host) GuardedRound(ctx context.Context, sess executor.Session, in executor.Input) (executor.RoundResult, error) {
	return h.b.runGuardedRound(ctx, sess, in)
}

func (h host) DiagnosticMessages() []any { return h.b.diagnosticMessages() }
func (h host) BaselineMessage(pass verify.Pass) []any {
	return h.b.reviewBaselineMessage(pass)
}
func (h host) BaselineFailing() bool         { return len(h.b.baselineFailures()) > 0 }
func (h host) ComparisonLines() []string     { return h.b.comparisonLines() }
func (h host) ValidateWorkingContext() error { return h.b.validateWorkingContext() }

func (h host) OnDemand() bool { return false } // the on_demand review mode was removed on main (dfa857f)
func (h host) Material(pass verify.Pass) (core.MaterialSet, error) {
	return h.b.materialOf(pass)
}
func (h host) MaterialBlocks(tree string, m core.MaterialSet) ([]core.MaterialSegment, error) {
	return h.b.reviewBlocks(tree, m)
}
func (h host) MaterialPrepare(tree string, m core.MaterialSet, role project.Role, instructions string, defs, diagnostic []any, force bool) ([]any, error) {
	return h.b.materialPrepare(tree, m, role, instructions, defs, diagnostic, force)
}
func (h host) MaterialUnread(input []any) ([]string, error) { return h.b.materialUnread(input) }
func (h host) MaterialRequiredUnread(input []any) ([]string, error) {
	return nil, nil // nothing is required unread without the on_demand mode (removed on main, dfa857f)
}
func (h host) MaterialEvent(input []any) error { return h.b.materialEvent(input) }
func (h host) PrefillSource(role project.Role, instructions string, defs, input []any) ([]any, error) {
	return h.b.prefillSource(role, instructions, defs, input)
}

func (h host) Invalid(end *core.InvalidEnd) { h.b.invalid = end }

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
