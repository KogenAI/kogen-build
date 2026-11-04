package consumes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"kogen-loop/provider"
	"kogen-loop/sys"
	"kogen-loop/tools"
)

// definitions are the model-facing definitions of the generic tools the builder sends
// (build/policy.go takes them from tools.Definition), as JSON.
var definitions = map[string]string{
	"shell":      `{"description":"Run a command with /bin/sh -c in the repository root, or retrieve it using its handle. Supply exactly one of command or handle. A new command waits up to 45 seconds; retrieval waits up to 30 seconds. Both return immediately on completion; running commands continue and return JSON with status, handle and cumulative output. Retrieval returns running or completed JSON with exit_status and cumulative output. Commands are stopped at round end. stdin is closed; stdout and stderr are combined.","name":"shell","parameters":{"additionalProperties":false,"properties":{"command":{"type":"string"},"handle":{"type":"string"}},"required":[],"type":"object"},"strict":false,"type":"function"}`,
	"read_file":  `{"description":"Read a file of the repository, with line numbers. Paths are relative to the repository root; absolute paths and symlinks must stay inside the root, and ~ is refused.","name":"read_file","parameters":{"additionalProperties":false,"properties":{"end_line":{"type":"integer"},"path":{"type":"string"},"start_line":{"type":"integer"}},"required":["path"],"type":"object"},"strict":false,"type":"function"}`,
	"view_image": `{"description":"View one selected local static non-interlaced RGB8 PNG (1..131072 bytes, dimensions 1..2048) inside the repository root; paths outside it, symlinks leaving it and ~ are refused. Read-only; no capture or provenance inference. Provider preprocessing may change resolution.","name":"view_image","parameters":{"additionalProperties":false,"properties":{"path":{"type":"string"}},"required":["path"],"type":"object"},"strict":false,"type":"function"}`,
	"write_file": `{"description":"Write the whole content of a file of the repository (created with its folders if needed).","name":"write_file","parameters":{"additionalProperties":false,"properties":{"content":{"type":"string"},"path":{"type":"string"}},"required":["path","content"],"type":"object"},"strict":false,"type":"function"}`,
}

// pngOf is a small opaque RGB8 PNG: view_image's input.
func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 40), G: uint8(y * 40), B: 99, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// gitRepo is a tools.Git that answers from a real repository, as kogen-build's adapter answers from
// Kogen's: Run is git in root, Head the commit HEAD names and Tree the candidate tree built in a
// private index. It keeps what the tools ask of it.
type gitRepo struct {
	home     string
	root     string
	runs     [][]string
	excluded [][]string
}

func (g *gitRepo) git(extra []string, args ...string) (tools.GitResult, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.root
	cmd.Env = append(hermeticEnv(g.home), extra...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	res := tools.GitResult{}
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return res, err
		}
		res.Status = ee.ExitCode()
	}
	res.Stdout, res.Stderr = out.String(), errb.String()
	return res, nil
}

func (g *gitRepo) Run(args ...string) (tools.GitResult, error) {
	g.runs = append(g.runs, args)
	return g.git(nil, args...)
}

