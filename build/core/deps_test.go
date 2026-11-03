package core

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/KogenAI/kogen-build"

// TestDependencyRule is the dependency rule of the seam between build, build/review and
// build/core: tests included, build/review depends on build/core and itself among the packages
// of this module, and build/core on itself alone. Packages of other modules are free. Both
// packages must stay movable to a repository of their own, so a package of this module that
// either of them imports must not appear.
func TestDependencyRule(t *testing.T) {
	for _, tc := range []struct {
		pkg     string
		allowed []string
	}{
		{module + "/build/review", []string{module + "/build/core", module + "/build/review"}},
		{module + "/build/core", []string{module + "/build/core"}},
	} {
		t.Run(strings.TrimPrefix(tc.pkg, module+"/"), func(t *testing.T) {
			for _, problem := range forbiddenImports(t, tc.pkg, tc.allowed) {
				t.Error(problem)
			}
		})
	}
}

// forbiddenImports are the imports of packages of this module, by a package pkg depends on
// (tests included), that are outside allowed.
func forbiddenImports(t *testing.T, pkg string, allowed []string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", "-test", "-f", `{{.ImportPath}}{{"\t"}}{{join .Imports " "}}`, pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps -test %s: %v\n%s", pkg, err, out)
	}
	var problems []string
	for _, line := range strings.Split(string(out), "\n") {
		path, imports, _ := strings.Cut(line, "\t")
		importer := ownPackage(path)
		if importer != module && !strings.HasPrefix(importer, module+"/") {
			continue
		}
		for _, imported := range strings.Fields(imports) {
			imported = ownPackage(imported)
			if (imported == module || strings.HasPrefix(imported, module+"/")) && !slices.Contains(allowed, imported) {
				problem := importer + " imports " + imported + ", which the dependency rule forbids"
				if !slices.Contains(problems, problem) {
					problems = append(problems, problem)
				}
			}
		}
	}
	return problems
}

// ownPackage is the package a go list entry belongs to: its test variants (`p [p.test]`,
// `p.test` and `p_test`) count as p.
func ownPackage(path string) string {
	path, _, _ = strings.Cut(strings.TrimSpace(path), " [")
	return strings.TrimSuffix(strings.TrimSuffix(path, ".test"), "_test")
}
