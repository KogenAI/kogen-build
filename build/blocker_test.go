package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

func blockerReport(path string) map[string]any {
	return map[string]any{
		"paths":    []any{path},
		"reason":   "The fixture requires the same value to equal A and B.",
		"evidence": "Real acceptance probe: exit status 1, fixture requires mutually exclusive B at integration_steps.js.",
	}
}

// failingCheck adds a project check that always fails and names path in its output: the red
// item of every verification pass that backs a blocker claim on path.
func failingCheck(path string) func(*project.Config) {
	return func(c *project.Config) {
		c.Checks = append(c.Checks, project.Check{Name: "contradiction", Run: "echo 'cannot pass: " + path + "' >&2; exit 1"})
	}
}

// failingWhile is failingCheck that passes until the flag file exists, so it is red only while
// the flag stands.
func failingWhile(flag, path string) func(*project.Config) {
	return func(c *project.Config) {
		c.Checks = append(c.Checks, project.Check{Name: "contradiction", Run: "[ ! -e '" + flag + "' ] || { echo 'cannot pass: " + path + "' >&2; exit 1; }"})
	}
}

func with(m map[string]any, kv ...any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	for i := 0; i < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

// authority is what a stopped Build must leave as it found it.
type custody struct{ head, index, approval string }

func (e *env) authority() custody {
	idx, _ := os.ReadFile(filepath.Join(e.root, ".git/index"))
	ap, _ := os.ReadFile(project.ApprovalPath(e.root, testSlug))
	return custody{e.gitOutSafe("rev-parse", "--verify", "-q", "HEAD"), string(idx), string(ap)}
}

func (e *env) gitOutSafe(args ...string) string {
	out, _ := execGit(e, args...)
	return strings.TrimSpace(out)
}

func TestABuilderReportedProtectedContradictionStopsBeforeLaterCallsOrGates(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/builder_blocker_test.exs: LOOP-7 a builder-reported protected contradiction stops before later calls or gates
	marker := filepath.Join(t.TempDir(), "fixer-ran")
	e := newEnv(t, failingCheck(stepsFile), func(c *project.Config) { c.Fix = []string{"echo ran >> " + marker} })
	report := blockerReport(stepsFile)
	f := e.prov
	// The first round ends in a finish: the fixer runs and pass 1 is red, naming the protected
	// step file. The claim of the second round is backed by that pass.
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "A\n"},
			"shell", map[string]any{"command": "echo 'fixture requires mutually exclusive B' >&2; exit 1"},
			"finish", map[string]any{"summary": "implemented A"}),
		sse(
			f.call("finish", map[string]any{"summary": "earlier finish must not start gates"}),
			f.call("report_blocker", report),
			f.call("write_file", map[string]any{"path": "late.txt", "content": "must not run"}),
			f.call("shell", map[string]any{"command": "touch late-shell"}),
			f.call("finish", map[string]any{"summary": "must not verify"}),
		),
	)
	before := e.authority()
	a := e.build()
	if a.exit != 1 || a.err != nil {
		t.Fatalf("exit %d err %v\n%s", a.exit, a.err, a.shown())
	}
	out := a.out.String()
	id := e.onlyRun()
	contains(t, "stdout", out, "failed fixture: build_blocked (Build "+id+")\n")
	contains(t, "stdout", out, "candidate/build_blocked: BUILDER-REPORTED protected-baseline contradiction; edits are left in place; this report is not independently proven\n")
	contains(t, "stdout", out, "  paths: "+stepsFile+"\n")
	contains(t, "stdout", out, "  reason: The fixture requires the same value to equal A and B.\n")
	contains(t, "stdout", out, "  evidence: "+report["evidence"].(string)+"\n")
	contains(t, "stdout", out, "  edited: app.txt\n")
	contains(t, "stdout", out, "  run log: "+e.runPath(id, "log.jsonl")+"\n")
	contains(t, "stdout", out, "  fixture Build "+id+" stopped in building: build_blocked; candidate: "+e.root+"; evidence: "+e.runPath(id, "")+"; index: ")
	contains(t, "stdout", out, "  kogen intent recover "+id+" --output <new-external-directory> --project '"+e.root+"'\n")
	contains(t, "stdout", out, "  Recovery preserves and clears only implementation; the builder-reported claim is unproven. Independently Refine, freshly approve and start a fresh Build; resume cannot override a protected blocker.\n")
	contains(t, "stdout", out, "  kogen intent inspect "+id+" --project '"+e.root+"'\n")
	found := false
	for _, o := range f.toolOutputs() {
		if strings.HasPrefix(o, "exit status: 1") && strings.Contains(o, "fixture requires mutually exclusive B") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no failing shell output in %q", f.toolOutputs())
	}
	if got := e.authority(); got != before {
		t.Fatal("head, index or approval changed")
	}
	eq(t, "app.txt", e.read("app.txt"), "A\n")
	for _, p := range []string{"late.txt", "late-shell"} {
		if _, err := os.Stat(filepath.Join(e.root, p)); !os.IsNotExist(err) {
			t.Fatalf("%s exists", p)
		}
	}
	// Only the first round's end ran the fixer and a pass; the claim's finish did not.
	ran, _ := os.ReadFile(marker)
	eq(t, "fixer runs", string(ran), "ran\n")
	res := e.result(id)
	eq(t, "outcome", res["outcome"], any("build_blocked"))
	eq(t, "passes", res["passes"], any(float64(1)))
	eq(t, "reviews", res["reviews"], any(float64(0)))
	blocker := res["blocker"].(map[string]any)
	eq(t, "classification", blocker["classification"], any("BUILDER-REPORTED"))
	eq(t, "evidence", blocker["evidence"], report["evidence"])
	if len(eventsNamed(e.events(id), "builder_blocker")) != 1 {
		t.Fatal("no builder_blocker event")
	}
	facts := e.jsonFile(filepath.Join(e.root, ".kogen/local/facts", id+".json"))
	eq(t, "resume_status", facts["resume_status"], any("unsupported"))
	eq(t, "state", facts["state"], any("build_blocked"))
	blockers := facts["blockers"].([]any)
	if paths := blockers[0].(map[string]any)["paths"].([]any); len(paths) != 1 || paths[0] != stepsFile {
		t.Fatalf("blockers %v", blockers)
	}
	requests := f.count()
	r := e.resume(id)
	eq(t, "resume exit", r.exit, 1)
	contains(t, "resume", r.shown(), "candidate/resume_refused: Build "+id+": uncertain tool boundary; inspect effects before starting a freshly approved Build;")
	if got := e.authority(); got != before {
		t.Fatal("head, index or approval changed by the refused resume")
	}
	eq(t, "requests", f.count(), requests)
}

