package build

import (
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

const timeoutScript = "if grep -q '^slow' app.txt; then printf 'ok  \\tkogen/verify\\t1.2s\\npanic: test timed out after 10m0s\\n\\trunning tests:\\n'; exit 1; fi"

func TestRepeatedHostTimeoutStopsTheNativeBuildAfterTheSecondPass(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *project.Config) { c.Checks = []project.Check{{Name: "go-test", Run: timeoutScript}} })
	f := e.prov
	for _, w := range []string{"slow-1", "slow-2", "slow-3"} {
		f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": w + "\n"}, "finish", map[string]any{"summary": "round"}))
	}
	start := e.head()
	a := e.build()
	if a.exit != 3 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	out := a.out.String()
	contains(t, "stop", out, "environment/host_or_base_failure: check go-test failed the same way in verification passes 1 and 2: panic: test timed out after 10m0s; this failure is in the host or the base")
	contains(t, "check line", out, "  check: go-test ("+timeoutScript)
	contains(t, "failure line", out, "  failure: panic: test timed out after 10m0s\n")
	contains(t, "evidence line", out, "  evidence: go-test output of verification passes 1 and 2 in ")
	contains(t, "tree line", out, "  candidate tree: ")
	eq(t, "requests", len(f.requests()), 2)
	id := e.onlyRun()
	res := e.result(id)
	eq(t, "outcome", res["outcome"], any("host_or_base_failure"))
	eq(t, "passes", res["passes"], any(float64(2)))
	eq(t, "repairs", res["repairs"], any(float64(1)))
	evs := eventsNamed(e.events(id), "host_or_base_failure")
	eq(t, "events", len(evs), 1)
	eq(t, "event", evs[0]["passes"], any([]any{float64(1), float64(2)}))
	eq(t, "head", e.head(), start)
	eq(t, "candidate", e.read("app.txt"), "slow-2\n")
}

func TestHostTimeoutsThatAreNotConsecutiveDoNotStopTheBuild(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *project.Config) {
		c.Checks = []project.Check{{Name: "go-test", Run: timeoutScript + "; grep -q '^ordinary' app.txt && { echo '--- FAIL: TestX'; exit 1; }; true"}}
	})
	f := e.prov
	for _, w := range []string{"slow-1", "ordinary-2", "slow-3", "ready"} {
		f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": w + "\n"}, "finish", map[string]any{"summary": "round"}))
	}
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	eq(t, "events", len(eventsNamed(e.events(e.onlyRun()), "host_or_base_failure")), 0)
}

func TestRepeatedHostTimeoutInTwoGatesOfAStageStopsTheBuild(t *testing.T) {
	t.Parallel()
	e, _ := stagedEnv(t, []string{timeoutScript})
	f := e.prov
	f.then(
		f.calls(stageWrite(f, "slow-1")...), f.calls(stageFinish()...),
		f.calls(stageWrite(f, "slow-2")...), f.calls(stageFinish()...),
		f.calls(stageWrite(f, "ready")...), f.calls(stageFinish()...),
	)
	a := e.build()
	if a.exit != 3 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stop", a.out.String(), "environment/host_or_base_failure: check stage-foundation-1 failed the same way in verification passes 1 and 2: panic: test timed out after 10m0s")
	id := e.onlyRun()
	evs := eventsNamed(e.events(id), "host_or_base_failure")
	eq(t, "events", len(evs), 1)
	eq(t, "check", evs[0]["check"], any("stage-foundation-1"))
	eq(t, "outcome", e.result(id)["outcome"], any("host_or_base_failure"))
	if strings.Contains(e.read("app.txt"), "ready") {
		t.Fatal("a builder round ran after the stop")
	}
}

// kindStopEnv runs a Build whose single check "lint" prints the given line while app.txt starts
// with the marker of the pass (a, b), and whose builder writes a-1, b-2 and then ready.
func kindStopEnv(t *testing.T, checks ...project.Check) *env {
	t.Helper()
	e := newEnv(t, func(c *project.Config) { c.Checks = checks })
	f := e.prov
	for _, w := range []string{"a-1", "b-2", "ready"} {
		f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": w + "\n"}, "finish", map[string]any{"summary": "round"}))
	}
	return e
}

func kindStopScript(first, second string) string {
	return "if grep -q '^a' app.txt; then echo '" + first + "'; exit 1; fi; if grep -q '^b' app.txt; then echo '" + second + "'; exit 1; fi"
}

func kindStopNone(t *testing.T, e *env) {
	t.Helper()
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	eq(t, "events", len(eventsNamed(e.events(e.onlyRun()), "host_or_base_failure")), 0)
}

func TestHostStopSameKindDifferentLinesStopsQuotingTheSecond(t *testing.T) {
	t.Parallel()
	e := kindStopEnv(t, project.Check{Name: "lint", Run: kindStopScript("write /tmp/a: no space left on device", "write /tmp/b: no space left on device")})
	a := e.build()
	if a.exit != 3 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	out := a.out.String()
	contains(t, "stop", out, "environment/host_or_base_failure: check lint failed the same way in verification passes 1 and 2: write /tmp/b: no space left on device; this failure is in the host or the base")
	contains(t, "failure line", out, "  failure: write /tmp/b: no space left on device\n")
	eq(t, "requests", len(e.prov.requests()), 2)
	id := e.onlyRun()
	eq(t, "outcome", e.result(id)["outcome"], any("host_or_base_failure"))
	evs := eventsNamed(e.events(id), "host_or_base_failure")
	eq(t, "events", len(evs), 1)
	eq(t, "event failure", evs[0]["failure"], any("write /tmp/b: no space left on device"))
}

