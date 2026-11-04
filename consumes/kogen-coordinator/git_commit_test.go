package consumes

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-loop/sys"
)

// candidateFx is a repository with an edit and a new file: the candidate tree and the HEAD it
// started from, as build/commit.go receives them.
func candidateFx(t *testing.T) (f *repoFx, repo *git.Repo, tree, start string) {
	t.Helper()
	f = newRepo(t)
	repo = git.New(f.Dir, os.Environ())
	start = f.head(t)
	f.write(t, "app.txt", "verified edit\n")
	f.write(t, "README.md", "readme\nchanged\n")
	tree, err := repo.Tree(start, nil)
	check(t, err)
	return f, repo, tree, start
}

func (f *repoFx) log(t *testing.T, format string) string {
	t.Helper()
	return f.git(t, "log", "-1", "--format="+format)
}

// build/commit.go lands the reviewed tree as one commit on top of the start HEAD: the message
// is the Intent's title and the Kogen-Intent trailer, the message file is the run's
// commit-message.txt, and what landed is the reviewed tree, not whatever the checkout holds now.
func TestCommitLandsTheReviewedTree(t *testing.T) {
	var _ func(*git.Repo, *sys.Session, git.CommitSpec) (string, error) = (*git.Repo).Commit
	spec := git.CommitSpec{}
	var _ string = spec.Tree
	var _ string = spec.Start
	var _ string = spec.Title
	var _ string = spec.Slug
	var _ string = spec.MessageFile

	f, repo, tree, start := candidateFx(t)
	spec = git.CommitSpec{
		Tree: tree, Start: start, Title: "Add the app", Slug: "add-app",
		MessageFile: filepath.Join(t.TempDir(), "commit-message.txt"),
	}
	f.write(t, "later.txt", "not in the reviewed tree\n")
	s, _ := session(f.Dir)

	commit, err := repo.Commit(s, spec)
	check(t, err)
	equal(t, "git.Repo.Commit returns the new HEAD", commit, f.head(t))
	equal(t, "git.Repo.Head after git.Repo.Commit", repo.Head(), commit)
	equal(t, "git.CommitSpec.Title and Slug: the commit message", f.log(t, "%B"), "Add the app\n\nKogen-Intent: add-app\n\n")
	equal(t, "git.CommitSpec.MessageFile: the message file", readFile(t, spec.MessageFile), "Add the app\n\nKogen-Intent: add-app\n")
	equal(t, "git.CommitSpec.Start: the parent", strings.TrimSpace(f.log(t, "%P")), start)
	equal(t, "git.CommitSpec.Tree: the committed tree", strings.TrimSpace(f.log(t, "%T")), tree)
	equal(t, "git.Repo.Commit author", strings.TrimSpace(f.log(t, "%an <%ae>")), fixtureIdentity)
	equal(t, "git.Repo.Commit leaves later work uncommitted", f.git(t, "status", "--porcelain"), "?? later.txt\n")
	equal(t, "git.Repo.Commit: the checkout keeps the candidate's content", readFile(t, filepath.Join(f.Dir, "README.md")), "readme\nchanged\n")
}

