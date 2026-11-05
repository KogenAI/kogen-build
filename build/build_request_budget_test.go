package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// redTry is a builder response that leaves the candidate red and finishes: the pass fails
// and the same builder gets the feedback. Try n writes content no other try writes, so that each
// round edits the candidate and is verified.
func redTry(f *fakeProvider, n int) handler {
	return f.calls("write_file", map[string]any{"path": "app.txt", "content": redContent(n)}, "finish", map[string]any{"summary": "try"})
}

func redContent(n int) string { return fmt.Sprintf("red\n%d\n", n) }

// budgetOf is the saved budget of a checkpoint's state (nil when absent).
func budgetOf(state map[string]any) any { return state["build_request_budget"] }

func budgetEvents(e *env, id string) []map[string]any {
	return eventsNamed(e.events(id), "build_request_budget")
}

func TestBuildRequestBudgetAcrossPasses(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("stops before request %d", limit+1), func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			f := e.prov
			for i := 0; i < limit+2; i++ {
				f.then(redTry(f, i+1))
			}
			start := e.head()
			a := e.build(func(in *Input) { in.BuildRequestBudget = limit })
			if a.exit != 1 || f.count() != limit || e.head() != start {
				t.Fatalf("exit=%d requests=%d head changed=%v: %s", a.exit, f.count(), e.head() != start, a.shown())
			}
			contains(t, "stdout", a.shown(), "candidate/build_request_budget")
			id := e.onlyRun()
			res := e.result(id)
			eq(t, "outcome", res["outcome"], any("build_request_budget"))
			eq(t, "passes", res["passes"], any(float64(limit)))
			eq(t, "retained candidate", e.read("app.txt"), redContent(limit))
			ev := budgetEvents(e, id)
			if len(ev) != 1 || ev[0]["role"] != "build" || ev[0]["count"] != float64(limit) || ev[0]["limit"] != float64(limit) {
				t.Fatalf("events: %v", ev)
			}
			doc, state := e.checkpoint(id)
			eq(t, "budget", budgetOf(state), any(map[string]any{"limit": float64(limit), "count": float64(limit)}))
			if doc["approval"] == "" || e.gitOut("status", "--porcelain", "app.txt") == "" {
				t.Fatalf("checkpoint approval or candidate lost: %v", doc["approval"])
			}
			if _, err := os.Stat(e.root + "/.kogen/local/approvals/fixture.json"); err != nil {
				t.Fatalf("approval not preserved: %v", err)
			}
		})
		t.Run(fmt.Sprintf("finish on request %d lands", limit), func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			f := e.prov
			for i := 1; i < limit; i++ {
				f.then(redTry(f, i))
			}
			f.then(landScript(f)...)
			start := e.head()
			a := e.build(func(in *Input) { in.BuildRequestBudget = limit })
			if a.exit != 0 || f.count() != limit || e.gitOut("rev-list", "--count", start+"..HEAD") != "1" {
				t.Fatalf("exit=%d requests=%d: %s", a.exit, f.count(), a.shown())
			}
			if ev := budgetEvents(e, e.onlyRun()); len(ev) != 0 {
				t.Fatalf("events: %v", ev)
			}
		})
	}
	t.Run("a response batch completes before the stop", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := e.prov
		f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": "partial\n"}, "read_file", map[string]any{"path": "app.txt"}))
		f.then(landScript(f)...)
		a := e.build(func(in *Input) { in.BuildRequestBudget = 1 })
		if a.exit != 1 || f.count() != 1 {
			t.Fatalf("exit=%d requests=%d: %s", a.exit, f.count(), a.shown())
		}
		eq(t, "write ran", e.read("app.txt"), "partial\n")
		id := e.onlyRun()
		_, state := e.checkpoint(id)
		cont := state["continuation"].(map[string]any)
		input, _ := cont["input"].([]any)
		var outputs int
		for _, it := range input {
			if m, _ := it.(map[string]any); m["type"] == "function_call_output" {
				outputs++
			}
		}
		eq(t, "tool outputs kept", outputs, 2)
		eq(t, "events", len(budgetEvents(e, id)), 1)
	})
}