func TestHostStopDifferentKindInThePreviousPassDoesNotStop(t *testing.T) {
	t.Parallel()
	kindStopNone(t, kindStopEnv(t, project.Check{Name: "lint", Run: kindStopScript("no space left on device", "cannot allocate memory")}))
}

func TestHostStopOrdinaryFailureInThePreviousPassDoesNotStop(t *testing.T) {
	t.Parallel()
	kindStopNone(t, kindStopEnv(t, project.Check{Name: "lint", Run: kindStopScript("--- FAIL: TestX", "no space left on device")}))
}

func TestHostStopMemoryIsPerCheckName(t *testing.T) {
	t.Parallel()
	kindStopNone(t, kindStopEnv(t,
		project.Check{Name: "lint", Run: "if grep -q '^a' app.txt; then echo 'no space left on device'; exit 1; fi"},
		project.Check{Name: "test", Run: "if grep -q '^b' app.txt; then echo 'no space left on device'; exit 1; fi"},
	))
}

func TestHostStopIdenticalLinesStillStop(t *testing.T) {
	t.Parallel()
	e := kindStopEnv(t, project.Check{Name: "lint", Run: kindStopScript("too many open files", "too many open files")})
	a := e.build()
	if a.exit != 3 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stop", a.out.String(), "environment/host_or_base_failure: check lint failed the same way in verification passes 1 and 2: too many open files; this failure is in the host or the base")
	evs := eventsNamed(e.events(e.onlyRun()), "host_or_base_failure")
	eq(t, "events", len(evs), 1)
	eq(t, "event failure", evs[0]["failure"], any("too many open files"))
	eq(t, "event passes", evs[0]["passes"], any([]any{float64(1), float64(2)}))
}

func TestNoEditRoundAfterATimeoutRedStopsAsHostOrBaseOnTheRepeat(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *project.Config) { c.Checks = []project.Check{{Name: "go-test", Run: timeoutScript}} })
	f := e.prov
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": "slow-1\n"}, "finish", map[string]any{"summary": "round"}), says("No edit."))
	a := e.build()
	if a.exit != 3 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stop", a.out.String(), "environment/host_or_base_failure: check go-test failed the same way in verification passes 1 and 2")
	eq(t, "requests", len(f.requests()), 2)
	for _, req := range f.requests() {
		if strings.Contains(lastInput(req), "made no edits") {
			t.Fatalf("a no-edit notice was given:\n%s", lastInput(req))
		}
	}
	eq(t, "passes", e.result(e.onlyRun())["passes"], any(float64(2)))
}

func TestNoEditRoundAfterAStageGateTimeoutStopsAsHostOrBaseOnTheRepeat(t *testing.T) {
	t.Parallel()
	e, _ := stagedEnv(t, []string{"if grep -q '^slow' app.txt; then echo 'panic: test timed out after 10m0s'; exit 1; fi"})
	f := e.prov
	f.then(f.calls(stageWrite(f, "slow-1")...), f.calls(stageFinish()...), says("No edit."))
	a := e.build()
	if a.exit != 3 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stop", a.out.String(), "environment/host_or_base_failure: check stage-foundation-1 failed the same way in verification passes 1 and 2")
	eq(t, "passes", e.result(e.onlyRun())["passes"], any(float64(2)))
}

func TestMemoryFailurePhrasesClassifyAsMemory(t *testing.T) {
	for _, line := range []string{"cannot allocate memory", "out of memory", "Out of memory: Killed process"} {
		t.Run(line, func(t *testing.T) {
			eq(t, "kind", envFailureKind(line), "memory")
		})
	}
}

func TestHostStopDifferentMemoryPhrasesQuotesSecondLine(t *testing.T) {
	t.Parallel()
	first := "worker: cannot allocate memory"
	second := "worker: Out of memory: Killed process 42"
	e := kindStopEnv(t, project.Check{Name: "lint", Run: kindStopScript(first, second)})
	a := e.build()
	if a.exit != 3 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stop", a.out.String(), "environment/host_or_base_failure: check lint failed the same way in verification passes 1 and 2: "+second+"; this failure is in the host or the base")
	contains(t, "failure line", a.out.String(), "  failure: "+second+"\n")
	eq(t, "requests", len(e.prov.requests()), 2)
	id := e.onlyRun()
	eq(t, "outcome", e.result(id)["outcome"], any("host_or_base_failure"))
	eq(t, "passes", e.result(id)["passes"], any(float64(2)))
	evs := eventsNamed(e.events(id), "host_or_base_failure")
	eq(t, "events", len(evs), 1)
	eq(t, "event failure", evs[0]["failure"], any(second))
	eq(t, "event passes", evs[0]["passes"], any([]any{float64(1), float64(2)}))
}

func TestHostStopMemoryAfterOtherEnvironmentKindsDoesNotStop(t *testing.T) {
	for _, first := range []string{"too many open files", "panic: test timed out after 10m0s"} {
		t.Run(first, func(t *testing.T) {
			kindStopNone(t, kindStopEnv(t, project.Check{Name: "lint", Run: kindStopScript(first, "cannot allocate memory")}))
		})
	}
}
