package build

import (
	"context"
	"strings"
	"testing"
)

const ruEdit = "printf changed >> "

// ruResumeInterrupted resumes id, interrupts the resumed run at its first request and
// returns the resumed run's id.
func ruResumeInterrupted(e *env, id string) string {
	e.t.Helper()
	before := map[string]bool{}
	for _, r := range e.runIDs() {
		before[r] = true
	}
	opened := make(chan struct{})
	e.prov.replace(hang(opened))
	e.credentials()
	s, _, _ := e.session()
	done := make(chan struct{})
	go func() {
		defer close(done)
		Resume(context.Background(), s, ResumeInput{Deps: e.deps(), Project: e.root, ID: id})
	}()
	<-opened
	s.Interrupt("SIGINT")
	<-done
	for _, r := range e.runIDs() {
		if !before[r] {
			return r
		}
	}
	e.t.Fatal("the resume created no run")
	return ""
}

// ruSecondEditRefused uses the restore, resumes the Build `resumes` times (each interrupted),
// then resumes it to a second protected-file edit, which must be refused without a restore.
func ruSecondEditRefused(t *testing.T, resumes int) {
	e := newEnv(t)
	f := e.prov
	orig := e.read(stepsFile)
	id, _ := e.interruptedAtRequest(f.calls("shell", map[string]any{"command": ruEdit + stepsFile}))
	if got := e.read(stepsFile); got != orig {
		t.Fatalf("the first edit was not restored: %q", got)
	}
	for i := 0; i < resumes; i++ {
		_, state := e.checkpoint(id)
		if state["restore_used"] != true {
			t.Fatalf("restore_used = %v before resume %d, want true", state["restore_used"], i+1)
		}
		id = ruResumeInterrupted(e, id)
	}
	_, state := e.checkpoint(id)
	if state["restore_used"] != true {
		t.Fatalf("restore_used = %v, want true", state["restore_used"])
	}
	f.replace(f.calls("shell", map[string]any{"command": ruEdit + stepsFile}, "finish", map[string]any{"summary": "x"}))
	r := e.resume(id)
	if r.exit != 1 || !strings.Contains(r.shown(), "baseline_changed") {
		t.Fatalf("the second protected edit was not refused: exit %d\n%s", r.exit, r.shown())
	}
	if got := e.read(stepsFile); got != orig+"changed" {
		t.Fatalf("the file was restored a second time: %q", got)
	}
}

func TestResumeKeepsRestoreUsed(t *testing.T) {
	t.Parallel()
	ruSecondEditRefused(t, 0)
}

func TestRestoreUsedSurvivesTwoResumes(t *testing.T) {
	t.Parallel()
	ruSecondEditRefused(t, 2)
}

// ruUnusedStillRestored resumes a Build that has not used its restore (with the restore_used
// key dropped from the record when dropKey) and edits a protected file: it is restored.
func ruUnusedStillRestored(t *testing.T, dropKey bool) {
	e := newEnv(t)
	f := e.prov
	orig := e.read(stepsFile)
	id, _ := e.interruptedAtRequest(f.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}))
	doc, state := e.checkpoint(id)
	if state["restore_used"] != false {
		t.Fatalf("restore_used = %v, want false written", state["restore_used"])
	}
	if dropKey {
		delete(state, "restore_used")
		doc["state"] = state
		e.writeCheckpoint(id, doc)
	}
	f.replace(f.calls("shell", map[string]any{"command": ruEdit + stepsFile}, "finish", map[string]any{"summary": "x"}))
	r := e.resume(id)
	if r.exit != 0 || r.err != nil {
		t.Fatalf("the resume did not land: exit %d err %v\n%s", r.exit, r.err, r.shown())
	}
	if got := e.read(stepsFile); got != orig {
		t.Fatalf("the first edit was not restored: %q", got)
	}
}

func TestResumeUnusedRestoreStillAvailable(t *testing.T) {
	t.Parallel()
	ruUnusedStillRestored(t, false)
}

func TestResumeOldRecordWithoutRestoreField(t *testing.T) {
	t.Parallel()
	ruUnusedStillRestored(t, true)
}