func TestMalformedEmptyAndUnprotectedBlockerReportsGrantNoWrites(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/builder_blocker_test.exs: LOOP-7 malformed, empty and unprotected blocker reports return feedback and grant no writes
	e := newEnv(t)
	f := e.prov
	report := blockerReport(stepsFile)
	invalid := []map[string]any{
		blockerReport("app.txt"),
		with(report, "paths", []any{}),
		with(report, "reason", " "),
		with(report, "evidence", ""),
		with(report, "paths", []any{123}),
		with(report, "paths", []any{stepsFile, stepsFile}),
		with(report, "unexpected", true),
	}
	for _, r := range invalid {
		f.then(f.calls("report_blocker", r))
	}
	f.then(
		f.calls("write_file", map[string]any{"path": stepsFile, "content": "forbidden"}),
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "ordinary implementation\n"}),
		f.calls("finish", map[string]any{"summary": "ready"}))
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "stdout", a.out.String(), "landed fixture ")
	eq(t, "step file", e.read(stepsFile), "// steps\n")
	feedback := strings.Join(f.toolOutputs(), "\n")
	contains(t, "feedback", feedback, "invalid blocker")
	contains(t, "feedback", feedback, "refused: "+stepsFile)
	res := e.result(e.onlyRun())
	if _, ok := res["blocker"]; ok {
		t.Fatal("a blocker was recorded")
	}
}

