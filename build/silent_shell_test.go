package build

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// A native shell call that writes nothing until its wait elapses yields: the model gets a running
// status and an opaque handle, retrieves that handle to completion, and nothing stops: no
// command_no_progress event. The loop's real shell wait runs on a controlled clock (Deps.shellAfter),
// so no test sleeps through it.

// silentClock is one test's controlled time for the loop's shell wait, and the release of its
// silent command. The first wait it is asked for (the new command's) elapses at once, while the
// command is still running and silent. A later wait (a handle retrieval's) never elapses by
// itself: asking for it releases the command, and the retrieval returns when the command completes.
type silentClock struct {
	t       *testing.T
	command string
	release string
	mu      sync.Mutex
	asked   int
	once    sync.Once
}

// newSilentClock returns a clock whose command prints nothing until released and then prints
// "done\n". The release file is outside the repository, so the candidate tree stays clean, and
// is created at the latest when the test ends, so the command never outlives a failed test.
func newSilentClock(t *testing.T) *silentClock {
	t.Helper()
	c := &silentClock{t: t, release: filepath.Join(t.TempDir(), "release")}
	c.command = "while [ ! -e '" + c.release + "' ]; do sleep 0.05; done; printf 'done\\n'"
	t.Cleanup(c.open)
	return c
}

func (c *silentClock) open() {
	c.once.Do(func() {
		if err := os.WriteFile(c.release, nil, 0o600); err != nil {
			c.t.Errorf("release the silent command: %v", err)
		}
	})
}

// after is the Deps.shellAfter of a test.
func (c *silentClock) after(time.Duration) <-chan time.Time {
	c.mu.Lock()
	c.asked++
	first := c.asked == 1
	c.mu.Unlock()
	ch := make(chan time.Time, 1)
	if first {
		ch <- time.Now()
		return ch
	}
	c.open()
	return ch
}

// continueSilent answers the provider's turn after the silent shell call. While the command
// runs it retrieves the actual handle it observed; once it completed it checks the cumulative
// output and exit status and plays terminal, the model's verdict. Each request carries the
// whole conversation, so the newest tool output is the answer to the previous call.
func continueSilent(t *testing.T, f *fakeProvider, terminal handler) handler {
	t.Helper()
	type shellResult struct {
		Status     string `json:"status"`
		Handle     string `json:"handle"`
		Output     string `json:"output"`
		ExitStatus *int   `json:"exit_status"`
	}
	var handle string
	var next handler
	next = func(w http.ResponseWriter, r *http.Request) {
		reqs := f.requests()
		outputs := toolOutputsOf(reqs[len(reqs)-1])
		var res shellResult
		if len(outputs) == 0 || json.Unmarshal([]byte(outputs[len(outputs)-1]), &res) != nil {
			t.Errorf("the shell result is not JSON: %q", outputs)
			refused()(w, r)
			return
		}
		switch res.Status {
		case "running":
			if res.Handle == "" || (handle != "" && res.Handle != handle) {
				t.Errorf("running result %+v after handle %q", res, handle)
				refused()(w, r)
				return
			}
			handle = res.Handle
			f.then(next)
			f.calls("shell", map[string]any{"handle": handle})(w, r)
		case "completed":
			if handle == "" || res.Handle != handle || res.ExitStatus == nil || *res.ExitStatus != 0 || res.Output != "done\n" {
				t.Errorf("completion %+v for handle %q: want exit 0 and output \"done\\n\"", res, handle)
			}
			terminal(w, r)
		default:
			t.Errorf("unexpected shell result %+v", res)
			refused()(w, r)
		}
	}
	return next
}

func TestSilentBuildShellNotStopped(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	clock := newSilentClock(t)
	e.prov.then(
		e.prov.calls("shell", map[string]any{"command": clock.command}),
		continueSilent(t, e.prov, e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}, "finish", map[string]any{"summary": "done"})),
	)
	start := e.head()
	a := e.build(func(in *Input) { in.shellAfter = clock.after })
	if a.exit != 0 || a.err != nil {
		t.Fatalf("exit %d err %v\n%s\n%s", a.exit, a.err, a.out, a.errs)
	}
	eq(t, "commits", e.gitOut("rev-list", "--count", start+"..HEAD"), "1")
	// The command, at least one retrieval of its handle, then the terminal turn.
	if n := len(e.prov.requests()); n < 3 {
		t.Fatalf("%d requests: the handle was not retrieved", n)
	}
	if stops := eventsNamed(e.events(e.onlyRun()), "command_no_progress"); len(stops) != 0 {
		t.Fatalf("command_no_progress events %v", stops)
	}
}

func TestSilentReviewShellNotStopped(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	clock := newSilentClock(t)
	f.then(landScript(f)...)
	f.then(f.calls("shell", map[string]any{"command": clock.command}), continueSilent(t, f, submit(f, "approve")))
	start := e.head()
	a := e.reviewed(func(in *Input) { in.shellAfter = clock.after })
	if a.exit != 0 || a.err != nil {
		t.Fatalf("exit %d err %v\n%s\n%s", a.exit, a.err, a.out, a.errs)
	}
	eq(t, "commits", e.gitOut("rev-list", "--count", start+"..HEAD"), "1")
	if n := len(f.reviewRequests()); n < 3 {
		t.Fatalf("%d review requests: the handle was not retrieved", n)
	}
	if stops := eventsNamed(e.events(e.onlyRun()), "command_no_progress"); len(stops) != 0 {
		t.Fatalf("command_no_progress events %v", stops)
	}
}
