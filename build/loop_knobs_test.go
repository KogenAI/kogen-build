package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The defaults of the native builder and the independent review: tool results bounded at 16384
// bytes, the 524288-byte input budget and no authority breakpoint.

// knobShell is a shell command whose output is far over any result bound under test.
const knobShell = "head -c 30000 /dev/zero | tr '\\000' a"

var knobMarker = regexp.MustCompile(`\n\[\.\.\. (\d+) bytes omitted \.\.\.\]\n`)

// knobCut checks what kogen-build relies on from a cut tool result: it fits the bound and carries
// the omission marker. Where kogen-loop cuts and which bytes it keeps are its own.
func knobCut(t *testing.T, what, out string, bound int) {
	t.Helper()
	if len(out) > bound {
		t.Fatalf("%s: %d bytes delivered, want at most %d", what, len(out), bound)
	}
	if !knobMarker.MatchString(out) {
		t.Fatalf("%s: no omission marker in %d bytes: %.200q", what, len(out), out)
	}
}

// knobOutput is the one delivered tool result that was cut.
func knobOutput(t *testing.T, outputs []string) string {
	t.Helper()
	for _, o := range outputs {
		if strings.Contains(o, "bytes omitted") {
			return o
		}
	}
	t.Fatalf("no cut tool result among %d", len(outputs))
	return ""
}

// anthropicRole makes the role of the fixture's Build an Anthropic one with a saved key and
// approves the changed configuration.
func anthropicRole(e *env, role string) {
	e.t.Helper()
	e.setRole(role, "  "+role+": {provider: anthropic, model: claude-sonnet-5-5, effort: high}\n")
	e.run("git", "commit", "-qm", "anthropic "+role+" role")
	e.approve()
	dir := filepath.Join(e.home, ".kogen", "credentials")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		e.t.Fatal(err)
	}
	key, _ := json.Marshal(map[string]any{"provider": "anthropic", "api_key": "sk-ant-FIXTURE-KEY"})
	if err := os.WriteFile(filepath.Join(dir, "anthropic.json"), key, 0o600); err != nil {
		e.t.Fatal(err)
	}
}

// knobMarks counts the cache_control locations of an Anthropic request's messages and says
// whether the first content block of the first message carries one.
func knobMarks(body map[string]any) (total int, first bool) {
	var walk func(v any) int
	walk = func(v any) int {
		n := 0
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				n += walk(e)
			}
		case map[string]any:
			if x["cache_control"] != nil {
				n++
			}
			for _, e := range x {
				n += walk(e)
			}
		}
		return n
	}
	messages, _ := body["messages"].([]any)
	if len(messages) == 0 {
		return 0, false
	}
	head, _ := messages[0].(map[string]any)
	blocks, _ := head["content"].([]any)
	if len(blocks) > 0 {
		block, _ := blocks[0].(map[string]any)
		first = block["cache_control"] != nil
	}
	return walk(messages), first
}

func bigDiagnostic() *Diagnostic {
	text := strings.Repeat("diagnostic line\n", 6400)
	return &Diagnostic{Text: text, SourcePath: "/diagnostic/label.txt", SHA256: digestOf(text)}
}

func TestLoopKnobsNativeBuilder(t *testing.T) {
	t.Parallel()
	t.Run("a long tool result is cut within 16384 bytes and still delivered", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.prov.then(e.prov.calls("shell", map[string]any{"command": knobShell}))
		e.prov.then(landScript(e.prov)...)
		a := e.build()
		if a.exit != 0 {
			t.Fatalf("exit %d: %s", a.exit, a.shown())
		}
		knobCut(t, "shell result", knobOutput(t, e.prov.toolOutputs()), 16384)
	})

	t.Run("a large diagnostic fits the input budget and is sent", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.prov.then(landScript(e.prov)...)
		if a := e.build(func(in *Input) { in.Context = bigDiagnostic() }); a.exit != 0 {
			t.Fatalf("exit %d: %s", a.exit, a.shown())
		}
	})

	t.Run("the sent request carries no authority breakpoint", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		anthropicRole(e, "build")
		e.prov.then(refused())
		e.build()
		reqs := e.prov.requests()
		if len(reqs) == 0 {
			t.Fatal("no request reached the provider")
		}
		if _, first := knobMarks(reqs[0]); first {
			t.Fatal("the first block carries a breakpoint")
		}
	})
}

func TestLoopKnobsReview(t *testing.T) {
	t.Parallel()
	bigApp := "// candidate\n" + strings.Repeat("padding of the candidate source line\n", 1800)

	t.Run("a long tool result is cut within 16384 bytes and still delivered", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := e.prov
		f.then(landScript(f)...)
		f.then(f.calls("shell", map[string]any{"command": knobShell}), submit(f, "approve"))
		if a := e.reviewed(); a.exit != 0 {
			t.Fatalf("exit %d: %s", a.exit, a.shown())
		}
		var outputs []string
		for _, req := range f.reviewRequests() {
			items, _ := req["input"].([]any)
			for _, it := range items {
				if m, _ := it.(map[string]any); m["type"] == "function_call_output" {
					outputs = append(outputs, fmt.Sprint(m["output"]))
				}
			}
		}
		knobCut(t, "reviewer shell result", knobOutput(t, outputs), 16384)
	})

	t.Run("the candidate source stays inline within the input budget", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := e.prov
		f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": bigApp}, "finish", map[string]any{"summary": "done"}))
		f.then(submit(f, "approve"))
		if a := e.reviewed(); a.exit != 0 {
			t.Fatalf("exit %d: %s", a.exit, a.shown())
		}
		reqs := f.reviewRequests()
		eq(t, "review requests", len(reqs), 1)
		text := requestText(reqs[0])
		contains(t, "inline source", text, "padding of the candidate source line")
		lacks(t, "catalog", text, "Pinned complete candidate source evidence")
	})

	t.Run("the sent request carries no authority breakpoint", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		anthropicRole(e, "review")
		f := e.prov
		f.then(landScript(f)...)
		f.then(refused())
		e.reviewed()
		for _, req := range f.requests() {
			if _, anth := req["messages"]; anth {
				if _, first := knobMarks(req); first {
					t.Fatal("the first block carries a breakpoint")
				}
				return
			}
		}
		t.Fatal("no review request reached the provider")
	})
}
