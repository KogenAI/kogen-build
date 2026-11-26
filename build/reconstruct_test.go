package build

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-loop/sys"
)

type reconFixture struct {
	t        *testing.T
	checkout string
	evidence string
	archive  string
	ev       map[string]any
	receipts []string
}

func newReconFixture(t *testing.T) *reconFixture {
	t.Helper()
	dir := t.TempDir()
	f := &reconFixture{t: t, checkout: filepath.Join(dir, "checkout"), evidence: filepath.Join(dir, "evidence.json"), archive: filepath.Join(dir, "archive")}
	if err := os.Mkdir(f.checkout, 0o700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", f.checkout, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("commit", "-q", "--allow-empty", "-m", "baseline")
	builds := []any{}
	for i, id := range []string{"b97d96270b9f4801aa09a7ba00e64f84", "c3b576c985bb4299b1b443666eed172a"} {
		p := filepath.Join(dir, id+".json")
		body := []byte(`{"receipt":` + string(rune('1'+i)) + "}\n")
		if err := os.WriteFile(p, body, 0o600); err != nil {
			t.Fatal(err)
		}
		f.receipts = append(f.receipts, p)
		builds = append(builds, map[string]any{"id": id, "receipt_sha256": strings.Repeat("a", 63) + string(rune('0'+i)), "candidate_tree": strings.Repeat("b", 39) + string(rune('0'+i)), "path": p, "copy_sha256": sha(body)})
	}
	f.ev = map[string]any{
		"queue_item": map[string]any{"id": problem20Item, "version": problem20Version}, "obligation": problem20Obligation, "owner": "fleet",
		"request":         map[string]any{"document": problem20Document, "revision": problem20Revision, "sha256": problem20RequestSHA},
		"current_request": map[string]any{"revision": problem20Revision, "sha256": problem20RequestSHA},
		"baseline":        map[string]any{"head": git("rev-parse", "HEAD")},
		"prior":           map[string]any{"attempts": []any{"577e7c601aab41a39eb0820fc5fee5fb"}, "builds": builds, "spend": map[string]any{"units": 42}},
		"effects":         map[string]any{"publication": "none", "external": "none"},
		"owners":          []any{}, "custody": map[string]any{"generation": "gen-1", "revoked": true}, "claims": map[string]any{},
	}
	f.write()
	return f
}

func (f *reconFixture) write() {
	f.t.Helper()
	b, err := json.Marshal(f.ev)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(f.evidence, b, 0o600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *reconFixture) run(stopAfter string) (Reconstruction, int) {
	f.t.Helper()
	r, code, _ := Reconstruct(sys.New(), ReconstructInput{Project: f.checkout, Evidence: f.evidence, Archive: f.archive, StopAfter: stopAfter})
	return r, code
}

func (f *reconFixture) records() int {
	entries, _ := os.ReadDir(filepath.Join(f.checkout, reconstructionsDir))
	return len(entries)
}

func TestReconstructCreatesOneBoundAttemptAndReplaysUnchanged(t *testing.T) {
	f := newReconFixture(t)
	r, code := f.run("")
	if code != 0 || r.State != reconCreated || len(r.Attempts) != 1 || len(r.Wakes) != 1 {
		t.Fatalf("created run: exit %d %+v", code, r)
	}
	a := r.Attempts[0]
	if a.RequestRevision != problem20Revision || a.RequestSHA256 != problem20RequestSHA || a.BaselineHead != f.ev["baseline"].(map[string]any)["head"] || a.Generation == "gen-1" || r.Wakes[0].Attempt != a.ID {
		t.Fatalf("attempt: %+v wake %+v", a, r.Wakes[0])
	}
	f.archive = filepath.Join(filepath.Dir(f.archive), "other")
	again, code := f.run("")
	if code != 0 || again.Attempts[0] != a || again.Wakes[0] != r.Wakes[0] || again.Archive != r.Archive {
		t.Fatalf("replay changed: exit %d %+v", code, again)
	}
	if _, err := os.Stat(f.archive); err == nil {
		t.Fatal("a replay created another archive")
	}
	back, code, _ := ReadReconstruction(f.checkout, problem20Item, nil)
	if code != 0 || back.Attempts[0] != a {
		t.Fatalf("readback: %d %+v", code, back)
	}
}

func TestReconstructHoldsWriteNothing(t *testing.T) {
	cases := map[string]func(f *reconFixture){
		"effects missing":     func(f *reconFixture) { delete(f.ev, "effects") },
		"publication unknown": func(f *reconFixture) { f.ev["effects"] = map[string]any{"publication": "unknown", "external": "none"} },
		"external published": func(f *reconFixture) {
			f.ev["effects"] = map[string]any{"publication": "none", "external": "published"}
		},
		"owners missing":    func(f *reconFixture) { delete(f.ev, "owners") },
		"owner live":        func(f *reconFixture) { f.ev["owners"] = []any{map[string]any{"name": "worker-7"}} },
		"custody unrevoked": func(f *reconFixture) { f.ev["custody"] = map[string]any{"generation": "gen-1", "revoked": false} },
		"custody missing":   func(f *reconFixture) { delete(f.ev, "custody") },
		"revision changed": func(f *reconFixture) {
			f.ev["current_request"] = map[string]any{"revision": 4, "sha256": problem20RequestSHA}
		},
		"baseline moved":   func(f *reconFixture) { f.ev["baseline"] = map[string]any{"head": strings.Repeat("1", 40)} },
		"spend missing":    func(f *reconFixture) { delete(f.ev["prior"].(map[string]any), "spend") },
		"claimed landing":  func(f *reconFixture) { f.ev["claims"] = map[string]any{"landed": true} },
		"reused gate":      func(f *reconFixture) { f.ev["claims"] = map[string]any{"reused_gates": []any{"review"}} },
		"other item":       func(f *reconFixture) { f.ev["queue_item"] = map[string]any{"id": "other", "version": 1} },
		"other obligation": func(f *reconFixture) { f.ev["obligation"] = "Problem21" },
		"receipt changed":  func(f *reconFixture) { f.receiptEdit() },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newReconFixture(t)
			mutate(f)
			f.write()
			r, code := f.run("")
			if code != 1 || r.State != reconHeld || r.Reason == "" || r.Owner == "" || r.NextAction == "" || len(r.Attempts) != 0 || len(r.Wakes) != 0 {
				t.Fatalf("not held: exit %d %+v", code, r)
			}
			if name == "owner live" && r.Owner != "worker-7" {
				t.Fatalf("owner %q", r.Owner)
			}
			if name == "reused gate" || name == "claimed landing" {
				if r.Owner != "fleet" {
					t.Fatalf("owner %q", r.Owner)
				}
			}
			if _, err := os.Stat(f.archive); err == nil || f.records() != 0 {
				t.Fatalf("a hold wrote an archive or record (records %d)", f.records())
			}
			if _, err := os.Stat(filepath.Join(f.checkout, ".kogen")); err == nil {
				t.Fatal("a hold wrote under .kogen")
			}
		})
	}
}