func TestBuildRequestBudgetResume(t *testing.T) {
	t.Parallel()
	t.Run("interrupted, resumed and exhausted", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := e.prov
		start := e.head()
		opened := make(chan struct{})
		f.then(f.calls("read_file", map[string]any{"path": "app.txt"}), hang(opened))
		a, done := e.start(func(in *Input) { in.BuildRequestBudget = 4 })
		<-opened
		a.s.Interrupt("SIGINT")
		<-done
		eq(t, "interrupted exit", a.exit, 130)
		id := e.onlyRun()
		_, state := e.checkpoint(id)
		eq(t, "saved budget", budgetOf(state), any(map[string]any{"limit": float64(4), "count": float64(2)}))
		before := e.result(id)

		f.replace(f.calls("read_file", map[string]any{"path": "app.txt"}), f.calls("read_file", map[string]any{"path": "app.txt"}),
			f.calls("read_file", map[string]any{"path": "app.txt"}), f.calls("finish", map[string]any{"summary": "late"}))
		r := e.resume(id)
		if r.exit != 1 || f.count() != 4 || e.head() != start {
			t.Fatalf("exit=%d requests=%d: %s", r.exit, f.count(), r.shown())
		}
		child := ""
		for _, c := range e.runIDs() {
			if c != id {
				child = c
			}
		}
		eq(t, "outcome", e.result(child)["outcome"], any("build_request_budget"))
		ev := budgetEvents(e, child)
		if len(ev) != 1 || ev[0]["count"] != float64(4) || ev[0]["limit"] != float64(4) || ev[0]["role"] != "build" {
			t.Fatalf("events: %v", ev)
		}
		_, state = e.checkpoint(child)
		eq(t, "child budget", budgetOf(state), any(map[string]any{"limit": float64(4), "count": float64(4)}))

		// An exhausted Build sends nothing more, however often it is resumed.
		again := e.resume(child)
		if again.exit != 1 || f.count() != 4 {
			t.Fatalf("exit=%d requests=%d: %s", again.exit, f.count(), again.shown())
		}
		var grand string
		for _, c := range e.runIDs() {
			if c != id && c != child {
				grand = c
			}
		}
		eq(t, "outcome", e.result(grand)["outcome"], any("build_request_budget"))
		if ev := budgetEvents(e, grand); len(ev) != 1 || ev[0]["count"] != float64(4) || ev[0]["limit"] != float64(4) {
			t.Fatalf("events: %v", ev)
		}
		// The earlier runs keep their evidence.
		eq(t, "interrupted result", e.result(id), before)
		eq(t, "candidate", e.head(), start)
	})
	t.Run("a failed request is an attempt", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := e.prov
		f.then(f.calls("read_file", map[string]any{"path": "app.txt"}), failure(500, "server_error", "scripted failure"))
		a := e.build(func(in *Input) { in.BuildRequestBudget = 2 })
		if a.exit == 0 || f.count() < 2 {
			t.Fatalf("exit=%d requests=%d: %s", a.exit, f.count(), a.shown())
		}
		sent := f.count()
		id := e.onlyRun()
		_, state := e.checkpoint(id)
		eq(t, "count", budgetOf(state), any(map[string]any{"limit": float64(2), "count": float64(2)}))
		f.replace(landScript(f)...)
		r := e.resume(id)
		if r.exit != 1 || f.count() != sent {
			t.Fatalf("exit=%d requests=%d (was %d): %s", r.exit, f.count(), sent, r.shown())
		}
	})
}

// observation is one thing a user or caller of Build observes: its name in a failure message
// and its value printed for the comparison.
type observation struct{ name, value string }

