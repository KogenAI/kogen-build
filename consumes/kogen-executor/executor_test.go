package consumes

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/sys"
)

func TestExecutorConstants(t *testing.T) {
	// Strings build persists, logs and compares: the contract id, the executor name, the roles, the
	// resume and usage words, the write prevention and the error reasons.
	for name, tc := range map[string]struct{ got, want string }{
		"executor.Contract":          {executor.Contract, "kogen.executor/v2"},
		"executor.Native":            {executor.Native, "native"},
		"executor.RoleBuild":         {executor.RoleBuild, "build"},
		"executor.RoleReview":        {executor.RoleReview, "review"},
		"executor.ResumeExact":       {executor.ResumeExact, "exact"},
		"executor.UsageProvider":     {executor.UsageProvider, "provider"},
		"executor.UsageComplete":     {executor.UsageComplete, "complete"},
		"executor.PreventWriteTool":  {executor.PreventWriteTool, "write_tool_refusal"},
		"executor.ReasonUnavailable": {executor.ReasonUnavailable, "unavailable"},
		"executor.ReasonUsageLimit":  {executor.ReasonUsageLimit, "usage_limit"},
		"executor.ReasonProtocol":    {executor.ReasonProtocol, "protocol"},
		"executor.ReasonDeadline":    {executor.ReasonDeadline, "deadline"},
		"executor.ReasonInterrupted": {executor.ReasonInterrupted, "interrupted"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", name, tc.got, tc.want)
		}
	}
	// The endings are claims of their own type, whose text build records and checks.
	var _ executor.Ending = executor.EndFinish
	var _ executor.Ending = executor.EndNoCall
	var _ executor.Ending = executor.EndBlocker
	for name, tc := range map[string]struct {
		got  executor.Ending
		want string
	}{
		"executor.EndFinish":  {executor.EndFinish, "finish"},
		"executor.EndNoCall":  {executor.EndNoCall, "no_call"},
		"executor.EndBlocker": {executor.EndBlocker, "blocker"},
	} {
		if string(tc.got) != tc.want {
			t.Errorf("%s = %q, want %q", name, tc.got, tc.want)
		}
	}
	if executor.EndFinish == executor.EndNoCall || executor.EndNoCall == executor.EndBlocker || executor.EndFinish == executor.EndBlocker {
		t.Errorf("executor.End*: the endings must be distinct")
	}
}

func TestExecutorFail(t *testing.T) {
	var _ func(string, string) *sys.Error = executor.Fail
	var _ func(int) *sys.Error = executor.RoundLimit
	e := executor.Fail(executor.ReasonUnavailable, "executor x is not available in this program")
	if e.Class != "executor" || e.Reason != "unavailable" || e.Detail != "executor x is not available in this program" || e.Exit != 1 || len(e.Lines) != 0 {
		t.Errorf("executor.Fail = %#v, want class executor, the reason, the detail and exit 1", e)
	}
	if got, want := e.Render(), "executor/unavailable: executor x is not available in this program\n"; got != want {
		t.Errorf("executor.Fail: rendered %q, want %q", got, want)
	}
	// build tells a usage limit and an interrupt from the error's class and reason.
	for _, reason := range []string{executor.ReasonUsageLimit, executor.ReasonInterrupted, executor.ReasonProtocol, executor.ReasonDeadline} {
		var err error = executor.Fail(reason, "d")
		if got, ok := sys.AsError(err); !ok || got.Class != "executor" || got.Reason != reason {
			t.Errorf("executor.Fail: a %q stop must be recognised as a sys.Error of class executor, got %v, %v", reason, got, ok)
		}
	}

	r := executor.RoundLimit(6)
	wantDetail := "the executor used its 6 rounds without a verified, reviewed candidate; the candidate is left in place"
	if r.Class != "candidate" || r.Reason != "executor_round_limit" || r.Detail != wantDetail || r.Exit != 1 {
		t.Errorf("executor.RoundLimit(6) = %#v, want candidate/executor_round_limit %q with exit 1", r, wantDetail)
	}
	if got := executor.RoundLimit(1).Detail; got != "the executor used its 1 rounds without a verified, reviewed candidate; the candidate is left in place" {
		t.Errorf("executor.RoundLimit(1): detail %q", got)
	}
}

func serving(roles ...string) executor.Descriptor {
	return executor.Descriptor{Name: "ext", Contract: executor.Contract, Version: "1.0", Capabilities: executor.Capabilities{Roles: roles}}
}