func (f *reconFixture) receiptEdit() {
	fh, err := os.OpenFile(f.receipts[1], os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		f.t.Fatal(err)
	}
	fh.WriteString("tampered")
	fh.Close()
}

func TestReconstructResumesFromTheLastDurablePhase(t *testing.T) {
	for _, phase := range []string{"preserved", "fenced"} {
		t.Run(phase, func(t *testing.T) {
			f := newReconFixture(t)
			r, code := f.run(phase)
			if code != 1 || r.State != reconInterrupted || len(r.Attempts) != 0 || len(r.Wakes) != 0 || r.Archive != f.archive {
				t.Fatalf("interrupted: %d %+v", code, r)
			}
			if (phase == "fenced") != (r.Generation != nil) {
				t.Fatalf("generation after %s: %+v", phase, r.Generation)
			}
			manifest, _ := os.ReadFile(filepath.Join(f.archive, "manifest.json"))
			back, code, _ := ReadReconstruction(f.checkout, problem20Item, nil)
			if code != 1 || back.State != reconInterrupted {
				t.Fatalf("readback: %d %+v", code, back)
			}
			done, code := f.run("")
			if code != 0 || done.State != reconCreated || len(done.Attempts) != 1 {
				t.Fatalf("resume: %d %+v", code, done)
			}
			if phase == "fenced" && done.Generation.New != r.Generation.New {
				t.Fatalf("the fenced generation was reallocated: %+v -> %+v", r.Generation, done.Generation)
			}
			if after, _ := os.ReadFile(filepath.Join(f.archive, "manifest.json")); string(after) != string(manifest) {
				t.Fatal("the archive changed after preservation")
			}
			if f.records() != 1 {
				t.Fatalf("records %d", f.records())
			}
		})
	}
}