// build/commit.go treats an `environment/commit_failed` error with HEAD still at the start as a
// definite failure (the verified candidate stays intact and resumable) and anything else as an
// uncertain commit boundary; build/stop.go then puts the user's index back from the custody copy
// taken at the start.
func TestCommitFailureLeavesHeadAndTheIndexRestorable(t *testing.T) {
	f, repo, tree, start := candidateFx(t)
	spec := git.CommitSpec{
		Tree: tree, Start: start, Title: "Add the app", Slug: "add-app",
		MessageFile: filepath.Join(t.TempDir(), "commit-message.txt"),
	}
	backup := filepath.Join(t.TempDir(), "index-at-start")
	check(t, repo.SaveIndex(backup))
	f.hook(t, "pre-commit", "echo 'pre-commit says no'\nexit 1\n")
	s, _ := session(f.Dir)

	_, err := repo.Commit(s, spec)
	var failure *sys.Error
	if !errors.As(err, &failure) {
		t.Fatalf("git.Repo.Commit error = %v, want a *sys.Error", err)
	}
	equal(t, "git.Repo.Commit failure Class", failure.Class, "environment")
	equal(t, "git.Repo.Commit failure Reason", failure.Reason, "commit_failed")
	equal(t, "git.Repo.Commit failure Exit", failure.Exit, 3)
	equal(t, "git.Repo.Commit failure Detail", failure.Detail, "git commit failed (status 1); the verified changes are left uncommitted")
	equalList(t, "git.Repo.Commit failure Lines", failure.Lines, []string{"pre-commit says no"})
	equal(t, "git.Repo.Head after a failed commit", repo.Head(), start)

	if staged := f.git(t, "diff", "--cached", "--name-only"); staged == "" {
		t.Errorf("git.Repo.Commit left the index as it was; the candidate should be staged for the caller to restore")
	}
	check(t, repo.RestoreIndex(backup))
	equal(t, "git.Repo.RestoreIndex after a failed commit", f.git(t, "diff", "--cached", "--name-only"), "")
	equal(t, "git.Repo.RestoreIndex bytes", readFile(t, filepath.Join(f.Dir, ".git", "index")), readFile(t, backup))
}

// A commit whose tree a hook changed is undone and is not the commit_failed of a definite
// failure: build/commit.go leaves such a commit an uncertain boundary.
func TestCommitUndoesACommitAHookChanged(t *testing.T) {
	f, repo, tree, start := candidateFx(t)
	spec := git.CommitSpec{
		Tree: tree, Start: start, Title: "Add the app", Slug: "add-app",
		MessageFile: filepath.Join(t.TempDir(), "commit-message.txt"),
	}
	f.hook(t, "pre-commit", "echo hooked > hooked.txt\ngit add hooked.txt\n")
	s, _ := session(f.Dir)

	_, err := repo.Commit(s, spec)
	e := wantSysError(t, "git.Repo.Commit of a commit a hook changed", err, "candidate", "hook_changed_commit", 1)
	if !strings.Contains(e.Detail, "hooked.txt") {
		t.Errorf("git.Repo.Commit detail %q does not name the changed path", e.Detail)
	}
	equal(t, "git.Repo.Head after an undone commit", repo.Head(), start)
}

// build/build.go runs the user's commit-msg hook on the message the Build would commit before
// any model time is spent: no hook or an accepting hook passes (the hook sees the Intent's
// title and the trailer), a rejecting hook is environment/commit_rejected with its output.
func TestCheckCommitMsgRunsTheCommitMsgHook(t *testing.T) {
	var _ func(*git.Repo, *sys.Session, string, string) error = (*git.Repo).CheckCommitMsg

	f := newRepo(t)
	repo := git.New(f.Dir, os.Environ())
	s, _ := session(f.Dir)
	if err := repo.CheckCommitMsg(s, "A title", "slug"); err != nil {
		t.Fatalf("git.Repo.CheckCommitMsg without a hook: %v", err)
	}
	seen := filepath.Join(t.TempDir(), "message")
	f.hook(t, "commit-msg", fmt.Sprintf("cp \"$1\" '%s'\nexit 0\n", seen))
	if err := repo.CheckCommitMsg(s, "A title", "slug"); err != nil {
		t.Fatalf("git.Repo.CheckCommitMsg with an accepting hook: %v", err)
	}
	equal(t, "git.Repo.CheckCommitMsg: the message the hook saw", readFile(t, seen), "A title\n\nKogen-Intent: slug\n")

	f.hook(t, "commit-msg", "echo 'subject too vague'\nexit 9\n")
	err := repo.CheckCommitMsg(s, "A title", "slug")
	e := wantSysError(t, "git.Repo.CheckCommitMsg with a rejecting hook", err, "environment", "commit_rejected", 3)
	equal(t, "git.Repo.CheckCommitMsg detail", e.Detail,
		"the commit-msg hook rejects this Intent's commit message (status=9); change the Intent's title or the hook, then approve again")
	equalList(t, "git.Repo.CheckCommitMsg lines", e.Lines, []string{"subject too vague"})
	equal(t, "git.Repo.CheckCommitMsg commits nothing", f.log(t, "%s"), "baseline\n")
}

