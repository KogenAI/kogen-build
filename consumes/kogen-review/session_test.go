package consumes

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-review/review"
)

// The trees a session names: a session's bytes do not depend on the trees existing.
const (
	sessionTree = "1111111111111111111111111111111111111111"
	sessionSeen = "2222222222222222222222222222222222222222"
)

// sessionJSON is the exact encoding of a full session as a checkpoint keeps it in
// continuation.review: build's state goldens hold such sessions, build/legacy.go builds them.
const sessionJSON = `{"role":{"Provider":"chatgpt","Model":"fixture-review-model","Effort":"medium"},` +
	`"input":[{"content":"x < y & z","role":"user"}],"usage":{"input":10,"cached":0,"output":2,"reasoning":null},` +
	`"attempts":[{"pass":1,"tree":"1111111111111111111111111111111111111111","verdict":"changes","findings":[{"line":3,"message":"m","path":"app.txt","severity":"blocking"}]}],` +
	`"verdict":{"verdict":"changes","findings":[{"line":3,"message":"m","path":"app.txt","severity":"blocking"}]},` +
	`"tree":"1111111111111111111111111111111111111111","seen_tree":"2222222222222222222222222222222222222222",` +
	`"tools":[{"name":"read_file"}],"rejected_verdicts":1,"no_tool_reminders":1,"remind":true,"prev_unread":2,"reopened":true,"repaired":true}`

func fullSession() review.Session {
	finding := map[string]any{"path": "app.txt", "line": json.Number("3"), "severity": "blocking", "message": "m"}
	return review.Session{
		Role:     reviewRole,
		Input:    []any{map[string]any{"role": "user", "content": "x < y & z"}},
		Usage:    core.Tokens{Input: core.Count(10), Cached: core.Count(0), Output: core.Count(2)},
		Attempts: []review.Submitted{{Pass: 1, Tree: sessionTree, Verdict: "changes", Findings: []map[string]any{finding}}},
		Verdict:  &review.Verdict{Verdict: "changes", Findings: []map[string]any{finding}},
		Tree:     sessionTree, Seen: sessionSeen, Tools: []any{map[string]any{"name": "read_file"}},
		Rejected: 1, NoTool: 1, Remind: true, PrevUnread: 2, Reopened: true, Repaired: true,
	}
}

// TestSessionEncodeIsTheCheckpointBytes pins the reviewer session's JSON as build writes it into
// a checkpoint (it builds sessions in build/legacy.go): the keys and their order, empty fields
// omitted, nil lists as [] and no HTML escaping.
func TestSessionEncodeIsTheCheckpointBytes(t *testing.T) {
	t.Parallel()
	var _ func(review.Session) (json.RawMessage, error) = review.Session.Encode
	raw, err := fullSession().Encode()
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.Session.Encode full", string(raw), sessionJSON)

	// A session of the stage review that has no verdict yet: the nil lists encode as [].
	raw, err = review.Session{Role: project.Role{Model: "m"}, Usage: core.ZeroTokens(), Verdict: &review.Verdict{Verdict: "approve"}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.Session.Encode empty", string(raw),
		`{"role":{"Provider":"","Model":"m","Effort":""},"input":[],"usage":{"input":0,"cached":0,"output":0,"reasoning":0},"attempts":[],"verdict":{"verdict":"approve","findings":[]}}`)

	// The encoding does not change the session it was made from.
	s := review.Session{Role: reviewRole, Verdict: &review.Verdict{Verdict: "approve"}}
	if _, err := s.Encode(); err != nil {
		t.Fatal(err)
	}
	if s.Verdict.Findings != nil || s.Attempts != nil || s.Input != nil {
		t.Errorf("review.Session.Encode changed its receiver: %+v", s)
	}
}

// TestDecodeSessionReadsWhatACheckpointKept pins the decoding build relies on (build/review_retry.go
// reads Verdict.Verdict of a review_verify checkpoint's session): every field round-trips,
// numbers stay as written (json.Number) and malformed or missing state is an error.
func TestDecodeSessionReadsWhatACheckpointKept(t *testing.T) {
	t.Parallel()
	var _ func(json.RawMessage) (review.Session, error) = review.DecodeSession
	s, err := review.DecodeSession(json.RawMessage(sessionJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, fullSession()) {
		t.Errorf("review.DecodeSession: got %#v, want %#v", s, fullSession())
	}
	eq(t, "review.DecodeSession verdict", s.Verdict.Verdict, "changes")
	if _, ok := s.Verdict.Findings[0]["line"].(json.Number); !ok {
		t.Errorf("review.DecodeSession: a finding's line is %T, want json.Number", s.Verdict.Findings[0]["line"])
	}
	// Encode and DecodeSession round-trip the checkpoint bytes.
	raw, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "review.DecodeSession round trip", string(raw), sessionJSON)
	if _, err := review.DecodeSession(json.RawMessage(`{"attempts":"no"}`)); err == nil {
		t.Error("review.DecodeSession: malformed session state decoded without an error")
	}
	if _, err := review.DecodeSession(nil); err == nil {
		t.Error("review.DecodeSession: missing session state decoded without an error")
	}
}

// TestSessionFieldTypes pins the fields of review.Session, review.Submitted and review.Verdict
// that build writes (build/legacy.go converts a version 3 checkpoint into them) and reads, with
// their types written out.
func TestSessionFieldTypes(t *testing.T) {
	t.Parallel()
	s := fullSession()
	var _ project.Role = s.Role
	var _ []any = s.Input
	var _ core.Tokens = s.Usage
	var _ []review.Submitted = s.Attempts
	var _ *review.Verdict = s.Verdict
	var _ string = s.Tree
	var _ []any = s.Tools
	eq(t, "review.Session.Role", s.Role, reviewRole)
	eq(t, "review.Session.Tree", s.Tree, sessionTree)
	eq(t, "review.Session.Usage", s.Usage.Input != nil && *s.Usage.Input == 10, true)
	eq(t, "review.Session.Input", len(s.Input), 1)
	eq(t, "review.Session.Tools", len(s.Tools), 1)

	a := s.Attempts[0]
	var _ int = a.Pass
	var _ string = a.Tree
	var _ string = a.Verdict
	var _ []map[string]any = a.Findings
	eq(t, "review.Submitted.Pass", a.Pass, 1)
	eq(t, "review.Submitted.Tree", a.Tree, sessionTree)
	eq(t, "review.Submitted.Verdict", a.Verdict, "changes")
	eq(t, "review.Submitted.Findings", len(a.Findings), 1)

	var v review.Verdict = *s.Verdict
	var _ string = v.Verdict
	var _ []map[string]any = v.Findings
	eq(t, "review.Verdict.Verdict", v.Verdict, "changes")
	eq(t, "review.Verdict.Findings", len(v.Findings), 1)
	eq(t, "review.Session.Verdict", s.Verdict.Verdict, "changes")
}
