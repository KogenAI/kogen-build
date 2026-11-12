package build

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

// latestPass is this Build's most recent verification pass: the whole-Intent pass b.LastPass
// or a later stage-gate pass.
func (b *Attempt) latestPass() *verify.Pass {
	pass := b.LastPass
	for _, e := range b.StageEvidence {
		if pass == nil || e.Pass.N > pass.N {
			p := e.Pass
			pass = &p
		}
	}
	return pass
}

// unbackedBlocker is the feedback for a well-formed blocker claim that no failure of the
// latest verification pass of the current candidate tree supports, or "" when one of its paths
// is backed by such a failure: named by it, or reached through a file that it and the claimed
// protected file both name. Kogen does not run a fresh pass to check a claim.
func (b *Attempt) unbackedBlocker(args map[string]any) (string, error) {
	tree, err := b.checkRepo().Tree(b.Start, b.Excluded)
	if err != nil {
		return "", err
	}
	var pass *verify.Pass
	if latest := b.latestPass(); latest != nil && latest.Tree == tree {
		pass = latest
	}
	var paths []string
	for _, p := range args["paths"].([]any) {
		paths = append(paths, p.(string))
	}
	if blockerBacked(b.Slug, pass, paths) || blockerReferenced(b.Slug, pass, paths, func(rel string) string {
		text, _ := readFile(b.Root, rel)
		return string(text)
	}) {
		return "", nil
	}
	return unbackedBlockerMessage(pass, paths), nil
}

// blockerBacked is whether one of the claimed paths occurs in the evidence of a red item of
// the pass; a nil pass backs nothing.
func blockerBacked(slug string, pass *verify.Pass, paths []string) bool {
	if pass == nil {
		return false
	}
	for _, evidence := range redEvidence(slug, pass) {
		for _, p := range paths {
			if strings.Contains(evidence, p) {
				return true
			}
		}
	}
	return false
}

// fileRef matches the file names a failure's output carries: a path or bare name with an extension.
var fileRef = regexp.MustCompile(`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*\.[A-Za-z][A-Za-z0-9]{0,7}`)

// minRefBase is the shortest file name that links a failure to a protected file, so that short
// names such as a.c do not back a claim by coincidence.
const minRefBase = 6

// blockerReferenced is whether a red item of the pass names a file that a claimed protected file
// also names. A protected Intent text that demands what existing code contradicts is named by
// the failure only through the file in conflict (a test failing to open a document the Intent
// removes), never by its own path. Kogen's own files are no link: every Intent text names its
// feature. A nil pass, a green pass and a claimed file that cannot be read back nothing.
func blockerReferenced(slug string, pass *verify.Pass, paths []string, read func(rel string) string) bool {
	if pass == nil {
		return false
	}
	bases := map[string]bool{}
	for _, evidence := range redEvidence(slug, pass) {
		for _, ref := range fileRef.FindAllString(evidence, -1) {
			if base := filepath.Base(ref); len(base) >= minRefBase && !strings.Contains(ref, ".kogen/") {
				bases[base] = true
			}
		}
	}
	for _, p := range paths {
		text := read(p)
		for base := range bases {
			if strings.Contains(text, base) {
				return true
			}
		}
	}
	return false
}

// redEvidence is the evidence text of each red item of a pass.
func redEvidence(slug string, pass *verify.Pass) []string {
	var out []string
	if a := pass.Acceptance; a != nil && len(a.Problems) > 0 {
		out = append(out, strings.Join(append([]string{project.FeaturePath(slug), a.Output}, a.Problems...), "\n"))
	}
	if pass.Model.State == "failed" {
		out = append(out, strings.Join(append([]string{project.ModelPath(slug)}, pass.Model.Problems...), "\n"))
	}
	for _, m := range pass.MaintainedModels {
		if m.Result.State == "failed" {
			out = append(out, strings.Join(append([]string{m.Path}, m.Result.Problems...), "\n"))
		}
	}
	for _, list := range [][]verify.CommandResult{pass.Setup, pass.Checks} {
		for _, c := range list {
			if c.Status != 0 {
				out = append(out, strings.Join([]string{c.Name, c.Command, c.Output}, "\n"))
			}
		}
	}
	return out
}

func unbackedBlockerMessage(pass *verify.Pass, paths []string) string {
	named := strings.Join(paths, ", ")
	if pass == nil {
		return "blocker not backed by verification: no verification pass has run on the current tree yet, so no failure names " + named +
			"; no protected edit is granted; make the failing items pass, or finish so verification runs on the current tree and report again with a protected path that a failure names"
	}
	return "blocker not backed by verification: no failure of verification pass " + strconv.Itoa(pass.N) + " names " + named +
		"; no protected edit is granted; make the failing items pass, or report again with a protected path that a failure names"
}
