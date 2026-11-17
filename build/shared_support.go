package build

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// sharedSupportManifests are the dependency manifests and the test-environment configuration
// that count as shared test support when the project does not list its own.
var sharedSupportManifests = []string{
	"Gemfile", "Gemfile.lock", "package.json", "package-lock.json", "go.mod", "go.sum",
	"mix.exs", "mix.lock", "pyproject.toml", "requirements.txt", "config/environments/test.rb",
}

var requireLine = regexp.MustCompile(`^\s*(?:require_relative|require|load)\b[\s(]*(?:"([^"]*)"|'([^']*)')`)
var requireRelativeLine = regexp.MustCompile(`^\s*require_relative\b`)

// sharedSupportLine is the landed summary line naming the test support paths that the
// committed tree changed against the start commit, or "" when it changed none or the
// comparison is not possible.
func (b *Attempt) sharedSupportLine(committed string) string {
	if committed == "" {
		return ""
	}
	support, err := b.sharedSupportPaths()
	if err != nil || len(support) == 0 {
		return ""
	}
	changed, err := b.repo.Changed(b.Start, committed)
	if err != nil {
		return ""
	}
	var hit []string
	for _, p := range changed {
		if support[p] && !slices.Contains(hit, p) {
			hit = append(hit, p)
		}
	}
	if len(hit) == 0 {
		return ""
	}
	slices.Sort(hit)
	return "  shared test support changed: " + strings.Join(hit, ", ")
}

// sharedSupportPaths is the set of test support paths of the start commit: the paths the
// README names on a "test support" line, otherwise the files the unchanged tests require
// first plus the fixed manifests.
func (b *Attempt) sharedSupportPaths() (map[string]bool, error) {
	out, err := b.repo.OK("ls-tree", "-r", "--name-only", "-z", b.Start)
	if err != nil {
		return nil, err
	}
	tracked := map[string]bool{}
	for _, p := range strings.Split(out, "\x00") {
		if p = strings.Trim(p, "\r\n"); p != "" {
			tracked[p] = true
		}
	}
	if readme, ok := b.repo.Show(b.Start, "README.md"); ok {
		if named := readmeSupportPaths(readme, tracked); len(named) > 0 {
			return named, nil
		}
	}
	support := map[string]bool{}
	var queue []string
	show := func(p string) []string {
		content, ok := b.repo.Show(b.Start, p)
		if !ok {
			return nil
		}
		return strings.Split(content, "\n")
	}
	add := func(p string) {
		if !support[p] {
			support[p] = true
			queue = append(queue, p)
		}
	}
	for p := range tracked {
		if !isTestFile(p) {
			continue
		}
		for _, line := range show(p) {
			if m := requireLine.FindStringSubmatch(line); m != nil {
				if r := resolveRequire(p, line, m, tracked); r != "" {
					add(r)
				}
				break
			}
		}
	}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, line := range show(p) {
			if m := requireLine.FindStringSubmatch(line); m != nil {
				if r := resolveRequire(p, line, m, tracked); r != "" {
					add(r)
				}
			}
		}
	}
	for _, p := range sharedSupportManifests {
		if tracked[p] {
			support[p] = true
		}
	}
	return support, nil
}

func isTestFile(p string) bool {
	for _, dir := range []string{"test/", "tests/", "spec/"} {
		if strings.HasPrefix(p, dir) {
			return true
		}
	}
	base := path.Base(p)
	if strings.HasPrefix(base, "test_") {
		return true
	}
	stem := strings.TrimSuffix(base, path.Ext(base))
	return path.Ext(base) != "" && (strings.HasSuffix(stem, "_test") || strings.HasSuffix(stem, "_spec"))
}

// resolveRequire is the tracked start-commit file a require, require_relative or load line
// names, or "" when the string is not a tracked file of the repository.
func resolveRequire(from, line string, m []string, tracked map[string]bool) string {
	name := m[1]
	if name == "" {
		name = m[2]
	}
	if name == "" {
		return ""
	}
	bases := []string{"", "test", "spec", "lib"}
	if requireRelativeLine.MatchString(line) {
		bases = []string{path.Dir(from)}
	}
	for _, base := range bases {
		stem := path.Join(base, name)
		for _, ext := range []string{"", ".rb", ".exs", ".py", ".js"} {
			if tracked[stem+ext] {
				return stem + ext
			}
		}
	}
	return ""
}

// readmeSupportPaths are the tracked paths named, as exact paths, on any README line that
// contains "test support".
func readmeSupportPaths(readme string, tracked map[string]bool) map[string]bool {
	named := map[string]bool{}
	for _, line := range strings.Split(readme, "\n") {
		if !strings.Contains(strings.ToLower(line), "test support") {
			continue
		}
		for p := range tracked {
			if namesPath(line, p) {
				named[p] = true
			}
		}
	}
	return named
}

func namesPath(line, p string) bool {
	for from := 0; ; {
		i := strings.Index(line[from:], p)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(p)
		if (start == 0 || pathBoundary(line[start-1])) && (end == len(line) || pathBoundary(line[end])) {
			return true
		}
		from = start + 1
	}
}

func pathBoundary(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', '\v', '\f', '`', '"', '\'', '(', ')', ',', ':', ';':
		return true
	}
	return false
}
