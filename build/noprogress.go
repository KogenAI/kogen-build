package build

import (
	"strings"

	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

// noEditWatch is the Build's evidence for rounds that leave the candidate unchanged. It is not
// part of the State: a fresh or resumed invocation starts without a remembered red pass, so its
// first round is always verified.
type noEditWatch struct {
	// roundStart is the candidate tree at the start of the builder round in progress ("" when
	// the round's start is unknown, as when a Build resumes at the end of a round).
	roundStart string
	// red is the latest clean red pass, nil after a green pass.
	red *redPass
	// rounds counts the builder rounds this invocation ran, and noEdits those without edits.
	rounds, noEdits int
}

// redPass is a remembered red verification pass with its stage ("" for the whole Intent) and
// the round that received the no-edit notice for it (0: none yet).
type redPass struct {
	stage       string
	pass        verify.Pass
	noticeRound int
}

// remember records what a verification pass means for the no-edit rule: a green pass clears the
// remembered red pass, a clean red pass over a nonempty tree replaces it.
func (b *Attempt) remember(stage string, pass verify.Pass) {
	switch {
	case pass.Green:
		b.noEdit.red = nil
	case pass.Clean && len(pass.Moved) == 0 && pass.Tree != "":
		b.noEdit.red = &redPass{stage: stage, pass: pass}
	}
}

// firstFailure is the first named failure of a pass: its first missing acceptance item, else its
// first failing check.
func firstFailure(pass verify.Pass) string {
	if missing := missingAcceptanceItems(pass); len(missing) > 0 {
		return missing[0]
	}
	if len(pass.Failing) > 0 {
		return pass.Failing[0]
	}
	return ""
}

// noEditNotice is the whole message to a builder whose round left the tree of a red pass as it was.
func noEditNotice(round int, pass verify.Pass) string {
	n := itoa(pass.N)
	return "Round " + itoa(round) + " made no edits: the candidate is byte-identical to the tree verification pass " + n +
		" checked, so it was not verified again; pass " + n + " is still red.\n" +
		"Make an edit toward the first named failure: " + firstFailure(pass) + ".\n" +
		"If a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker " +
		"with its paths, the contradiction and your observed evidence instead.\n" +
		"Another round without edits stops the Build as no_progress (no edits)."
}

// noEditRound decides, after the fixers, what a round that left the candidate tree unchanged
// means. stage is the active stage ID ("" for the whole Intent). A round with edits, or whose tree
// no remembered red pass checked, is verified as usual (check true). A round that left the tree
// of the remembered red pass gets one notice instead of a verification, and a second one stops
// the Build as no_progress; neither runs a pass, its checks or its acceptance.
func (b *Attempt) noEditRound(stage string) (feedback string, check bool, err error) {
	w := &b.noEdit
	start := w.roundStart
	w.roundStart = ""
	if start == "" {
		return "", true, nil
	}
	tree, err := b.checkRepo().Tree(b.Start, b.Excluded)
	if err != nil {
		return "", false, err
	}
	if tree != start {
		return "", true, nil
	}
	w.noEdits++
	red := w.red
	if red == nil || red.stage != stage || red.pass.Tree != tree {
		b.s.Progress("kogen: round " + itoa(b.Round) + " made no edits")
		return "", true, nil
	}
	pass := red.pass
	if red.noticeRound == 0 {
		red.noticeRound = b.Round
		first := firstFailure(pass)
		b.s.Progress("kogen: round " + itoa(b.Round) + " made no edits; not verifying the tree of pass " + itoa(pass.N) +
			" again; asked the builder to edit toward " + first + " or report a protected-file blocker")
		return noEditNotice(b.Round, pass), false, nil
	}
	b.s.Progress("kogen: round " + itoa(b.Round) + " made no edits")
	e := sys.Fail("candidate", "no_progress", "no edits: round "+itoa(b.Round)+" made no edits after the builder was told round "+
		itoa(red.noticeRound)+" made none; verification pass "+itoa(pass.N)+" is still red and was not run again; edits are left in place", 1)
	e.Lines = []string{
		"candidate tree: " + pass.Tree,
		"failing: " + strings.Join(pass.Failing, ", "),
		"first failure: " + firstFailure(pass),
		"rounds without edits: " + itoa(w.noEdits) + " of " + itoa(w.rounds),
		"No further verification or model request was made. Fix the named failure by hand, or `kogen intent resume " +
			b.Run.ID + "` to verify again and give the builder the feedback again.",
	}
	return "", false, e
}

// missingAcceptanceItems are the expected scenarios of the pass's acceptance that did not pass,
// in feature order, each as `<status>: <name> (line <line>)`. A pass whose acceptance did not
// run has none.
func missingAcceptanceItems(pass verify.Pass) []string {
	if pass.Acceptance == nil {
		return nil
	}
	var items []string
	for _, s := range pass.Acceptance.Scenarios {
		if s.Status != "passed" {
			items = append(items, s.Status+": "+s.Name+" (line "+itoa(s.Line)+")")
		}
	}
	return items
}
