package build

import (
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

// A failing check names the file a protected Intent text also names (the conflict between what
// the protected text demands and what the existing code needs), not the protected path itself.
// The claim on that Intent text is backed by the red pass, so the Build stops as build_blocked.
func TestABlockerOnAProtectedFileThatTheFailureReferencesIsBacked(t *testing.T) {
	t.Parallel()
	intent := project.IntentMDPath(testSlug)
	e := newEnv(t, failingCheck("app.txt"))
	f := e.prov
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "A\n"}, "finish", map[string]any{"summary": "implemented A"}),
		f.calls("report_blocker", blockerReport(intent)),
	)
	a := e.build()
	if a.exit != 1 {
		t.Fatalf("exit %d, want 1 (build_blocked)\n%s", a.exit, a.shown())
	}
	contains(t, "stdout", a.out.String(), "build_blocked")
	for _, o := range f.toolOutputs() {
		if strings.HasPrefix(o, "blocker not backed") {
			t.Fatalf("blocker refused: %s", o)
		}
	}
	eq(t, "outcome", e.result(e.onlyRun())["outcome"], any("build_blocked"))
}

func TestBlockerBackingByReferenceNeedsAFailureNamingAFileTheProtectedTextNames(t *testing.T) {
	t.Parallel()
	text := map[string]string{
		"intent.md": "docs must not contain testing-rules.md after the change; see .kogen/intents/x/acceptance.feature",
		"other.md":  "nothing about files here",
	}
	read := func(rel string) string { return text[rel] }
	red := func(out string) *verify.Pass {
		return &verify.Pass{Checks: []verify.CommandResult{{Name: "go-test", Command: "go test", Status: 1, Output: out}}}
	}
	out := "--- FAIL: TestRules\n    testing_rules_test.go:13: open ../docs/testing-rules.md: no such file or directory\n"
	for _, c := range []struct {
		name   string
		pass   *verify.Pass
		path   string
		backed bool
	}{
		{"protected text names the failing file", red(out), "intent.md", true},
		{"protected text names nothing the failure names", red(out), "other.md", false},
		{"kogen's own file is no link", red("see .kogen/intents/x/acceptance.feature:3"), "intent.md", false},
		{"no pass", nil, "intent.md", false},
		{"green pass", &verify.Pass{Green: true, Checks: []verify.CommandResult{{Output: out}}}, "intent.md", false},
		{"unreadable protected file", red(out), "missing.md", false},
	} {
		eq(t, c.name, blockerReferenced(testSlug, c.pass, []string{c.path}, read), c.backed)
	}
}
