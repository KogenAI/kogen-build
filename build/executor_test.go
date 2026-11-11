package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

// ---- a fake external executor ----

// fakeExecutor is a scripted external executor (as claude-code is for the Build): each round
// is a function that edits the checkout and claims an ending. It records what the Build gave
// it. No real harness, process or model is involved.
type fakeExecutor struct {
	t     *testing.T
	desc  executor.Descriptor
	round func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error)

	mu       sync.Mutex
	tasks    []executor.Task
	inputs   []executor.Input
	closed   int
	startErr error
	// reportsWrites makes every round report a Write tool call, as claude-code's adapter does.
	reportsWrites bool
	events        []map[string]any
}

func newFakeExecutor(t *testing.T, round func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error)) *fakeExecutor {
	return &fakeExecutor{t: t, round: round, desc: executor.Descriptor{
		Name: executor.ClaudeCode, Contract: executor.Contract, Version: "fake-claude 9.9.9",
		Capabilities: executor.Capabilities{
			Roles: []string{executor.RoleBuild}, Resume: executor.ResumeNone, Continuation: true,
			Usage: executor.UsageHarness, Prevention: []string{executor.PreventPreToolWrite}, MaxRounds: 6,
		},
	}}
}

func (f *fakeExecutor) Describe() executor.Descriptor { return f.desc }

func (f *fakeExecutor) Start(ctx context.Context, t executor.Task, emit executor.Sink) (executor.Session, error) {
	f.mu.Lock()
	f.tasks = append(f.tasks, t)
	f.mu.Unlock()
	if f.startErr != nil {
		return nil, f.startErr
	}
	emit(map[string]any{"event": "executor_started", "model": t.Model})
	return &fakeSession{f: f, emit: emit}, nil
}

func (f *fakeExecutor) seen() []executor.Input {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]executor.Input(nil), f.inputs...)
}

type fakeSession struct {
	f    *fakeExecutor
	emit executor.Sink
	n    int
}

func (s *fakeSession) Round(ctx context.Context, in executor.Input) (executor.RoundResult, error) {
	s.n++
	s.f.mu.Lock()
	s.f.inputs = append(s.f.inputs, in)
	root := s.f.tasks[0].Root
	s.f.mu.Unlock()
	s.emit(map[string]any{"event": "executor_fake_round", "n": s.n})
	for _, event := range s.f.events {
		s.emit(event)
	}
	if s.f.reportsWrites {
		s.emit(map[string]any{"event": "executor_tool", "name": "Write"})
	}
	res, err := s.f.round(s.n, ctx, root, in)
	if err != nil {
		return executor.RoundResult{}, err
	}
	if res.Ending == "" {
		res.Ending = executor.EndFinish
	}
	if res.Handle.Executor == "" {
		res.Handle = s.Handle()
	}
	return res, nil
}

func (s *fakeSession) Handle() executor.Handle {
	return executor.Handle{Executor: executor.ClaudeCode, Version: "fake-claude 9.9.9", SessionID: "fake-session-1", Resume: executor.ResumeNone}
}

func (s *fakeSession) Close() error {
	s.f.mu.Lock()
	s.f.closed++
	s.f.mu.Unlock()
	return nil
}

func intp(n int) *int { return &n }

func costp(c float64) *float64 { return &c }

// harnessUsage is a round's complete harness-reported usage.
func harnessUsage(in, cached, out int, cost float64) executor.Usage {
	return executor.Usage{Input: intp(in), Cached: intp(cached), Output: intp(out), Reasoning: nil, State: executor.UsagePartial,
		Source: executor.UsageHarness, CostUSD: costp(cost)}
}

// externalWritesApp is a round that writes app.txt (the fixture's acceptance command is red while it
// holds "red").
func externalWritesApp(content string, usage executor.Usage) func(int, context.Context, string, executor.Input) (executor.RoundResult, error) {
	return func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte(content), 0o644); err != nil {
			return executor.RoundResult{}, err
		}
		return executor.RoundResult{Ending: executor.EndFinish, Claim: "the fixture is " + content, Usage: usage}, nil
	}
}

// ---- the project with an external build role ----