// build/build.go keeps a custody copy of the user's index before a Build (GIT-8); build/stop.go
// restores it; the custody copy is the bytes build/resume.go compares with the current index.
func TestIndexCustody(t *testing.T) {
	var _ func(*git.Repo) (string, error) = (*git.Repo).IndexPath
	var _ func(*git.Repo, string) error = (*git.Repo).SaveIndex
	var _ func(*git.Repo, string) error = (*git.Repo).RestoreIndex

	f := newRepo(t)
	repo := git.New(f.Dir, os.Environ())
	f.write(t, "user-staged.txt", "staged by the user\n")
	f.git(t, "add", "user-staged.txt")

	path, err := repo.IndexPath()
	check(t, err)
	equal(t, "git.Repo.IndexPath", path, filepath.Join(f.Dir, ".git", "index"))
	before := readFile(t, path)
	backup := filepath.Join(t.TempDir(), "index-at-start")
	check(t, repo.SaveIndex(backup))
	equal(t, "git.Repo.SaveIndex copies the index bytes", readFile(t, backup), before)

	f.git(t, "reset", "-q")
	equal(t, "the control: the index no longer stages the file", f.git(t, "diff", "--cached", "--name-only"), "")
	check(t, repo.RestoreIndex(backup))
	equal(t, "git.Repo.RestoreIndex bytes", readFile(t, path), before)
	equal(t, "git.Repo.RestoreIndex staging", f.git(t, "diff", "--cached", "--name-only"), "user-staged.txt\n")
	if err := repo.RestoreIndex(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Errorf("git.Repo.RestoreIndex of a missing custody copy succeeded")
	}
	equal(t, "git.Repo.RestoreIndex of a missing copy keeps the index", readFile(t, path), before)
	if err := repo.SaveIndex(filepath.Join(t.TempDir(), "no", "such", "dir", "backup")); err == nil {
		t.Errorf("git.Repo.SaveIndex into a missing folder succeeded")
	}
}

// build/resume.go and build/review_retry.go accept an index that differs from the saved one only
// by the stat cache a plain `git status` refreshes; any other staging, or damaged bytes, is a
// change.
func TestStagedEquivalentIgnoresOnlyTheStatCache(t *testing.T) {
	var _ func([]byte, []byte, string) bool = git.StagedEquivalent

	f := newRepo(t)
	repo := git.New(f.Dir, os.Environ())
	f.write(t, "user-staged.txt", "staged by the user\n")
	f.git(t, "add", "user-staged.txt")
	indexPath := filepath.Join(f.Dir, ".git", "index")
	saved := []byte(readFile(t, indexPath))
	format, err := repo.ObjectFormat()
	check(t, err)

	equal(t, "git.StagedEquivalent of equal bytes", git.StagedEquivalent(saved, saved, format), true)
	// A plain status refreshes the stat cache of an entry whose timestamps moved.
	later := f.mtime(t, "README.md").Add(3e9)
	check(t, os.Chtimes(filepath.Join(f.Dir, "README.md"), later, later))
	f.git(t, "status", "--porcelain")
	refreshed := []byte(readFile(t, indexPath))
	if bytes.Equal(refreshed, saved) {
		t.Fatalf("the control failed: a status did not refresh the index")
	}
	equal(t, "git.StagedEquivalent after a status refresh", git.StagedEquivalent(refreshed, saved, format), true)

	f.write(t, "README.md", "changed\n")
	f.git(t, "add", "README.md")
	restaged := []byte(readFile(t, indexPath))
	equal(t, "git.StagedEquivalent after another staging", git.StagedEquivalent(restaged, saved, format), false)
	equal(t, "git.StagedEquivalent of a truncated index", git.StagedEquivalent(saved[:len(saved)-1], saved, format), false)
	equal(t, "git.StagedEquivalent of an empty index", git.StagedEquivalent(nil, saved, format), false)
}

func (f *repoFx) mtime(t *testing.T, rel string) (mod time.Time) {
	t.Helper()
	st, err := os.Stat(filepath.Join(f.Dir, rel))
	check(t, err)
	return st.ModTime()
}