func (g *gitRepo) Head() string {
	res, err := g.git(nil, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil || res.Status != 0 {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

func (g *gitRepo) Tree(start string, excluded []string) (string, error) {
	g.excluded = append(g.excluded, excluded)
	index := filepath.Join(g.home, "candidate-index")
	defer os.Remove(index)
	env := []string{"GIT_INDEX_FILE=" + index}
	add := []string{"add", "-A", "--", "."}
	for _, p := range excluded {
		add = append(add, ":(exclude,literal)"+p)
	}
	for _, args := range [][]string{{"read-tree", start}, add} {
		if res, err := g.git(env, args...); err != nil || res.Status != 0 {
			return "", errors.New("git " + args[0] + " failed: " + res.Stderr)
		}
	}
	res, err := g.git(env, "write-tree")
	return strings.TrimSpace(res.Stdout), err
}

func run(t *testing.T, tl *tools.Tools, name string, args map[string]any) string {
	t.Helper()
	out, err := tl.Run(name, args)
	if err != nil {
		t.Fatalf("tools.Tools.Run of %s: %v", name, err)
	}
	if out.Typed() {
		t.Fatalf("tools.Tools.Run of %s: typed result", name)
	}
	return out.Text
}

// The builder's tools (build/executor_native.go, build/host.go) are one tools.Tools of a root, the
// session that runs commands, the enabled names, the protected baseline and the excluded paths; a
// call outside the enabled set is an unknown tool, a write to a protected path is refused, and the
// results are text, never a Go error.
func TestToolsRunsTheBuildersCalls(t *testing.T) {
	var _ = tools.Tools{
		Root: "", Session: (*sys.Session)(nil), Repo: tools.Git(nil), Enabled: []string{}, Protected: []string{}, Excluded: []string{},
	}
	var _ func(*tools.Tools, tools.Call) (tools.Output, error) = (*tools.Tools).Call
	var _ func(*tools.Tools, context.Context, tools.Call) (tools.Output, error) = (*tools.Tools).CallContext
	var _ string = tools.ReadFileTool

	// tools.Tools.Repo is a tools.Git; its method set is pinned in both directions.
	type gitRunner = interface {
		Run(args ...string) (tools.GitResult, error)
		Head() string
		Tree(start string, excluded []string) (string, error)
	}
	var _ gitRunner = tools.Git(nil)
	var _ tools.Git = gitRunner(nil)

	s, root, _ := newSession(t, "FROM_SESSION=yes")
	tl := &tools.Tools{
		Root: root, Session: s, Enabled: []string{"shell", "read_file", "view_image", "write_file"},
		Protected: []string{"features/accept.feature"}, Excluded: []string{"other-intent"},
	}
	var _ string = tl.Root
	var _ *sys.Session = tl.Session
	var _ tools.Git = tl.Repo
	var _ []string = tl.Enabled
	var _ []string = tl.Protected
	var _ []string = tl.Excluded
	if tools.ReadFileTool != "read_file" {
		t.Errorf("tools.ReadFileTool = %q", tools.ReadFileTool)
	}

	// write_file under Root, relative; read_file numbers lines.
	if got := run(t, tl, "write_file", map[string]any{"path": "dir/new.txt", "content": "one\ntwo\n"}); got != "ok" {
		t.Errorf("tools.Tools.Root: write_file = %q", got)
	}
	if b, err := os.ReadFile(filepath.Join(root, "dir", "new.txt")); err != nil || string(b) != "one\ntwo\n" {
		t.Errorf("tools.Tools.Root: the file is %q, %v", b, err)
	}
	if got := run(t, tl, tools.ReadFileTool, map[string]any{"path": "dir/new.txt"}); got != "1\tone\n2\ttwo" {
		t.Errorf("tools.ReadFileTool: read_file = %q", got)
	}
	if got := run(t, tl, tools.ReadFileTool, map[string]any{"path": "dir/new.txt", "start_line": int64(2), "end_line": int64(2)}); got != "2\ttwo" {
		t.Errorf("tools.ReadFileTool: read_file of a range = %q", got)
	}
	if got := run(t, tl, tools.ReadFileTool, map[string]any{"path": "missing"}); got != "error: missing does not exist" {
		t.Errorf("tools.ReadFileTool: read_file of a missing file = %q", got)
	}
	if got := run(t, tl, tools.ReadFileTool, map[string]any{}); got != "error: argument path must be a string" {
		t.Errorf("tools.ReadFileTool: read_file without a path = %q", got)
	}
	// Protected paths are refused and nothing is created.
	if got := run(t, tl, "write_file", map[string]any{"path": "features/accept.feature", "content": "changed"}); got != "refused: features/accept.feature is part of the approved acceptance baseline" {
		t.Errorf("tools.Tools.Protected: write_file of a protected path = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "features")); err == nil {
		t.Errorf("tools.Tools.Protected: a protected write created its folder")
	}
	// The shell runs in Root with the session's environment.
	if got := run(t, tl, "shell", map[string]any{"command": "pwd -P; echo $FROM_SESSION"}); got != "exit status: 0\n"+root+"\nyes\n" {
		t.Errorf("tools.Tools.Session: shell = %q", got)
	}
	// Any other name is unknown, enabled or not.
	for _, name := range []string{"list_files", "finish", "report_blocker", "bogus"} {
		if got := run(t, tl, name, map[string]any{"command": "touch x"}); got != "error: unknown tool "+name {
			t.Errorf("tools.Tools.Enabled: %q = %q", name, got)
		}
	}
	tl.Enabled = []string{"read_file"}
	if got := run(t, tl, "shell", map[string]any{"command": "touch x"}); got != "error: unknown tool shell" {
		t.Errorf("tools.Tools.Enabled: a disabled shell = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "x")); err == nil {
		t.Errorf("tools.Tools.Enabled: a disabled shell ran")
	}

	// tools.Call and CallContext run a response's call the same way.
	call := tools.Call{Item: map[string]any{"type": "function_call", "call_id": "call-1"}, Name: "read_file", Arguments: `{"path":"dir/new.txt","end_line":1}`}
	out, err := tl.Call(call)
	if err != nil || out.Text != "1\tone" {
		t.Errorf("tools.Tools.Call = %+v, %v", out, err)
	}
	out, err = tl.CallContext(context.Background(), call)
	if err != nil || out.Text != "1\tone" {
		t.Errorf("tools.Tools.CallContext = %+v, %v", out, err)
	}
	out, err = tl.Call(tools.Call{Name: "read_file", Arguments: "oops"})
	if err != nil || out.Text != "error: argument path must be a string" {
		t.Errorf("tools.Tools.Call with invalid arguments = %+v, %v", out, err)
	}
}

// A call of a response (executor_native.go, builder.go, material.go): the item as received, its
// name and argument text, the arguments as an object and the id its answer carries.
func TestToolsCallFieldsAndArguments(t *testing.T) {
	var _ func(tools.Call) map[string]any = tools.Call.Args
	var _ func(tools.Call) any = tools.Call.CallID
	var _ func(string) map[string]any = tools.DecodeArguments

	item := map[string]any{"type": "function_call", "id": "fc-1", "call_id": "call-1", "name": "finish", "arguments": `{"summary":"done","n":2}`}
	call := tools.Call{Item: item, Name: "finish", Arguments: `{"summary":"done","n":2}`}
	var _ map[string]any = call.Item
	var _ string = call.Name
	var _ string = call.Arguments
	if call.CallID() != "call-1" || !reflect.DeepEqual(call.Item, item) || call.Name != "finish" || call.Arguments != `{"summary":"done","n":2}` {
		t.Errorf("tools.Call = %+v", call)
	}
	if got := call.Args(); got["summary"] != "done" || len(got) != 2 {
		t.Errorf("tools.Call.Args = %v", got)
	}
	if n, ok := call.Args()["n"]; !ok || fmt.Sprint(n) != "2" {
		t.Errorf("tools.Call.Args: n = %v", n)
	}
	if got := (tools.Call{Item: map[string]any{"type": "function_call"}}).CallID(); got != nil {
		t.Errorf("tools.Call.CallID of an item without one = %v, want nil", got)
	}
	if got := (tools.Call{Item: map[string]any{"call_id": 7}}).CallID(); got != 7 {
		t.Errorf("tools.Call.CallID = %v, want the id as received", got)
	}

	// Anything but an object is the invalid-argument object, so a tool reports the missing
	// argument instead of failing the conversation.
	invalid := map[string]any{"__invalid__": true}
	for _, in := range []string{"", "not json", "[1,2]", `"text"`, "42", "null", `{"a":1} trailing`} {
		if got := tools.DecodeArguments(in); !reflect.DeepEqual(got, invalid) {
			t.Errorf("tools.DecodeArguments(%q) = %v", in, got)
		}
		if got := (tools.Call{Arguments: in}).Args(); !reflect.DeepEqual(got, invalid) {
			t.Errorf("tools.Call.Args of %q = %v", in, got)
		}
	}
	if got := tools.DecodeArguments(`{"path":"a.txt","start_line":3}`); got["path"] != "a.txt" || len(got) != 2 {
		t.Errorf("tools.DecodeArguments(object) = %v", got)
	}
	// Numbers are one of the types build's line arguments accept.
	switch n := tools.DecodeArguments(`{"start_line":3}`)["start_line"].(type) {
	case int64, int, float64:
		if fmt.Sprint(n) != "3" {
			t.Errorf("tools.DecodeArguments: start_line = %v", n)
		}
	default:
		t.Errorf("tools.DecodeArguments: start_line is a %T", n)
	}
}

// The model-facing definitions the builder sends are the generic tools' own, byte for byte
// (build/policy.go): shell, read_file, view_image and write_file are defined, Kogen's own tools are not.
func TestToolsDefinitionsOfTheGenericTools(t *testing.T) {
	var _ func(string) (map[string]any, bool) = tools.Definition
	for _, name := range []string{"shell", "read_file", "view_image", "write_file"} {
		def, ok := tools.Definition(name)
		if !ok || jsonText(t, def) != definitions[name] {
			t.Errorf("tools.Definition(%q) = %s, %v\nwant %s", name, jsonText(t, def), ok, definitions[name])
		}
	}
	for _, name := range []string{"finish", "report_blocker", "probe", "bogus", ""} {
		if def, ok := tools.Definition(name); ok || def != nil {
			t.Errorf("tools.Definition(%q) = %v, %v; a name that is not a generic tool has no definition", name, def, ok)
		}
	}
}

// Delivered text is bounded to 16384 bytes with a marker that counts the bytes absent
// (build/builder.go feedback, claims and blocker text; build/records.go computes the uncut size from the
// delivered text): the head ends within the first 4096 bytes, and delivered text, marker and
// omitted bytes add up to the whole text.
func TestToolsCutBoundsTextWithAMarkerCountingTheOmission(t *testing.T) {
	var _ func(string) string = tools.Cut
	marker := regexp.MustCompile(`\n\[\.\.\. (\d+) bytes omitted \.\.\.\]\n`)

	long := strings.Repeat("0123456789", 7) + "\n"
	for name, text := range map[string]string{
		"lines":   strings.Repeat(long, 500),
		"single":  strings.Repeat("x", 30000),
		"unicode": strings.Repeat("€", 10000),
		"16385":   strings.Repeat("x", 16385),
	} {
		got := tools.Cut(text)
		if len(got) > 16384 {
			t.Errorf("tools.Cut(%s): %d bytes", name, len(got))
		}
		loc := marker.FindStringSubmatchIndex(got)
		if loc == nil || loc[0] > 4096 || loc[0] < 4000 {
			t.Errorf("tools.Cut(%s): marker at %v, want the head to end between byte 4000 and 4096", name, loc)
			continue
		}
		omitted, _ := strconv.Atoi(got[loc[2]:loc[3]])
		if want := len(got) - (loc[1] - loc[0]) + omitted; want != len(text) {
			t.Errorf("tools.Cut(%s): delivered %d bytes with %d omitted, which is %d of a %d-byte text", name, len(got), omitted, want, len(text))
		}
		if tools.Cut(got) != got {
			t.Errorf("tools.Cut(%s): cutting a cut text changed it", name)
		}
		if !strings.HasPrefix(got, text[:loc[0]]) || !strings.HasSuffix(got, text[len(text)-(len(got)-loc[1]):]) {
			t.Errorf("tools.Cut(%s): the delivered text is not the head and tail of the text", name)
		}
	}
	for _, text := range []string{"", "short", strings.Repeat("x", 16384), "line\n" + strings.Repeat("é", 8000)} {
		if got := tools.Cut(text); got != text {
			t.Errorf("tools.Cut of a text of %d bytes changed it", len(text))
		}
	}
	if got := tools.Cut("bad \xff byte"); got != "bad � byte" {
		t.Errorf("tools.Cut of invalid UTF-8 = %q", got)
	}
}

// The function_call_output that answers a call (builder.go afterRound): the call's id and the
// output, text or typed.
func TestToolsOutputItemAnswersACall(t *testing.T) {
	var _ func(any, any) map[string]any = tools.OutputItem
	got := tools.OutputItem("call-1", "1\tx")
	if !reflect.DeepEqual(got, map[string]any{"type": "function_call_output", "call_id": "call-1", "output": "1\tx"}) || len(got) != 3 {
		t.Errorf("tools.OutputItem = %v", got)
	}
	if text := jsonText(t, got); text != `{"call_id":"call-1","output":"1\tx","type":"function_call_output"}` {
		t.Errorf("tools.OutputItem as JSON = %s", text)
	}
	if got := tools.OutputItem(nil, "x"); got["call_id"] != nil || got["type"] != "function_call_output" {
		t.Errorf("tools.OutputItem of a call without an id = %v", got)
	}
	parts := []any{map[string]any{"type": "input_text", "text": "meta"}, map[string]any{"type": "input_image", "image_url": "data:image/png;base64,AA=="}}
	if got := tools.OutputItem("c", (tools.Output{Parts: parts}).Value()); !reflect.DeepEqual(got["output"], parts) {
		t.Errorf("tools.OutputItem of typed parts = %v", got)
	}
}

// What a tool returns (executor_native.go records it, records.go reads it): text, or the typed
// parts of an image; Value is the function_call_output's "output".
func TestToolsOutputIsTextOrTypedParts(t *testing.T) {
	var _ func(tools.Output) bool = tools.Output.Typed
	var _ func(tools.Output) any = tools.Output.Value

	text := tools.Output{Text: "done"}
	var _ string = text.Text
	if text.Typed() || text.Value() != "done" || text.Text != "done" {
		t.Errorf("tools.Output of text: typed %v, value %v", text.Typed(), text.Value())
	}
	parts := []any{map[string]any{"type": "input_text", "text": "meta"}, map[string]any{"type": "input_image", "image_url": "data:image/png;base64,AA=="}}
	typed := tools.Output{Parts: parts}
	if !typed.Typed() || !reflect.DeepEqual(typed.Value(), parts) || typed.Text != "" {
		t.Errorf("tools.Output of parts: typed %v, value %v, text %q", typed.Typed(), typed.Value(), typed.Text)
	}
	if empty := (tools.Output{}); empty.Typed() || empty.Value() != "" {
		t.Errorf("tools.Output of nothing: typed %v, value %v", empty.Typed(), empty.Value())
	}
}

// The replayed items of a response (executor_native.go builds the round's input from them): the
// fields the provider accepts back, in the order received, nothing for other items.
func TestToolsReplayKeepsWhatTheProviderAcceptsBack(t *testing.T) {
	var _ func([]map[string]any) []any = tools.Replay
	call := map[string]any{"type": "function_call", "id": "fc-1", "call_id": "call-1", "name": "shell", "arguments": "{}", "status": "completed", "extra": "dropped"}
	reasoning := map[string]any{"type": "reasoning", "id": "rs-1", "encrypted_content": "blob", "summary": []any{}, "extra": 1}
	msg := map[string]any{"type": "message", "id": "m-1", "role": "assistant", "status": "completed", "content": []any{
		map[string]any{"type": "output_text", "text": "hello", "annotations": []any{}},
		map[string]any{"type": "other"},
	}}
	got := tools.Replay([]map[string]any{reasoning, {"type": "web_search_call"}, call, msg, {"type": "message"}})
	want := []any{
		map[string]any{"type": "reasoning", "id": "rs-1", "encrypted_content": "blob", "summary": []any{}},
		map[string]any{"type": "function_call", "id": "fc-1", "call_id": "call-1", "name": "shell", "arguments": "{}", "status": "completed"},
		map[string]any{"type": "message", "id": "m-1", "role": "assistant", "status": "completed",
			"content": []any{map[string]any{"type": "output_text", "text": "hello"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tools.Replay = %v\nwant %v", got, want)
	}
	if got := tools.Replay(nil); len(got) != 0 {
		t.Errorf("tools.Replay(nil) = %v", got)
	}
}

// A request is measured against the input budget before it is sent (build/material.go, context.go): a
// body of exactly 524288 bytes fits, one byte more does not; a role may lower the budget.
func TestToolsMeasureAgainstTheInputBudget(t *testing.T) {
	var _ func(map[string]any, ...provider.Role) (tools.Measurement, error) = tools.Measure
	var _ int = tools.InputBudgetBytes
	var _ int = tools.OutputReserveTokens
	if tools.InputBudgetBytes != 524288 || tools.OutputReserveTokens != 131072 {
		t.Fatalf("tools.InputBudgetBytes = %d, tools.OutputReserveTokens = %d", tools.InputBudgetBytes, tools.OutputReserveTokens)
	}
	// {"x":"<n a's>"} is n+8 bytes.
	fit, err := tools.Measure(map[string]any{"x": strings.Repeat("a", 524288-8)})
	var _ int = fit.RenderedBytes
	var _ int = fit.InputBudgetBytes
	var _ bool = fit.Fits
	if err != nil || fit.RenderedBytes != 524288 || fit.InputBudgetBytes != 524288 || fit.OutputReserveTokens != 131072 || !fit.Fits {
		t.Fatalf("tools.Measure at the ceiling: %+v, %v", fit, err)
	}
	var m tools.Measurement = fit
	over, err := tools.Measure(map[string]any{"x": strings.Repeat("a", 524288-7)})
	if err != nil || over.RenderedBytes != 524289 || over.Fits || over.InputBudgetBytes != m.InputBudgetBytes {
		t.Fatalf("tools.Measure one byte over: %+v, %v", over, err)
	}
	// The rendered size is the JSON the provider receives: a request body measures as its bytes.
	body := provider.NewClient(fixtureVersion).Body(provider.Request{Role: chatgptRole, Instructions: "policy", Input: []any{user("hello")}, CacheID: "conversation-0123456789abcdef"})
	enc, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tools.Measure(body, chatgptRole)
	if err != nil || got.RenderedBytes != len(enc) || !got.Fits || got.InputBudgetBytes > tools.InputBudgetBytes || got.InputBudgetBytes <= 0 {
		t.Errorf("tools.Measure of a request body with its role = %+v, %v (%d bytes of JSON)", got, err, len(enc))
	}
	unknown, err := tools.Measure(map[string]any{"x": "y"}, provider.Role{Provider: "openai-compatible", Model: "no-such-model", BaseURL: "https://x.example.invalid/v1"})
	if err != nil || unknown.InputBudgetBytes != tools.InputBudgetBytes || !unknown.Fits {
		t.Errorf("tools.Measure with a role of an unknown family = %+v, %v", unknown, err)
	}
}

// view_image's result is typed parts: the metadata text, then the pixels. ImageEntries lists those
// outputs of a conversation (material.go) and ReadEvent is the image_read event build records
// (executor_native.go): its metadata with event, owner, call_id and status; nil for any other result.
func TestToolsReadEventOfAnImageAndOfText(t *testing.T) {
	var _ func(tools.Call, tools.Output, string) map[string]any = tools.ReadEvent
	var _ func([]any) []tools.ImageEntry = tools.ImageEntries

	root := repo(t, map[string]string{"a.txt": "text\n"})
	data := pngOf(t, 3, 2)
	writeFile(t, filepath.Join(root, "shot.png"), string(data), 0o644)
	g := &gitRepo{home: t.TempDir(), root: root}
	s := sys.NewWith(hermeticEnv(t.TempDir()), root)
	tl := &tools.Tools{Root: root, Session: s, Repo: g, Enabled: []string{"view_image", "read_file"}, Excluded: []string{"other-intent"}}
	call := tools.Call{Item: map[string]any{"type": "function_call", "call_id": "call-img"}, Name: "view_image", Arguments: `{"path":"shot.png"}`}
	out, err := tl.Call(call)
	if err != nil || !out.Typed() || len(out.Parts) != 2 {
		t.Fatalf("tools.Output of view_image = %+v, %v", out, err)
	}

	candidate, err := g.Tree(g.Head(), []string{"other-intent"})
	if err != nil || len(candidate) != 40 {
		t.Fatalf("tools.Tools.Repo: the candidate tree is %q, %v", candidate, err)
	}
	sum := sha256.Sum256(data)
	want := map[string]any{
		"event": "image_read", "owner": "build", "call_id": "call-img", "status": "read",
		"version": float64(1), "path": "shot.png", "format": "png", "bytes": float64(len(data)), "width": float64(3), "height": float64(2),
		"sha256": hex.EncodeToString(sum[:]), "read_candidate": candidate,
		"capture_candidate": "UNKNOWN", "fixture_state": "UNKNOWN", "route": "UNKNOWN", "viewport": "UNKNOWN",
	}
	event := tools.ReadEvent(call, out, "build")
	if event == nil {
		t.Fatalf("tools.ReadEvent of an image result is nil; build records the image_read event it returns")
	}
	if got := jsonRoundTrip(t, event); !reflect.DeepEqual(got, want) {
		t.Errorf("tools.ReadEvent of an image = %v\nwant %v", got, want)
	}
	if event["owner"] != "build" || event["call_id"] != "call-img" || event["event"] != "image_read" || event["status"] != "read" {
		t.Errorf("tools.ReadEvent: event %v owner %v call_id %v status %v", event["event"], event["owner"], event["call_id"], event["status"])
	}
	if got := tools.ReadEvent(call, out, "review"); got["owner"] != "review" || got["event"] != "image_read" {
		t.Errorf("tools.ReadEvent for the review: %v", got)
	}
	if got := tools.ReadEvent(tools.Call{Item: map[string]any{}}, out, "build"); got == nil || got["call_id"] != nil {
		t.Errorf("tools.ReadEvent of a call without an id: %v", got)
	}
	// Text, a refusal and parts that are not an image have no event.
	if got := tools.ReadEvent(call, tools.Output{Text: "text"}, "build"); got != nil {
		t.Errorf("tools.ReadEvent of a plain text result = %v, want nil", got)
	}
	textCall := tools.Call{Item: map[string]any{"call_id": "call-text"}, Name: "read_file", Arguments: `{"path":"a.txt"}`}
	textOut, err := tl.Call(textCall)
	if err != nil || textOut.Typed() || tools.ReadEvent(textCall, textOut, "build") != nil {
		t.Errorf("tools.ReadEvent of a read_file result = %v (%+v, %v), want nil", tools.ReadEvent(textCall, textOut, "build"), textOut, err)
	}
	bad, err := tl.Call(tools.Call{Item: map[string]any{"call_id": "call-bad"}, Name: "view_image", Arguments: `{"path":"a.txt"}`})
	if err != nil || bad.Typed() || !strings.HasPrefix(bad.Text, "error: a.txt: unreadable/nonregular artifact or invalid PNG") || tools.ReadEvent(call, bad, "build") != nil {
		t.Errorf("tools.ReadEvent of a refused image = %v (%+v, %v), want nil", tools.ReadEvent(call, bad, "build"), bad, err)
	}
	notImage := tools.Output{Parts: []any{map[string]any{"type": "input_text", "text": "only text"}}}
	if got := tools.ReadEvent(call, notImage, "build"); got != nil {
		t.Errorf("tools.ReadEvent of typed parts that are not an image = %v, want nil", got)
	}

	// ImageEntries lists the image outputs of a conversation in order, with the metadata each carried.
	second := pngOf(t, 1, 1)
	writeFile(t, filepath.Join(root, "two.png"), string(second), 0o644)
	call2 := tools.Call{Item: map[string]any{"call_id": "call-img2"}, Name: "view_image", Arguments: `{"path":"two.png"}`}
	out2, err := tl.Call(call2)
	if err != nil {
		t.Fatal(err)
	}
	input := []any{user("look"), tools.OutputItem("call-text", "1\ttext"), tools.OutputItem("call-img", out.Value()), map[string]any{"type": "function_call_output"}, tools.OutputItem("call-img2", out2.Value())}
	entries := tools.ImageEntries(input)
	if len(entries) != 2 {
		t.Fatalf("tools.ImageEntries = %+v, want the two image outputs", entries)
	}
	var entry tools.ImageEntry = entries[0]
	var _ map[string]any = entry.Meta
	if entries[0].Meta["path"] != "shot.png" || entries[0].Meta["sha256"] != hex.EncodeToString(sum[:]) || entries[0].Meta["read_candidate"] != candidate {
		t.Errorf("tools.ImageEntry.Meta of the first image = %v", entries[0].Meta)
	}
	if entries[1].Meta["path"] != "two.png" || fmt.Sprint(entries[1].Meta["width"]) != "1" {
		t.Errorf("tools.ImageEntry.Meta of the second image = %v", entries[1].Meta)
	}
	if got := tools.ImageEntries([]any{user("no image"), tools.OutputItem("c", "text")}); len(got) != 0 {
		t.Errorf("tools.ImageEntries of text = %v", got)
	}
	if got := tools.ImageEntries(nil); len(got) != 0 {
		t.Errorf("tools.ImageEntries(nil) = %v", got)
	}

	// The candidate is read through Tools.Repo (Head, then Tree without Tools.Excluded) and the
	// excluded paths are the tools' own.
	if len(g.excluded) < 2 || !reflect.DeepEqual(g.excluded[1], []string{"other-intent"}) {
		t.Errorf("tools.Tools.Repo and Excluded: Tree was asked for %v", g.excluded)
	}
	tl.Excluded = []string{"shot.png", "two.png"}
	headTree := strings.TrimSpace(gitIn(t, root, "rev-parse", "HEAD^{tree}"))
	out3, err := tl.Call(call)
	if err != nil {
		t.Fatal(err)
	}
	if e := tools.ReadEvent(call, out3, "build"); e == nil || e["read_candidate"] != headTree {
		t.Errorf("tools.Tools.Excluded: with the images excluded the candidate is %v, want HEAD's tree %s", e, headTree)
	}
}

func jsonRoundTrip(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(jsonText(t, v)), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// A request that carries images reports what became of them (build/build.go): one image_delivery
// event per image, completed, failed when the provider rejected the request, else unknown; the
// request's own result and error pass through unchanged, and a request without images emits nothing.
func TestToolsDeliverReportsWhatBecameOfTheImages(t *testing.T) {
	var _ func([]any, string, func(map[string]any), func() (*provider.Response, error)) (*provider.Response, error) = tools.Deliver

	s, root, _ := newSession(t)
	writeFile(t, filepath.Join(root, "shot.png"), string(pngOf(t, 3, 2)), 0o644)
	tl := &tools.Tools{Root: root, Session: s, Enabled: []string{"view_image"}}
	call := tools.Call{Item: map[string]any{"call_id": "call-img"}, Name: "view_image", Arguments: `{"path":"shot.png"}`}
	out, err := tl.Call(call)
	if err != nil {
		t.Fatal(err)
	}
	input := []any{user("look"), call.Output(out)}
	sha := tools.ImageEntries(input)[0].Meta["sha256"]

	var events []map[string]any
	emit := func(e map[string]any) { events = append(events, e) }
	response := &provider.Response{ID: "resp-1"}
	resp, err := tools.Deliver(input, "build", emit, func() (*provider.Response, error) { return response, nil })
	if err != nil || resp != response || len(events) != 1 {
		t.Fatalf("tools.Deliver = %+v, %v, %v", resp, err, events)
	}
	if e := events[0]; e["event"] != "image_delivery" || e["owner"] != "build" || e["status"] != "completed" || e["call_id"] != "call-img" ||
		e["response_id"] != "resp-1" || e["sha256"] != sha || e["request_id"] == "" || e["request_id"] == nil {
		t.Errorf("tools.Deliver: completed event %v", e)
	}

	events = nil
	rejected := &sys.Error{Class: "provider", Reason: "refused", Detail: "no", Exit: 4, ProviderFailure: map[string]any{"http_status": 400}}
	if resp, err := tools.Deliver(input, "review", emit, func() (*provider.Response, error) { return nil, rejected }); err != rejected || resp != nil || len(events) != 1 {
		t.Fatalf("tools.Deliver of a rejected request: %v, %v, %v", resp, err, events)
	}
	if e := events[0]; e["status"] != "failed" || e["owner"] != "review" || e["response_id"] != nil || e["call_id"] != "call-img" {
		t.Errorf("tools.Deliver: failed event %v", e)
	}

	events = nil
	lost := errors.New("connection reset")
	if _, err := tools.Deliver(input, "build", emit, func() (*provider.Response, error) { return nil, lost }); err != lost || len(events) != 1 || events[0]["status"] != "unknown" {
		t.Errorf("tools.Deliver of a request with an unknown outcome: %v, %v", err, events)
	}

	events = nil
	if resp, err := tools.Deliver([]any{user("no image")}, "build", emit, func() (*provider.Response, error) { return response, nil }); err != nil || resp != response || len(events) != 0 {
		t.Errorf("tools.Deliver without images: %v, %v, %v", resp, err, events)
	}
	if resp, err := tools.Deliver(input, "build", nil, func() (*provider.Response, error) { return response, nil }); err != nil || resp != response {
		t.Errorf("tools.Deliver without a sink: %v, %v", resp, err)
	}
}
