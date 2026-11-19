package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

func shadowChecks(t *testing.T) (*env, string) {
	t.Helper()
	return reuseEnv(t,
		project.Check{Name: "a", Run: `echo a >> "$KBO_LOG"`},
		project.Check{Name: "b", Run: `echo b >> "$KBO_LOG"`})
}

func reuseShadowEventsOf(e *env, id, name string) []map[string]any {
	return eventsNamed(e.events(id), name)
}

func reuseShadowEventFor(t *testing.T, e *env, id, check string) map[string]any {
	t.Helper()
	for _, ev := range reuseShadowEventsOf(e, id, "reuse_shadow") {
		if ev["name"] == check {
			return ev
		}
	}
	t.Fatalf("no reuse_shadow event for %s", check)
	return nil
}

func shadowReceipts(t *testing.T, e *env) []reuseReceipt {
	t.Helper()
	r, err := readReuseReceipts(filepath.Join(e.root, ".git", reuseReceiptFile))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The baseline of two checks takes one Verification snapshot, not one per check, and every
// check of it still runs.
func TestReuseShadowTakesOneSnapshotPerBaseline(t *testing.T) {
	calls := 0
	old := newReuseSnapshot
	newReuseSnapshot = func(s *sys.Session, tools verify.Tools, tree, dir string) verify.ReuseSnapshot {
		calls++
		return old(s, tools, tree, dir)
	}
	t.Cleanup(func() { newReuseSnapshot = old })

	e, log := shadowChecks(t)
	e.prov.then(e.prov.calls("finish", map[string]any{"summary": "done"}))
	e.build()
	id := e.onlyRun()
	if calls != 1 {
		t.Fatalf("Verification snapshots computed = %d, want 1 for a baseline of two checks", calls)
	}
	if got := len(reuseShadowEventsOf(e, id, "reuse_shadow")); got != 2 {
		t.Fatalf("reuse_shadow events = %d, want one per check", got)
	}
	if got := executionsOf(t, log); len(got) < 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("the baseline ran %v, want both checks in order", got)
	}
}

// A first Build's receipts carry the documented fields, a baseline receipt per check and a
// pass receipt for each check the pass ran; the event of a check without receipt agrees.
func TestReuseShadowReceiptsAndEvent(t *testing.T) {
	e, _ := shadowChecks(t)
	e.prov.then(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}, "finish", map[string]any{"summary": "done"}))
	e.build()
	id := e.onlyRun()
	ev := reuseShadowEventFor(t, e, id, "a")
	eq(t, "would_reuse", ev["would_reuse"], any(false))
	eq(t, "agrees", ev["agrees"], any(true))
	eq(t, "receipt", ev["receipt"], any(""))
	if !hex64.MatchString(ev["reuse_sha256"].(string)) {
		t.Fatalf("reuse_sha256 %v", ev["reuse_sha256"])
	}
	sources := map[string]int{}
	for _, r := range shadowReceipts(t, e) {
		sources[r.Source]++
		if !hex64.MatchString(r.Key) || r.Status != 0 || r.Log != e.runPath(id, "log.jsonl") || (r.Name != "a" && r.Name != "b") {
			t.Fatalf("receipt %+v", r)
		}
	}
	eq(t, "receipts by source", sources, map[string]int{"baseline": 2, "pass": 2})
	raw, _ := os.ReadFile(filepath.Join(e.root, ".git", reuseReceiptFile))
	for _, field := range []string{`"key":`, `"name":`, `"status":`, `"wall_ms":`, `"source":`, `"log":`} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("receipt file lacks %s: %s", field, raw)
		}
	}
	// The pass receipt of a check answers the baseline of the next Build on the landed tree.
	e.prov.then(e.prov.calls("finish", map[string]any{"summary": "done"}))
	e.approve()
	e.build()
	for _, next := range e.runIDs() {
		if next == id {
			continue
		}
		for _, c := range []string{"a", "b"} {
			ev := reuseShadowEventFor(t, e, next, c)
			eq(t, "would_reuse "+c, ev["would_reuse"], any(true))
			eq(t, "receipt "+c, ev["receipt"], any(e.runPath(id, "log.jsonl")))
		}
	}
	eq(t, "runs", len(e.runIDs()), 2)
}

