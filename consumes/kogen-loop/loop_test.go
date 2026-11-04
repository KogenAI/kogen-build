package consumes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kogen-loop/api"
	"kogen-loop/loop"
	"kogen-loop/provider"
	"kogen-loop/sys"
	"kogen-loop/tools"
)

// scripted is a loop.Requester that answers each request with the next scripted response, as
// kogen-build's attemptRequester answers through its own bookkeeping, and keeps the requests. A
// request answers with the input it was asked to send unless resend is set (a working-context
// reset sends another input).
type scripted struct {
	responses []*provider.Response
	requests  []loop.Request
	resend    func(i int, in []any) []any
}

func (s *scripted) Request(ctx context.Context, q loop.Request) (*provider.Response, []any, error) {
	s.requests = append(s.requests, q)
	i := len(s.requests) - 1
	if i >= len(s.responses) {
		return nil, nil, fmt.Errorf("the script has no response for request %d", i+1)
	}
	sent := q.Input
	if s.resend != nil {
		sent = s.resend(i, q.Input)
	}
	return s.responses[i], sent, nil
}

func answer(items ...map[string]any) *provider.Response {
	return &provider.Response{Items: items, Known: provider.KnownUsage{Input: intp(10), Cached: intp(2), Output: intp(5), Reasoning: intp(1)}, Model: "gpt-6.1-sol"}
}

// builderRound is a round like kogen-build's builder round: its own tools, finish the terminal call.
func builderRound(t *testing.T) (loop.Round, string) {
	t.Helper()
	s, root, _ := newSession(t)
	tl := &tools.Tools{
		Root: root, Session: s, Enabled: []string{tools.ReadFileTool, tools.WriteFileTool},
		Protected: []string{"protected.txt"},
	}
	rd := loop.Round{
		Owner: "build", Role: chatgptRole, Instructions: "builder policy",
		Definitions: tools.AsAny(tools.Defs(tools.ReadFileTool, tools.WriteFileTool)),
		Tools:       tl,
		Terminal:    []string{"finish"},
	}
	return rd, root
}