// buildEvents are the documented events of the run record: the Build's own boundaries, its
// requests, tool calls, verification passes, reviews and the outcome. Producer diagnostics
// (timing, provider attempts, command records and the like) are not observations.
var buildEvents = map[string]bool{
	"start": true, "request": true, "tool_call": true, "pass": true, "review": true, "commit": true, "outcome": true,
}

// requestBudgetNames are the names in a decoded document that mention the request budget: an
// object key at any depth, or the string value of an "event" key. Each is listed once, in
// document order of discovery with the keys of an object sorted.
func requestBudgetNames(doc any) []string {
	var names []string
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if strings.Contains(k, "request_budget") {
					add(k)
				}
				if s, ok := x[k].(string); ok && k == "event" && strings.Contains(s, "request_budget") {
					add(s)
				}
				walk(x[k])
			}
		case []any:
			for _, val := range x {
				walk(val)
			}
		}
	}
	walk(doc)
	return names
}

// requestBudgetNamesIn applies requestBudgetNames to a file of a run: log.jsonl line by line,
// the other files as one document.
func requestBudgetNamesIn(file string, data []byte) ([]string, error) {
	texts := []string{string(data)}
	if file == "log.jsonl" {
		texts = strings.Split(strings.TrimSpace(string(data)), "\n")
	}
	var names []string
	for _, text := range texts {
		var doc any
		if err := json.Unmarshal([]byte(text), &doc); err != nil {
			return nil, fmt.Errorf("%s: %v", file, err)
		}
		names = append(names, requestBudgetNames(doc)...)
	}
	return names, nil
}

func TestRequestBudgetNameCheck(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		file, clean, injected, name string
	}{
		{"checkpoint.json", `{"phase":"builder","state":{"count":1}}`, `{"phase":"builder","state":{"count":1,"build_request_budget":{"limit":0}}}`, "build_request_budget"},
		{"result.json", `{"outcome":"landed","limits":{"response_budget":1,"budget_ms":2}}`, `{"outcome":"landed","limits":{"request_budget_left":0}}`, "request_budget_left"},
		{"log.jsonl", "{\"event\":\"response_budget\"}\n{\"event\":\"outcome\"}", "{\"event\":\"response_budget\"}\n{\"event\":\"build_request_budget\"}\n{\"event\":\"outcome\"}", "build_request_budget"},
	} {
		got, err := requestBudgetNamesIn(c.file, []byte(c.clean))
		if err != nil || len(got) != 0 {
			t.Errorf("%s without the injection: names %q, error %v", c.file, got, err)
		}
		got, err = requestBudgetNamesIn(c.file, []byte(c.injected))
		if err != nil || len(got) != 1 || got[0] != c.name {
			t.Errorf("%s with the injection: names %q, error %v, want %q", c.file, got, err, c.name)
		}
	}
}

