package build

import (
	"strings"
	"testing"
)

// supportRun commits the baseline files, runs one scripted native Build writing the given
// files, and returns the landed summary and the first request the provider received.
func supportRun(t *testing.T, readme string, committed, written map[string]string) (string, string) {
	t.Helper()
	e := newEnv(t)
	e.write("README.md", readme)
	add := []string{"README.md"}
	for p, c := range committed {
		e.write(p, c)
		add = append(add, p)
	}
	e.run("git", append([]string{"add"}, add...)...)
	e.run("git", "commit", "-q", "--allow-empty", "-m", "support baseline")
	e.approve()
	var pairs []any
	for p, c := range written {
		pairs = append(pairs, "write_file", map[string]any{"path": p, "content": c})
	}
	pairs = append(pairs, "finish", map[string]any{"summary": "done"})
	e.prov.then(e.prov.calls(pairs...))
	a := e.build()
	if a.exit != 0 || a.err != nil {
		t.Fatalf("exit %d err %v\n%s\n%s", a.exit, a.err, a.out, a.errs)
	}
	raw := e.prov.rawRequests()
	if len(raw) == 0 {
		t.Fatal("the provider received no request")
	}
	return a.out.String(), string(raw[0])
}

func supportLines(out string) []string {
	var found []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "shared test support changed:") {
			found = append(found, l)
		}
	}
	return found
}

func TestBriefStatesSharedTestSupportRule(t *testing.T) {
	_, request := supportRun(t, "# P\n", nil, map[string]string{"app.txt": "x\n"})
	for _, want := range []string{"Shared test support is not edited unless the Intent names it", "beside your own new test files", "byte-identical"} {
		if !strings.Contains(request, want) {
			t.Errorf("the brief lacks %q", want)
		}
	}
}

func TestSummaryNamesChangedSharedTestSupport(t *testing.T) {
	readme := "# P\n\nShared test support: (test/support/a.rb), test/support/b.rb;\n"
	out, _ := supportRun(t, readme,
		map[string]string{"test/support/a.rb": "1\n", "test/support/b.rb": "1\n", "test/support/c.rb": "1\n", "Gemfile": "1\n"},
		map[string]string{"test/support/b.rb": "2\n", "test/support/a.rb": "2\n", "test/support/c.rb": "2\n", "Gemfile": "2\n", "app.txt": "x\n"})
	got := supportLines(out)
	if len(got) != 1 || got[0] != "  shared test support changed: test/support/a.rb, test/support/b.rb" {
		t.Fatalf("lines = %q\n%s", got, out)
	}
}

func TestSummaryFallsBackToRequiredFirstFilesAndManifests(t *testing.T) {
	out, _ := supportRun(t, "# P\n",
		map[string]string{
			"test/a_test.rb":      "# c\nrequire \"test_helper\"\nrequire_relative \"other\"\n",
			"test/test_helper.rb": "require_relative 'support/s'\nrequire 'minitest/autorun'\n",
			"test/support/s.rb":   "require_relative \"../lib_x\"\n",
			"test/lib_x.rb":       "1\n",
			"test/other.rb":       "1\n",
			"go.mod":              "module x\n",
		},
		map[string]string{
			"test/a_test.rb":      "changed\n",
			"test/test_helper.rb": "changed\n",
			"test/support/s.rb":   "changed\n",
			"test/lib_x.rb":       "changed\n",
			"test/other.rb":       "changed\n",
			"go.mod":              "module y\n",
		})
	got := supportLines(out)
	want := "  shared test support changed: go.mod, test/lib_x.rb, test/support/s.rb, test/test_helper.rb"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("lines = %q\n%s", got, out)
	}
}

func TestSummaryHasNoSharedSupportLineForNewFiles(t *testing.T) {
	readme := "# P\n\n`test/support/a.rb` is test support.\n"
	out, _ := supportRun(t, readme, map[string]string{"test/support/a.rb": "1\n"},
		map[string]string{"test/new_test.rb": "1\n", "test/support/new.rb": "1\n", "app.txt": "x\n"})
	if got := supportLines(out); len(got) != 0 {
		t.Fatalf("unexpected lines %q", got)
	}
}