func asSysError(t *testing.T, item string, err error) *sys.Error {
	t.Helper()
	e, ok := sys.AsError(err)
	if !ok {
		t.Fatalf("%s: want a *sys.Error, got %v", item, err)
	}
	return e
}

func TestExecutorDescriptorFor(t *testing.T) {
	var _ func(executor.Descriptor, string) (executor.RoleCapabilities, bool) = executor.Descriptor.For
	d := executor.Descriptor{
		Name: "ext", Contract: executor.Contract, Version: "1.0",
		Capabilities: executor.Capabilities{
			Roles: []string{executor.RoleBuild}, Resume: "none", Continuation: true, Usage: "harness",
			Prevention: []string{"pretool_hook_write_edit"}, Blocker: true, Images: true, MaxRounds: 4,
		},
	}
	// The Build role listed by the aggregate capabilities is served with those capabilities.
	rc, ok := d.For(executor.RoleBuild)
	if !ok || rc.Resume != "none" || !rc.Continuation || rc.Usage != "harness" || !reflect.DeepEqual(rc.Prevention, []string{"pretool_hook_write_edit"}) || !rc.Blocker || !rc.Images || rc.MaxRounds != 4 {
		t.Errorf("executor.Descriptor.For: build from the aggregate capabilities = %+v, %v", rc, ok)
	}
	// A role the aggregate does not list is not served; the aggregate is only the Build claim.
	if rc, ok := d.For(executor.RoleReview); ok || !reflect.DeepEqual(rc, executor.RoleCapabilities{}) {
		t.Errorf("executor.Descriptor.For: review without role capabilities = %+v, %v; want the zero value and false", rc, ok)
	}
	if rc, ok := serving().For(executor.RoleBuild); ok || !reflect.DeepEqual(rc, executor.RoleCapabilities{}) {
		t.Errorf("executor.Descriptor.For: build not listed = %+v, %v; want the zero value and false", rc, ok)
	}
	// An explicit role entry wins, whether or not the aggregate lists the role.
	d.RoleCapabilities = map[string]executor.RoleCapabilities{
		executor.RoleReview: {MaxRounds: 9, Blocker: true},
		executor.RoleBuild:  {MaxRounds: 2},
	}
	if rc, ok := d.For(executor.RoleReview); !ok || rc.MaxRounds != 9 || !rc.Blocker {
		t.Errorf("executor.Descriptor.For: review = %+v, %v; want its own capabilities", rc, ok)
	}
	if rc, ok := d.For(executor.RoleBuild); !ok || rc.MaxRounds != 2 || rc.Blocker {
		t.Errorf("executor.Descriptor.For: build = %+v, %v; want the role entry, not the aggregate", rc, ok)
	}
	if _, ok := d.For("other"); ok {
		t.Errorf("executor.Descriptor.For: an unknown role must not be served")
	}
}

func TestExecutorDescriptorValidate(t *testing.T) {
	var _ func(executor.Descriptor) error = executor.Descriptor.Validate
	var _ func(executor.Descriptor, string) error = executor.Descriptor.ValidateRole
	if err := serving(executor.RoleBuild).Validate(); err != nil {
		t.Errorf("executor.Descriptor.Validate: a descriptor of the contract serving build must validate, got %v", err)
	}
	if err := serving(executor.RoleBuild).ValidateRole(executor.RoleBuild); err != nil {
		t.Errorf("executor.Descriptor.ValidateRole: build must validate, got %v", err)
	}

	// A descriptor of another contract is refused as unavailable.
	wrong := serving(executor.RoleBuild)
	wrong.Contract = "kogen.executor/v1"
	for name, err := range map[string]error{"Validate": wrong.Validate(), "ValidateRole": wrong.ValidateRole(executor.RoleReview)} {
		e := asSysError(t, "executor.Descriptor."+name, err)
		if e.Class != "executor" || e.Reason != "unavailable" || e.Exit != 1 || e.Detail != "executor ext implements kogen.executor/v1, not kogen.executor/v2" {
			t.Errorf("executor.Descriptor.%s: another contract = %#v", name, e)
		}
	}

	// A descriptor that does not serve the role is refused as unavailable, naming the role.
	e := asSysError(t, "executor.Descriptor.Validate", serving(executor.RoleReview).Validate())
	if e.Class != "executor" || e.Reason != "unavailable" || e.Exit != 1 || e.Detail != "executor ext does not serve the build role" {
		t.Errorf("executor.Descriptor.Validate: build not served = %#v", e)
	}
	e = asSysError(t, "executor.Descriptor.ValidateRole", serving(executor.RoleBuild).ValidateRole(executor.RoleReview))
	if e.Class != "executor" || e.Reason != "unavailable" || e.Exit != 1 || e.Detail != "executor ext does not serve the review role" {
		t.Errorf("executor.Descriptor.ValidateRole: review not served = %#v", e)
	}
	reviewer := serving(executor.RoleBuild)
	reviewer.RoleCapabilities = map[string]executor.RoleCapabilities{executor.RoleReview: {MaxRounds: 3}}
	if err := reviewer.ValidateRole(executor.RoleReview); err != nil {
		t.Errorf("executor.Descriptor.ValidateRole: a review role entry must validate, got %v", err)
	}
	if rc, _ := reviewer.For(executor.RoleReview); rc.MaxRounds != 3 {
		t.Errorf("executor.Descriptor.For: review max rounds = %d, want 3", rc.MaxRounds)
	}
}