// useExecutor selects `executor: claude-code` for the build role (and the extra role keys),
// commits the configuration and approves again.
func (e *env) useExecutor(extra string) {
	e.t.Helper()
	e.setRole("build", "  build:\n    executor: claude-code\n    model: claude-sonnet-5-5\n    effort: high\n"+extra)
	e.run("git", "commit", "-qm", "build with claude-code")
	e.approve()
}

// withExecutor injects the fake as the program's claude-code executor and counts the reviews.
func withExecutor(f *fakeExecutor, reviews *int) func(*Input) {
	return func(in *Input) {
		in.External = func(role project.Role) (executor.Executor, error) { return f, nil }
		in.Reviewer = countingReviewer{approveWithoutReview{}, reviews}
	}
}

// countingReviewer is the scripted reviewer: it approves like the stub and counts.
type countingReviewer struct {
	approveWithoutReview
	n *int
}

func (r countingReviewer) Review(ctx context.Context, b *Attempt, pass verify.Pass) (Approved, error) {
	if r.n != nil {
		*r.n++
	}
	return r.approveWithoutReview.Review(ctx, b, pass)
}

// ---- the Build over a fake external executor ----

func TestExternalFinishClaimOnARedTreeGetsFeedbackNotACommit(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	start := e.head()
	var headAfterFirst string
	var reviewsAtFirst int
	reviews := 0
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		if n == 2 {
			headAfterFirst, reviewsAtFirst = e.head(), reviews
			return externalWritesApp("candidate\n", harnessUsage(200, 150, 60, 0.0125))(n, ctx, root, in)
		}
		return externalWritesApp("red\n", harnessUsage(100, 0, 40, 0.5))(n, ctx, root, in)
	})
	a := e.build(withExecutor(f, &reviews))
	if a.exit != 0 || a.err != nil {
		t.Fatalf("exit %d err %v\n%s\n%s", a.exit, a.err, a.out, a.errs)
	}
	// Round 1 was a finish claim on a red tree: no commit, no review; feedback went back.
	eq(t, "HEAD after the red round", headAfterFirst, start)
	eq(t, "reviews after the red round", reviewsAtFirst, 0)
	in := f.seen()
	if len(in) != 2 || in[0].Feedback != "" || strings.TrimSpace(in[1].Feedback) == "" {
		t.Fatalf("round inputs %+v", in)
	}
	contains(t, "feedback", in[1].Feedback, "finish")
	// Round 2 was green: native review, then the one commit.
	eq(t, "reviews", reviews, 1)
	if e.gitOut("rev-list", "--count", start+"..HEAD") != "1" {
		t.Fatal("not exactly one commit")
	}
	eq(t, "committed app.txt", e.gitOut("show", "HEAD:app.txt"), "candidate")
	if !strings.Contains(a.errs.String(), "kogen: building (round 2)") {
		t.Fatalf("progress %q", a.errs)
	}
	// The commit summary reports the executor's own, estimated figures.
	contains(t, "stdout", a.out.String(), "  executor: claude-code (harness-reported tokens input=300 cached=150 output=100 reasoning=unknown, estimate $0.5125)\n")
	if f.closed != 1 {
		t.Fatalf("session closed %d times", f.closed)
	}
}

