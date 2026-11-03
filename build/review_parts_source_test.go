package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitpkg "github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
	"kogen-loop/provider"
)

// The parts source is today's source with only its third section changed: a changed text file is
// listed by its heading alone, and every other kind of path stays as in the source of one session.
func TestPartsSourceOmitsChangedFileTexts(t *testing.T) {
	t.Parallel()
	root := physical(t, t.TempDir())
	genv := isolatedEnv("GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=x@example.invalid", "GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL=x@example.invalid")
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir, cmd.Env = dir, genv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, content string, mode os.FileMode) { writeFile(t, filepath.Join(root, path), content, mode) }
	git(root, "init", "-q")
	write("README.md", "# Readme\n", 0o644)
	write("app.txt", "line one\nold marker\nline three\n", 0o644)
	write("gone.txt", "to be deleted\n", 0o644)
	write("pic.bin", "\x00\x01", 0o644)
	write(".gitignore", ".kogen/local/\n", 0o644)
	git(root, "add", ".")
	git(root, "commit", "-qm", "base")
	start := git(root, "rev-parse", "HEAD")

	write("app.txt", "line one\nnew marker\nline three\n", 0o644)
	os.Remove(filepath.Join(root, "gone.txt"))
	write("pic.bin", "\x00\x01\x02", 0o644)
	write("new.txt", "added text\nsecond\n", 0o644)
	write("new.bin", "\x00\xff", 0o644)
	git(root, "init", "-q", "sub")
	git(filepath.Join(root, "sub"), "commit", "--allow-empty", "-qm", "Nested fixture")
	subCommit := git(filepath.Join(root, "sub"), "rev-parse", "HEAD")
	write(".kogen/intents/fixture/acceptance.feature", "Feature: Gen\n", 0o644)

	run, err := StartRun(root)
	if err != nil {
		t.Fatal(err)
	}
	repo := gitpkg.New(root, genv)
	tree, err := repo.Tree(start, nil)
	if err != nil {
		t.Fatal(err)
	}
	b := &Attempt{
		State: State{
			Root: root, Slug: "fixture", Start: start, Excluded: []string{}, Run: run, Review: 1,
			Baseline: []string{".kogen/intents/fixture/acceptance.feature"},
			Config:   &project.Config{Acceptance: &project.Acceptance{Run: "npx cucumber"}},
		},
		deps: Deps{Client: &provider.Client{Cache: provider.NewCache(nil)}}, repo: repo, s: sessionOfEnv(genv, root),
		intent: &project.Intent{Text: "# Gen intent\n"},
	}
	pass := verify.Pass{Tree: tree, Model: verify.ModelNone, Acceptance: &verify.Acceptance{Scenarios: make([]verify.ScenarioResult, 1)}}
	m, err := materialOf(b, pass)
	if err != nil {
		t.Fatal(err)
	}

	const third = "# Full text of every changed or added file\n\n"
	full, tail, ok := strings.Cut(m.Source, third)
	if !ok {
		t.Fatalf("the source has no third section:\n%s", m.Source)
	}
	parts, partsTail, ok := strings.Cut(m.PartsSource, "# Full text of every added file\n\n")
	if !ok {
		t.Fatalf("the parts source has no third section:\n%s", m.PartsSource)
	}
	eq(t, "check results and diff", parts, full)
	contains(t, "tail of the source", tail, "--- app.txt\nline one\nnew marker\nline three\n")
	contains(t, "tail of the source", m.PartsSource, "Newly added files appear here once, with their mode, instead of being repeated in the diff. A changed file's hunks are in the diff above and its full text is in the checkout, so only its heading is listed here.\n\n")
	lacks(t, "parts source", m.PartsSource, "new marker\nline three")

	body := partsTail[strings.Index(partsTail, "\n\n")+2:]
	eq(t, "entries", body, strings.Join([]string{
		"--- .kogen/intents/fixture/acceptance.feature (added; mode 100644)",
		"Feature: Gen",
		"",
		"--- app.txt (changed; full text in the checkout)",
		"--- gone.txt (deleted)",
		"--- new.bin (added; mode 100644) (binary)",
		"--- new.txt (added; mode 100644)",
		"added text",
		"second",
		"",
		"--- pic.bin (binary)",
		"--- sub (added; mode 160000)",
		"Subproject commit " + subCommit,
		"",
	}, "\n"))
}