func TestExecutorDescriptorFields(t *testing.T) {
	// The descriptor build's native executor describes itself with.
	d := executor.Descriptor{
		Name: executor.Native, Contract: executor.Contract, Version: "kogen-loop v",
		Capabilities: executor.Capabilities{
			Roles: []string{executor.RoleBuild}, Resume: executor.ResumeExact, Continuation: true, Usage: executor.UsageProvider,
			Prevention: []string{executor.PreventWriteTool}, Blocker: true, Images: true, MaxRounds: 0,
		},
	}
	var _ string = d.Name
	var _ string = d.Contract
	var _ string = d.Version
	var _ executor.Capabilities = d.Capabilities
	c := d.Capabilities
	var _ []string = c.Roles
	var _ string = c.Resume
	var _ bool = c.Continuation
	var _ string = c.Usage
	var _ []string = c.Prevention
	var _ bool = c.Blocker
	var _ bool = c.Images
	var _ int = c.MaxRounds
	var _ bool = executor.RoleCapabilities{}.Blocker
	var _ int = executor.RoleCapabilities{}.MaxRounds
	if err := d.Validate(); err != nil {
		t.Errorf("executor.Descriptor: the native descriptor must validate, got %v", err)
	}
	if rc, ok := d.For(executor.RoleBuild); !ok || !rc.Blocker || rc.MaxRounds != 0 || rc.Resume != "exact" {
		t.Errorf("executor.Capabilities: the native build capabilities = %+v, %v", rc, ok)
	}
}

func TestExecutorTaskFields(t *testing.T) {
	deadline := time.Unix(1700000000, 0).UTC()
	task := executor.Task{
		Role: executor.RoleBuild, Root: "/checkout", Brief: executor.Brief{Stable: "stable", Dynamic: "dynamic"},
		Protected: []string{"p"}, Excluded: []string{"e"}, Model: "m", Effort: "high", Approval: "abc",
		StartTree: "tree", RunID: "run1", Deadline: deadline, MaxRounds: 6, Schema: json.RawMessage(`{"type":"object"}`),
		Home: "/home", Dir: "/run/executor",
	}
	var _ string = task.Role
	var _ string = task.Root
	var _ executor.Brief = task.Brief
	var _ string = task.Brief.Stable
	var _ string = task.Brief.Dynamic
	var _ []string = task.Protected
	var _ []string = task.Excluded
	var _ string = task.Model
	var _ string = task.Effort
	var _ string = task.Approval
	var _ string = task.StartTree
	var _ string = task.RunID
	var _ time.Time = task.Deadline
	var _ int = task.MaxRounds
	var _ json.RawMessage = task.Schema
	var _ string = task.Home
	var _ string = task.Dir
	var in executor.Input = executor.Input{Feedback: "fix it"}
	var _ string = in.Feedback
	if (executor.Task{}).Deadline.IsZero() != true || !(executor.Input{}.Feedback == "") {
		t.Errorf("executor.Task: the zero Deadline must mean none and the zero Input.Feedback the first round")
	}
}