func TestExternalTaskAndRunRecords(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	start := e.head()
	f := newFakeExecutor(t, externalWritesApp("candidate\n", harnessUsage(10, 0, 5, 0.01)))
	f.desc.Capabilities.MaxRounds = 3
	a := e.build(withExecutor(f, nil))
	if a.exit != 0 {
		t.Fatalf("exit %d %s", a.exit, a.shown())
	}
	id := e.onlyRun()
	if len(f.tasks) != 1 {
		t.Fatalf("%d tasks", len(f.tasks))
	}
	task := f.tasks[0]
	eq(t, "role", task.Role, executor.RoleBuild)
	eq(t, "root", task.Root, e.root)
	eq(t, "model", task.Model, "claude-sonnet-5-5")
	eq(t, "effort", task.Effort, "high")
	eq(t, "start tree", task.StartTree, start)
	eq(t, "run id", task.RunID, id)
	eq(t, "max rounds", task.MaxRounds, 3)
	eq(t, "home", task.Home, e.home)
	eq(t, "dir", task.Dir, e.runPath(id, "executor"))
	// The builder's structured submission is the Build's own, handed over with the task.
	eq(t, "schema", string(task.Schema), SubmissionSchema)
	if st, err := os.Stat(task.Dir); err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("task dir %v %v", st, err)
	}
	if len(task.Approval) != 64 {
		t.Fatalf("approval %q", task.Approval)
	}
	if !contains2(task.Protected, project.ConfigPath) || !contains2(task.Protected, project.FeaturePath(testSlug)) {
		t.Fatalf("protected %v", task.Protected)
	}
	if !strings.Contains(task.Brief.Stable, "# Intent: Preserve the fixture") || !strings.HasPrefix(task.Brief.Dynamic, "# Files\n\n") ||
		!strings.Contains(task.Brief.Stable, "# Protected paths") {
		t.Fatalf("brief %q / %q", task.Brief.Stable, task.Brief.Dynamic)
	}
	// Kogen's native builder prompt is not given to an external executor (E8).
	lacks(t, "brief", task.Brief.Stable, "finish")

	events := e.events(id)
	if len(eventsNamed(events, "executor_started")) != 1 || len(eventsNamed(events, "executor_fake_round")) != 1 {
		t.Fatalf("adapter events were not forwarded to the run log: %v", events)
	}
	rounds := eventsNamed(events, "executor_round")
	if len(rounds) != 1 || rounds[0]["executor"] != "claude-code" || rounds[0]["role"] != executor.RoleBuild || rounds[0]["ending"] != "finish" || rounds[0]["session_id"] != "fake-session-1" {
		t.Fatalf("executor_round %v", rounds)
	}
	if len(eventsNamed(events, "request")) != 0 || len(eventsNamed(events, "tool_call")) != 0 {
		t.Fatal("an external round has native request or tool events")
	}
	_, state := e.checkpoint(id)
	x, _ := state["executor"].(map[string]any)
	if x == nil || x["name"] != "claude-code" || x["version"] != "fake-claude 9.9.9" || x["session_id"] != "fake-session-1" || x["rounds"] != float64(1) {
		t.Fatalf("state.executor %v", x)
	}
	u := x["usage"].(map[string]any)
	if u["input"] != float64(10) || u["output"] != float64(5) || u["cached"] != float64(0) || u["reasoning"] != nil ||
		u["state"] != "partial" || u["source"] != "harness" || u["cost_usd"] != 0.01 {
		t.Fatalf("state.executor.usage %v", u)
	}
	// result.json names the executor and what it reported; a native result has no such key.
	res, _ := e.result(id)["executor"].(map[string]any)
	if res == nil || res["name"] != "claude-code" || res["rounds"] != float64(1) {
		t.Fatalf("result.executor %v", res)
	}
	// The role keeps its identity in the checkpoint's configuration.
	models := state["config"].(map[string]any)["Models"].(map[string]any)
	build := models["build"].(map[string]any)
	eq(t, "config build role", build, map[string]any{"Provider": "", "Model": "claude-sonnet-5-5", "Effort": "high", "Executor": "claude-code"})
}

func contains2(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestExternalNeedsNoProviderLoginForTheBuildRole(t *testing.T) {
	t.Parallel()
	// The build role uses Claude Code's own sign-in; only the native review needs a Kogen login.
	cfg := loadLineup(t, "  build: {executor: claude-code, model: m, effort: low}\n")
	roles := buildRoles(cfg)
	if len(roles) != 1 || roles[0].Model != cfg.Models["review"].Model {
		t.Fatalf("roles %+v", roles)
	}
	native, err := project.ParseConfig("version: 1\n", "f")
	if err != nil {
		t.Fatal(err)
	}
	if got := buildRoles(native); len(got) != 2 || got[0].Model != native.Models["build"].Model {
		t.Fatalf("native roles %+v", got)
	}
}

func TestExternalProtectedMutationStopsWithBaselineChanged(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	start := e.head()
	reviews := 0
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		// A mutation of the approved feature: not prevented here, detected by the Build.
		writeFile(t, filepath.Join(root, project.FeaturePath(testSlug)), testFeature+"# tampered\n", 0o644)
		return executor.RoundResult{Ending: executor.EndFinish, Claim: "done"}, nil
	})
	a := e.build(withExecutor(f, &reviews))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stdout", a.out.String(), "candidate/baseline_changed")
	contains(t, "stdout", a.out.String(), project.FeaturePath(testSlug))
	eq(t, "HEAD", e.head(), start)
	eq(t, "reviews", reviews, 0)
	eq(t, "rounds", len(f.seen()), 1)
}

