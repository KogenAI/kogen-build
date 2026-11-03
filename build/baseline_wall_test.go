package build

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/KogenAI/kogen-coordinator/project"
	"kogen-loop/sys"
)

var baselineInstant = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$`)

// instants reads started_at and endKey of m (run-log instants) and the wall_ms between them.
func instants(t *testing.T, what string, m map[string]any, endKey string) (start, end time.Time, wall int64) {
	t.Helper()
	parse := func(key string) time.Time {
		s, _ := m[key].(string)
		if !baselineInstant.MatchString(s) {
			t.Fatalf("%s: %s = %#v, want a UTC instant", what, key, m[key])
		}
		v, err := time.Parse("2006-01-02T15:04:05.000000Z", s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	start, end = parse("started_at"), parse(endKey)
	w, ok := m["wall_ms"].(float64)
	if !ok || w != float64(int64(w)) || w < 0 {
		t.Fatalf("%s: wall_ms = %#v, want a nonnegative integer", what, m["wall_ms"])
	}
	if end.Before(start) {
		t.Fatalf("%s: %s is before started_at", what, endKey)
	}
	if d := end.Sub(start).Milliseconds() - int64(w); d < -2 || d > 2 {
		t.Fatalf("%s: wall_ms %v disagrees with %s - started_at", what, w, endKey)
	}
	return start, end, int64(w)
}

// requireBaselineTimings asserts the baseline's command_result events and its baseline_checks
// event carry consistent start/end/wall values, and that timings names exactly the checks that ran.
func requireBaselineTimings(t *testing.T, evs []map[string]any, ran []string) {
	t.Helper()
	bcs := eventsNamed(evs, "baseline_checks")
	if len(bcs) != 1 {
		t.Fatalf("%d baseline_checks events", len(bcs))
	}
	bc := bcs[0]
	whole0, whole1, _ := instants(t, "baseline_checks", bc, "ended_at")
	timings, _ := bc["timings"].([]any)
	if len(timings) != len(ran) {
		t.Fatalf("timings %v, want one entry per check that ran %v", bc["timings"], ran)
	}
	for i, name := range ran {
		got, _ := timings[i].(map[string]any)
		if len(got) != 4 || got["name"] != name {
			t.Fatalf("timings[%d] = %v, want name %s and four keys", i, got, name)
		}
		start, end, _ := instants(t, "timings "+name, got, "ended_at")
		if start.Before(whole0) || end.After(whole1) {
			t.Fatalf("timings %s %v..%v outside the baseline %v..%v", name, start, end, whole0, whole1)
		}
		var result map[string]any
		for _, ev := range evs {
			if ev["event"] == "command_result" && ev["name"] == name {
				result = ev
				break
			}
		}
		if result == nil {
			t.Fatalf("no command_result for %s", name)
		}
		instants(t, "command_result "+name, result, "ended_at")
		for _, k := range []string{"started_at", "ended_at", "wall_ms"} {
			if result[k] != got[k] {
				t.Fatalf("%s %s: command_result %v, timings %v", name, k, result[k], got[k])
			}
		}
	}
}

// requireBaselineStop asserts the stop record of a Build that ended while the baseline check
// name was running command: outcome event, result.json and the stdout line agree.
func requireBaselineStop(t *testing.T, e *env, id, out, name, command, reason string) {
	t.Helper()
	res := e.result(id)
	var outcome map[string]any
	for _, ev := range e.events(id) {
		if ev["event"] == "outcome" {
			outcome = ev
		}
	}
	if res["outcome"] != reason || res["stopped_during"] != "baseline" || outcome["outcome"] != reason || outcome["stopped_during"] != "baseline" {
		t.Fatalf("result %v / outcome %v, want %s stopped during baseline", res["outcome"], outcome["outcome"], reason)
	}
	eq(t, "outcome baseline_check", outcome["baseline_check"], res["baseline_check"])
	bc, _ := res["baseline_check"].(map[string]any)
	if len(bc) != 5 || bc["name"] != name || bc["command"] != command {
		t.Fatalf("baseline_check = %v, want %s (%s) with five keys", bc, name, command)
	}
	_, _, wall := instants(t, "baseline_check", bc, "stopped_at")
	contains(t, "stdout", out, "  stopped during the check baseline: check "+name+" ("+command+") ran "+strconv.FormatInt(wall, 10)+" ms (started "+bc["started_at"].(string)+")\n")
}

func baselineStopEnv(t *testing.T) (*env, string, string) {
	marker := filepath.Join(t.TempDir(), "check-running")
	command := "touch " + marker + "; sleep 600"
	e := newEnv(t, func(c *project.Config) { c.Checks = []project.Check{{Name: "slow", Run: command}} })
	return e, marker, command
}

func TestInterruptedBaselineCheckNamesTheBaseline(t *testing.T) {
	t.Parallel()
	e, marker, command := baselineStopEnv(t)
	a, done := e.start()
	t.Cleanup(func() { a.s.Interrupt("SIGTERM"); <-done })
	waitForRunning(t, marker, done, a, 15*time.Second)
	time.Sleep(200 * time.Millisecond)
	a.s.Interrupt("SIGTERM")
	<-done
	eq(t, "exit", a.exit, 143)
	id := e.onlyRun()
	out := a.out.String()
	contains(t, "stdout", out, "stopped fixture: interrupted (Build "+id+")\ncandidate/interrupted: Build "+id+" stopped by SIGTERM; edits are left in place\n  stopped during the check baseline:")
	requireBaselineStop(t, e, id, out, "slow", command, "interrupted")
}

func TestErrorStopDuringBaselineCheckNamesTheBaseline(t *testing.T) {
	t.Parallel()
	e, marker, command := baselineStopEnv(t)
	e.credentials()
	s, out, _ := e.session()
	var b *Attempt
	done := make(chan error, 1)
	go func() {
		done <- buildRun(context.Background(), s, Input{Deps: e.deps(), Project: e.root, Slug: testSlug}, &b)
	}()
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the baseline check never started")
		}
	}
	time.Sleep(200 * time.Millisecond)
	// The check is still running when the Build ends on an error stop (as command_no_progress does).
	s.Interrupt("SIGTERM")
	<-done
	exit, err := conclude(s, &b, sys.Fail("environment", "command_no_progress", "slow had no live process; stopped waiting for: "+command, 1))
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "exit", exit, 1)
	id := e.onlyRun()
	contains(t, "stdout", out.String(), "environment/command_no_progress:")
	requireBaselineStop(t, e, id, out.String(), "slow", command, "command_no_progress")
}

func TestBaselineStopOutsideTheBaselineNamesNone(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *project.Config) { c.Checks = []project.Check{{Name: "fast", Run: "echo FAST"}} })
	e.prov.then(refused())
	a := e.build()
	if a.exit == 0 {
		t.Fatal("provider refusal landed")
	}
	id := e.onlyRun()
	eq(t, "stopped_during", e.result(id)["stopped_during"], nil)
	eq(t, "baseline_check", e.result(id)["baseline_check"], nil)
	lacks(t, "stdout", a.out.String(), "stopped during the check baseline")
	requireBaselineTimings(t, e.events(id), []string{"fast"})
}