func TestExecutorRoundResultFields(t *testing.T) {
	cost := 0.1
	res := executor.RoundResult{
		Ending: executor.EndBlocker, Claim: "claim",
		Blocker: &executor.BlockerClaim{Paths: []string{"a", "b"}, Reason: "why", Evidence: "seen"},
		Usage:   executor.Usage{Input: new(int), Cached: nil, Output: new(int), Reasoning: new(int), State: executor.UsageComplete, Source: executor.UsageProvider, CostUSD: &cost, Model: "m"},
		Handle:  executor.Handle{Role: executor.RoleBuild, Executor: executor.Native, Version: "v", SessionID: "s", Resume: executor.ResumeExact},
	}
	var _ executor.Ending = res.Ending
	var _ string = res.Claim
	var _ *executor.BlockerClaim = res.Blocker
	var _ []string = res.Blocker.Paths
	var _ string = res.Blocker.Reason
	var _ string = res.Blocker.Evidence
	var _ executor.Usage = res.Usage
	var _ *int = res.Usage.Input
	var _ *int = res.Usage.Cached
	var _ *int = res.Usage.Output
	var _ *int = res.Usage.Reasoning
	var _ string = res.Usage.State
	var _ string = res.Usage.Source
	var _ string = res.Usage.Model
	var _ executor.Handle = res.Handle
	var _ string = res.Handle.Executor
	var _ string = res.Handle.Version
	var _ string = res.Handle.SessionID
	var _ string = res.Handle.Resume

	// build writes the round's usage into the run log as it is, and an external executor's
	// structured submission decodes into the blocker claim by these keys.
	if got, want := encoded(t, "executor.Usage", res.Usage), `{"input":0,"cached":null,"cache_write":null,"output":0,"reasoning":0,"state":"complete","source":"provider","cost_usd":0.1,"model":"m"}`; got != want {
		t.Errorf("executor.Usage: JSON = %s, want %s", got, want)
	}
	if got, want := encoded(t, "executor.Usage", executor.Usage{State: "unknown", Source: "none"}), `{"input":null,"cached":null,"cache_write":null,"output":null,"reasoning":null,"state":"unknown","source":"none"}`; got != want {
		t.Errorf("executor.Usage: JSON of an unreported round = %s, want %s", got, want)
	}
	if got, want := encoded(t, "executor.BlockerClaim", res.Blocker), `{"paths":["a","b"],"reason":"why","evidence":"seen"}`; got != want {
		t.Errorf("executor.BlockerClaim: JSON = %s, want %s", got, want)
	}
	var claim executor.BlockerClaim
	if err := json.Unmarshal([]byte(`{"paths":["x"],"reason":"r","evidence":"e"}`), &claim); err != nil || !reflect.DeepEqual(claim, executor.BlockerClaim{Paths: []string{"x"}, Reason: "r", Evidence: "e"}) {
		t.Errorf("executor.BlockerClaim: a structured submission must decode by paths, reason and evidence, got %+v, %v", claim, err)
	}
	if (executor.RoundResult{}).Blocker != nil {
		t.Errorf("executor.RoundResult: a round with no claim must have a nil Blocker")
	}
}

// scriptedExecutor and scriptedSession are an external executor written against the contract.
type scriptedExecutor struct {
	desc   executor.Descriptor
	task   executor.Task
	starts int
	sess   *scriptedSession
	err    error
}

func (e *scriptedExecutor) Describe() executor.Descriptor { return e.desc }

func (e *scriptedExecutor) Start(ctx context.Context, t executor.Task, emit executor.Sink) (executor.Session, error) {
	e.starts++
	e.task = t
	if e.err != nil {
		return nil, e.err
	}
	emit(map[string]any{"event": "executor_started", "role": t.Role})
	return e.sess, nil
}

type scriptedSession struct {
	inputs  []executor.Input
	results []executor.RoundResult
	closed  int
}

func (s *scriptedSession) Round(ctx context.Context, in executor.Input) (executor.RoundResult, error) {
	s.inputs = append(s.inputs, in)
	if err := ctx.Err(); err != nil {
		return executor.RoundResult{}, executor.Fail(executor.ReasonInterrupted, "the round was interrupted")
	}
	r := s.results[0]
	s.results = s.results[1:]
	return r, nil
}

func (s *scriptedSession) Handle() executor.Handle {
	return executor.Handle{Executor: "ext", Version: "1.0", SessionID: "sess-1", Resume: "none"}
}

func (s *scriptedSession) Close() error { s.closed++; return nil }

func TestExecutorInterfaceMethodSets(t *testing.T) {
	// The complete method sets, each signature written out: an interface identical to the
	// producer's in both directions has exactly these methods.
	type exactExecutor interface {
		Describe() executor.Descriptor
		Start(ctx context.Context, t executor.Task, emit executor.Sink) (executor.Session, error)
	}
	type exactSession interface {
		Round(ctx context.Context, in executor.Input) (executor.RoundResult, error)
		Handle() executor.Handle
		Close() error
	}
	var _ exactExecutor = executor.Executor(nil)
	var _ executor.Executor = exactExecutor(nil)
	var _ exactSession = executor.Session(nil)
	var _ executor.Session = exactSession(nil)
	var _ func(executor.Executor) executor.Descriptor = executor.Executor.Describe
	var _ func(executor.Executor, context.Context, executor.Task, executor.Sink) (executor.Session, error) = executor.Executor.Start
	var _ func(executor.Session, context.Context, executor.Input) (executor.RoundResult, error) = executor.Session.Round
	var _ func(executor.Session) executor.Handle = executor.Session.Handle
	var _ func(executor.Session) error = executor.Session.Close
	var _ func(map[string]any) = executor.Sink(nil)
	var _ executor.Sink = func(map[string]any) {}
}