// build/review_retry.go puts the checkout back to the reviewed green tree when a reviewer's
// uncommitted edits must go: modified files are rewritten, deleted ones come back, added ones
// (and the folders that thereby became empty) are removed, the index and HEAD stay, and the
// Restored records say path and change from the checkout's point of view.
func TestRestoreTreePutsBackTheDifferingPaths(t *testing.T) {
	var _ func(*git.Repo, string, string) ([]git.Restored, error) = (*git.Repo).RestoreTree

	f := newRepo(t)
	f.write(t, "keep.txt", "kept\n")
	f.write(t, "dir/gone.txt", "gone\n")
	f.git(t, "add", ".")
	f.git(t, "commit", "-qm", "reviewed")
	repo := git.New(f.Dir, os.Environ())
	head := f.head(t)
	green, err := repo.Resolve("HEAD^{tree}")
	check(t, err)
	indexBefore := readFile(t, filepath.Join(f.Dir, ".git", "index"))

	f.write(t, "README.md", "edited\n")
	check(t, os.Remove(filepath.Join(f.Dir, "dir", "gone.txt")))
	f.write(t, "new/deep/added.txt", "added\n")
	current, err := repo.Tree(head, nil)
	check(t, err)
	equal(t, "the control: the edits changed the tree", current != green, true)

	got, err := repo.RestoreTree(current, green)
	check(t, err)
	changes := map[string]string{}
	for _, r := range got {
		var restored git.Restored = r
		var _ string = restored.Path
		var _ string = restored.Change
		changes[restored.Path] = restored.Change
	}
	want := map[string]string{"README.md": "modified", "dir/gone.txt": "deleted", "new/deep/added.txt": "added"}
	if len(changes) != len(want) {
		t.Fatalf("git.Repo.RestoreTree = %v, want %v", changes, want)
	}
	for path, change := range want {
		equal(t, "git.Restored.Change of "+path, changes[path], change)
	}
	after, err := repo.Tree(head, nil)
	check(t, err)
	equal(t, "git.Repo.RestoreTree: the checkout is the green tree", after, green)
	equal(t, "git.Repo.RestoreTree rewrites a modified file", readFile(t, filepath.Join(f.Dir, "README.md")), "readme\n")
	equal(t, "git.Repo.RestoreTree brings back a deleted file", readFile(t, filepath.Join(f.Dir, "dir", "gone.txt")), "gone\n")
	if _, err := os.Stat(filepath.Join(f.Dir, "new")); !os.IsNotExist(err) {
		t.Errorf("git.Repo.RestoreTree left the folders of the removed file: %v", err)
	}
	equal(t, "git.Repo.RestoreTree keeps HEAD", f.head(t), head)
	equal(t, "git.Repo.RestoreTree keeps the index", readFile(t, filepath.Join(f.Dir, ".git", "index")), indexBefore)
	none, err := repo.RestoreTree(green, green)
	check(t, err)
	equal(t, "git.Repo.RestoreTree of equal trees", len(none), 0)
}

// build/builder.go lists the files for the builder's brief, build/recover.go reads tree entries
// and blobs: Repo.OK returns the command's stdout and an error when the command fails.
func TestOKReturnsTheStdoutOfASuccessfulCommand(t *testing.T) {
	var _ func(*git.Repo, ...string) (string, error) = (*git.Repo).OK

	f := newRepo(t)
	f.write(t, ".gitignore", "ignored.log\n")
	f.write(t, "dir/tracked.txt", "tracked\n")
	f.git(t, "add", ".")
	f.git(t, "commit", "-qm", "files")
	f.write(t, "untracked.txt", "untracked\n")
	f.write(t, "ignored.log", "ignored\n")
	repo := git.New(f.Dir, os.Environ())

	files, err := repo.OK("ls-files", "--cached", "--others", "--exclude-standard")
	check(t, err)
	lines := strings.Split(strings.TrimSuffix(files, "\n"), "\n")
	slices.Sort(lines)
	equalList(t, "git.Repo.OK ls-files", lines, []string{".gitignore", "README.md", "dir/tracked.txt", "untracked.txt"})
	equal(t, "git.Repo.OK keeps the trailing newline", strings.HasSuffix(files, "\n"), true)

	out, err := repo.OK("ls-tree", "-z", "HEAD", "--", ":(literal)dir/tracked.txt")
	check(t, err)
	meta, path, _ := strings.Cut(out, "\t")
	equal(t, "git.Repo.OK ls-tree path", path, "dir/tracked.txt\x00")
	fields := strings.Split(meta, " ")
	if len(fields) != 3 || fields[0] != "100644" || fields[1] != "blob" {
		t.Fatalf("git.Repo.OK ls-tree entry = %q", meta)
	}
	blob, err := repo.OK("cat-file", "blob", fields[2])
	check(t, err)
	equal(t, "git.Repo.OK cat-file", blob, "tracked\n")

	if out, err := repo.OK("rev-parse", "--verify", "refs/heads/nope"); err == nil {
		t.Errorf("git.Repo.OK of a failing command = %q, want an error", out)
	}
}

