package build

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

func landedLines(out string) []string { return strings.Split(strings.TrimRight(out, "\n"), "\n") }

// indexOfLinePrefix returns the index of the first line with the prefix, or -1.
func indexOfLinePrefix(lines []string, prefix string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}

func landedExternalReview(t *testing.T, extra ...string) []string {
	t.Helper()
	e := newEnv(t)
	e.useLineup("")
	f := newLineupExec(t)
	f.review = func(n int, root string, in executor.Input, task executor.Task) (executor.RoundResult, error) {
		lineupWriteApp(t, root, "repaired\n")
		for _, p := range extra {
			writeFile(t, filepath.Join(root, p), "reviewer file\n", 0o644)
		}
		return executor.RoundResult{Structured: verdictJSON("approve")}, nil
	}
	a := e.build(withLineup(f))
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	return landedLines(a.out.String())
}

func TestLandedSummaryNamesExternalReviewRepair(t *testing.T) {
	t.Parallel()
	lines := landedLines(strings.Join(landedExternalReview(t), "\n"))
	i := indexOfLinePrefix(lines, "  review: approved")
	if i < 0 || lines[i+1] != "  review repaired: app.txt" || !strings.HasPrefix(lines[i+2], "  repairs: ") {
		t.Fatalf("review repaired line is not between review and repairs:\n%s", strings.Join(lines, "\n"))
	}
}

func TestLandedSummaryOmitsLineForBuilderRepair(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *project.Config) {
		c.Checks = []project.Check{{Name: "count", Run: "true"}}
	})
	e.prov.then(
		e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}),
		e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "green\n"}, "finish", map[string]any{"summary": "second"}),
	)
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	out := a.out.String()
	if !strings.Contains(out, "  repairs: 1 ") || strings.Contains(out, "review repaired") {
		t.Fatalf("summary:\n%s", out)
	}
}

func TestLandedSummaryCapsRepairedPaths(t *testing.T) {
	t.Parallel()
	lines := landedExternalReview(t, "r1.txt", "r2.txt", "r3.txt", "r4.txt", "r5.txt", "r6.txt")
	if indexOfLinePrefix(lines, "  review repaired: app.txt, r1.txt, r2.txt, r3.txt, r4.txt, and 2 more") < 0 {
		t.Fatalf("summary:\n%s", strings.Join(lines, "\n"))
	}
}

func TestLandedSummaryNamesNativeReviewRepair(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *project.Config) {
		c.Checks = []project.Check{{Name: "count", Run: "true"}}
	})
	f := e.prov
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "first"}),
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "green\n"}, "finish", map[string]any{"summary": "second"}),
	)
	f.then(f.calls(append(writeApp("reviewer repair\n"), "submit_review", map[string]any{"verdict": "approve", "findings": []any{}})...))
	a := e.reviewed()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	lines := landedLines(a.out.String())
	i := indexOfLinePrefix(lines, "  review: approved")
	if i < 0 || lines[i+1] != "  review repaired: app.txt" || !strings.HasPrefix(lines[i+2], "  repairs: 1 ") {
		t.Fatalf("summary:\n%s", a.out.String())
	}

	// A review interrupted and resumed before it approves still names the repaired paths.
	e = newEnv(t)
	e.prov.then(landScript(e.prov)...)
	session := json.RawMessage(`{"input":[{"role":"user","content":"review"}]}`)
	first := &scriptedReviewer{review: func(ctx context.Context, b *Attempt, pass verify.Pass) (Approved, error) {
		b.Review++
		b.LastReview = &ReviewRecord{N: b.Review, Verdict: "pending", Findings: []map[string]any{}}
		if err := b.Save(Continuation{Stage: StageReview, Pass: &pass, Review: session}); err != nil {
			return Approved{}, err
		}
		return Approved{}, sys.Fail("provider", "unavailable", "the reviewer's provider is down", 4)
	}}
	if a := e.build(func(in *Input) { in.Reviewer = first }); a.exit != 4 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	second := &scriptedReviewer{resume: func(ctx context.Context, b *Attempt) (Approved, error) {
		writeFile(t, filepath.Join(b.Root, "app.txt"), "resumed reviewer repair\n", 0o644)
		pass, err := b.Verify(ctx)
		if err != nil {
			return Approved{}, err
		}
		rec := ReviewRecord{N: b.Review, Verdict: "approve", Findings: []map[string]any{}, Tree: pass.Tree}
		b.LastReview = &rec
		return Approved{Pass: pass, Review: rec}, nil
	}}
	r := e.resume(e.onlyRun(), func(in *ResumeInput) { in.Reviewer = second })
	if r.exit != 0 || !strings.Contains(r.out.String(), "\n  review repaired: app.txt\n") {
		t.Fatalf("resume exit %d\n%s", r.exit, r.shown())
	}
}