func TestExecutorDrivenThroughTheContract(t *testing.T) {
	// A Build selects an executor, validates its descriptor, starts a session with a task and a
	// sink, runs rounds with the feedback of the last one, reads each handle and closes it.
	blocker := &executor.BlockerClaim{Paths: []string{"f.txt"}, Reason: "contradiction", Evidence: "go test failed"}
	sess := &scriptedSession{results: []executor.RoundResult{
		{Ending: executor.EndFinish, Claim: "done", Usage: executor.Usage{State: executor.UsageComplete}},
		{Ending: executor.EndBlocker, Claim: "blocked", Blocker: blocker},
	}}
	var ex executor.Executor = &scriptedExecutor{desc: executor.Descriptor{
		Name: "ext", Contract: executor.Contract, Version: "1.0",
		Capabilities: executor.Capabilities{Roles: []string{executor.RoleBuild}, Resume: "none", Blocker: true, MaxRounds: 2},
	}, sess: sess}
	desc := ex.Describe()
	if err := desc.Validate(); err != nil || desc.Name != "ext" || desc.Capabilities.MaxRounds != 2 {
		t.Fatalf("executor.Executor.Describe: %+v, %v", desc, err)
	}
	var events []map[string]any
	task := executor.Task{Role: executor.RoleBuild, Root: t.TempDir(), Brief: executor.Brief{Stable: "s", Dynamic: "d"}, Dir: t.TempDir(), MaxRounds: 2, RunID: "r"}
	var s executor.Session
	s, err := ex.Start(context.Background(), task, func(event map[string]any) { events = append(events, event) })
	if err != nil || s == nil {
		t.Fatalf("executor.Executor.Start: %v, %v", s, err)
	}
	if got := ex.(*scriptedExecutor).task; !reflect.DeepEqual(got, task) {
		t.Errorf("executor.Executor.Start: the executor must receive the task as given, got %+v", got)
	}
	if len(events) != 1 || events[0]["event"] != "executor_started" {
		t.Errorf("executor.Sink: events are maps with an \"event\" key, got %v", events)
	}
	if h := s.Handle(); h.Executor != "ext" || h.Version != "1.0" || h.SessionID != "sess-1" || h.Resume != "none" {
		t.Errorf("executor.Session.Handle = %+v", h)
	}
	r1, err := s.Round(context.Background(), executor.Input{})
	if err != nil || r1.Ending != executor.EndFinish || r1.Claim != "done" || r1.Usage.State != "complete" {
		t.Errorf("executor.Session.Round: first round = %+v, %v", r1, err)
	}
	r2, err := s.Round(context.Background(), executor.Input{Feedback: "check failed"})
	if err != nil || r2.Ending != executor.EndBlocker || r2.Blocker == nil || !reflect.DeepEqual(*r2.Blocker, *blocker) {
		t.Errorf("executor.Session.Round: second round = %+v, %v", r2, err)
	}
	if want := []executor.Input{{}, {Feedback: "check failed"}}; !reflect.DeepEqual(sess.inputs, want) {
		t.Errorf("executor.Session.Round: inputs = %+v, want %+v", sess.inputs, want)
	}
	// A cancelled round context ends the round with the interrupted stop build recognises.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.Round(ctx, executor.Input{})
	if e, ok := sys.AsError(err); !ok || e.Class != "executor" || e.Reason != executor.ReasonInterrupted {
		t.Errorf("executor.Session.Round: a cancelled round = %v, want an executor/interrupted stop", err)
	}
	if err := s.Close(); err != nil || sess.closed != 1 {
		t.Errorf("executor.Session.Close = %v, closed %d times", err, sess.closed)
	}

	// An executor that cannot start returns its error, and no session.
	refusing := executor.Executor(&scriptedExecutor{err: executor.Fail(executor.ReasonUnavailable, "no harness")})
	if got, err := refusing.Start(context.Background(), task, func(map[string]any) {}); got != nil || !errors.As(err, new(*sys.Error)) {
		t.Errorf("executor.Executor.Start: a refusal must give no session and a sys.Error, got %v, %v", got, err)
	}
}