// A would-reuse hit whose check fails is printed once to stderr and logged as disagreeing, and
// the Build still ends on the real failing verdict.
func TestReuseShadowDisagreementIsPrinted(t *testing.T) {
	e, _ := reuseEnv(t, project.Check{Name: "a", Run: `test "$KOGEN_ATTEMPT_ID" != second`})
	e.extraEnv = append(e.extraEnv, "KOGEN_ATTEMPT_ID=first")
	e.prov.then(e.prov.calls("finish", map[string]any{"summary": "done"}), refused())
	e.build()
	e.extraEnv[len(e.extraEnv)-1] = "KOGEN_ATTEMPT_ID=second"
	e.prov.then(e.prov.calls("finish", map[string]any{"summary": "done"}), refused())
	e.approve()
	a := e.build()
	var second string
	for _, id := range e.runIDs() {
		if ev := reuseShadowEventsOf(e, id, "reuse_shadow"); len(ev) == 1 && ev[0]["would_reuse"] == true {
			second = id
		}
	}
	if second == "" {
		t.Fatal("no run found whose baseline would have reused the check")
	}
	eq(t, "agrees", reuseShadowEventFor(t, e, second, "a")["agrees"], any(false))
	if n := strings.Count(a.errs.String(), "reuse shadow"); n != 1 {
		t.Fatalf("stderr names the disagreement %d times, want once:\n%s", n, a.errs.String())
	}
	eq(t, "baseline failing", reuseShadowEventsOf(e, second, "baseline_checks")[0]["failing"], any([]any{"a"}))
}

// The shadow reports itself unavailable once per run log, however often it would fail.
func TestReuseShadowUnavailableOncePerRunLog(t *testing.T) {
	e, _ := shadowChecks(t)
	if err := os.Mkdir(filepath.Join(e.root, ".git", reuseReceiptFile), 0o755); err != nil {
		t.Fatal(err)
	}
	e.prov.then(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}, "finish", map[string]any{"summary": "done"}))
	e.build()
	id := e.onlyRun()
	eq(t, "outcome", e.result(id)["outcome"], any("landed"))
	un := reuseShadowEventsOf(e, id, "reuse_shadow_unavailable")
	if len(un) != 1 || strings.TrimSpace(fmt.Sprint(un[0]["reason"])) == "" {
		t.Fatalf("reuse_shadow_unavailable events: %v", un)
	}
	eq(t, "reuse_shadow events", len(reuseShadowEventsOf(e, id, "reuse_shadow")), 0)
}

// The receipt file keeps its newest 2000 lines.
func TestReuseReceiptFileIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), reuseReceiptFile)
	for i := 0; i < reuseReceiptBound+5; i++ {
		if err := appendReuseReceipts(path, reuseReceipt{Key: fmt.Sprintf("k%d", i), Name: "a", Source: reuseSourceBase, Log: "log"}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := readReuseReceipts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != reuseReceiptBound || got[0].Key != "k5" || got[len(got)-1].Key != fmt.Sprintf("k%d", reuseReceiptBound+4) {
		t.Fatalf("%d receipts from %s to %s, want the newest %d", len(got), got[0].Key, got[len(got)-1].Key, reuseReceiptBound)
	}
	if err := appendReuseReceipts(path, reuseReceipt{Key: "x1"}, reuseReceipt{Key: "x2"}); err != nil {
		t.Fatal(err)
	}
	got, _ = readReuseReceipts(path)
	if len(got) != reuseReceiptBound || got[len(got)-1].Key != "x2" || got[len(got)-2].Key != "x1" {
		t.Fatalf("after a two-receipt append: %d receipts ending %v", len(got), got[len(got)-2:])
	}
}
