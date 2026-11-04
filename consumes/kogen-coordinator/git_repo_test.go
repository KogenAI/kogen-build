package consumes

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/git"
)

// build/build.go, build/recover.go, build/resume.go and cmd/kogen-build/main.go open the
// checkout named by --project, which may be any directory inside it, and use Repo.Dir as the
// root; a directory that is no repository is the environment/not_a_git_repo error.
func TestOpenFindsTheCheckoutRoot(t *testing.T) {
	var _ func(string, []string) (*git.Repo, error) = git.Open

	f := newRepo(t)
	f.write(t, "sub/dir/file.txt", "x\n")
	var repo *git.Repo
	repo, err := git.Open(filepath.Join(f.Dir, "sub", "dir"), os.Environ())
	check(t, err)
	equal(t, "git.Repo.Dir through git.Open of a subdirectory", repo.Dir, f.Dir)
	var _ string = repo.Dir
	repo, err = git.Open(f.Dir, os.Environ())
	check(t, err)
	equal(t, "git.Repo.Dir through git.Open of the root", repo.Dir, f.Dir)

	dir := t.TempDir()
	_, err = git.Open(dir, append(os.Environ(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(dir)))
	e := wantSysError(t, "git.Open of a directory that is no repository", err, "environment", "not_a_git_repo", 3)
	equal(t, "git.Open detail", e.Detail, dir+"; run git init first")
}

// build/checkreuse.go uses the checkout's own Git for an Attempt made without one, without
// checking that the directory is a repository.
func TestNewMakesARepoWithoutChecking(t *testing.T) {
	var _ func(string, []string) *git.Repo = git.New

	dir := t.TempDir()
	plain := git.New(dir, os.Environ())
	equal(t, "git.New Dir of a directory that is no repository", plain.Dir, dir)

	f := newRepo(t)
	f.write(t, "new.txt", "new\n")
	repo := git.New(f.Dir, os.Environ())
	equal(t, "git.New Dir", repo.Dir, f.Dir)
	tree, err := repo.Tree(f.head(t), nil)
	check(t, err)
	if want := strings.TrimSpace(f.git(t, "ls-tree", tree, "new.txt")); !strings.HasSuffix(want, "\tnew.txt") {
		t.Errorf("git.New(...).Tree = %s does not hold new.txt: %q", tree, want)
	}
}

// build/build.go starts from HEAD (no commits is environment/no_head), build/commit.go,
// build/resume.go and build/recover.go resolve trees and parents of commits.
func TestHeadAndResolve(t *testing.T) {
	var _ func(*git.Repo) string = (*git.Repo).Head
	var _ func(*git.Repo, string) (string, error) = (*git.Repo).Resolve

	bare := newBareRepo(t)
	equal(t, "git.Repo.Head without a commit", git.New(bare.Dir, os.Environ()).Head(), "")

	f := newRepo(t)
	first := f.head(t)
	f.write(t, "second.txt", "second\n")
	f.git(t, "add", ".")
	f.git(t, "commit", "-qm", "second")
	second := f.head(t)
	repo := git.New(f.Dir, os.Environ())
	equal(t, "git.Repo.Head", repo.Head(), second)

	got, err := repo.Resolve("HEAD")
	check(t, err)
	equal(t, "git.Repo.Resolve HEAD", got, second)
	got, err = repo.Resolve(second + "^{tree}")
	check(t, err)
	equal(t, "git.Repo.Resolve <commit>^{tree}", got, strings.TrimSpace(f.git(t, "rev-parse", "HEAD^{tree}")))
	got, err = repo.Resolve(second + "^")
	check(t, err)
	equal(t, "git.Repo.Resolve <commit>^", got, first)
	if got, err := repo.Resolve(first + "^"); err == nil {
		t.Errorf("git.Repo.Resolve of the parent of a root commit = %q, want an error", got)
	}
	if got, err := repo.Resolve("refs/heads/nope"); err == nil {
		t.Errorf("git.Repo.Resolve of a missing revision = %q, want an error", got)
	}
}

// build/build.go refuses a Build while a merge, rebase, cherry-pick or revert is unfinished and
// names the operation; build/recover.go refuses recovery the same way.
func TestOperationInProgress(t *testing.T) {
	var _ func(*git.Repo) (string, error) = (*git.Repo).OperationInProgress

	f := newRepo(t)
	repo := git.New(f.Dir, os.Environ())
	op, err := repo.OperationInProgress()
	check(t, err)
	equal(t, "git.Repo.OperationInProgress of a quiet repository", op, "")
	gitDir := filepath.Join(f.Dir, ".git")
	for _, c := range []struct{ file, want string }{
		{"MERGE_HEAD", "merge"}, {"CHERRY_PICK_HEAD", "cherry-pick"}, {"REVERT_HEAD", "revert"}, {"rebase-merge", "rebase"},
	} {
		path := filepath.Join(gitDir, c.file)
		check(t, os.WriteFile(path, nil, 0o644))
		op, err := repo.OperationInProgress()
		check(t, err)
		equal(t, "git.Repo.OperationInProgress with "+c.file, op, c.want)
		check(t, os.Remove(path))
	}
}

// build/review_retry.go, build/resume.go and kogen's index custody hand the repository's object
// format to StagedEquivalent.
func TestObjectFormat(t *testing.T) {
	var _ func(*git.Repo) (string, error) = (*git.Repo).ObjectFormat

	f := newRepo(t)
	format, err := git.New(f.Dir, os.Environ()).ObjectFormat()
	check(t, err)
	equal(t, "git.Repo.ObjectFormat", format, "sha1")
}

// build/build.go refuses a Build whose Git produces no author and committer identity from
// configuration (environment/git_identity_missing); it never guesses from the host.
func TestCanCommitNeedsAConfiguredIdentity(t *testing.T) {
	var _ func(*git.Repo) bool = (*git.Repo).CanCommit

	f := newRepo(t)
	repo := git.New(f.Dir, os.Environ())
	equal(t, "git.Repo.CanCommit with an identity", repo.CanCommit(), true)
	f.git(t, "config", "--unset", "user.email")
	equal(t, "git.Repo.CanCommit without user.email", repo.CanCommit(), false)
	f.git(t, "config", "user.email", "fixture@example.invalid")
	f.git(t, "config", "--unset", "user.name")
	equal(t, "git.Repo.CanCommit without user.name", repo.CanCommit(), false)
	f.git(t, "config", "user.name", "Fixture")
	equal(t, "git.Repo.CanCommit again", repo.CanCommit(), true)
}

// build/build.go lists the changed paths of the checkout to find what is not part of an
// Intent: staged, unstaged, deleted and untracked-but-not-ignored files, one path per file,
// relative to the root.
func TestChangedPaths(t *testing.T) {
	var _ func(*git.Repo, ...string) ([]string, error) = (*git.Repo).ChangedPaths

	f := newRepo(t)
	repo := git.New(f.Dir, os.Environ())
	clean, err := repo.ChangedPaths()
	check(t, err)
	equal(t, "git.Repo.ChangedPaths of a clean checkout", len(clean), 0)

	f.write(t, ".gitignore", "ignored.log\n")
	f.git(t, "add", ".gitignore")
	f.git(t, "commit", "-qm", "ignore")
	f.write(t, "README.md", "readme\nedited\n")
	f.write(t, "staged.txt", "staged\n")
	f.git(t, "add", "staged.txt")
	f.write(t, ".kogen/intents/other/intent.md", "other\n")
	f.write(t, ".kogen/intents/other/acceptance.feature", "Feature: x\n")
	f.write(t, "ignored.log", "x\n")
	got, err := repo.ChangedPaths()
	check(t, err)
	slices.Sort(got)
	equalList(t, "git.Repo.ChangedPaths", got,
		[]string{".kogen/intents/other/acceptance.feature", ".kogen/intents/other/intent.md", "README.md", "staged.txt"})

	f.git(t, "add", "-A")
	f.git(t, "commit", "-qm", "everything")
	check(t, os.Remove(filepath.Join(f.Dir, "README.md")))
	got, err = repo.ChangedPaths()
	check(t, err)
	equalList(t, "git.Repo.ChangedPaths of a deleted tracked file", got, []string{"README.md"})
	f.git(t, "checkout", "--", "README.md")
	f.git(t, "mv", "staged.txt", "moved.txt")
	got, err = repo.ChangedPaths()
	check(t, err)
	equalList(t, "git.Repo.ChangedPaths of a renamed file (the new path, then the old one)", got, []string{"moved.txt", "staged.txt"})
}

// build/review_retry.go, build/recover.go, build/stop.go and build/unmoved.go list the paths that
// differ between two trees.
func TestChangedBetweenTrees(t *testing.T) {
	var _ func(*git.Repo, string, string) ([]string, error) = (*git.Repo).Changed

	f := newRepo(t)
	f.write(t, "old name.txt", "content that is long enough to be a rename of itself\n")
	f.git(t, "add", ".")
	f.git(t, "commit", "-qm", "add")
	repo := git.New(f.Dir, os.Environ())
	before := f.head(t)
	f.git(t, "mv", "old name.txt", "new name.txt")
	f.write(t, "a.txt", "a\n")
	f.git(t, "add", ".")
	f.git(t, "commit", "-qm", "rename")
	after := f.head(t)

	got, err := repo.Changed(before, after)
	check(t, err)
	slices.Sort(got)
	equalList(t, "git.Repo.Changed (no rename detection)", got, []string{"a.txt", "new name.txt", "old name.txt"})
	got, err = repo.Changed(after, after)
	check(t, err)
	equal(t, "git.Repo.Changed of one tree", len(got), 0)
	treeBefore, err := repo.Resolve(before + "^{tree}")
	check(t, err)
	treeAfter, err := repo.Resolve(after + "^{tree}")
	check(t, err)
	got, err = repo.Changed(treeBefore, treeAfter)
	check(t, err)
	equal(t, "git.Repo.Changed of trees", len(got), 3)
}

// build keeps the candidate as the tree of every tracked file and every untracked file that is
// not ignored, built outside the user's index and seeded from the start commit, minus the
// excluded Intents' folders.
func TestTreeIsTheCandidate(t *testing.T) {
	var _ func(*git.Repo, string, []string) (string, error) = (*git.Repo).Tree

	f := newRepo(t)
	f.write(t, ".gitignore", "ignored.log\n")
	f.write(t, "gone.txt", "gone\n")
	f.git(t, "add", ".")
	f.git(t, "commit", "-qm", "base")
	start := f.head(t)
	repo := git.New(f.Dir, os.Environ())

	clean, err := repo.Tree(start, nil)
	check(t, err)
	equal(t, "git.Repo.Tree of a clean checkout is HEAD's tree", clean, strings.TrimSpace(f.git(t, "rev-parse", "HEAD^{tree}")))

	f.write(t, "staged-by-user.txt", "the user staged this\n")
	f.git(t, "add", "staged-by-user.txt")
	indexBefore := f.git(t, "ls-files", "--stage")
	indexBytes := readFile(t, filepath.Join(f.Dir, ".git", "index"))
	f.write(t, "README.md", "readme\nedited\n")
	f.write(t, "run.sh", "#!/bin/sh\n")
	chmod(t, filepath.Join(f.Dir, "run.sh"), 0o755)
	f.write(t, ".kogen/intents/mine/intent.md", "mine\n")
	f.write(t, ".kogen/intents/other/intent.md", "other\n")
	f.write(t, "ignored.log", "ignored\n")
	check(t, os.Remove(filepath.Join(f.Dir, "gone.txt")))

	tree, err := repo.Tree(start, []string{".kogen/intents/other"})
	check(t, err)
	listing := f.git(t, "ls-tree", "-r", tree)
	for _, want := range []string{"\tREADME.md", "\trun.sh", "\t.kogen/intents/mine/intent.md", "\tstaged-by-user.txt", "\t.gitignore"} {
		if !strings.Contains(listing, want) {
			t.Errorf("git.Repo.Tree misses %q:\n%s", strings.TrimPrefix(want, "\t"), listing)
		}
	}
	for _, unwanted := range []string{"\tignored.log", "\tgone.txt", ".kogen/intents/other"} {
		if strings.Contains(listing, unwanted) {
			t.Errorf("git.Repo.Tree holds %q:\n%s", strings.TrimPrefix(unwanted, "\t"), listing)
		}
	}
	if !strings.Contains(listing, "100755 blob") || !strings.Contains(listing, "run.sh") {
		t.Errorf("git.Repo.Tree lost the executable mode of run.sh:\n%s", listing)
	}
	equal(t, "git.Repo.Tree leaves the user's index", f.git(t, "ls-files", "--stage"), indexBefore)
	equal(t, "git.Repo.Tree leaves the index file", readFile(t, filepath.Join(f.Dir, ".git", "index")), indexBytes)

	withOther, err := repo.Tree(start, nil)
	check(t, err)
	equal(t, "git.Repo.Tree without exclusions differs from the excluded one", withOther != tree, true)
	again, err := repo.Tree(start, []string{".kogen/intents/other"})
	check(t, err)
	equal(t, "git.Repo.Tree is deterministic", again, tree)

	// cmd/kogen-build/main.go reports repo.Tree(repo.Head(), nil) as the stdout `tree` of a
	// refusal and ignores the error: in a repository without commits that is no tree at all.
	bare := newBareRepo(t)
	bare.write(t, "a.txt", "a\n")
	empty := git.New(bare.Dir, os.Environ())
	noTree, err := empty.Tree(empty.Head(), nil)
	if err == nil || noTree != "" {
		t.Errorf("git.Repo.Tree(git.Repo.Head(), nil) without a commit = %q, %v; want no tree and an error", noTree, err)
	}
	head, err := repo.Tree(repo.Head(), nil)
	check(t, err)
	equal(t, "git.Repo.Tree(git.Repo.Head(), nil) with uncommitted work", head != clean, true)
}