// build/facts.go and build/build.go: one Build runs per repository. AcquireBuild takes the
// lock, a running owner is environment/build_running (exit 3) naming its pid, a lock left by a
// process that no longer runs is removed with a notice on stderr, and Release removes the file.
func TestBuildLock(t *testing.T) {
	var _ func(string, *sys.Session) (*git.Lock, error) = git.AcquireBuild
	var _ func(string) string = git.BuildLockPath
	var _ func(string) (string, string, bool) = git.LockOwner
	var _ func(*git.Lock) = (*git.Lock).Release

	root := t.TempDir()
	path := git.BuildLockPath(root)
	equal(t, "git.BuildLockPath", path, filepath.Join(root, ".kogen", "local", "build.lock"))
	if pid, token, held := git.LockOwner(path); pid != "" || token != "" || held {
		t.Errorf("git.LockOwner without a lock = %q %q %v, want nothing", pid, token, held)
	}

	s, stderr := session(root)
	lock, err := git.AcquireBuild(root, s)
	check(t, err)
	pid, token, held := git.LockOwner(path)
	equal(t, "git.LockOwner pid of the holder", pid, fmt.Sprint(os.Getpid()))
	equal(t, "git.LockOwner held", held, true)
	if token == "" {
		t.Errorf("git.LockOwner token of a held lock is empty")
	}
	equal(t, "git.AcquireBuild notice", stderr.String(), "")

	_, err = git.AcquireBuild(root, s)
	e := wantSysError(t, "git.AcquireBuild while a Build runs", err, "environment", "build_running", 3)
	equal(t, "git.AcquireBuild detail", e.Detail, "a Build is running in this repository (pid "+fmt.Sprint(os.Getpid())+")")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("a refused git.AcquireBuild removed the owner's lock: %v", err)
	}

	lock.Release()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("git.Lock.Release left the lock file: %v", err)
	}
	if pid, token, held := git.LockOwner(path); pid != "" || token != "" || held {
		t.Errorf("git.LockOwner after git.Lock.Release = %q %q %v, want nothing", pid, token, held)
	}
	lock.Release() // repeated release does nothing
	again, err := git.AcquireBuild(root, s)
	check(t, err)
	again.Release()
}

func TestBuildLockOfAProcessThatNoLongerRunsIsRemoved(t *testing.T) {
	root := t.TempDir()
	path := git.BuildLockPath(root)
	writeFile(t, root, ".kogen/local/build.lock", "987654321\n")
	if _, _, held := git.LockOwner(path); held {
		t.Fatalf("git.LockOwner reports a lock of a process that does not run as held")
	}
	s, stderr := session(root)
	lock, err := git.AcquireBuild(root, s)
	check(t, err)
	equal(t, "git.AcquireBuild notice for a stale lock", stderr.String(), "kogen: removed the lock of a Build that is no longer running\n")
	if pid, _, held := git.LockOwner(path); !held || pid != fmt.Sprint(os.Getpid()) {
		t.Errorf("git.LockOwner after taking over = %q %v, want this process holding it", pid, held)
	}
	lock.Release()
}
