package build

import (
	"os"
	"strings"
	"sync"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-loop/sys"
)

// seedIndexes keeps, for a start tree of a repository, the index file `git read-tree` made of it.
type seedIndexes struct {
	mu    sync.Mutex
	files map[string][]byte
}

var seeds seedIndexes

// seedIndexLimit bounds what seeds keeps: a Build works on one start tree, so a few suffice.
const seedIndexLimit = 32

// snapshotTree is git.Repo.Tree: the tree of start's files with every other file of the work
// tree not ignored, except the excluded paths, built in a private index so that the user's
// index is untouched. The index it starts from is the one `git read-tree start` writes; that
// index is a file that depends only on start, so the Build reads it once and writes a copy of
// it for each later snapshot, which saves one Git process in each of the many a Build takes.
func snapshotTree(repo *git.Repo, start string, excluded []string) (string, error) {
	if !objectID(start) {
		// A name can mean another tree later, and a bad one is refused by Tree with its error.
		return repo.Tree(start, excluded)
	}
	index := sys.TmpPath("kogen-index")
	defer os.Remove(index)
	env := []string{"GIT_INDEX_FILE=" + index}
	key := repo.Dir + "\x00" + start
	seeds.mu.Lock()
	seed, ok := seeds.files[key]
	seeds.mu.Unlock()
	if ok && os.WriteFile(index, seed, 0o600) == nil {
		return addAndWrite(repo, env, excluded)
	}
	if _, err := repo.OKEnv(env, "read-tree", start); err != nil {
		return "", err
	}
	if data, err := os.ReadFile(index); err == nil {
		seeds.mu.Lock()
		if seeds.files == nil || len(seeds.files) >= seedIndexLimit {
			seeds.files = map[string][]byte{}
		}
		seeds.files[key] = data
		seeds.mu.Unlock()
	}
	return addAndWrite(repo, env, excluded)
}

func addAndWrite(repo *git.Repo, env []string, excluded []string) (string, error) {
	add := []string{"add", "-A", "--", "."}
	for _, p := range excluded {
		add = append(add, ":(exclude,literal)"+p)
	}
	if _, err := repo.OKEnv(env, add...); err != nil {
		return "", err
	}
	out, err := repo.OKEnv(env, "write-tree")
	return strings.TrimSpace(out), err
}

// objectID reports whether s is a full object name, which names the same tree for good.
func objectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