func TestExternalHeadMoveStops(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	start := e.head()
	reviews := 0
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		cmd := exec.Command("git", "commit", "--allow-empty", "-qm", "moved by the executor")
		cmd.Dir, cmd.Env = root, e.git
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("git commit: %v\n%s", err, out)
		}
		return executor.RoundResult{Ending: executor.EndFinish, Claim: "done"}, nil
	})
	a := e.build(withExecutor(f, &reviews))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stdout", a.out.String(), "candidate/head_moved")
	if e.head() == start {
		t.Fatal("the fake did not move HEAD")
	}
	eq(t, "reviews", reviews, 0)
}

func TestExternalRoundLimitKeepsTheCandidate(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	start := e.head()
	reviews := 0
	f := newFakeExecutor(t, externalWritesApp("red\n", harnessUsage(1, 0, 1, 0.001)))
	f.desc.Capabilities.MaxRounds = 2
	a := e.build(withExecutor(f, &reviews))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stdout", a.out.String(), "candidate/executor_round_limit")
	contains(t, "stdout", a.out.String(), "2 rounds")
	eq(t, "rounds", len(f.seen()), 2)
	eq(t, "reviews", reviews, 0)
	eq(t, "HEAD", e.head(), start)
	eq(t, "candidate", e.read("app.txt"), "red\n")
	contains(t, "stdout", a.out.String(), "edited: app.txt")
	id := e.onlyRun()
	eq(t, "outcome", e.result(id)["outcome"], any("executor_round_limit"))
}

func TestExternalRoundLimitDefaultsToTheExecutorsOwn(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	reviews := 0
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		return externalWritesApp(fmt.Sprintf("red\n%d\n", n), harnessUsage(1, 0, 1, 0))(n, ctx, root, in)
	})
	f.desc.Capabilities.MaxRounds = 3
	a := e.build(withExecutor(f, &reviews))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stdout", a.out.String(), "candidate/executor_round_limit")
	eq(t, "rounds", len(f.seen()), 3)
	eq(t, "task max rounds", f.tasks[0].MaxRounds, 3)
}

func TestExternalCheckpointRefusesResume(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	f := newFakeExecutor(t, externalWritesApp("red\n", harnessUsage(1, 0, 1, 0)))
	f.desc.Capabilities.MaxRounds = 1
	a := e.build(withExecutor(f, nil))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	id := e.onlyRun()
	_, state := e.checkpoint(id)
	if state["executor"] == nil {
		t.Fatal("the checkpoint does not name its executor")
	}
	eq(t, "stopped stage", state["continuation"].(map[string]any)["stage"], any("builder_verify"))
	runs := e.runIDs()
	b := e.resume(id, func(in *ResumeInput) {
		in.External = func(project.Role) (executor.Executor, error) {
			t.Error("a refused resume must not reach the executor")
			return nil, nil
		}
	})
	if b.exit != 1 {
		t.Fatalf("resume exit %d err %v\n%s", b.exit, b.err, b.shown())
	}
	contains(t, "resume", b.shown(), "candidate/resume_refused")
	contains(t, "resume", b.shown(), "Builds by executor claude-code cannot be resumed in this version (resume capability: none)")
	contains(t, "resume", b.shown(), "inspect the preserved candidate or start a fresh Build")
	eq(t, "no request after the refusal", e.prov.count(), 0)
	eq(t, "candidate", e.read("app.txt"), "red\n")
	if !reflect.DeepEqual(e.runIDs(), runs) {
		t.Fatalf("the refused resume started a run: %v, before %v", e.runIDs(), runs)
	}
}

