package build

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// fileIndexBytes is the fixed bound of the builder brief's file index, counted from its
// "# Files" heading to the end of Dynamic's index section. It is also the threshold below
// which the whole listing is shown verbatim. It is a constant, not a setting.
const fileIndexBytes = 65536

const fileIndexNotice = "The omitted paths still exist in the repository: they remain readable and searchable " +
	"with the existing file, read and search tools."

// fileIndex is the index section of the builder brief's Dynamic part for the output of
// `git ls-files --cached --others --exclude-standard`. A listing that fits fileIndexBytes is
// shown verbatim; a larger one becomes a deterministic bounded index: the true path counts,
// every required path (the protected baseline and the Intent's acceptance paths) listed
// individually, a breadth-first selection of other paths across directories, and the omitted
// paths counted per disjoint directory. The result depends only on the path set.
func fileIndex(listing string, required []string) string {
	if full := "# Files\n\n" + listing; len(full) <= fileIndexBytes {
		return full
	}
	paths := strings.Split(strings.TrimRight(listing, "\n"), "\n")
	sort.Strings(paths)
	paths = slices.Compact(paths)

	must := map[string]bool{}
	for _, p := range required {
		if _, found := slices.BinarySearch(paths, p); found {
			must[p] = true
		}
	}
	// The other paths, by rank inside their directory first so that every directory is
	// represented before any is listed twice, then by directory and path.
	type cand struct {
		rank     int
		dir, key string
	}
	var cands []cand
	seen := map[string]int{}
	for _, p := range paths {
		if must[p] {
			continue
		}
		d := path.Dir(p)
		cands = append(cands, cand{seen[d], d, p})
		seen[d]++
	}
	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.dir != b.dir {
			return a.dir < b.dir
		}
		return a.key < b.key
	})

	render := func(extra, depth int) (string, int) {
		listed := map[string]bool{}
		for p := range must {
			listed[p] = true
		}
		for _, c := range cands[:extra] {
			listed[c.key] = true
		}
		var shown []string
		root := &omitNode{}
		for _, p := range paths {
			if listed[p] {
				shown = append(shown, p)
			} else {
				root.add(path.Dir(p))
			}
		}
		var b strings.Builder
		b.WriteString("# Files\n\n")
		fmt.Fprintf(&b, "total paths: %d\nlisted paths: %d\nomitted paths: %d\n\n%s\n\n", len(paths), len(shown), len(paths)-len(shown), fileIndexNotice)
		b.WriteString("## Listed paths\n" + strings.Join(shown, "\n") + "\n")
		if root.total > 0 {
			b.WriteString("\n## Omitted directories\n")
			if depth == 0 {
				// The coarsest summary: the repository root covers every omitted path.
				b.WriteString("./: " + strconv.Itoa(root.total) + " omitted paths\n")
			} else {
				root.emit(&b, "", 0, depth)
			}
		}
		return b.String(), len(shown)
	}

	// The most breadth that leaves the directory summary of the given depth within the bound,
	// using at most three quarters of it for the listed paths.
	breadth := func(depth int) int {
		fits := func(extra int) bool {
			out, _ := render(extra, depth)
			listedEnd := strings.Index(out, "\n## Omitted directories\n")
			if listedEnd < 0 {
				listedEnd = len(out)
			}
			return len(out) <= fileIndexBytes && (extra == 0 || listedEnd <= fileIndexBytes/4*3)
		}
		lo, hi := 0, len(cands)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if fits(mid) {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		return lo
	}
	lo := breadth(1)
	best, _ := render(lo, 1)
	if len(best) > fileIndexBytes {
		// Even the top-level directories do not fit (a very wide repository): the root
		// alone summarizes the omitted paths.
		lo = breadth(0)
		best, _ = render(lo, 0)
		return best
	}
	// The finest directory summary that still fits.
	for depth := 2; ; depth++ {
		out, _ := render(lo, depth)
		if len(out) > fileIndexBytes || out == best {
			break
		}
		best = out
	}
	return best
}

// omitNode is a directory of the omitted paths: the count of omitted paths directly in it and
// below it.
type omitNode struct {
	children map[string]*omitNode
	direct   int
	total    int
}

// add counts one omitted path whose directory is dir ("." for the repository root).
func (n *omitNode) add(dir string) {
	n.total++
	if dir == "." {
		n.direct++
		return
	}
	for _, part := range strings.Split(dir, "/") {
		if n.children == nil {
			n.children = map[string]*omitNode{}
		}
		c := n.children[part]
		if c == nil {
			c = &omitNode{}
			n.children[part] = c
		}
		c.total++
		n = c
	}
	n.direct++
}

// emit writes the disjoint directory lines: a directory with omitted paths directly in it, or
// at the given depth, covers everything below it; the repository root's own files are "./".
func (n *omitNode) emit(b *strings.Builder, dir string, level, depth int) {
	if level == 0 {
		if n.direct > 0 {
			b.WriteString("./: " + strconv.Itoa(n.direct) + " omitted paths\n")
		}
	} else if n.direct > 0 || level >= depth {
		b.WriteString(dir + "/: " + strconv.Itoa(n.total) + " omitted paths\n")
		return
	}
	names := make([]string, 0, len(n.children))
	for name := range n.children {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		child := name
		if dir != "" {
			child = dir + "/" + name
		}
		n.children[name].emit(b, child, level+1, depth)
	}
}
