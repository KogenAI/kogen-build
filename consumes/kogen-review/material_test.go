package consumes

import (
	"context"
	"strings"
	"testing"

	gitpkg "github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-review/review"
	"github.com/KogenAI/kogen-verification/verify"
)

// TestMaterialSetIsMadeByTheReviewFromItsInput pins review.MaterialSet, which build keeps in the
// review's working window (build/material.go reads Authority, Source, Checks and Files; its
// legacy conversion builds one): the review makes it from its Input and hands it to Env.Prepare.
// Authority is the approved Intent, the protected paths and the configured commands
// (review.Commands), Source starts with the check results (Checks) and holds the diff and the
// text of every changed file, Files is the catalog of changed paths.
func TestMaterialSetIsMadeByTheReviewFromItsInput(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	in.Candidate.Protected = []string{"README.md", "go.mod"}
	in.Evidence.Commands = review.Commands{
		Fix:        []string{"gofmt -w ."},
		Checks:     []project.Check{{Name: "unit", Run: "make unit"}, {Name: "lint", Run: "make lint"}},
		Acceptance: "make accept",
	}
	cmds := in.Evidence.Commands
	var _ []string = cmds.Fix
	var _ []project.Check = cmds.Checks
	var _ string = cmds.Acceptance
	in.Evidence.Pass = verify.Pass{N: 1, Clean: true, Green: true, Tree: f.green, Model: verify.ModelResult{State: "none"},
		Checks: []verify.CommandResult{{Name: "unit", Command: "make unit"}, {Name: "lint", Command: "make lint"}}}
	var _ verify.Pass = in.Evidence.Pass
	var _ review.Commands = in.Evidence.Commands

	env := newEnv(t, f)
	env.answers = []*provider.Response{submitReview(`{"verdict":"approve","findings":[]}`)}
	if _, err := (review.Independent{}).Review(context.Background(), in, env); err != nil {
		t.Fatal(err)
	}
	m := env.prepares[0].material
	var _ string = m.Authority
	var _ string = m.Source
	var _ []map[string]any = m.Files
	var _ string = m.Checks

	for _, want := range []string{
		"# Intent\n\n# Fixture intent\n",
		"# acceptance.feature\n\n" + feature,
		"The approved baseline cannot change: README.md, go.mod.\n",
		"Fixers: gofmt -w ..\n",
		"Project checks: unit: make unit; lint: make lint.\n",
		"Acceptance: make accept.\n",
	} {
		if !strings.Contains(m.Authority, want) {
			t.Errorf("review.MaterialSet.Authority lacks %q", want)
		}
	}
	eq(t, "review.MaterialSet.Checks", m.Checks, "acceptance: passed (0 scenarios)\nmodel: not applicable\ncheck unit: passed\ncheck lint: passed")
	if !strings.HasPrefix(m.Source, "# Check results of the final pass\n\n"+m.Checks+"\n\n# Diff of existing/deleted paths against start HEAD "+f.start[:8]+"\n\n") {
		t.Errorf("review.MaterialSet.Source starts: %q", m.Source[:min(len(m.Source), 200)])
	}
	for _, want := range []string{"diff --git a/app.txt b/app.txt", "-start\n+candidate\n", "deleted file mode 100644", "--- app.txt (changed; hunks in diff)", "--- deleted.txt (deleted)"} {
		if !strings.Contains(m.Source, want) {
			t.Errorf("review.MaterialSet.Source lacks %q", want)
		}
	}

	appObject := git(t, f.root, "rev-parse", f.green+":app.txt")
	deletedObject := git(t, f.root, "rev-parse", f.start+":deleted.txt")
	deepEq(t, "review.MaterialSet.Files", m.Files, []map[string]any{
		{"path": "app.txt", "status": "changed", "origin": "", "rename": false, "mode": "100644", "git_object": appObject, "bytes": 10, "sha256": core.HashHex([]byte("candidate\n"))},
		{"path": "deleted.txt", "status": "deleted", "origin": "", "rename": false, "mode": "100644", "git_object": deletedObject, "bytes": 5, "sha256": core.HashHex([]byte("gone\n"))},
	})
}

// TestCandidateFieldsAreWhatTheReviewReads pins the fields of review.Candidate for a native
// review: Root is the checkout the review reads the Intent's feature and the files its findings
// name from (a finding at a path that is no file of the checkout or of the start commit is
// rejected), Repo and Start are where the candidate's trees, diffs and start files come from, Tree
// is the pass's tree (the candidate's), and Protected and Excluded are stated or applied (see
// their own tests). Builder acts for an external review.
func TestCandidateFieldsAreWhatTheReviewReads(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := f.input()
	c := in.Candidate
	var _ string = c.Root
	var _ *gitpkg.Repo = c.Repo
	var _ string = c.Start
	var _ string = c.Tree
	var _ []string = c.Protected
	var _ []string = c.Excluded
	var _ string = c.Builder
	eq(t, "review.Candidate.Tree", c.Tree, in.Evidence.Pass.Tree)
	eq(t, "review.Candidate.Root", c.Root, f.root)
	eq(t, "review.Candidate.Start", c.Start, f.start)

	// A finding names a file of the checkout, or of the start commit (a deleted one), and
	// nothing else.
	env := newEnv(t, f)
	env.answers = []*provider.Response{
		submitReview(`{"verdict":"approve","findings":[{"path":"missing.txt","line":1,"severity":"note","message":"x"}]}`),
		submitReview(`{"verdict":"approve","findings":[{"path":"app.txt","line":1,"severity":"note","message":"x"},{"path":"deleted.txt","line":1,"severity":"note","message":"x"}]}`),
	}
	res, err := review.Independent{}.Review(context.Background(), in, env)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.Candidate.Root findings checked", len(env.requests), 2)
	out := env.requests[1].Input[len(env.requests[1].Input)-1].(map[string]any)
	if s, _ := out["output"].(string); !strings.Contains(s, "missing.txt is not a file of the repository") {
		t.Errorf("review.Candidate.Repo: the rejection of a finding at an unknown path: %v", out)
	}
	eq(t, "review.Candidate.Start findings", len(res.Record.Findings), 2)
}