func TestExternalWithoutAProvidedExecutorIsUnavailable(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	a := e.build()
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	se, ok := sys.AsError(a.err)
	if !ok || se.Class != "executor" || se.Reason != executor.ReasonUnavailable {
		t.Fatalf("error %v", a.err)
	}
	contains(t, "refusal", se.Detail, "claude-code is not available in this program")
	eq(t, "runs created", len(e.runIDs()), 0)
	eq(t, "requests", e.prov.count(), 0)
}

func TestExternalDescriptorMustFitTheContract(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	f := newFakeExecutor(t, externalWritesApp("candidate\n", executor.Usage{}))
	f.desc.Contract = "kogen.executor/v0"
	a := e.build(withExecutor(f, nil))
	se, ok := sys.AsError(a.err)
	if !ok || se.Class != "executor" || se.Reason != executor.ReasonUnavailable || len(f.tasks) != 0 {
		t.Fatalf("error %v, %d tasks", a.err, len(f.tasks))
	}
}

func TestExternalStartFailureStopsTheBuild(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	f := newFakeExecutor(t, externalWritesApp("candidate\n", executor.Usage{}))
	f.startErr = executor.Fail(executor.ReasonAuthRefused, "the sign-in is not a subscription")
	a := e.build(withExecutor(f, nil))
	if a.exit != 1 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stdout", a.shown(), "executor/auth_refused")
	if _, err := os.Stat(project.ApprovalPath(e.root, testSlug)); err != nil {
		t.Fatal("the approval must stay after a failed start")
	}
}

func TestExternalInterruptKeepsTheInterruptExitCode(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	inRound := make(chan struct{})
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		close(inRound)
		<-ctx.Done()
		// The adapters return the context's cause: the Session's own interrupt.
		return executor.RoundResult{}, context.Cause(ctx)
	})
	a, done := e.start(withExecutor(f, nil))
	<-inRound
	a.s.Interrupt("SIGTERM")
	<-done
	if a.exit != 143 {
		t.Fatalf("exit %d err %v\n%s", a.exit, a.err, a.shown())
	}
	var interrupted *sys.Interrupted
	if !errors.As(a.err, &interrupted) || interrupted.Signal != "SIGTERM" {
		t.Fatalf("error %v", a.err)
	}
	contains(t, "stdout", a.out.String(), "candidate/interrupted")
}

func TestExternalInterruptReportedAsAnExecutorFailureStillExitsWithTheSignal(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	inRound := make(chan struct{})
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		close(inRound)
		<-ctx.Done()
		return executor.RoundResult{}, executor.Fail(executor.ReasonInterrupted, "the round was interrupted")
	})
	a, done := e.start(withExecutor(f, nil))
	<-inRound
	a.s.Interrupt("SIGINT")
	<-done
	if a.exit != 130 {
		t.Fatalf("exit %d err %v\n%s", a.exit, a.err, a.shown())
	}
}

func TestExternalUnsupportedEndingsAreProtocolFailures(t *testing.T) {
	t.Parallel()
	for name, ending := range map[string]executor.Ending{"blocker": executor.EndBlocker, "unknown": "completed"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.useExecutor("")
			f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
				return executor.RoundResult{Ending: ending, Claim: "x", Blocker: &executor.BlockerClaim{Paths: []string{project.ConfigPath}, Reason: "r", Evidence: "e"}}, nil
			})
			a := e.build(withExecutor(f, nil))
			if a.exit != 1 {
				t.Fatalf("exit %d\n%s", a.exit, a.shown())
			}
			contains(t, "stdout", a.out.String(), "executor/protocol")
			_, state := e.checkpoint(e.onlyRun())
			if state["blocker"] != nil {
				t.Fatal("an executor cannot report a builder-reported blocker")
			}
		})
	}
}

func TestExternalNoCallEndingGetsFeedbackLikeAFinishClaim(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.useExecutor("")
	reviews := 0
	f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
		if n == 1 {
			writeFile(t, filepath.Join(root, "app.txt"), "red\n", 0o644)
			return executor.RoundResult{Ending: executor.EndNoCall, Claim: "nothing more"}, nil
		}
		return externalWritesApp("candidate\n", executor.Usage{})(n, ctx, root, in)
	})
	a := e.build(withExecutor(f, &reviews))
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	eq(t, "rounds", len(f.seen()), 2)
	eq(t, "reviews", reviews, 1)
}

