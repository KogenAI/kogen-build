package consumes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/KogenAI/kogen-loop/sys"
)

// kogen-build ends a refusal with sys.Fail and reads it back with sys.AsError and errors.As:
// build.go and stages.go make the refusals, builder.go, executor_native.go and review_retry.go
// match their Class and Reason, and cmd/kogen-build prints Class, Reason and Detail.
func TestSysFailAndAsError(t *testing.T) {
	var _ func(string, string, string, int) *sys.Error = sys.Fail
	var _ func(error) (*sys.Error, bool) = sys.AsError

	e := sys.Fail("environment", "no_head", "/repo has no commits; run kogen init", 3)
	var _ string = e.Class
	var _ string = e.Reason
	var _ string = e.Detail
	var _ int = e.Exit
	var _ []string = e.Lines
	var _ map[string]any = e.ProviderFailure
	if e.Class != "environment" || e.Reason != "no_head" || e.Detail != "/repo has no commits; run kogen init" || e.Exit != 3 {
		t.Errorf("sys.Fail: %+v", e)
	}
	if e.Lines != nil || e.ProviderFailure != nil {
		t.Errorf("sys.Error.Lines and ProviderFailure of sys.Fail are not empty: %v %v", e.Lines, e.ProviderFailure)
	}
	var err error = e
	if got := err.Error(); got != "environment/no_head: /repo has no commits; run kogen init" {
		t.Errorf("sys.Error: error text %q", got)
	}

	// build.go returns the refusal as an error and the callers find it through wrapping.
	got, ok := sys.AsError(fmt.Errorf("round: %w", e))
	if !ok || got != e {
		t.Errorf("sys.AsError of a wrapped refusal = %v, %v; want the refusal itself", got, ok)
	}
	if got.Class != "environment" || got.Reason != "no_head" || got.Exit != 3 {
		t.Errorf("sys.AsError: the found error is %+v", got)
	}
	if got, ok := sys.AsError(errors.New("plain")); ok || got != nil {
		t.Errorf("sys.AsError of a plain error = %v, %v", got, ok)
	}
	if got, ok := sys.AsError(nil); ok || got != nil {
		t.Errorf("sys.AsError(nil) = %v, %v", got, ok)
	}
	// conclude (build.go) matches the refusal with errors.As and appends its notice.
	var refusal *sys.Error
	if !errors.As(fmt.Errorf("stop: %w", e), &refusal) || refusal != e {
		t.Fatalf("sys.Error: errors.As found %v", refusal)
	}
	refusal.Lines = append(refusal.Lines, "No run was created.")
	if e.Lines == nil || e.Lines[0] != "No run was created." {
		t.Errorf("sys.Error.Lines: the appended notice is not on the error: %v", e.Lines)
	}
}

// The error a stopped Build prints (stop.go) is the line, then each detail line indented by two
// spaces, each ending in a newline; the stop's reason and exit code are those of the error.
func TestSysErrorRender(t *testing.T) {
	var _ func(*sys.Error) string = (*sys.Error).Render

	e := sys.Fail("candidate", "head_moved", "HEAD moved", 1)
	if got := e.Render(); got != "candidate/head_moved: HEAD moved\n" {
		t.Errorf("sys.Error.Render without lines = %q", got)
	}
	e.Lines = []string{"paths: a, b", "reason: r"}
	if got := e.Render(); got != "candidate/head_moved: HEAD moved\n  paths: a, b\n  reason: r\n" {
		t.Errorf("sys.Error.Render with lines = %q", got)
	}
	// stopError (stop.go) builds the error literally, with the provider's diagnostics.
	lit := &sys.Error{
		Class: "candidate", Reason: "context_exhausted", Exit: 1, Lines: []string{"l1"},
		ProviderFailure: map[string]any{"http_status": 400, "request_id": "req-1"},
		Detail:          "the conversation no longer fits",
	}
	if got := lit.Render(); got != "candidate/context_exhausted: the conversation no longer fits\n  l1\n" {
		t.Errorf("sys.Error.Render of a literal = %q", got)
	}
	if lit.ProviderFailure["http_status"] != 400 || lit.ProviderFailure["request_id"] != "req-1" || len(lit.ProviderFailure) != 2 {
		t.Errorf("sys.Error.ProviderFailure = %v", lit.ProviderFailure)
	}
	if strings.Contains(lit.Render(), "req-1") {
		t.Errorf("sys.Error.Render prints the provider diagnostics: %q", lit.Render())
	}
}