// observeBudgetBuild runs one Build of the scripted fixture in its own env: five reads, two red
// tries, then the landing. The budget is omitted, or switched off when off is set. It reports the
// request-budget names of the run's files and returns what the Build's user or caller observes.
func observeBudgetBuild(t *testing.T, off bool, label string) []observation {
	t.Helper()
	e := newEnv(t)
	f := e.prov
	// More than four builder requests: nothing stops a Build whose budget is off.
	for i := 0; i < 5; i++ {
		f.then(f.calls("read_file", map[string]any{"path": "app.txt"}))
	}
	f.then(redTry(f, 1), redTry(f, 2))
	f.then(landScript(f)...)
	out := &Outcome{}
	start := e.head()
	a := e.build(func(in *Input) {
		in.Deps.Outcome = out
		if off {
			in.BuildRequestBudget = 0
		}
	})
	id := e.onlyRun()
	for _, file := range []string{"log.jsonl", "checkpoint.json", "result.json"} {
		data, err := os.ReadFile(e.runPath(id, file))
		if err != nil {
			t.Fatal(err)
		}
		names, err := requestBudgetNamesIn(file, data)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			t.Errorf("request-budget name %q in %s of the %s Build", name, file, label)
		}
	}
	stop := ""
	if out.Error != nil {
		stop = out.Error.Class + "/" + out.Error.Reason
	}
	parent := false
	if fields := strings.Fields(e.gitOut("rev-list", "--parents", "-n", "1", "HEAD")); len(fields) > 1 {
		parent = fields[1] == start
	}
	var events []string
	for _, ev := range e.events(id) {
		if name, _ := ev["event"].(string); buildEvents[name] {
			events = append(events, name)
		}
	}
	facts := e.jsonFile(filepath.Join(e.root, ".kogen/local/facts", id+".json"))
	reason, _ := facts["reason"].(string)
	builder := f.count() - len(f.reviewRequests())
	commits, _ := strconv.Atoi(e.gitOut("rev-list", "--count", start+"..HEAD"))
	outcome, _ := e.result(id)["outcome"].(string)
	phase, _ := facts["phase"].(string)
	return []observation{
		{"exit code", fmt.Sprintf("%d", a.exit)},
		{"stop code", fmt.Sprintf("%q", stop)},
		{"outcome", fmt.Sprintf("%q", outcome)},
		{"commits", fmt.Sprintf("%d", commits)},
		{"parent", fmt.Sprintf("%t", parent)},
		{"tree", fmt.Sprintf("%q", e.gitOut("diff-tree", "-r", "--no-commit-id", start, "HEAD"))},
		{"message title", fmt.Sprintf("%q", e.gitOut("log", "-1", "--format=%s"))},
		{"trailer", fmt.Sprintf("%q", e.gitOut("log", "-1", "--format=%(trailers)"))},
		{"status", fmt.Sprintf("%q", e.run("git", "status", "--porcelain"))},
		{"builder requests", fmt.Sprintf("%d", builder)},
		{"review requests", fmt.Sprintf("%d", len(f.reviewRequests()))},
		{"events", fmt.Sprintf("%q", events)},
		{"phase", fmt.Sprintf("%q", phase)},
		{"reason", fmt.Sprintf("%q", reason)},
	}
}

func TestBuildRequestBudgetOff(t *testing.T) {
	t.Parallel()
	omitted := observeBudgetBuild(t, false, "budget-omitted")
	// The reference Build must be the plain landing the comparison is meant to hold.
	if len(omitted) != 14 || omitted[0].value != "0" || omitted[9].value != "8" {
		t.Fatalf("budget-omitted Build: %v, want exit code 0 and 8 builder requests", omitted)
	}
	off := observeBudgetBuild(t, true, "budget-off")
	for i, want := range omitted {
		if got := off[i]; got.value != want.value {
			t.Errorf("budget-off Build differs in %s: budget omitted %s, budget off %s", want.name, want.value, got.value)
		}
	}
}

func TestBuildRequestBudgetReview(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	approve := map[string]any{"verdict": "approve", "findings": []any{}}
	// The review and its repair send two requests of their own after the single builder request.
	f.then(f.calls("read_file", map[string]any{"path": "app.txt"}),
		f.calls(append(writeApp("candidate\nrepaired\n"), "submit_review", approve)...))
	start := e.head()
	a := e.reviewed(func(in *Input) { in.BuildRequestBudget = 1 })
	if a.exit != 0 || len(f.reviewRequests()) != 2 || f.count() != 3 {
		t.Fatalf("exit=%d review requests=%d total=%d: %s", a.exit, len(f.reviewRequests()), f.count(), a.shown())
	}
	eq(t, "commits", e.gitOut("rev-list", "--count", start+"..HEAD"), "1")
	id := e.onlyRun()
	if ev := budgetEvents(e, id); len(ev) != 0 {
		t.Fatalf("events: %v", ev)
	}
	_, state := e.checkpoint(id)
	eq(t, "count", budgetOf(state), any(map[string]any{"limit": float64(1), "count": float64(1)}))
}
