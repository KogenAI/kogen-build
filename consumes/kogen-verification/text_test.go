package consumes

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

func TestCommandResultJSON(t *testing.T) {
	r := verify.CommandResult{Name: "build", Command: "go build ./...", Status: 2, Output: "a <b> & \"c\"\n"}
	var _ string = r.Name
	var _ string = r.Command
	var _ int = r.Status
	var _ string = r.Output

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]any
	if err := json.Unmarshal(b, &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 4 {
		t.Errorf("verify.CommandResult: JSON keys = %s, want exactly name, command, status and output", b)
	}
	for key, want := range map[string]any{"name": "build", "command": "go build ./...", "status": float64(2), "output": "a <b> & \"c\"\n"} {
		if keys[key] != want {
			t.Errorf("verify.CommandResult: JSON %q = %v, want %v", key, keys[key], want)
		}
	}

	// The command's stdout `checks` list is a JSON array of these records, `[]` when empty.
	list := struct {
		Checks []verify.CommandResult `json:"checks"`
	}{Checks: []verify.CommandResult{}}
	if b, _ := json.Marshal(list); string(b) != `{"checks":[]}` {
		t.Errorf("verify.CommandResult: an empty checks list encodes as %s", b)
	}
	list.Checks = []verify.CommandResult{{Name: "a", Command: "x", Status: 0, Output: ""}, r}
	var out strings.Builder
	if err := json.NewEncoder(&out).Encode(list); err != nil {
		t.Fatal(err)
	}
	want := `{"checks":[{"name":"a","command":"x","status":0,"output":""},{"name":"build","command":"go build ./...","status":2,"output":"a \u003cb\u003e \u0026 \"c\"\n"}]}` + "\n"
	if out.String() != want {
		t.Errorf("verify.CommandResult: checks list = %s, want %s", out.String(), want)
	}
	var back verify.CommandResult
	if err := json.Unmarshal([]byte(`{"name":"n","command":"c","status":7,"output":"o"}`), &back); err != nil || back != (verify.CommandResult{Name: "n", Command: "c", Status: 7, Output: "o"}) {
		t.Errorf("verify.CommandResult: decoded %+v, %v", back, err)
	}
}

func TestEnvFailure(t *testing.T) {
	var _ func(verify.CommandResult) (string, bool) = verify.EnvFailure

	const panicLine = "panic: test timed out after 10m0s"
	for _, c := range []struct {
		name string
		r    verify.CommandResult
		text string
		ok   bool
	}{
		{"timeout panic", verify.CommandResult{Status: 2, Output: "ok a\n" + panicLine + "\nrunning tests:\n"}, panicLine, true},
		{"indented line is trimmed", verify.CommandResult{Status: 1, Output: "x\n \t" + panicLine + "  \t\ny"}, panicLine, true},
		{"the first such line", verify.CommandResult{Status: 2, Output: panicLine + "\npanic: test timed out after 20m0s\n"}, panicLine, true},
		{"zero status", verify.CommandResult{Status: 0, Output: panicLine + "\n"}, "", false},
		{"other failure", verify.CommandResult{Status: 1, Output: "FAIL\n--- FAIL: TestX\n"}, "", false},
		{"not at the start of the line", verify.CommandResult{Status: 1, Output: "got " + panicLine + "\n"}, "", false},
		{"no output", verify.CommandResult{Status: 1}, "", false},
	} {
		text, ok := verify.EnvFailure(c.r)
		if text != c.text || ok != c.ok {
			t.Errorf("verify.EnvFailure: %s: got (%q, %v), want (%q, %v)", c.name, text, ok, c.text, c.ok)
		}
	}
}

func TestCap(t *testing.T) {
	var _ func(string) string = verify.Cap

	const limit = 1 << 20
	for _, s := range []string{"", "short output\n", strings.Repeat("x", limit)} {
		if got := verify.Cap(s); got != s {
			t.Errorf("verify.Cap: output of %d bytes (within 1 MiB) changed to %d bytes", len(s), len(got))
		}
	}
	long := strings.Repeat("a", limit/2) + strings.Repeat("b", limit)
	got := verify.Cap(long)
	const marker = "\n[... cut in the run log ...]\n"
	if len(got) > limit {
		t.Errorf("verify.Cap: %d bytes, want at most %d", len(got), limit)
	}
	head, tail, ok := strings.Cut(got, marker)
	if !ok || !strings.HasPrefix(head, "aaaa") || !strings.HasSuffix(tail, "bbbb") || head == "" || tail == "" {
		t.Errorf("verify.Cap: want both ends around the cut marker, got %d bytes", len(got))
	}
	// A cut never splits a rune.
	runes := strings.Repeat("é", limit)
	if got := verify.Cap(runes); !strings.Contains(got, marker) || strings.ContainsRune(got, '�') {
		t.Errorf("verify.Cap: a cut split a multi-byte character")
	}
}