// ---- the native default is unchanged ----

func TestNativeCheckpointAndConfigCarryNoExecutorIdentity(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.prov.then(landScript(e.prov)...)
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("exit %d %s", a.exit, a.shown())
	}
	id := e.onlyRun()
	_, state := e.checkpoint(id)
	if _, ok := state["executor"]; ok {
		t.Fatal("a native checkpoint names an executor")
	}
	if _, ok := e.result(id)["executor"]; ok {
		t.Fatal("a native result names an executor")
	}
	build := state["config"].(map[string]any)["Models"].(map[string]any)["build"].(map[string]any)
	eq(t, "native build role", build, map[string]any{"Provider": "chatgpt", "Model": "gpt-6.1-sol", "Effort": "medium"})
	if len(eventsNamed(e.events(id), "executor_round")) != 0 {
		t.Fatal("a native Build logs executor rounds")
	}
	if _, err := os.Stat(e.runPath(id, "executor")); !os.IsNotExist(err) {
		t.Fatal("a native Build creates an executor folder")
	}
	contains(t, "stdout", a.out.String(), "  repairs: 0")
	lacks(t, "stdout", a.out.String(), "executor:")
}

func TestNativeSelectionIgnoresDepsExternal(t *testing.T) {
	t.Parallel()
	// executor: native (written out) is the native builder, and never asks Deps.External.
	cfg := loadLineup(t, "  build: {executor: native, model: gpt-6.1-sol, effort: medium}\n")
	ext, err := selectExecutor(Deps{External: func(project.Role) (executor.Executor, error) {
		t.Error("the native builder asked for an external executor")
		return nil, nil
	}}, cfg.Models["build"])
	if ext != nil || err != nil {
		t.Fatalf("%v %v", ext, err)
	}
}

func TestExternalFailedRoundStillDetectsCustodyChanges(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"protected", "head"} {
		for _, outcome := range []string{"error", "unknown_ending"} {
			t.Run(mutation+"/"+outcome, func(t *testing.T) {
				t.Parallel()
				e := newEnv(t)
				e.useExecutor("")
				start := e.head()
				reviews := 0
				f := newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
					if mutation == "protected" {
						writeFile(t, filepath.Join(root, project.FeaturePath(testSlug)), testFeature+"# tampered\n", 0o644)
					} else {
						e.run("git", "commit", "--allow-empty", "-qm", "moved by the executor")
					}
					if outcome == "error" {
						return executor.RoundResult{}, executor.Fail(executor.ReasonProtocol, "malformed stream")
					}
					return executor.RoundResult{Ending: "completed"}, nil
				})
				a := e.build(withExecutor(f, &reviews))
				if a.exit != 1 {
					t.Fatalf("exit %d\n%s", a.exit, a.shown())
				}
				reason := "baseline_changed"
				if mutation == "head" {
					reason = "head_moved"
					if e.head() == start {
						t.Fatal("the fake did not move HEAD")
					}
				} else {
					eq(t, "HEAD", e.head(), start)
				}
				contains(t, "stdout", a.shown(), "candidate/"+reason)
				eq(t, "reviews", reviews, 0)
				eq(t, "rounds", len(f.seen()), 1)
				eq(t, "passes", e.result(e.onlyRun())["passes"], any(float64(0)))
			})
		}
	}
}

