package consumes

import (
	"context"
	"regexp"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

// reuseKeyOf is the key of command on tree in a session with the extra variables added over the project's environment plus extra variables.
func reuseKeyOf(p *proj, extra []string, tree, command string) string {
	s, _ := p.session()
	s.Env = append(append([]string{}, s.Env...), extra...)
	return verify.NewReuseSnapshot(s, verify.Tools{}, tree, p.root).Key(command)
}

// TestReuseSnapshotKey states what the baseline reuse shadow relies on: one snapshot per tree
// gives the key of any command without another probe, the key is the reuse_sha256 that a pass
// puts on the command event of that check, it changes with the command, the tree and the check
// environment, and it ignores KOGEN_ATTEMPT_ID.
func TestReuseSnapshotKey(t *testing.T) {
	p := newProj(t)
	p.cfg.Checks = []project.Check{{Name: "build", Run: "echo built"}}
	s, _ := p.session()
	var log memLog
	in := p.input(1)
	in.Log = &log
	pass, err := verify.Verify(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := verify.NewReuseSnapshot(s, in.Tools, pass.Tree, p.root)
	var key func(verify.ReuseSnapshot, string) string = verify.ReuseSnapshot.Key
	got := key(snapshot, "echo built")
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(got) {
		t.Fatalf("ReuseSnapshot.Key = %q, want a SHA-256 in hexadecimal", got)
	}
	var onEvent any
	for _, e := range log.named("command") {
		if e["name"] == "build" {
			onEvent = e["reuse_sha256"]
		}
	}
	if onEvent != got {
		t.Errorf("reuse_sha256 of the pass's command event = %v, want the key of the snapshot of the same tree %q", onEvent, got)
	}
	if again := key(snapshot, "echo built"); again != got {
		t.Errorf("ReuseSnapshot.Key twice = %q and %q, want the same key", got, again)
	}
	if other := key(snapshot, "echo other"); other == got {
		t.Error("ReuseSnapshot.Key of another command is the same key")
	}
	if other := reuseKeyOf(p, nil, "0000000000000000000000000000000000000000", "echo built"); other == got {
		t.Error("ReuseSnapshot.Key of another tree is the same key")
	}
	if other := reuseKeyOf(p, []string{"KOGEN_REUSE_PROBE=changed"}, pass.Tree, "echo built"); other == got {
		t.Error("ReuseSnapshot.Key of another check environment is the same key")
	}
	first := reuseKeyOf(p, []string{"KOGEN_ATTEMPT_ID=one"}, pass.Tree, "echo built")
	second := reuseKeyOf(p, []string{"KOGEN_ATTEMPT_ID=two"}, pass.Tree, "echo built")
	if first != second {
		t.Errorf("ReuseSnapshot.Key with KOGEN_ATTEMPT_ID one and two = %q and %q, want the same key", first, second)
	}
}
