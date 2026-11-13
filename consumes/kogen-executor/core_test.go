package consumes

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-loop/tools"
)

func encoded(t *testing.T, item string, v any) string {
	t.Helper()
	b, err := core.EncodeJSON(v)
	if err != nil {
		t.Fatalf("%s: core.EncodeJSON: %v", item, err)
	}
	return string(b)
}

func TestCoreEncodeJSON(t *testing.T) {
	var _ func(any) ([]byte, error) = core.EncodeJSON
	// No HTML escaping and no final newline; map keys sorted.
	if got, want := encoded(t, "core.EncodeJSON", map[string]any{"b": "<a&b>", "a": []int{1, 2}}), `{"a":[1,2],"b":"<a&b>"}`; got != want {
		t.Errorf("core.EncodeJSON = %s, want %s", got, want)
	}
	if got := encoded(t, "core.EncodeJSON", "x"); got != `"x"` {
		t.Errorf("core.EncodeJSON of a string = %s, want \"x\" with no newline", got)
	}
	if b, err := core.EncodeJSON(make(chan int)); err == nil || b != nil {
		t.Errorf("core.EncodeJSON: a value that cannot be encoded must give an error and no bytes, got %q, %v", b, err)
	}
}

func TestCoreHashHex(t *testing.T) {
	var _ func([]byte) string = core.HashHex
	for in, want := range map[string]string{
		"":    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"abc": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
	} {
		if got := core.HashHex([]byte(in)); got != want {
			t.Errorf("core.HashHex(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestCoreNonNil(t *testing.T) {
	var _ func([]string) []string = core.NonNil[string]
	var _ func([]map[string]any) []map[string]any = core.NonNil[map[string]any]
	var _ func([]any) []any = core.NonNil[any]
	// A nil list becomes an empty one that encodes as a JSON array, never null.
	for name, got := range map[string]string{
		"string": encoded(t, "core.NonNil", map[string]any{"v": core.NonNil[string](nil)}),
		"map":    encoded(t, "core.NonNil", map[string]any{"v": core.NonNil[map[string]any](nil)}),
		"any":    encoded(t, "core.NonNil", map[string]any{"v": core.NonNil[any](nil)}),
	} {
		if got != `{"v":[]}` {
			t.Errorf("core.NonNil[%s]: a nil list must encode as [], got %s", name, got)
		}
	}
	if got := core.NonNil[string](nil); got == nil || len(got) != 0 {
		t.Errorf("core.NonNil: nil must give a non-nil empty list, got %#v", got)
	}
	in := []string{"a", "b"}
	if got := core.NonNil(in); !reflect.DeepEqual(got, in) || &got[0] != &in[0] {
		t.Errorf("core.NonNil: a non-nil list must be returned as it is, got %v", got)
	}
	if got := core.NonNil([]string{}); got == nil || len(got) != 0 {
		t.Errorf("core.NonNil: an empty list must stay an empty non-nil list, got %#v", got)
	}
}

func TestCoreDisplay(t *testing.T) {
	var _ func(any) string = core.Display
	for _, tc := range []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"text", "text"},
		{json.Number("12"), "12"},
		{json.Number("1.50"), "1.50"},
		{float64(3), "3.0"},
		{float64(2.5), "2.5"},
		{7, "7"},
		{true, "true"},
	} {
		if got := core.Display(tc.in); got != tc.want {
			t.Errorf("core.Display(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCoreInspectString(t *testing.T) {
	var _ func(string) string = core.InspectString
	for in, want := range map[string]string{
		"plain":              `"plain"`,
		`a"b\c`:              `"a\"b\\c"`,
		"l1\nl2\t\r":         `"l1\nl2\t\r"`,
		"\x1b\a\b\f\v":       `"\e\a\b\f\v"`,
		"\x00":               `"\0"`,
		"#{x}":               `"\#{x}"`,
		"# {x} #":            `"# {x} #"`,
		"\x01\x7f":           `"\x01\d"`,
		"é✓":                 `"é✓"`,
		string(rune(0x200b)): "\"\\" + "u200B\"",
		"":                   `""`,
	} {
		if got := core.InspectString(in); got != want {
			t.Errorf("core.InspectString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestCoreMaterialText(t *testing.T) {
	var _ func(any) (string, bool) = core.MaterialText
	if got, ok := core.MaterialText(map[string]any{"type": "text", "text": "hello"}); !ok || got != "hello" {
		t.Errorf("core.MaterialText of a text block = %q, %v; want \"hello\", true", got, ok)
	}
	for name, in := range map[string]any{
		"not a map":     "text",
		"no text":       map[string]any{"type": "image"},
		"text not text": map[string]any{"text": 3},
		"nil":           nil,
	} {
		if got, ok := core.MaterialText(in); ok || got != "" {
			t.Errorf("core.MaterialText: %s must give \"\", false, got %q, %v", name, got, ok)
		}
	}
}

func TestCoreDiagnosticText(t *testing.T) {
	var _ func(any) string = core.DiagnosticText
	for name, tc := range map[string]struct {
		in   any
		want string
	}{
		"plain content": {map[string]any{"role": "user", "content": "diagnostic"}, "diagnostic"},
		"text blocks": {map[string]any{"content": []any{
			map[string]any{"type": "text", "text": "one"},
			map[string]any{"type": "image"},
			map[string]any{"type": "text", "text": "two"},
		}}, "one\ntwo"},
		"other content": {map[string]any{"content": 3}, ""},
		"no content":    {map[string]any{}, ""},
		"not a map":     {"x", ""},
		"nil":           {nil, ""},
	} {
		if got := core.DiagnosticText(tc.in); got != tc.want {
			t.Errorf("core.DiagnosticText: %s = %q, want %q", name, got, tc.want)
		}
	}
}

func TestCoreResumeRefused(t *testing.T) {
	var _ func(string) *sys.Error = core.ResumeRefused
	e := core.ResumeRefused("the candidate changed")
	if e.Class != "candidate" || e.Reason != "resume_refused" || e.Detail != "the candidate changed" || e.Exit != 1 || len(e.Lines) != 0 {
		t.Errorf("core.ResumeRefused = %#v, want candidate/resume_refused with the detail and exit 1", e)
	}
	if e.Error() != "candidate/resume_refused: the candidate changed" {
		t.Errorf("core.ResumeRefused: text = %q", e.Error())
	}
	var err error = e
	if got, ok := sys.AsError(err); !ok || got != e {
		t.Errorf("core.ResumeRefused: must be recognised as a sys.Error, got %v, %v", got, ok)
	}
	if !errors.As(err, new(*sys.Error)) {
		t.Errorf("core.ResumeRefused: must be a *sys.Error")
	}
}

func TestCoreShellDefinitions(t *testing.T) {
	var _ func(*tools.Tools, []string) []any = core.ShellDefinitions
	tl := &tools.Tools{}
	names := []string{"shell", "read_file", "view_image", "write_file"}
	defs := core.ShellDefinitions(tl, names)
	if len(defs) != len(names) {
		t.Fatalf("core.ShellDefinitions: want %d definitions, got %d", len(names), len(defs))
	}
	for i, name := range names {
		m, ok := defs[i].(map[string]any)
		if !ok || m["name"] != name || m["type"] != "function" {
			t.Errorf("core.ShellDefinitions: definition %d must be the function definition of %q, got %#v", i, name, defs[i])
			continue
		}
		want, found := tl.Definition(name)
		if !found || !reflect.DeepEqual(m, want) {
			t.Errorf("core.ShellDefinitions: the definition of %q must be the executing tool's own (%v), got %#v", name, found, m)
		}
	}
	if got := core.ShellDefinitions(tl, nil); len(got) != 0 {
		t.Errorf("core.ShellDefinitions: no names must give no definitions, got %v", got)
	}
}

func TestCoreTokens(t *testing.T) {
	var _ func(int) *int = core.Count
	var _ func() core.Tokens = core.ZeroTokens
	var _ func(core.Tokens, core.Tokens) core.Tokens = core.Tokens.Plus
	var _ func(core.Tokens) string = core.Tokens.Line
	var _ func(core.Tokens) string = core.Tokens.Total
	tk := core.Tokens{Input: core.Count(1), Cached: nil, Output: core.Count(3), Reasoning: core.Count(0)}
	var _ *int = tk.Input
	var _ *int = tk.Cached
	var _ *int = tk.Output
	var _ *int = tk.Reasoning

	if a, b := core.Count(5), core.Count(5); *a != 5 || a == b {
		t.Errorf("core.Count: must point to a fresh known count, got %d, same pointer %v", *a, a == b)
	}
	zero := core.ZeroTokens()
	if zero.Input == nil || zero.Cached == nil || zero.Output == nil || zero.Reasoning == nil || *zero.Input != 0 || *zero.Cached != 0 || *zero.Output != 0 || *zero.Reasoning != 0 {
		t.Errorf("core.ZeroTokens: every count must be a known zero, got %+v", zero)
	}
	if got, want := encoded(t, "core.Tokens", tk), `{"input":1,"cached":null,"output":3,"reasoning":0}`; got != want {
		t.Errorf("core.Tokens: JSON = %s, want %s", got, want)
	}
	var back core.Tokens
	if err := json.Unmarshal([]byte(`{"input":1,"cached":null,"output":3,"reasoning":0}`), &back); err != nil || !reflect.DeepEqual(back, tk) {
		t.Errorf("core.Tokens: a checkpoint's JSON must decode to %+v, got %+v, %v", tk, back, err)
	}
	if got, want := tk.Line(), "input=1 cached=unknown output=3 reasoning=0"; got != want {
		t.Errorf("core.Tokens.Line = %q, want %q", got, want)
	}
	if got, want := zero.Line(), "input=0 cached=0 output=0 reasoning=0"; got != want {
		t.Errorf("core.Tokens.Line of zero = %q, want %q", got, want)
	}
	if got := tk.Total(); got != "4" {
		t.Errorf("core.Tokens.Total = %q, want input plus output \"4\"", got)
	}
	for name, u := range map[string]core.Tokens{"input": {Output: core.Count(1)}, "output": {Input: core.Count(1)}} {
		if got := u.Total(); got != "unknown" {
			t.Errorf("core.Tokens.Total: unknown %s must give \"unknown\", got %q", name, got)
		}
	}
	// Unknown stays unknown in every sum.
	sum := tk.Plus(core.Tokens{Input: core.Count(10), Cached: core.Count(2), Output: core.Count(20), Reasoning: nil})
	if want := (core.Tokens{Input: core.Count(11), Cached: nil, Output: core.Count(23), Reasoning: nil}); !reflect.DeepEqual(sum, want) {
		t.Errorf("core.Tokens.Plus = %+v, want %+v", sum, want)
	}
	if got := zero.Plus(zero); !reflect.DeepEqual(got, zero) {
		t.Errorf("core.Tokens.Plus: zero plus zero = %+v", got)
	}
	if *tk.Input != 1 || tk.Cached != nil {
		t.Errorf("core.Tokens.Plus: must not change its operands, got %+v", tk)
	}
}

func TestCoreReviewRecordJSON(t *testing.T) {
	r := core.ReviewRecord{N: 2, Verdict: "changes", Tree: "abc123",
		Findings: []map[string]any{{"path": "a.go", "line": 3, "severity": "high", "message": "a<b"}}}
	var _ int = r.N
	var _ string = r.Verdict
	var _ []map[string]any = r.Findings
	var _ string = r.Tree
	want := `{"n":2,"verdict":"changes","findings":[{"line":3,"message":"a<b","path":"a.go","severity":"high"}],"tree":"abc123"}`
	if got := encoded(t, "core.ReviewRecord", r); got != want {
		t.Errorf("core.ReviewRecord: JSON = %s, want %s", got, want)
	}
	// An approving record has empty findings; the tree is left out when unknown.
	approve := core.ReviewRecord{N: 1, Verdict: "approve", Findings: core.NonNil[map[string]any](nil)}
	if got, want := encoded(t, "core.ReviewRecord", approve), `{"n":1,"verdict":"approve","findings":[]}`; got != want {
		t.Errorf("core.ReviewRecord: JSON = %s, want %s", got, want)
	}
	var back core.ReviewRecord
	if err := json.Unmarshal([]byte(`{"n":1,"verdict":"approve","findings":[]}`), &back); err != nil || back.N != 1 || back.Verdict != "approve" || back.Tree != "" || back.Findings == nil || len(back.Findings) != 0 {
		t.Errorf("core.ReviewRecord: a checkpoint's JSON must decode, got %+v, %v", back, err)
	}
}

func TestCoreExecutorStateJSON(t *testing.T) {
	cost := 0.5
	st := core.ExecutorState{
		Name: "claude-code", Version: "2.1.318", SessionID: "sess-1", Rounds: 2,
		Usage: core.ExecutorUsage{Input: core.Count(3), Cached: nil, Output: core.Count(4), Reasoning: core.Count(0), State: "partial", Source: "harness", CostUSD: &cost},
	}
	var _ string = st.Name
	var _ string = st.Version
	var _ string = st.SessionID
	var _ int = st.Rounds
	var _ core.ExecutorUsage = st.Usage
	var _ *int = st.Usage.Input
	var _ *int = st.Usage.Cached
	var _ *int = st.Usage.Output
	var _ *int = st.Usage.Reasoning
	var _ string = st.Usage.State
	var _ string = st.Usage.Source
	var _ *float64 = st.Usage.CostUSD
	want := `{"name":"claude-code","version":"2.1.318","session_id":"sess-1","rounds":2,"usage":{"input":3,"cached":null,"output":4,"reasoning":0,"state":"partial","source":"harness","cost_usd":0.5}}`
	if got := encoded(t, "core.ExecutorState", st); got != want {
		t.Errorf("core.ExecutorState: JSON = %s, want %s", got, want)
	}
	var back core.ExecutorState
	if err := json.Unmarshal([]byte(want), &back); err != nil || !reflect.DeepEqual(back, st) {
		t.Errorf("core.ExecutorState: a checkpoint's JSON must decode to %+v, got %+v, %v", st, back, err)
	}
	st.Usage.CostUSD = nil
	if got, want := encoded(t, "core.ExecutorUsage", st.Usage), `{"input":3,"cached":null,"output":4,"reasoning":0,"state":"partial","source":"harness"}`; got != want {
		t.Errorf("core.ExecutorUsage: an unknown cost must be left out, JSON = %s, want %s", got, want)
	}
}

func TestCoreExecutorUsagePlus(t *testing.T) {
	var _ func(core.ExecutorUsage, executor.Usage, bool) core.ExecutorUsage = core.ExecutorUsage.Plus
	cost := func(v float64) *float64 { return &v }
	// What build starts an external executor's totals with.
	start := core.ExecutorUsage{Input: core.Count(0), Cached: core.Count(0), Output: core.Count(0), Reasoning: core.Count(0), State: executor.UsageComplete, Source: "harness", CostUSD: cost(0)}
	round := executor.Usage{Input: core.Count(10), Cached: core.Count(4), Output: core.Count(6), Reasoning: core.Count(1), State: executor.UsageComplete, Source: "harness", CostUSD: cost(0.25)}

	first := start.Plus(round, true)
	want := core.ExecutorUsage{Input: core.Count(10), Cached: core.Count(4), Output: core.Count(6), Reasoning: core.Count(1), State: "complete", Source: "harness", CostUSD: cost(0.25)}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("core.ExecutorUsage.Plus: first round = %+v, want %+v", first, want)
	}
	second := first.Plus(round, false)
	want = core.ExecutorUsage{Input: core.Count(20), Cached: core.Count(8), Output: core.Count(12), Reasoning: core.Count(2), State: "complete", Source: "harness", CostUSD: cost(0.5)}
	if !reflect.DeepEqual(second, want) {
		t.Errorf("core.ExecutorUsage.Plus: second round = %+v, want %+v", second, want)
	}
	if *start.Input != 0 || *start.CostUSD != 0 || *first.Input != 10 {
		t.Errorf("core.ExecutorUsage.Plus: must not change its receiver")
	}

	// Rounds that disagree on the state make it partial; a round without one is unknown.
	partial := first.Plus(executor.Usage{Input: core.Count(1), State: executor.UsagePartial}, false)
	if partial.State != "partial" {
		t.Errorf("core.ExecutorUsage.Plus: differing states must give \"partial\", got %q", partial.State)
	}
	if got := start.Plus(executor.Usage{}, true).State; got != "unknown" {
		t.Errorf("core.ExecutorUsage.Plus: a first round with no state must be \"unknown\", got %q", got)
	}
	if got := (core.ExecutorUsage{State: "unknown"}).Plus(executor.Usage{}, false).State; got != "unknown" {
		t.Errorf("core.ExecutorUsage.Plus: unknown after unknown must stay \"unknown\", got %q", got)
	}
	if got := first.Plus(executor.Usage{State: executor.UsageUnknown}, false).State; got != "partial" {
		t.Errorf("core.ExecutorUsage.Plus: unknown after complete must be \"partial\", got %q", got)
	}

	// Unknown stays unknown: a count a round did not report, and the cost.
	lacking := first.Plus(executor.Usage{Input: core.Count(1), Cached: nil, Output: core.Count(1), Reasoning: nil, State: "complete"}, false)
	if lacking.Input == nil || *lacking.Input != 11 || lacking.Cached != nil || lacking.Reasoning != nil || lacking.CostUSD != nil {
		t.Errorf("core.ExecutorUsage.Plus: counts and cost a round lacks must become unknown, got %+v", lacking)
	}
	if got := lacking.Plus(round, false); got.Cached != nil || got.Reasoning != nil || got.CostUSD != nil {
		t.Errorf("core.ExecutorUsage.Plus: unknown must stay unknown in later rounds, got %+v", got)
	}
	// The source is the round's when it names one, otherwise the totals' own.
	if got := first.Plus(executor.Usage{State: "complete"}, false).Source; got != "harness" {
		t.Errorf("core.ExecutorUsage.Plus: a round with no source keeps the totals', got %q", got)
	}
	if got := first.Plus(executor.Usage{State: "complete", Source: "provider"}, false).Source; got != "provider" {
		t.Errorf("core.ExecutorUsage.Plus: a round's source replaces the totals', got %q", got)
	}
}
