package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/sys"
)

// commitApproved is the verified commit (GIT-3..8) of what review approved; on a commit it
// reports the landing. When the candidate is no longer the reviewed tree it commits nothing and
// reports drifted (the routing starts verification and review over); with restart false that
// stops the Build instead: the resume of a failed commit never runs a new cycle.
func (b *Attempt) commitApproved(ctx context.Context, ap Approved, restart bool) (drifted bool, err error) {
	b.s.Progress("kogen: committing")
	if err := b.s.Err(); err != nil {
		return false, err
	}
	if err := b.BaselineIntact(); err != nil {
		return false, err
	}
	if err := b.headIntact(); err != nil {
		return false, err
	}
	pass, review := ap.Pass, ap.Review
	tree, err := b.repo.Tree(b.Start, b.Excluded)
	if err != nil {
		return false, err
	}
	if !(pass.Green && pass.Clean && review.Tree == pass.Tree) || tree != pass.Tree {
		if !restart {
			return false, core.ResumeRefused("the candidate is no longer the reviewed tree " + pass.Tree + "; " + b.treeDrift(tree, pass.Tree))
		}
		// Something changed after the reviewed pass: verify and review again.
		return true, nil
	}
	headTree, err := b.repo.Resolve(b.Start + "^{tree}")
	if err != nil {
		return false, err
	}
	alreadyPresent := tree == headTree
	commit := b.Start
	if !alreadyPresent {
		if err := b.Save(Continuation{Stage: StageCommit, Tree: pass.Tree}); err != nil {
			return false, err
		}
		if err := b.facts.update(b, "committing", ""); err != nil {
			return false, err
		}
		b.Run.Event(map[string]any{"event": "commit", "tree": pass.Tree})
		commit, err = b.repo.Commit(b.s, git.CommitSpec{
			Tree: pass.Tree, Start: b.Start, Title: b.intent.Title, Slug: b.Slug,
			MessageFile: filepath.Join(b.Run.Dir, "commit-message.txt"),
		})
		if err != nil {
			// A definite failure with HEAD still at the start leaves the verified candidate intact
			// and resumable; an interrupt, a crash or a hook-undone commit stays an uncertain
			// commit boundary.
			var failure *sys.Error
			if errors.As(err, &failure) && failure.Class == "environment" && failure.Reason == "commit_failed" && b.repo.Head() == b.Start {
				b.Continuation = Continuation{Stage: StageCommitFailed, Tree: pass.Tree}
			}
			return false, err
		}
	}
	os.Remove(project.ApprovalPath(b.Root, b.Slug))
	b.SetPhase(phaseLanded)
	if err := b.Save(Continuation{Stage: StageLanded, Commit: commit, AlreadyPresent: alreadyPresent}); err != nil {
		return false, err
	}
	var notes []map[string]any
	for _, f := range review.Findings {
		if f["severity"] == "note" {
			notes = append(notes, f)
		}
	}
	outcome := "landed"
	if alreadyPresent {
		outcome = "already_present"
	}
	b.Run.Event(map[string]any{"event": "outcome", "outcome": outcome, "commit": commit})
	if err := b.Run.Write("result.json", b.result(outcome, map[string]any{"commit": commit})); err != nil {
		return false, err
	}
	if err := b.facts.update(b, "", outcome); err != nil {
		return false, err
	}
	if alreadyPresent {
		b.s.Puts(fmt.Sprintf("already present %s at HEAD %s (Build %s)", b.Slug, commit, b.Run.ID))
	} else {
		b.s.Puts(fmt.Sprintf("landed %s %s (Build %s)", b.Slug, short(commit), b.Run.ID))
	}
	b.s.Puts("  checks: " + pass.ChecksLine())
	for _, line := range b.comparisonLines() {
		b.s.Puts(line)
	}
	line := "  review: approved"
	if len(notes) > 0 {
		line += fmt.Sprintf(", %d note(s)", len(notes))
	}
	b.s.Puts(line)
	for _, f := range notes {
		b.s.Puts(fmt.Sprintf("  note: %s:%s: %s", core.Display(f["path"]), core.Display(f["line"]), core.Display(f["message"])))
	}
	b.s.Puts(fmt.Sprintf("  repairs: %d · tokens: %s · time: %s", b.Repairs, b.Usage.Total(), duration(b.elapsed())))
	if x := b.Executor; x != nil && x.Name != executor.Native {
		b.s.Puts("  executor: " + x.Name + " (" + executorUsageLine(x.Usage) + ")")
	}
	if x := b.ReviewExecutor; x != nil {
		b.s.Puts("  review executor: " + x.Name + " (" + executorUsageLine(x.Usage) + ")")
	}
	return false, nil
}

func duration(d interface{ Seconds() float64 }) string {
	s := int(d.Seconds())
	switch {
	case s >= 3600:
		return fmt.Sprintf("%dh%02dm%02ds", s/3600, s/60%60, s%60)
	case s >= 60:
		return fmt.Sprintf("%dm%02ds", s/60, s%60)
	}
	return fmt.Sprintf("%ds", s)
}