func TestReconstructAdoptsItsOwnArchiveAndRefusesAnotherDirectory(t *testing.T) {
	f := newReconFixture(t)
	if err := os.Mkdir(f.archive, 0o700); err != nil {
		t.Fatal(err)
	}
	if r, code := f.run(""); code != 1 || r.State != reconHeld {
		t.Fatalf("an unrelated existing directory was accepted: %d %+v", code, r)
	}
	if entries, _ := os.ReadDir(f.archive); len(entries) != 0 {
		t.Fatal("an unrelated directory was modified")
	}
	os.Remove(f.archive)
	// A crash after the archive was published but before its record was written.
	if r, code := f.run("preserved"); code != 1 || r.State != reconInterrupted {
		t.Fatalf("setup: %d %+v", code, r)
	}
	if err := os.RemoveAll(filepath.Join(f.checkout, ".kogen")); err != nil {
		t.Fatal(err)
	}
	if r, code := f.run(""); code != 0 || r.State != reconCreated {
		t.Fatalf("own archive not adopted: %d %+v", code, r)
	}
}

func TestReconstructDifferentEvidenceIsADuplicateReconciliation(t *testing.T) {
	f := newReconFixture(t)
	first, code := f.run("")
	if code != 0 {
		t.Fatalf("first: %d %+v", code, first)
	}
	f.ev["prior"].(map[string]any)["attempts"] = []any{"577e7c601aab41a39eb0820fc5fee5fb", strings.Repeat("f", 32)}
	f.write()
	f.archive += "-2"
	r, code := f.run("")
	if code != 1 || r.State != reconHeld || !strings.Contains(r.Reason, "duplicate reconciliation") || len(r.Attempts) != 0 {
		t.Fatalf("duplicate: %d %+v", code, r)
	}
	if _, err := os.Stat(f.archive); err == nil || f.records() != 1 {
		t.Fatalf("a duplicate wrote an archive or record (records %d)", f.records())
	}
	back, code, _ := ReadReconstruction(f.checkout, problem20Item, nil)
	if code != 0 || back.Attempts[0] != first.Attempts[0] {
		t.Fatalf("readback: %d %+v", code, back)
	}
}

func TestReconstructHoldsWhileAnotherProcessHoldsTheBuildLock(t *testing.T) {
	f := newReconFixture(t)
	lock, err := git.AcquireBuild(f.checkout, sys.New())
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	r, code := f.run("")
	if code != 1 || r.State != reconHeld || !strings.Contains(r.Reason, "Build lock") {
		t.Fatalf("lock: %d %+v", code, r)
	}
	if _, err := os.Stat(f.archive); err == nil || f.records() != 0 {
		t.Fatal("a held run wrote")
	}
}

func TestReadReconstructionOfAnUnreconstructedItemIsAbsent(t *testing.T) {
	f := newReconFixture(t)
	r, code, p := ReadReconstruction(f.checkout, problem20Item, nil)
	if code != 1 || p == nil || r.State != reconAbsent || len(r.Attempts) != 0 || len(r.Wakes) != 0 || r.QueueItem != nil {
		t.Fatalf("absent: %d %+v %+v", code, r, p)
	}
	if _, err := os.Stat(filepath.Join(f.checkout, ".kogen")); err == nil {
		t.Fatal("a readback created custody state")
	}
}