// A caller's interrupt is a *sys.Interrupted whose signal gives the exit code kogen-build returns
// (build.go conclude, recover.go); the session's context is cancelled with it (cmd/kogen-build
// passes it to the Build).
func TestSysInterruptedExitCodes(t *testing.T) {
	var _ func(*sys.Session) error = (*sys.Session).Err
	var _ func(*sys.Session) context.Context = (*sys.Session).Context
	var _ func(*sys.Interrupted) int = (*sys.Interrupted).ExitCode

	for _, c := range []struct {
		signal string
		exit   int
	}{{"SIGINT", 130}, {"SIGTERM", 143}, {"SIGHUP", 129}} {
		s, _, _ := newSession(t)
		if s.Err() != nil || s.Context().Err() != nil {
			t.Fatalf("sys.Session.Err of a new session = %v, context %v", s.Err(), s.Context().Err())
		}
		if !s.Interrupt(c.signal) {
			t.Fatalf("sys.Session.Err: %s: the first interrupt did not count", c.signal)
		}
		var interrupted *sys.Interrupted
		if err := s.Err(); !errors.As(err, &interrupted) {
			t.Fatalf("sys.Session.Err after %s = %v, not a *sys.Interrupted", c.signal, err)
		}
		var _ string = interrupted.Signal
		if interrupted.Signal != c.signal || interrupted.ExitCode() != c.exit {
			t.Errorf("sys.Interrupted after %s: signal %q, exit %d, want exit %d", c.signal, interrupted.Signal, interrupted.ExitCode(), c.exit)
		}
		if s.Context().Err() == nil {
			t.Errorf("sys.Session.Context after %s is not cancelled", c.signal)
		}
		if s.Interrupt("SIGINT") {
			t.Errorf("sys.Session.Err: a second interrupt counted")
		}
		if err := s.Err(); !errors.As(err, &interrupted) || interrupted.Signal != c.signal {
			t.Errorf("sys.Session.Err after a second interrupt = %v, want the first signal %s", err, c.signal)
		}
	}
}

// cmd/kogen-build watches signals for the whole run: a SIGHUP sent to the process becomes the
// session's interrupt until the returned stop is called.
func TestSysSessionWatchSignals(t *testing.T) {
	var _ func(*sys.Session) func() = (*sys.Session).WatchSignals

	for _, c := range []struct {
		signal syscall.Signal
		name   string
		exit   int
	}{{syscall.SIGHUP, "SIGHUP", 129}, {syscall.SIGTERM, "SIGTERM", 143}, {syscall.SIGINT, "SIGINT", 130}} {
		s, _, _ := newSession(t)
		stop := s.WatchSignals()
		if err := syscall.Kill(os.Getpid(), c.signal); err != nil {
			stop()
			t.Fatal(err)
		}
		<-s.Context().Done()
		stop()
		var interrupted *sys.Interrupted
		if err := s.Err(); !errors.As(err, &interrupted) || interrupted.Signal != c.name || interrupted.ExitCode() != c.exit {
			t.Errorf("sys.Session.WatchSignals: Err() after %s = %v", c.name, err)
		}
	}
}

