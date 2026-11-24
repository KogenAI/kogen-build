package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
)

func TestContinuationRemainingWork(t *testing.T) {
	e := newEnv(t, func(c *project.Config) {
		c.Checks = []project.Check{{Name: "first", Run: "test -f app.txt"}, {Name: "second", Run: "true"}}
	})
	id, _ := e.interruptedAtRequest(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}))
	raw, err := os.ReadFile(filepath.Join(e.root, ".kogen/local/runs", id, "checkpoint.json"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, why := decodeCheckpoint(raw)
	if why != "" {
		t.Fatal(why)
	}
	s, _, _ := e.session()
	b := &Attempt{State: *loaded.State, s: s, deps: e.deps(), repo: git.New(e.root, s.Env)}
	for _, stage := range []string{StageBuilder, StageBuilderVerify, StageUsageLimit, StageReview, StageReviewVerify, StageReviewRetry, StageReviewOnly, StageCommitFailed, StageLanded} {
		t.Run(stage, func(t *testing.T) {
			b.Continuation.Stage = stage
			roles, checks := b.continuationObligations()
			wantRoles, wantChecks := []string{"review"}, []string{"test -f app.txt", "true"}
			switch stage {
			case StageBuilder, StageBuilderVerify, StageUsageLimit:
				wantRoles = []string{"build", "review"}
			case StageCommitFailed, StageLanded:
				wantRoles, wantChecks = []string{}, []string{}
			}
			if !reflect.DeepEqual(roles, wantRoles) || !reflect.DeepEqual(checks, wantChecks) {
				t.Fatalf("roles/checks = %v/%v, want %v/%v", roles, checks, wantRoles, wantChecks)
			}
		})
	}
	b.Continuation.Stage = StageBuilderVerify
	point, err := b.checkPoint()
	if err != nil {
		t.Fatal(err)
	}
	b.ledgerRecord(b.Config.Checks[0], point, 0, "passed")
	_, checks := b.continuationObligations()
	if !reflect.DeepEqual(checks, []string{"true"}) {
		t.Fatalf("native ledger reuse: %v", checks)
	}
	e.write("app.txt", "new candidate\n")
	_, checks = b.continuationObligations()
	if !reflect.DeepEqual(checks, []string{"test -f app.txt", "true"}) {
		t.Fatalf("changed tree must invalidate reuse: %v", checks)
	}
}

func TestContinuationRequiresFreshAdmission(t *testing.T) {
	e := newEnv(t)
	id, _ := e.interruptedAtRequest(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}))
	s, _, _ := e.session()
	in := ResumeInput{Deps: e.deps(), Project: e.root, ID: id}
	binding := ContinuationBinding{Host: "host", Claim: "claim", Attempt: "attempt", RequestRevision: "rev3", RequestHash: "request"}
	plan, err := in.PrepareContinuation(context.Background(), s, binding)
	if err != nil {
		t.Fatal(err)
	}
	runs, calls := len(e.runIDs()), e.prov.count()
	code, err := in.ExecuteContinuation(context.Background(), s, plan, binding, nil)
	if code != 1 || err == nil {
		t.Fatalf("missing admission: %d %v", code, err)
	}
	consumed := false
	admit := func(_ context.Context, p ContinuationPlan, f ContinuationBinding) error {
		if consumed {
			return errors.New("duplicate fenced attempt")
		}
		consumed = true
		return errors.New("ordinary fleet provider admission held")
	}
	for range 2 {
		code, err = in.ExecuteContinuation(context.Background(), s, plan, binding, admit)
		if code != 1 || err == nil {
			t.Fatalf("held admission: %d %v", code, err)
		}
	}
	if len(e.runIDs()) != runs || e.prov.count() != calls {
		t.Fatal("held dispatch created a run or provider request")
	}
}

func TestContinuationUnknownCheckpointFacts(t *testing.T) {
	e := newEnv(t)
	id, _ := e.interruptedAtRequest(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}))
	s, _, _ := e.session()
	in := ResumeInput{Deps: e.deps(), Project: e.root, ID: id}
	path := filepath.Join(e.root, ".kogen/local/runs", id, "checkpoint.json")
	for _, missing := range []bool{false, true} {
		raw := []byte("{malformed checkpoint\n")
		if missing {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := in.PrepareContinuation(context.Background(), s, ContinuationBinding{})
		if err == nil || plan.HeldReason == "" {
			t.Fatal("unknown checkpoint was not held")
		}
		if plan.BuildID != id || plan.Root != e.root || plan.CheckoutIdentity == "" {
			t.Fatalf("lost observed checkout facts: %+v", plan)
		}
		if plan.Slug != "" || plan.Start != "" || plan.ApprovalSHA256 != "" || plan.SavedCheckoutIdentity != "" || plan.CandidateTree != "" {
			t.Fatalf("invented saved facts: %+v", plan)
		}
		want := sha256Hex(raw)
		if missing {
			want = ""
		}
		if plan.CheckpointSHA256 != want {
			t.Fatalf("checkpoint hash %q, want %q", plan.CheckpointSHA256, want)
		}
		if plan.RequiredChecks == nil || plan.RequiredRoles == nil {
			t.Fatal("empty work lists must be arrays")
		}
	}
}