func TestAcceptingABlockerStillRequiresTheOriginalHeadAndProtectedBytes(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/builder_blocker_test.exs: LOOP-7 accepting a blocker still requires the original HEAD and protected bytes
	for _, c := range []struct{ mutation, expected string }{
		{"printf changed >> " + stepsFile, "baseline_changed"},
		{"git -c commit.gpgsign=false commit --allow-empty -qm moved", "head_moved"},
		{"git update-ref -d HEAD", "head_moved"},
	} {
		t.Run(c.expected+" "+c.mutation, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			f := e.prov
			f.then(f.calls("shell", map[string]any{"command": c.mutation}, "report_blocker", blockerReport(stepsFile)))
			before := e.authority()
			a := e.build()
			eq(t, "exit", a.exit, 1)
			contains(t, "stdout", a.out.String(), c.expected)
			if res := e.result(e.onlyRun()); res["outcome"] == "build_blocked" {
				t.Fatal("the blocker was accepted")
			}
			after := e.authority()
			eq(t, "index", after.index, before.index)
			eq(t, "approval", after.approval, before.approval)
		})
	}
}

func TestAnUnbackedBlockerClaimGivesFeedbackAndTheBuildContinues(t *testing.T) {
	t.Parallel()
	redCheck := func(named string) func(*project.Config) {
		return func(c *project.Config) {
			c.Checks = []project.Check{{Name: "app", Run: "if grep -qx red app.txt; then echo 'cannot pass: " + named + "' >&2; exit 1; fi"}}
		}
	}
	for name, c := range map[string]struct {
		red     bool   // a red pass runs before the claim
		named   string // what the red check's failure names
		edit    bool   // the builder edits after the red pass, before the claim
		message string
	}{
		"no pass": {false, "app.txt", false, unbackedBlockerMessage(nil, []string{stepsFile})},
		"red pass naming another path": {true, "app.txt", false, "blocker not backed by verification: no failure of verification pass 1 names " + stepsFile +
			"; no protected edit is granted; make the failing items pass, or report again with a protected path that a failure names"},
		// The red pass names the claimed path, but it ran on another tree: no grounding pass.
		"red pass naming the path on another tree": {true, stepsFile, true, unbackedBlockerMessage(nil, []string{stepsFile})},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, redCheck(c.named))
			f := e.prov
			if c.red {
				f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "red"}))
			}
			claim := f.calls("report_blocker", blockerReport(stepsFile))
			if c.edit {
				claim = f.calls("write_file", map[string]any{"path": "app.txt", "content": "edited\n"}, "report_blocker", blockerReport(stepsFile))
			}
			f.then(
				claim,
				f.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}, "finish", map[string]any{"summary": "done"}))
			a := e.build()
			if a.exit != 0 {
				t.Fatalf("exit %d\n%s", a.exit, a.shown())
			}
			found := false
			for _, o := range f.toolOutputs() {
				found = found || o == c.message
			}
			if !found {
				t.Fatalf("no tool output %q in %q", c.message, f.toolOutputs())
			}
			eq(t, "step file", e.read(stepsFile), "// steps\n")
			res := e.result(e.onlyRun())
			if _, ok := res["blocker"]; ok || len(eventsNamed(e.events(e.onlyRun()), "builder_blocker")) != 0 {
				t.Fatal("a blocker was recorded")
			}
		})
	}
}