// The session's output goes to its Stdout (cmd/kogen-build points it at standard error) and its
// progress to standard error; Getenv and Env are the caller's environment.
func TestSysSessionOutputAndEnvironment(t *testing.T) {
	var _ func(*sys.Session, string) = (*sys.Session).Out
	var _ func(*sys.Session, string) = (*sys.Session).Puts
	var _ func(*sys.Session, string) = (*sys.Session).Progress
	var _ func(*sys.Session, string) string = (*sys.Session).Getenv

	var out, errs bytes.Buffer
	s := sys.NewWith([]string{"HOME=/h", "A=1", "A=3", "EMPTY="}, "/some/dir")
	var _ io.Writer = s.Stdout
	var _ []string = s.Env
	s.Stdout, s.Stderr = &out, &errs
	s.Out("no newline")
	s.Puts("line")
	s.Progress("working")
	if out.String() != "no newline"+"line\n" || errs.String() != "working\n" {
		t.Errorf("sys.Session.Out, Puts and Progress: stdout %q stderr %q", out.String(), errs.String())
	}
	if len(s.Env) != 4 || s.Env[0] != "HOME=/h" {
		t.Errorf("sys.Session.Env = %v", s.Env)
	}
	if s.Getenv("HOME") != "/h" || s.Getenv("A") != "3" || s.Getenv("EMPTY") != "" || s.Getenv("MISSING") != "" {
		t.Errorf("sys.Session.Getenv: HOME=%q A=%q EMPTY=%q MISSING=%q", s.Getenv("HOME"), s.Getenv("A"), s.Getenv("EMPTY"), s.Getenv("MISSING"))
	}
}

// sys.New is the session of this process (cmd/kogen-build): its environment and standard streams.
func TestSysNewTakesTheProcessEnvironment(t *testing.T) {
	var _ func() *sys.Session = sys.New

	s := sys.New()
	if len(s.Env) != len(os.Environ()) || s.Getenv("HOME") != os.Getenv("HOME") || s.Getenv("PATH") != os.Getenv("PATH") {
		t.Errorf("sys.New: %d variables (want %d), HOME %q (want %q)", len(s.Env), len(os.Environ()), s.Getenv("HOME"), os.Getenv("HOME"))
	}
	if s.Stdout != os.Stdout {
		t.Errorf("sys.New: Stdout is not the process's standard output")
	}
	s.Stdout = os.Stderr
	if s.Stdout != os.Stderr {
		t.Errorf("sys.Session.Stdout cannot be redirected")
	}
	if s.Err() != nil {
		t.Errorf("sys.New: a new session is interrupted: %v", s.Err())
	}
}

// baseline.go runs a project check with sys.Session.Run: the status and combined output come
// back in a sys.Result, Options.Env is added to the caller's environment and the command runs in
// the given directory; KillGroupOnExit leaves no process of the command's group behind.
func TestSysSessionRunResultAndOptions(t *testing.T) {
	var _ func(*sys.Session, string, string, sys.Options) (sys.Result, error) = (*sys.Session).Run
	opts := sys.Options{Env: map[string]string{"X": "x"}, KillGroupOnExit: true}
	var _ map[string]string = opts.Env
	var _ bool = opts.KillGroupOnExit

	s, root, _ := newSession(t, "A=1", "A=3")
	r, err := s.Run(`printf "$A-$X"; echo err >&2; exit 7`, root, sys.Options{Env: map[string]string{"X": "x"}})
	var _ int = r.Status
	var _ string = r.Output
	if err != nil || r.Status != 7 || r.Output != "3-xerr\n" {
		t.Errorf("sys.Session.Run, sys.Result and sys.Options.Env: %+v, %v", r, err)
	}
	r, err = s.Run("pwd -P; read x; echo \"read=$?\"", root, sys.Options{})
	if err != nil || r.Status != 0 || r.Output != root+"\nread=1\n" {
		t.Errorf("sys.Session.Run in a directory: %+v, %v (the command's standard input is closed)", r, err)
	}
	if r, err = s.Run("exit 0", root, sys.Options{}); err != nil || r != (sys.Result{}) {
		t.Errorf("sys.Result of a command without output = %+v, %v", r, err)
	}

	swept, sweptRoot, _ := newSession(t)
	r, err = swept.Run("sleep 60 & echo $! > bgpid; echo started", sweptRoot, sys.Options{KillGroupOnExit: true})
	if err != nil || r.Status != 0 || r.Output != "started\n" {
		t.Fatalf("sys.Options.KillGroupOnExit: Run = %+v, %v", r, err)
	}
	if !stopped(t, pidOf(t, filepath.Join(sweptRoot, "bgpid"))) {
		t.Errorf("sys.Options.KillGroupOnExit: the command's background process still runs when Run returns")
	}
	left, leftRoot, _ := newSession(t)
	if _, err := left.Run("sleep 60 & echo $! > bgpid; echo started", leftRoot, sys.Options{}); err != nil {
		t.Fatal(err)
	}
	pid := pidOf(t, filepath.Join(leftRoot, "bgpid"))
	if stopped(t, pid) {
		t.Errorf("sys.Options: without KillGroupOnExit the background process is gone when Run returns")
	}
	for _, g := range left.Groups() {
		syscall.Kill(-g, syscall.SIGKILL)
	}
}