func TestDiagnostic(t *testing.T) {
	var _ func(string) string = verify.Diagnostic

	if got := verify.Diagnostic("one\ntwo\n\n"); got != "one\ntwo" {
		t.Errorf("verify.Diagnostic: short output = %q, want it without trailing blank lines", got)
	}
	var lines []string
	for i := 1; i <= 60; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	if got := verify.Diagnostic(strings.Join(lines, "\n")); got != strings.Join(lines, "\n") {
		t.Errorf("verify.Diagnostic: 60 lines changed")
	}
	lines = nil
	for i := 1; i <= 100; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	got := strings.Split(verify.Diagnostic(strings.Join(lines, "\n")+"\n"), "\n")
	if len(got) != 51 || got[0] != "line 1" || got[29] != "line 30" || got[30] != "[... 50 lines omitted ...]" || got[31] != "line 81" || got[50] != "line 100" {
		t.Errorf("verify.Diagnostic: 100 lines = %d lines %q, want the first 30, a marker for 50 omitted and the last 20", len(got), got)
	}
	// One enormous line stays within 16 KiB, keeping both ends.
	huge := strings.Repeat("h", 4096) + strings.Repeat("m", 40000) + strings.Repeat("t", 12288)
	got2 := verify.Diagnostic(huge)
	if len(got2) > 16384 || !strings.HasPrefix(got2, strings.Repeat("h", 4096)) || !strings.HasSuffix(got2, "tttt") || !strings.Contains(got2, "bytes omitted") {
		t.Errorf("verify.Diagnostic: an enormous line gave %d bytes, want at most 16384 with both ends and an omitted-bytes marker", len(got2))
	}
}

func TestNotRunNotice(t *testing.T) {
	var _ func([]string) string = verify.NotRunNotice

	if got := verify.NotRunNotice(nil); got != "" {
		t.Errorf("verify.NotRunNotice: none = %q, want empty", got)
	}
	if got := verify.NotRunNotice([]string{}); got != "" {
		t.Errorf("verify.NotRunNotice: empty list = %q, want empty", got)
	}
	if got, want := verify.NotRunNotice([]string{"model", "lint"}), "not run because an earlier item failed: model, lint"; got != want {
		t.Errorf("verify.NotRunNotice = %q, want %q", got, want)
	}
}

type recordingLog struct{ events []map[string]any }

func (l *recordingLog) Log(e map[string]any) { l.events = append(l.events, e) }

func TestRunWatched(t *testing.T) {
	var _ func(*sys.Session, string, string, verify.Logger, func() (sys.Result, error)) (sys.Result, error) = verify.RunWatched

	p := newProj(t)
	s, _ := p.session()
	var l recordingLog

	var _ int = sys.Result{}.Status
	var _ string = sys.Result{}.Output
	got, err := verify.RunWatched(s, "check", "echo hi; exit 3", &l, func() (sys.Result, error) {
		return s.Run("echo hi; exit 3", p.root, sys.Options{KillGroupOnExit: true})
	})
	if err != nil || got.Status != 3 || got.Output != "hi\n" {
		t.Errorf("verify.RunWatched: a command's own result = (%+v, %v), want status 3 and its output", got, err)
	}
	got, err = verify.RunWatched(s, "check", ":", &l, func() (sys.Result, error) { return sys.Result{Status: 9, Output: "given"}, nil })
	if err != nil || got != (sys.Result{Status: 9, Output: "given"}) {
		t.Errorf("verify.RunWatched: run's result = (%+v, %v), want it unchanged", got, err)
	}
	boom := errors.New("boom")
	got, err = verify.RunWatched(s, "check", ":", nil, func() (sys.Result, error) { return sys.Result{Status: 1}, boom })
	if err != boom || got.Status != 1 {
		t.Errorf("verify.RunWatched: run's error = (%+v, %v), want it returned as is", got, err)
	}
	if len(l.events) != 0 {
		t.Errorf("verify.RunWatched: a command that ended normally logged %v", l.events)
	}
}