func TestBlockerBackingIsASubstringOfARedItemOfAPassOnTheCurrentTree(t *testing.T) {
	t.Parallel()
	const claimed = "features/steps/integration_steps.js"
	slug := testSlug
	failed := func(out string) verify.CommandResult {
		return verify.CommandResult{Name: "n", Command: "c", Status: 1, Output: out}
	}
	acceptance := func(problem string) *verify.Acceptance {
		return &verify.Acceptance{Status: 1, Output: "report", Problems: []string{problem}}
	}
	cases := []struct {
		name   string
		pass   *verify.Pass
		paths  []string
		backed bool
	}{
		{"no pass", nil, []string{claimed}, false},
		{"failing scenario detail", &verify.Pass{Acceptance: acceptance("at /tmp/x/" + claimed + ":2")}, []string{claimed}, true},
		{"acceptance output", &verify.Pass{Acceptance: &verify.Acceptance{Status: 1, Output: claimed + " failed", Problems: []string{"p"}}}, []string{claimed}, true},
		{"feature path of a failing acceptance", &verify.Pass{Acceptance: acceptance("p")}, []string{project.FeaturePath(slug)}, true},
		{"passing acceptance", &verify.Pass{Acceptance: &verify.Acceptance{Output: claimed}}, []string{claimed}, false},
		{"check output", &verify.Pass{Checks: []verify.CommandResult{failed("see " + claimed)}}, []string{claimed}, true},
		{"check command", &verify.Pass{Checks: []verify.CommandResult{{Name: "n", Command: "node " + claimed, Status: 2}}}, []string{claimed}, true},
		{"check name", &verify.Pass{Checks: []verify.CommandResult{{Name: claimed, Command: "c", Status: 2}}}, []string{claimed}, true},
		{"setup output", &verify.Pass{Setup: []verify.CommandResult{failed(claimed)}}, []string{claimed}, true},
		{"passing check", &verify.Pass{Checks: []verify.CommandResult{{Name: "n", Command: "c", Output: claimed}}}, []string{claimed}, false},
		{"failed intent model", &verify.Pass{Model: verify.ModelResult{State: "failed", Problems: []string{"p"}}}, []string{project.ModelPath(slug)}, true},
		{"passing intent model", &verify.Pass{Model: verify.ModelResult{State: "ok", Problems: []string{claimed}}}, []string{claimed}, false},
		{"failed maintained model", &verify.Pass{MaintainedModels: []verify.MaintainedModelResult{{Path: ".kogen/models/m/model.qnt", Result: verify.ModelResult{State: "failed"}}}}, []string{".kogen/models/m/model.qnt"}, true},
		{"green pass", &verify.Pass{Green: true}, []string{claimed}, false},
		{"moved only", &verify.Pass{Moved: []string{claimed}}, []string{claimed}, false},
		{"one of several paths", &verify.Pass{Checks: []verify.CommandResult{failed(claimed)}}, []string{".kogen/intents/fixture/intent.md", claimed}, true},
		{"another path", &verify.Pass{Checks: []verify.CommandResult{failed("app.txt")}}, []string{claimed}, false},
	}
	for _, c := range cases {
		eq(t, c.name, blockerBacked(slug, c.pass, c.paths), c.backed)
	}
}

func TestUnbackedBlockerMessages(t *testing.T) {
	t.Parallel()
	paths := []string{"a", "b"}
	eq(t, "no pass", unbackedBlockerMessage(nil, paths),
		"blocker not backed by verification: no verification pass has run on the current tree yet, so no failure names a, b; no protected edit is granted; make the failing items pass, or finish so verification runs on the current tree and report again with a protected path that a failure names")
	eq(t, "pass 3", unbackedBlockerMessage(&verify.Pass{N: 3}, paths),
		"blocker not backed by verification: no failure of verification pass 3 names a, b; no protected edit is granted; make the failing items pass, or report again with a protected path that a failure names")
}

func TestTheGroundingCandidateIsTheLatestWholeIntentOrStagePass(t *testing.T) {
	t.Parallel()
	b := &Attempt{}
	if b.latestPass() != nil {
		t.Fatal("a Build without a pass has a latest pass")
	}
	b.LastPass = &verify.Pass{N: 2}
	b.StageEvidence = []StageEvidence{{Pass: verify.Pass{N: 1}}}
	eq(t, "whole-Intent pass after a stage gate", b.latestPass().N, 2)
	b.StageEvidence = append(b.StageEvidence, StageEvidence{Pass: verify.Pass{N: 3}})
	eq(t, "later stage-gate pass", b.latestPass().N, 3)
	b.LastPass = nil
	eq(t, "stage-gate pass only", b.latestPass().N, 3)
}