func TestExternalBlockerFromADeclaringExecutorGoesThroughTheNativePath(t *testing.T) {
	t.Parallel()
	declare := func(f *fakeExecutor) *fakeExecutor {
		f.desc.Capabilities.Blocker = true
		return f
	}
	t.Run("valid claim stops as native", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t, failingCheck(project.ConfigPath))
		e.useExecutor("")
		f := declare(newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
			if n == 1 {
				return externalWritesApp("candidate\n", executor.Usage{})(n, ctx, root, in)
			}
			return executor.RoundResult{Ending: executor.EndBlocker, Claim: "x",
				Blocker: &executor.BlockerClaim{Paths: []string{project.ConfigPath}, Reason: "r", Evidence: "e"}}, nil
		}))
		a := e.build(withExecutor(f, nil))
		if a.exit != 1 {
			t.Fatalf("exit %d\n%s", a.exit, a.shown())
		}
		contains(t, "stdout", a.out.String(), "candidate/build_blocked")
		contains(t, "stdout", a.out.String(), "BUILDER-REPORTED")
		_, state := e.checkpoint(e.onlyRun())
		blocker, _ := state["blocker"].(map[string]any)
		if blocker == nil || blocker["classification"] != "BUILDER-REPORTED" || blocker["reason"] != "r" {
			t.Fatalf("blocker %v", state["blocker"])
		}
		eq(t, "rounds", len(f.seen()), 2)
	})
	t.Run("an unbacked claim does not stop and prefixes the next feedback", func(t *testing.T) {
		t.Parallel()
		redCheck := func(c *project.Config) {
			c.Checks = []project.Check{{Name: "app", Run: "if grep -qx red app.txt; then echo 'cannot pass: app.txt' >&2; exit 1; fi"}}
		}
		for name, c := range map[string]struct {
			config  []func(*project.Config)
			claimAt int // the round that reports the claim; the next feedback carries the message
			message string
		}{
			"no pass": {nil, 1, unbackedBlockerMessage(nil, []string{project.ConfigPath})},
			"red pass naming another path": {
				[]func(*project.Config){redCheck}, 2,
				"blocker not backed by verification: no failure of verification pass 1 names " + project.ConfigPath +
					"; no protected edit is granted; make the failing items pass, or report again with a protected path that a failure names",
			},
		} {
			e := newEnv(t, c.config...)
			e.useExecutor("")
			claim := &executor.BlockerClaim{Paths: []string{project.ConfigPath}, Reason: "r", Evidence: "e"}
			f := declare(newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
				switch {
				case n == c.claimAt:
					if n == 1 {
						writeFile(t, filepath.Join(root, "app.txt"), "red\n", 0o644)
					}
					return executor.RoundResult{Ending: executor.EndBlocker, Claim: "x", Blocker: claim}, nil
				case n < c.claimAt:
					return externalWritesApp("red\n", executor.Usage{})(n, ctx, root, in)
				}
				return externalWritesApp("candidate\n", executor.Usage{})(n, ctx, root, in)
			}))
			a := e.build(withExecutor(f, nil))
			seen := f.seen()
			if len(seen) <= c.claimAt || !strings.HasPrefix(seen[c.claimAt].Feedback, c.message) {
				t.Fatalf("%s: exit %d feedback %+v\n%s", name, a.exit, seen, a.shown())
			}
			if a.exit != 0 || strings.Contains(a.out.String(), "build_blocked") {
				t.Fatalf("%s: the claim stopped the Build\n%s", name, a.shown())
			}
			if res := e.result(e.onlyRun()); res["blocker"] != nil {
				t.Fatalf("%s: a blocker was recorded: %v", name, res["blocker"])
			}
		}
	})
	t.Run("invalid claims are refused with the native feedback", func(t *testing.T) {
		t.Parallel()
		for name, claim := range map[string]*executor.BlockerClaim{
			"unprotected": {Paths: []string{"app.txt"}, Reason: "r", Evidence: "e"},
			"empty":       {},
		} {
			e := newEnv(t)
			e.useExecutor("")
			f := declare(newFakeExecutor(t, func(n int, ctx context.Context, root string, in executor.Input) (executor.RoundResult, error) {
				if n == 1 {
					writeFile(t, filepath.Join(root, "app.txt"), "red\n", 0o644)
					return executor.RoundResult{Ending: executor.EndBlocker, Claim: "x", Blocker: claim}, nil
				}
				return externalWritesApp("candidate\n", executor.Usage{})(n, ctx, root, in)
			}))
			a := e.build(withExecutor(f, nil))
			if a.exit != 0 {
				t.Fatalf("%s: exit %d\n%s", name, a.exit, a.shown())
			}
			seen := f.seen()
			if len(seen) != 2 || !strings.HasPrefix(seen[1].Feedback, invalidBlockerMessage) {
				t.Fatalf("%s: feedback %+v", name, seen)
			}
		}
	})
}