// A round with all five hooks, as the native builder runs it (executor_native.go), keeps the
// engine order: BeforeRequest, the request, AfterResponse, then for each call BeforeCall,
// HandleCall (or the round's tools) and AfterCall; the terminal call is not executed and not
// answered, and the calls after it still run.
func TestLoopRunEngineOrderAndHookArguments(t *testing.T) {
	var _ func(context.Context, loop.Round, []any, loop.Requester, loop.Hooks) (loop.Result, error) = loop.Run
	// loop.Requester is the one-method interface kogen-build's attemptRequester implements; its
	// method set is pinned in both directions.
	type requester = interface {
		Request(context.Context, loop.Request) (*provider.Response, []any, error)
	}
	var _ requester = loop.Requester(nil)
	var _ loop.Requester = requester(nil)
	rd, root := builderRound(t)
	var _ string = rd.Owner
	var _ provider.Role = rd.Role
	var _ string = rd.Instructions
	var _ []any = rd.Definitions
	var _ *tools.Tools = rd.Tools
	var _ []string = rd.Terminal

	big := strings.Repeat("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n", 400)
	writeFile(t, filepath.Join(root, "big.txt"), big, 0o644)
	writeFile(t, filepath.Join(root, "protected.txt"), "keep\n", 0o644)
	finish := callItem("5", "finish", map[string]any{"summary": "all done"})
	rq := &scripted{responses: []*provider.Response{answer(
		callItem("1", "report_blocker", map[string]any{"reason": "stuck"}),
		callItem("2", "write_file", map[string]any{"path": "a.txt", "content": "one\n"}),
		callItem("3", "read_file", map[string]any{"path": "big.txt"}),
		callItem("4", "write_file", map[string]any{"path": "protected.txt", "content": "changed\n"}),
		finish,
		callItem("6", "write_file", map[string]any{"path": "b.txt", "content": "two\n"}),
	)}}

	var events []string
	var beforeInputs, afterInputs []int
	var cutText string
	var hooks loop.Hooks
	hooks = loop.Hooks{
		BeforeRequest: func(ctx context.Context, input []any) error {
			events = append(events, fmt.Sprintf("request:%d", len(input)))
			return nil
		},
		AfterResponse: func(ctx context.Context, resp *provider.Response, sent []any) error {
			events = append(events, fmt.Sprintf("response:%s:%d:%d", resp.Model, len(resp.Items), len(sent)))
			return nil
		},
		BeforeCall: func(ctx context.Context, call tools.Call, input []any) error {
			events = append(events, "before:"+call.Name)
			beforeInputs = append(beforeInputs, len(input))
			return nil
		},
		HandleCall: func(ctx context.Context, call tools.Call) (loop.Handled, error) {
			if call.Name != "report_blocker" {
				events = append(events, "handle:pass:"+call.Name)
				return loop.Handled{}, nil
			}
			events = append(events, "handle:"+call.Name)
			return loop.Handled{Handled: true, Output: tools.Output{Text: "noted: " + call.Args()["reason"].(string)}}, nil
		},
		AfterCall: func(ctx context.Context, call tools.Call, out tools.Output, terminal bool, input []any) error {
			text := out.Text
			if call.Name == "read_file" {
				cutText, text = out.Text, "(cut)"
			}
			events = append(events, fmt.Sprintf("after:%s:%v:%s", call.Name, terminal, text))
			afterInputs = append(afterInputs, len(input))
			return nil
		},
	}
	var _ func(context.Context, []any) error = hooks.BeforeRequest
	var _ func(context.Context, *provider.Response, []any) error = hooks.AfterResponse
	var _ func(context.Context, tools.Call, []any) error = hooks.BeforeCall
	var _ func(context.Context, tools.Call) (loop.Handled, error) = hooks.HandleCall
	var _ func(context.Context, tools.Call, tools.Output, bool, []any) error = hooks.AfterCall

	input := []any{user("do it")}
	res, err := loop.Run(context.Background(), rd, input, rq, hooks)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"request:1", "response:gpt-6.1-sol:6:1",
		"before:report_blocker", "handle:report_blocker", "after:report_blocker:false:noted: stuck",
		"before:write_file", "handle:pass:write_file", "after:write_file:false:ok",
		"before:read_file", "handle:pass:read_file", "after:read_file:false:(cut)",
		"before:write_file", "handle:pass:write_file", "after:write_file:false:refused: protected.txt is part of the approved acceptance baseline",
		"before:finish", "after:finish:true:",
		"before:write_file", "handle:pass:write_file", "after:write_file:false:ok",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("loop.Hooks: events\n got %q\nwant %q", events, want)
	}
	// BeforeCall and AfterCall see the conversation so far: the sent input, the replayed
	// calls (6) and the outputs of the calls already answered (the terminal call has none).
	if !reflect.DeepEqual(beforeInputs, []int{7, 8, 9, 10, 11, 11}) || !reflect.DeepEqual(afterInputs, beforeInputs) {
		t.Errorf("loop.Hooks.BeforeCall and AfterCall: input lengths %v and %v, want 7 8 9 10 11 11", beforeInputs, afterInputs)
	}
	// The engine bounds a tool result before AfterCall sees it (tools.Cut).
	if len(cutText) > 16384 || !strings.Contains(cutText, "bytes omitted ...]") {
		t.Errorf("loop.Hooks.AfterCall: a %d-byte read_file result was delivered as %d bytes, uncut", len(big), len(cutText))
	}

	// What Run returns: the terminal call as received, the conversation after the round and the
	// last response.
	if res.Terminal == nil || res.Terminal.Name != "finish" || res.Terminal.Arguments != `{"summary":"all done"}` || !reflect.DeepEqual(res.Terminal.Item, finish) {
		t.Fatalf("loop.Result.Terminal = %+v", res.Terminal)
	}
	var _ *tools.Call = res.Terminal
	var _ []any = res.Input
	var _ *provider.Response = res.Response
	if res.Response != rq.responses[0] {
		t.Errorf("loop.Result.Response is not the last response: %+v", res.Response)
	}
	if len(res.Input) != 1+6+5 || res.Input[0].(map[string]any)["role"] != "user" {
		t.Fatalf("loop.Result.Input has %d items, want the input, 6 replayed calls and 5 outputs: %v", len(res.Input), res.Input)
	}
	answered := map[any]any{}
	for _, it := range res.Input[7:] {
		m := it.(map[string]any)
		if m["type"] != "function_call_output" {
			t.Fatalf("loop.Result.Input: %v is not an output", m)
		}
		answered[m["call_id"]] = m["output"]
	}
	if _, ok := answered["call-5"]; ok || answered["call-1"] != "noted: stuck" || answered["call-2"] != "ok" {
		t.Errorf("loop.Result.Input: outputs %v (the handled call is answered with its output, the terminal call is not answered)", answered)
	}
	for name, text := range map[string]string{"a.txt": "one\n", "b.txt": "two\n"} {
		if b, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(b) != text {
			t.Errorf("loop.Round.Tools: %s = %q, %v (the calls of the batch, also after the terminal one, run)", name, b, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(root, "protected.txt")); string(b) != "keep\n" {
		t.Errorf("loop.Round.Tools: the protected file was written: %q", b)
	}
	if len(rq.requests) != 1 {
		t.Fatalf("loop.Run sent %d requests, want 1", len(rq.requests))
	}

	// The request carries the round's owner, role, instructions, tool definitions and the input.
	q := rq.requests[0]
	var _ string = q.Owner
	var _ string = q.Instructions
	var _ []any = q.Input
	var _ []any = q.Tools
	if q.Owner != "build" || q.Instructions != "builder policy" || !reflect.DeepEqual(q.Input, input) || !reflect.DeepEqual(q.Tools, rd.Definitions) || len(q.Tools) != 2 {
		t.Errorf("loop.Request = %+v", q)
	}
	if q.Role != chatgptRole {
		t.Errorf("loop.Round.Role: the request names %+v, want %+v", q.Role, chatgptRole)
	}
}

// A request that answers with the input it actually sent (a working-context reset sends less)
// is what AfterResponse is told and what the conversation after the round continues from.
func TestLoopRunSentInputIsWhatTheRequesterReturned(t *testing.T) {
	rd, _ := builderRound(t)
	rq := &scripted{
		responses: []*provider.Response{
			answer(callItem("1", "write_file", map[string]any{"path": "a.txt", "content": "x"})),
			answer(textItem("I am done")),
		},
		resend: func(i int, in []any) []any {
			if i == 0 {
				return in[len(in)-1:]
			}
			return in
		},
	}
	var sentSeen [][]any
	var beforeSeen []int
	hooks := loop.Hooks{
		BeforeRequest: func(ctx context.Context, input []any) error {
			beforeSeen = append(beforeSeen, len(input))
			return nil
		},
		AfterResponse: func(ctx context.Context, resp *provider.Response, sent []any) error {
			sentSeen = append(sentSeen, sent)
			return nil
		},
	}
	res, err := loop.Run(context.Background(), rd, []any{user("first"), user("second")}, rq, hooks)
	if err != nil {
		t.Fatal(err)
	}
	// Request 1 sent the last message only; request 2 continues from that, plus the replayed
	// call and its output.
	if !reflect.DeepEqual(beforeSeen, []int{2, 3}) || len(sentSeen) != 2 || len(sentSeen[0]) != 1 || len(sentSeen[1]) != 3 {
		t.Fatalf("loop.Hooks.BeforeRequest saw %v inputs, AfterResponse saw %v", beforeSeen, sentSeen)
	}
	if len(res.Input) != 1+1+1+1 || res.Input[0].(map[string]any)["content"] != "second" {
		t.Errorf("loop.Result.Input = %v, want the sent input, the replayed call, its output and the replayed answer", res.Input)
	}
	if res.Terminal != nil || res.Response != rq.responses[1] {
		t.Errorf("loop.Result of a round that ends with an answer and no call: terminal %+v, response %+v", res.Terminal, res.Response)
	}
	if last := res.Input[len(res.Input)-1].(map[string]any); last["type"] != "message" {
		t.Errorf("loop.Result.Input does not end with the replayed answer: %v", last)
	}
}

// The round's terminal names are its own: a call to finish is an unknown tool when the round
// names another terminal, and a round with the default names ends on finish.
func TestLoopRunTerminalNamesAreTheRounds(t *testing.T) {
	rd, _ := builderRound(t)
	rd.Terminal = []string{"report_done"}
	rq := &scripted{responses: []*provider.Response{
		answer(callItem("1", "finish", map[string]any{"summary": "s"})),
		answer(callItem("2", "report_done", map[string]any{"summary": "s"})),
	}}
	var outputs []string
	hooks := loop.Hooks{AfterCall: func(ctx context.Context, call tools.Call, out tools.Output, terminal bool, input []any) error {
		outputs = append(outputs, fmt.Sprintf("%s:%v:%s", call.Name, terminal, out.Text))
		return nil
	}}
	res, err := loop.Run(context.Background(), rd, []any{user("go")}, rq, hooks)
	if err != nil || res.Terminal == nil || res.Terminal.Name != "report_done" || len(rq.requests) != 2 {
		t.Fatalf("loop.Round.Terminal: %+v, %v, %d requests", res.Terminal, err, len(rq.requests))
	}
	if !reflect.DeepEqual(outputs, []string{"finish:false:error: unknown tool finish", "report_done:true:"}) {
		t.Errorf("loop.Round.Terminal: calls %q", outputs)
	}

	rd.Terminal = nil
	rq = &scripted{responses: []*provider.Response{answer(callItem("1", "finish", map[string]any{"summary": "s"}))}}
	res, err = loop.Run(context.Background(), rd, []any{user("go")}, rq, loop.Hooks{BeforeRequest: func(context.Context, []any) error { return nil }})
	if err != nil || res.Terminal == nil || res.Terminal.Name != "finish" {
		t.Errorf("loop.Round.Terminal: a round that names none ends on finish: %+v, %v", res.Terminal, err)
	}
}

// A hook error, a request error and a cancelled context stop the round, and Run returns the error
// unchanged (executor_native.go matches the custody stops and the cancellation).
func TestLoopRunStopsWithTheErrorUnchanged(t *testing.T) {
	stop := errors.New("custody stop")
	for name, hooks := range map[string]loop.Hooks{
		"BeforeRequest": {BeforeRequest: func(context.Context, []any) error { return stop }},
		"AfterResponse": {AfterResponse: func(context.Context, *provider.Response, []any) error { return stop }},
		"BeforeCall":    {BeforeCall: func(context.Context, tools.Call, []any) error { return stop }},
		"HandleCall": {HandleCall: func(context.Context, tools.Call) (loop.Handled, error) {
			return loop.Handled{Handled: true, Output: tools.Output{Text: "x"}}, stop
		}},
		"AfterCall": {AfterCall: func(context.Context, tools.Call, tools.Output, bool, []any) error { return stop }},
	} {
		rd, root := builderRound(t)
		rq := &scripted{responses: []*provider.Response{answer(callItem("1", "write_file", map[string]any{"path": "a.txt", "content": "x"})), answer(textItem("done"))}}
		_, err := loop.Run(context.Background(), rd, []any{user("start")}, rq, hooks)
		if err != stop {
			t.Errorf("loop.Hooks.%s: Run returned %v, want the hook's error unchanged", name, err)
		}
		if name == "BeforeRequest" && len(rq.requests) != 0 {
			t.Errorf("loop.Hooks.BeforeRequest: a request was sent after its error")
		}
		if name == "BeforeCall" || name == "HandleCall" {
			if _, err := os.Stat(filepath.Join(root, "a.txt")); err == nil {
				t.Errorf("loop.Hooks.%s: the call ran after the hook's error", name)
			}
		}
		if len(rq.requests) > 1 {
			t.Errorf("loop.Hooks.%s: the round went on to request %d after the error", name, len(rq.requests))
		}
	}

	// A failing request ends the round with the requester's error.
	rd, _ := builderRound(t)
	failing := &scripted{}
	if _, err := loop.Run(context.Background(), rd, []any{user("x")}, failing, loop.Hooks{}); err == nil || !strings.Contains(err.Error(), "no response for request 1") {
		t.Errorf("loop.Run: a request error ended as %v", err)
	}

	// A cancelled context stops the round before the next call of the batch.
	rd, root := builderRound(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rq := &scripted{responses: []*provider.Response{answer(
		callItem("1", "write_file", map[string]any{"path": "a.txt", "content": "x"}),
		callItem("2", "write_file", map[string]any{"path": "b.txt", "content": "y"}),
	)}}
	hooks := loop.Hooks{AfterCall: func(ctx context.Context, call tools.Call, out tools.Output, terminal bool, input []any) error {
		cancel()
		return nil
	}}
	_, err := loop.Run(ctx, rd, []any{user("x")}, rq, hooks)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("loop.Run with a cancelled context returned %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "b.txt")); err == nil {
		t.Errorf("loop.Run: a call started after the context was cancelled")
	}
}

// A handled call is answered with its output and not run with the round's tools; Handled false
// leaves it to them.
func TestLoopHandledCallIsAnsweredNotRun(t *testing.T) {
	h := loop.Handled{Handled: true, Output: tools.Output{Text: "noted"}}
	var _ bool = h.Handled
	var _ tools.Output = h.Output
	rd, root := builderRound(t)
	rq := &scripted{responses: []*provider.Response{
		answer(callItem("1", "write_file", map[string]any{"path": "a.txt", "content": "x"})),
		answer(textItem("done")),
	}}
	hooks := loop.Hooks{HandleCall: func(context.Context, tools.Call) (loop.Handled, error) { return h, nil }}
	res, err := loop.Run(context.Background(), rd, []any{user("start")}, rq, hooks)
	if err != nil || res.Terminal != nil {
		t.Fatalf("loop.Run = %+v, %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(root, "a.txt")); err == nil {
		t.Errorf("loop.Handled: a handled call ran with the round's tools")
	}
	out := rq.requests[1].Input[len(rq.requests[1].Input)-1].(map[string]any)
	if out["type"] != "function_call_output" || out["call_id"] != "call-1" || out["output"] != "noted" {
		t.Errorf("loop.Handled.Output: the second request ends with %v", out)
	}
}

// executor_native.go sums the usages of a round's responses and states each one: a state is
// complete when every count is known, unknown when none is and partial otherwise; a sum keeps a
// count nil as soon as one usage lacks it.
func TestLoopUsageOfAndSumUsage(t *testing.T) {
	var _ func(*provider.Response) api.Usage = loop.UsageOf
	var _ func(...api.Usage) api.Usage = loop.SumUsage

	complete := loop.UsageOf(&provider.Response{Known: provider.KnownUsage{Input: intp(10), Cached: intp(2), Output: intp(5), Reasoning: intp(1)}})
	var _ *int = complete.Input
	var _ *int = complete.Cached
	var _ *int = complete.Output
	var _ *int = complete.Reasoning
	var _ string = complete.State
	if complete.State != "complete" || *complete.Input != 10 || *complete.Cached != 2 || *complete.Output != 5 || *complete.Reasoning != 1 {
		t.Fatalf("loop.UsageOf of a complete usage = %+v", complete)
	}
	if got := loop.UsageOf(&provider.Response{}); got.State != "unknown" || got.Input != nil || got.Cached != nil || got.Output != nil || got.Reasoning != nil {
		t.Errorf("loop.UsageOf of a response without usage = %+v", got)
	}
	partial := loop.UsageOf(&provider.Response{Known: provider.KnownUsage{Input: intp(10), Output: intp(5)}})
	if partial.State != "partial" || *partial.Input != 10 || partial.Cached != nil || *partial.Output != 5 || partial.Reasoning != nil {
		t.Errorf("loop.UsageOf of a partial usage = %+v", partial)
	}
	inconsistent := loop.UsageOf(&provider.Response{UsageState: provider.UsageInconsistent, Known: provider.KnownUsage{Input: intp(10), Cached: intp(2), Output: intp(5), Reasoning: intp(1)}})
	if inconsistent.State != "unknown" {
		t.Errorf("loop.UsageOf of an inconsistent usage = %+v", inconsistent)
	}

	sum := loop.SumUsage(complete, complete)
	if sum.State != "complete" || *sum.Input != 20 || *sum.Cached != 4 || *sum.Output != 10 || *sum.Reasoning != 2 {
		t.Errorf("loop.SumUsage of complete usages = %+v", sum)
	}
	mixed := loop.SumUsage(complete, partial)
	if mixed.State != "partial" || *mixed.Input != 20 || mixed.Cached != nil || *mixed.Output != 10 || mixed.Reasoning != nil {
		t.Errorf("loop.SumUsage with a partial usage = %+v", mixed)
	}
	if got := loop.SumUsage(complete, loop.UsageOf(&provider.Response{})); got.State != "unknown" || got.Input != nil {
		t.Errorf("loop.SumUsage with an unknown usage = %+v", got)
	}
	if got := loop.SumUsage(); got.State != "unknown" || got.Input != nil || got.Output != nil {
		t.Errorf("loop.SumUsage of nothing = %+v", got)
	}
	usages := []api.Usage{complete, complete, complete}
	if got := loop.SumUsage(usages...); *got.Input != 30 || got.State != "complete" {
		t.Errorf("loop.SumUsage of a slice = %+v", got)
	}
}

// The library id kogen-build puts in the native executor's version.
func TestAPILibraryIDNamesTheEngine(t *testing.T) {
	var _ string = api.Library
	if api.Library != "loop/v1" {
		t.Errorf("api.Library = %q", api.Library)
	}
	if got := "kogen-loop " + api.Library; got != "kogen-loop loop/v1" {
		t.Errorf("api.Library: the native executor's version is %q", got)
	}
}

// A caller's interrupt ends a round with the *sys.Interrupted unchanged (executor_native.go keeps
// its exit code), and the shell call it interrupts never starts.
func TestLoopRunReturnsTheInterruptOfAShellCall(t *testing.T) {
	t.Parallel()
	s, root, _ := newSession(t)
	rd := loop.Round{
		Owner: "build", Role: chatgptRole, Instructions: "builder policy",
		Definitions: tools.AsAny(tools.Defs(tools.ShellTool)),
		Tools:       &tools.Tools{Root: root, Session: s, Enabled: []string{tools.ShellTool}},
		Terminal:    []string{"finish"},
	}
	rq := &scripted{responses: []*provider.Response{answer(callItem("1", "shell", map[string]any{"command": "touch ran"}))}}
	hooks := loop.Hooks{BeforeCall: func(ctx context.Context, call tools.Call, input []any) error {
		s.Interrupt("SIGINT")
		return nil
	}}
	_, err := loop.Run(context.Background(), rd, []any{user("go")}, rq, hooks)
	var interrupted *sys.Interrupted
	if !errors.As(err, &interrupted) || interrupted.Signal != "SIGINT" || interrupted.ExitCode() != 130 {
		t.Errorf("loop.Run of an interrupted round returned %v, want the *sys.Interrupted", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ran")); err == nil {
		t.Errorf("loop.Run: the shell call started after the interrupt")
	}
}

// Whatever ends a round, the process groups its shell calls started are stopped before Run returns:
// a command the builder left running in the background does not outlive the round.
func TestLoopRunStopsTheProcessGroupsOfTheRound(t *testing.T) {
	t.Parallel()
	s, root, _ := newSession(t)
	rd := loop.Round{
		Owner: "build", Role: chatgptRole, Instructions: "builder policy",
		Definitions: tools.AsAny(tools.Defs(tools.ShellTool)),
		Tools:       &tools.Tools{Root: root, Session: s, Enabled: []string{tools.ShellTool}},
		Terminal:    []string{"finish"},
	}
	rq := &scripted{responses: []*provider.Response{
		answer(callItem("1", "shell", map[string]any{"command": "sleep 60 & echo $! > bgpid; echo started"})),
		answer(callItem("2", "finish", map[string]any{"summary": "done"})),
	}}
	var out string
	hooks := loop.Hooks{AfterCall: func(ctx context.Context, call tools.Call, o tools.Output, terminal bool, input []any) error {
		if call.Name == "shell" {
			out = o.Text
			if stopped(t, pidOf(t, filepath.Join(root, "bgpid"))) {
				t.Errorf("loop.Round.Tools: the background process is not running during the round")
			}
		}
		return nil
	}}
	res, err := loop.Run(context.Background(), rd, []any{user("go")}, rq, hooks)
	if err != nil || res.Terminal == nil {
		t.Fatalf("loop.Run = %+v, %v", res, err)
	}
	if !strings.Contains(out, "started") {
		t.Errorf("loop.Round.Tools: shell output %q", out)
	}
	if !stopped(t, pidOf(t, filepath.Join(root, "bgpid"))) {
		t.Errorf("loop.Run returned with the round's background process still running")
	}
}
