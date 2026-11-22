package build

import (
	"testing"

	"github.com/KogenAI/kogen-coordinator/git"
)

// TestSnapshotTreeIsTheRepositoryTree: the candidate tree taken from the saved start index is
// the tree git.Repo.Tree makes, for the first snapshot and for the later ones that reuse the
// index, with excluded paths, after an edit, a new file and a deletion.
func TestSnapshotTreeIsTheRepositoryTree(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	repo := git.New(e.root, e.git)
	start := e.head()
	steps := []struct {
		name     string
		change   func()
		excluded []string
	}{
		{"unchanged", func() {}, nil},
		{"edited", func() { e.write("app.txt", "edited\n") }, nil},
		{"new file", func() { e.write("new.txt", "new\n") }, nil},
		{"new file excluded", func() {}, []string{"new.txt"}},
		{"deleted", func() { e.run("rm", "-f", "app.txt") }, nil},
	}
	for _, step := range steps {
		step.change()
		want, err := repo.Tree(start, step.excluded)
		if err != nil {
			t.Fatal(err)
		}
		got, err := snapshotTree(repo, start, step.excluded)
		if err != nil {
			t.Fatal(err)
		}
		eq(t, step.name, got, want)
	}
	if _, err := snapshotTree(repo, "-bad", nil); err == nil {
		t.Fatal("an option-like start was accepted")
	}
}
