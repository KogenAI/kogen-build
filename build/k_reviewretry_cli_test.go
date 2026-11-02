package build

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KogenAI/kogen-coordinator/project"
)

// The review-only resume is exercised through the real program: this builds kogen into an
// installation layout of a temporary prefix and runs `kogen intent build` and `kogen intent
// resume <build-id>` as processes against a temporary checkout and the scripted provider. Only
// processes the test starts exist, each waited for under its own deadline.

// checkoutSnapshot is everything a refused resume must leave exactly as it found it.
type checkoutSnapshot struct {
	files    map[string]string
	index    string
	head     string
	approval string
	runs     string
}

func (e *env) snapshot() checkoutSnapshot {
	e.t.Helper()
	c := checkoutSnapshot{files: map[string]string{}, head: e.head()}
	err := filepath.WalkDir(e.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(e.root, path)
		if d.IsDir() {
			if rel == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		c.files[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		e.t.Fatal(err)
	}
	idx, err := os.ReadFile(filepath.Join(e.root, ".git", "index"))
	if err != nil {
		e.t.Fatal(err)
	}
	sum := sha256.Sum256(idx)
	c.index = hex.EncodeToString(sum[:])
	rel, err := filepath.Rel(e.root, project.ApprovalPath(e.root, testSlug))
	if err != nil {
		e.t.Fatal(err)
	}
	c.approval = c.files[rel]
	ids := e.runIDs()
	sort.Strings(ids)
	c.runs = strings.Join(ids, ",")
	return c
}

// treeOfCandidate is the tree of the checkout's current candidate (start HEAD plus the files
// the Build left, excluding nothing for this fixture).
func treeOfCandidate(e *env) string {
	repo := e.gitRepo()
	tree, err := repo.Tree(e.gitOut("rev-parse", "HEAD"), nil)
	if err != nil {
		e.t.Fatal(err)
	}
	return tree
}
