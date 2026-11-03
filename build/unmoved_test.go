package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-verification/verify"
)

func TestUnmovedMajority(t *testing.T) {
	for _, tc := range []struct {
		texts []string
		want  string
	}{
		{[]string{"x"}, "x"}, {[]string{"", ""}, ""}, {[]string{"a", "b"}, ""},
		{[]string{"a", "a", "b", "b"}, ""}, {[]string{"a", "a", "b"}, "a"},
		{[]string{"a", "a", "b", "b", "a"}, "a"}, {[]string{"a", "b", "c"}, ""},
	} {
		if got := commonFailure(tc.texts); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.texts, got, tc.want)
		}
	}
}

func TestUnmovedSequence(t *testing.T) {
	e := newEnv(t)
	b := &Attempt{repo: e.gitRepo()}
	tree := func(path, content string) string {
		e.write(path, content)
		e.run("git", "add", "-A")
		return e.gitOut("write-tree")
	}
	a := tree("a.txt", "one")
	c := tree("b.txt", "two")
	d := tree("c.txt", "three")
	detail := "acceptance.feature line 3: Then x\nsame"
	pass := verify.Pass{Tree: a, Failing: []string{"acceptance"}, Acceptance: &verify.Acceptance{Scenarios: []verify.ScenarioResult{{Name: "one", Line: 2, Status: "failed", Detail: &detail}, {Name: "two", Line: 4, Status: "failed", Detail: &detail}}}}
	if n := b.unmovedNotice("s", pass); n != "" {
		t.Fatal(n)
	}
	pass.Tree = c
	if n := b.unmovedNotice("s", pass); !strings.Contains(n, "2 consecutive red passes. Last round changed: b.txt.") || !strings.Contains(n, "Common failure text (address first): same") {
		t.Fatal(n)
	}
	pass.Tree = d
	pass.Acceptance.Scenarios[0].Status = "passed"
	if n := b.unmovedNotice("s", pass); n != "" {
		t.Fatal(n)
	}
	pass.Tree = c
	if n := b.unmovedNotice("s", pass); !strings.Contains(n, "2 consecutive") {
		t.Fatal(n)
	}
	pass.Tree = d
	if n := b.unmovedNotice("other", pass); n != "" {
		t.Fatal(n)
	}
	pass.Green = true
	b.unmovedNotice("other", pass)
	pass.Green = false
	pass.Tree = c
	if n := b.unmovedNotice("other", pass); n != "" {
		t.Fatal(n)
	}
	if n := b.unmovedNotice("other", pass); n != "" {
		t.Fatal(n)
	}
}

func TestUnmovedScopedFeedback(t *testing.T) {
	e, _ := stagedEnv(t, []string{"echo scoped-failure; exit 1"})
	for i := 1; i <= 3; i++ {
		e.prov.then(e.prov.calls("write_file", map[string]any{"path": "unrelated.txt", "content": fmt.Sprint(i)}, "finish", map[string]any{"summary": "verify"}))
	}
	e.prov.then(refused())
	e.build()
	req := e.prov.requests()
	if len(req) < 3 {
		t.Fatalf("requests: %d", len(req))
	}
	first, second := lastInput(req[1]), lastInput(req[2])
	if strings.Contains(first, "Failing set did not move") || !strings.HasPrefix(second, "Failing set did not move for 2 consecutive red passes. Last round changed: unrelated.txt.") || !strings.Contains(second, "Stage foundation failed; remain at this stage.") {
		t.Fatalf("first %s\nsecond %s", first, second)
	}
}

func TestUnmovedExternalFeedback(t *testing.T) {
	e := newEnv(t)
	e.useExecutor("")
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("red\n"), 0644); err != nil {
			return executor.RoundResult{}, err
		}
		if err := os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte(fmt.Sprint(n)), 0644); err != nil {
			return executor.RoundResult{}, err
		}
		return executor.RoundResult{Ending: executor.EndFinish, Claim: "verify"}, nil
	})
	f.desc.Capabilities.MaxRounds = 3
	e.build(withExecutor(f, nil))
	seen := f.seen()
	if len(seen) != 3 {
		t.Fatalf("inputs %d", len(seen))
	}
	if strings.Contains(seen[1].Feedback, "Failing set did not move") || !strings.HasPrefix(seen[2].Feedback, "Failing set did not move for 2 consecutive red passes. Last round changed: unrelated.txt.") {
		t.Fatalf("inputs %+v", seen)
	}
}

func TestUnmovedPathsAndIdentity(t *testing.T) {
	e := newEnv(t)
	b := &Attempt{repo: e.gitRepo()}
	e.write("z.txt", "old")
	e.run("git", "add", "-A")
	first := e.gitOut("write-tree")
	e.run("rm", "z.txt")
	e.write("b.txt", "new")
	e.write("a.txt", "new")
	e.run("git", "add", "-A")
	second := e.gitOut("write-tree")
	p := verify.Pass{Tree: first, Failing: []string{"lint", "test"}, Checks: []verify.CommandResult{{Name: "lint", Status: 1, Output: "old"}, {Name: "test", Status: 1, Output: "old"}}}
	b.unmovedNotice("", p)
	p.Tree = second
	p.Failing = []string{"test", "lint"}
	p.Checks[0].Output = "new"
	p.Checks[1].Output = "new"
	n := b.unmovedNotice("", p)
	if !strings.Contains(n, "Last round changed: a.txt, b.txt, z.txt.") || !strings.Contains(n, "Common failure text (address first): new") {
		t.Fatal(n)
	}
}