// cmd/kogen-build starts with sys.MaybeSupervise(os.Args[1:]) and sys.EnableSupervisor(): the
// first returns for kogen-build's own arguments, and with the second every command runs as a
// child of this executable acting as the supervisor, in a group of its own.
func TestSysSuperviseHandshake(t *testing.T) {
	var _ func([]string) = sys.MaybeSupervise
	var _ func() = sys.EnableSupervisor

	sys.MaybeSupervise([]string{"build", "--project", ".", "--slug", "x"}) // returns: only sys.SuperviseArg makes it the supervisor
	sys.MaybeSupervise(nil)
	sys.EnableSupervisor() // already on (TestMain): calling it again changes nothing

	s, root, _ := newSession(t)
	r, err := s.Run(`ps -o pgid= -p $$; ps -o args= -p $PPID`, root, sys.Options{})
	if err != nil || r.Status != 0 {
		t.Fatalf("sys.EnableSupervisor: Run = %+v, %v", r, err)
	}
	lines := strings.Split(strings.TrimSpace(r.Output), "\n")
	groups := s.Groups()
	if len(lines) != 2 || len(groups) != 1 || strings.TrimSpace(lines[0]) != fmt.Sprint(groups[0]) || groups[0] == syscall.Getpgrp() {
		t.Fatalf("sys.EnableSupervisor: output %q, groups %v, own group %d", r.Output, groups, syscall.Getpgrp())
	}
	if !strings.Contains(lines[1], sys.SuperviseArg) {
		t.Errorf("sys.EnableSupervisor: the command's parent is %q, not the supervisor", lines[1])
	}
}

// A command that is running when the caller interrupts is stopped and Run returns the
// *sys.Interrupted (baseline.go, verification and the builder's shell end that way); nothing starts
// once the session is interrupted.
func TestSysSessionRunIsInterrupted(t *testing.T) {
	t.Parallel()
	s, root, _ := newSession(t)
	fifo := filepath.Join(root, "ready")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := s.Run("echo ready > ready; sleep 30; touch after", root, sys.Options{})
		done <- err
	}()
	f, err := os.Open(fifo) // opens once the command opens it for writing
	if err != nil {
		t.Fatal(err)
	}
	word, _ := io.ReadAll(f) // until the command closes it: it is running
	f.Close()
	if string(word) != "ready\n" {
		t.Fatalf("sys.Session.Run: the command wrote %q", word)
	}
	s.Interrupt("SIGINT")
	var interrupted *sys.Interrupted
	if err := <-done; !errors.As(err, &interrupted) || interrupted.Signal != "SIGINT" || interrupted.ExitCode() != 130 {
		t.Errorf("sys.Session.Run of an interrupted command returned %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "after")); err == nil {
		t.Errorf("sys.Session.Run: the interrupted command kept running")
	}
	r, err := s.Run("touch never", root, sys.Options{})
	if !errors.As(err, &interrupted) || r != (sys.Result{}) {
		t.Errorf("sys.Session.Run after an interrupt = %+v, %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(root, "never")); err == nil {
		t.Errorf("sys.Session.Run: a command started after the interrupt")
	}
}
