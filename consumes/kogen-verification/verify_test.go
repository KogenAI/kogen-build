package consumes

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

func TestVerifyInputFields(t *testing.T) {
	var in verify.Input
	var _ string = in.Root
	var _ *git.Repo = in.Repo
	var _ *project.Config = in.Config
	var _ *project.Intent = in.Intent
	var _ []project.ModelSpec = in.MaintainedModels
	var _ string = in.Start
	var _ []string = in.Excluded
	var _ int = in.N
	var _ string = in.RunDir
	var _ verify.Tools = in.Tools
	var _ verify.Logger = in.Log
	var _ *verify.Stage = in.Stage
	var _ func() error = in.Intact
	var _ func(context.Context, *sys.Session, verify.Input) (verify.Pass, error) = verify.Verify

	st := verify.Stage{ID: "s1", Checks: []string{"true"}}
	var _ string = st.ID
	var _ []string = st.Checks
	if st.ID != "s1" || len(st.Checks) != 1 {
		t.Errorf("verify.Stage: %+v", st)
	}
}

// TestVerifyGreenPass is a whole green pass: the setup commands, then acceptance, then the
// checks, each in a scratch copy of the candidate, with the record build keeps.
func TestVerifyGreenPass(t *testing.T) {
	p := newProj(t)
	s, stderr := p.session()
	p.cfg.Setup = []string{"echo setup-out"}
	p.cfg.Checks = []project.Check{
		{Name: "build", Run: "echo built; pwd"},
		{Name: "tools", Run: `printf '%s|%s|%s\n' "$KOGEN_NODE" "$KOGEN_QUINT" "${KOGEN_CUCUMBER-unset}"`},
	}
	runDir := t.TempDir()
	var log memLog
	in := p.input(3)
	in.Tools = verify.Tools{Node: "/n/node", Quint: "/q/cli.js"}
	in.RunDir = runDir
	in.Log = &log
	intacts := 0
	in.Intact = func() error { intacts++; return nil }

	pass, err := verify.Verify(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}
	var _ int = pass.N
	var _ bool = pass.Clean
	var _ bool = pass.Green
	var _ string = pass.Tree
	var _ []verify.CommandResult = pass.Setup
	var _ *verify.Acceptance = pass.Acceptance
	var _ verify.ModelResult = pass.Model
	var _ []verify.MaintainedModelResult = pass.MaintainedModels
	var _ []verify.CommandResult = pass.Checks
	var _ []string = pass.NotRun
	var _ []string = pass.Moved
	var _ []string = pass.Failing

	if pass.N != 3 || !pass.Clean || !pass.Green {
		t.Errorf("verify.Pass: N=%d Clean=%v Green=%v, want 3 true true", pass.N, pass.Clean, pass.Green)
	}
	if pass.Tree != p.headTree() {
		t.Errorf("verify.Pass.Tree = %q, want the clean candidate's tree %q", pass.Tree, p.headTree())
	}
	if len(pass.Setup) != 1 || pass.Setup[0] != (verify.CommandResult{Name: "setup-1", Command: "echo setup-out", Status: 0, Output: "setup-out\n"}) {
		t.Errorf("verify.Pass.Setup = %+v", pass.Setup)
	}
	if len(pass.Checks) != 2 || pass.Checks[0].Name != "build" || pass.Checks[0].Command != "echo built; pwd" || pass.Checks[0].Status != 0 ||
		pass.Checks[1] != (verify.CommandResult{Name: "tools", Command: p.cfg.Checks[1].Run, Status: 0, Output: "/n/node|/q/cli.js|unset\n"}) {
		t.Errorf("verify.Pass.Checks = %+v, want both checks in order with the Kogen tool variables (unset when empty)", pass.Checks)
	}
	built, scratch, _ := strings.Cut(strings.TrimSpace(pass.Checks[0].Output), "\n")
	if built != "built" || scratch == p.root || !strings.Contains(scratch, "kogen-verify") {
		t.Errorf("verify.Pass.Checks[0].Output = %q, want the check run in a kogen-verify scratch folder, not the checkout", pass.Checks[0].Output)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Errorf("verify.Verify: scratch folder %s left behind (%v)", scratch, err)
	}
	a := pass.Acceptance
	if a == nil || a.Status != 0 || a.Output != "" || len(a.Problems) != 0 || len(a.Scenarios) != 2 {
		t.Fatalf("verify.Pass.Acceptance = %+v, want exit 0, no output, no problems and two scenarios", a)
	}
	var _ string = a.Output
	var _ []string = a.Problems
	var _ []verify.ScenarioResult = a.Scenarios
	for i, want := range []struct {
		name string
		line int
	}{{"First", 2}, {"Later", 4}} {
		sc := a.Scenarios[i]
		var _ string = sc.Name
		var _ int = sc.Line
		var _ string = sc.Status
		var _ *string = sc.Detail
		if sc.Name != want.name || sc.Line != want.line || sc.Status != "passed" || sc.Detail != nil {
			t.Errorf("verify.ScenarioResult %d = %q line %d %q detail %v, want %q line %d passed", i, sc.Name, sc.Line, sc.Status, sc.Detail, want.name, want.line)
		}
	}
	if pass.Model.State != "none" || len(pass.Model.Problems) != 0 {
		t.Errorf("verify.Pass.Model = %+v, want none", pass.Model)
	}
	var _ string = pass.Model.State
	var _ []string = pass.Model.Problems
	if pass.NotRun == nil || len(pass.NotRun) != 0 || pass.Moved == nil || len(pass.Moved) != 0 || pass.Failing == nil || len(pass.Failing) != 0 || len(pass.MaintainedModels) != 0 {
		t.Errorf("verify.Pass: NotRun=%v Moved=%v Failing=%v MaintainedModels=%v, want empty non-nil lists", pass.NotRun, pass.Moved, pass.Failing, pass.MaintainedModels)
	}
	if _, err := os.Stat(filepath.Join(runDir, "acceptance-3.ndjson")); err != nil {
		t.Errorf("verify.Input.RunDir: acceptance-3.ndjson not written there: %v", err)
	}
	if intacts < 1 {
		t.Errorf("verify.Input.Intact never ran")
	}
	if got := strings.Join(log.names("command", "pass"), ","); !strings.HasPrefix(got, "command setup-1,command acceptance,command build,command tools,pass <nil>") {
		t.Errorf("verify.Input.Log: events %s, want setup, acceptance and the checks in order, then the pass", got)
	}
	if ev := log.named("pass"); len(ev) != 1 || ev[0]["pass"] != 3 || ev[0]["green"] != true {
		t.Errorf("verify.Input.Log: pass event %v", ev)
	}
	if !strings.Contains(stderr.String(), "kogen: pass 3: all checks passed") {
		t.Errorf("verify.Verify: progress %q", stderr.String())
	}

	// What a green pass says about itself.
	expected, err := project.ParseGherkin(feature)
	if err != nil {
		t.Fatal(err)
	}
	if !pass.Complete(p.cfg, expected, nil) {
		t.Errorf("verify.Pass.Complete: a green pass of this configuration is not complete")
	}
	if !pass.Valid() {
		t.Errorf("verify.Pass.Valid: a clean pass with a tree is not valid")
	}
	if got, want := pass.ChecksLine(), "acceptance 2/2, model not applicable, build passed, tools passed"; got != want {
		t.Errorf("verify.Pass.ChecksLine = %q, want %q", got, want)
	}
}

func TestVerifyRepoDefault(t *testing.T) {
	p := newProj(t)
	s, _ := p.session()
	in := p.input(1)
	in.Repo = nil
	pass, err := verify.Verify(context.Background(), s, in)
	if err != nil || !pass.Green || pass.Tree != p.headTree() {
		t.Errorf("verify.Input.Repo: nil repo gave (%+v, %v), want a green pass over the checkout's own Git", pass, err)
	}
}

func TestVerifyFailedCheck(t *testing.T) {
	p := newProj(t)
	s, _ := p.session()
	p.cfg.Checks = []project.Check{
		{Name: "build", Run: "echo built"},
		{Name: "vet", Run: "i=1; while [ $i -le 80 ]; do echo vet line $i; i=$((i+1)); done; exit 2"},
		{Name: "test", Run: "echo never"},
	}
	pass, err := verify.Verify(context.Background(), s, p.input(1))
	if err != nil {
		t.Fatal(err)
	}
	if pass.Green || !pass.Clean || len(pass.Moved) != 0 {
		t.Errorf("verify.Pass: Green=%v Clean=%v Moved=%v, want a clean red pass", pass.Green, pass.Clean, pass.Moved)
	}
	if !slices.Equal(pass.Failing, []string{"vet"}) || !slices.Equal(pass.NotRun, []string{"test"}) {
		t.Errorf("verify.Pass: Failing=%v NotRun=%v, want [vet] and [test]", pass.Failing, pass.NotRun)
	}
	if len(pass.Checks) != 2 || pass.Checks[0].Status != 0 || pass.Checks[1].Name != "vet" || pass.Checks[1].Status != 2 || !strings.HasSuffix(pass.Checks[1].Output, "vet line 80\n") {
		t.Errorf("verify.Pass.Checks = %+v, want the passed check, then the failed one with its status and whole output, and no third", pass.Checks)
	}
	if pass.Acceptance == nil || len(pass.Acceptance.Problems) != 0 {
		t.Errorf("verify.Pass.Acceptance = %+v, want it run first and passed", pass.Acceptance)
	}
	if got, want := verify.NotRunNotice(pass.NotRun), "not run because an earlier item failed: test"; got != want {
		t.Errorf("verify.NotRunNotice = %q, want %q", got, want)
	}
	expected, _ := project.ParseGherkin(feature)
	if pass.Complete(p.cfg, expected, nil) {
		t.Errorf("verify.Pass.Complete: a red pass is complete")
	}

	fb := pass.Feedback(p.cfg.Acceptance.Run, "")
	var tail []string
	for i := 21; i <= 80; i++ {
		tail = append(tail, "  vet line "+itoa(i))
	}
	want := "Verification pass 1 is not green. Fix the implementation (the approved acceptance files cannot change), then call finish again.\n" +
		"check vet (" + p.cfg.Checks[1].Run + "): failed, exit status 2\n" + strings.Join(tail, "\n") + "\n" +
		"not run because an earlier item failed: test"
	if fb != want {
		t.Errorf("verify.Pass.Feedback = %q\nwant %q", fb, want)
	}
	if got := pass.Feedback("x", "submit_review"); !strings.HasPrefix(got, "Verification pass 1 is not green. Fix the implementation (the approved acceptance files cannot change), then call submit_review again.") {
		t.Errorf("verify.Pass.Feedback: completion name not used: %q", got)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestVerifyFailedAcceptance(t *testing.T) {
	p := newProj(t)
	s, _ := p.session()
	p.write("app.txt", "red\n")
	p.commit("red")
	p.cfg.Setup = []string{"echo setup-ran"}
	p.cfg.Checks = []project.Check{{Name: "build", Run: "echo never"}}
	pass, err := verify.Verify(context.Background(), s, p.input(2))
	if err != nil {
		t.Fatal(err)
	}
	if pass.Green || !slices.Equal(pass.Failing, []string{"acceptance"}) || !slices.Equal(pass.NotRun, []string{"build"}) || len(pass.Checks) != 0 {
		t.Errorf("verify.Pass: Green=%v Failing=%v NotRun=%v Checks=%v, want red on acceptance with the check not run", pass.Green, pass.Failing, pass.NotRun, pass.Checks)
	}
	if len(pass.Setup) != 1 || pass.Setup[0].Output != "setup-ran\n" {
		t.Errorf("verify.Pass.Setup = %+v, want setup run before acceptance", pass.Setup)
	}
	a := pass.Acceptance
	if a == nil || a.Status != 1 || a.Output != "actual-case-red\n" || len(a.Scenarios) != 2 || len(a.Problems) != 2 {
		t.Fatalf("verify.Pass.Acceptance = %+v, want exit 1, its output, two failed scenarios and two problems", a)
	}
	for i, line := range []int{2, 4} {
		sc := a.Scenarios[i]
		if sc.Status != "failed" || sc.Line != line || sc.Detail == nil || !strings.Contains(*sc.Detail, "boom: the candidate is red") {
			t.Errorf("verify.ScenarioResult %d = %+v, want failed with the report's message as detail", i, sc)
		}
		if !strings.HasPrefix(a.Problems[i], "failed: "+sc.Name+" (line "+itoa(line)+")\n") {
			t.Errorf("verify.Acceptance.Problems[%d] = %q", i, a.Problems[i])
		}
	}

	fb := pass.Feedback(p.cfg.Acceptance.Run, "finish")
	if !strings.Contains(fb, "acceptance ("+p.cfg.Acceptance.Run+"): failed, exit status 1\n  failed: First (line 2)\n  acceptance.feature line 3: Then the candidate succeeds\n  boom: the candidate is red\n  failed: Later (line 4)") ||
		!strings.HasSuffix(fb, "not run because an earlier item failed: build") {
		t.Errorf("verify.Pass.Feedback = %q, want the acceptance failure with indented problems, then the not-run notice", fb)
	}
	if got := pass.ChecksLine(); got != "acceptance 0/2, model not applicable" {
		t.Errorf("verify.Pass.ChecksLine = %q", got)
	}
}

func TestVerifyFailedSetup(t *testing.T) {
	p := newProj(t)
	s, _ := p.session()
	p.cfg.Setup = []string{"echo one", "echo two; exit 5", "echo three"}
	p.cfg.Checks = []project.Check{{Name: "build", Run: "echo never"}}
	pass, err := verify.Verify(context.Background(), s, p.input(1))
	if err != nil {
		t.Fatal(err)
	}
	if pass.Green || !slices.Equal(pass.Failing, []string{"setup-2"}) || !slices.Equal(pass.NotRun, []string{"setup-3", "acceptance", "build"}) {
		t.Errorf("verify.Pass: Green=%v Failing=%v NotRun=%v, want setup-2 failing and everything after it not run", pass.Green, pass.Failing, pass.NotRun)
	}
	if len(pass.Setup) != 2 || pass.Setup[1] != (verify.CommandResult{Name: "setup-2", Command: "echo two; exit 5", Status: 5, Output: "two\n"}) {
		t.Errorf("verify.Pass.Setup = %+v", pass.Setup)
	}
	if pass.Acceptance != nil {
		t.Errorf("verify.Pass.Acceptance = %+v, want nil: not run", pass.Acceptance)
	}
	if fb := pass.Feedback("run", "finish"); !strings.Contains(fb, "check setup-2 (echo two; exit 5): failed, exit status 5\n  two") {
		t.Errorf("verify.Pass.Feedback = %q, want the failed setup command with its output", fb)
	}
}

func TestVerifyStageGate(t *testing.T) {
	p := newProj(t)
	s, _ := p.session()
	p.cfg.Setup = []string{"echo setup-ran"}
	p.cfg.Checks = []project.Check{{Name: "build", Run: "echo never"}}
	in := p.input(4)
	in.Stage = &verify.Stage{ID: "s1", Checks: []string{"echo one", "exit 6", "echo three"}}
	pass, err := verify.Verify(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}
	if pass.Green || pass.Acceptance != nil || pass.Model.State != "none" {
		t.Errorf("verify.Pass: Green=%v Acceptance=%+v Model=%+v, want a red scoped pass without acceptance", pass.Green, pass.Acceptance, pass.Model)
	}
	if len(pass.Setup) != 1 || pass.Setup[0].Name != "setup-1" {
		t.Errorf("verify.Pass.Setup = %+v, want the setup commands to run", pass.Setup)
	}
	if len(pass.Checks) != 2 || pass.Checks[0].Name != "stage-s1-1" || pass.Checks[0].Command != "echo one" || pass.Checks[1].Name != "stage-s1-2" || pass.Checks[1].Status != 6 {
		t.Errorf("verify.Pass.Checks = %+v, want the stage's checks named stage-s1-<n>, not the project's", pass.Checks)
	}
	if !slices.Equal(pass.Failing, []string{"stage-s1-2"}) || !slices.Equal(pass.NotRun, []string{"stage-s1-3"}) {
		t.Errorf("verify.Pass: Failing=%v NotRun=%v", pass.Failing, pass.NotRun)
	}
	// Even all green, a scoped pass is no proof of the whole Intent.
	in.Stage = &verify.Stage{ID: "s2", Checks: []string{"true"}}
	pass, err = verify.Verify(context.Background(), s, in)
	if err != nil || !pass.Green {
		t.Fatalf("verify.Verify: green stage gate: %+v, %v", pass, err)
	}
	expected, _ := project.ParseGherkin(feature)
	if pass.Complete(p.cfg, expected, nil) {
		t.Errorf("verify.Pass.Complete: a scoped stage pass proves the whole Intent")
	}
}

func TestVerifyMovedFiles(t *testing.T) {
	// A check that changes the scratch copy's files.
	p := newProj(t)
	s, _ := p.session()
	p.cfg.Checks = []project.Check{{Name: "build", Run: "echo changed >> app.txt"}}
	pass, err := verify.Verify(context.Background(), s, p.input(1))
	if err != nil {
		t.Fatal(err)
	}
	if pass.Green || !slices.Equal(pass.Moved, []string{"app.txt"}) || len(pass.Failing) != 0 || len(pass.NotRun) != 0 || !pass.Clean {
		t.Errorf("verify.Pass: Green=%v Moved=%v Failing=%v NotRun=%v, want a pass not green because app.txt moved", pass.Green, pass.Moved, pass.Failing, pass.NotRun)
	}
	if fb := pass.Feedback("run", "finish"); !strings.HasSuffix(fb, "files changed while checks ran: app.txt") {
		t.Errorf("verify.Pass.Feedback = %q, want the moved files named", fb)
	}

	// A check that changes the checkout.
	p = newProj(t)
	s, _ = p.session()
	p.cfg.Checks = []project.Check{{Name: "build", Run: "echo changed > " + quote(filepath.Join(p.root, "moved.txt"))}}
	pass, err = verify.Verify(context.Background(), s, p.input(1))
	if err != nil {
		t.Fatal(err)
	}
	if pass.Green || !slices.Equal(pass.Moved, []string{"moved.txt"}) || pass.Tree != p.headTree() {
		t.Errorf("verify.Pass: Green=%v Moved=%v Tree=%q, want moved.txt moved and the tree checked before the change", pass.Green, pass.Moved, pass.Tree)
	}
}

func TestVerifyExcludedIntents(t *testing.T) {
	p := newProj(t)
	s, _ := p.session()
	p.write(".kogen/intents/other/intent.md", "# another uncommitted intent\n")
	p.cfg.Checks = []project.Check{{Name: "list", Run: "ls .kogen/intents"}}

	in := p.input(1)
	in.Excluded = []string{".kogen/intents/other"}
	pass, err := verify.Verify(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}
	if !pass.Green || pass.Tree != p.headTree() || pass.Checks[0].Output != "fixture\n" {
		t.Errorf("verify.Input.Excluded: Green=%v Tree=%q (HEAD tree %q) list=%q, want the excluded folder left out of the candidate", pass.Green, pass.Tree, p.headTree(), pass.Checks[0].Output)
	}
	in.Excluded = nil
	pass, err = verify.Verify(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}
	if pass.Tree == p.headTree() || pass.Checks[0].Output != "fixture\nother\n" {
		t.Errorf("verify.Verify: without exclusion Tree=%q list=%q, want the folder in the candidate", pass.Tree, pass.Checks[0].Output)
	}
}

func TestVerifyIntactRefusal(t *testing.T) {
	p := newProj(t)
	s, _ := p.session()
	p.cfg.Checks = []project.Check{{Name: "build", Run: "echo ran"}}
	refusal := sys.Fail("baseline", "changed", "the approved baseline changed", 1)
	in := p.input(1)
	in.Intact = func() error { return refusal }
	pass, err := verify.Verify(context.Background(), s, in)
	if err != error(refusal) || pass.N != 0 || pass.Tree != "" || pass.Checks != nil {
		t.Errorf("verify.Input.Intact: Verify gave (%+v, %v), want an empty pass and the refusal as is", pass, err)
	}
}

func TestVerifyModelAndMaintainedModels(t *testing.T) {
	p := newProj(t)
	s, _ := p.session()
	trace := filepath.Join(p.aux, "quint.txt")
	node := script(t, "node", "echo \"$@\" >> "+quote(trace)+"\ncase \"$*\" in *'--invariant bad'*) echo '[violation] Invariant violated'; exit 1;; *typecheck*broken*) echo 'type error'; exit 2;; esac\n")
	tools := verify.Tools{Node: node, Quint: "/q/cli.js"}
	const modelPath = ".kogen/intents/fixture/model.qnt"
	p.write(modelPath, "module m { val x = 1 }\n")
	p.write("specs/ok.qnt", "module ok { val x = 1 }\n")
	p.write("specs/broken.qnt", "module broken { val x = 1 }\n")
	p.commit("models")
	p.cfg.Checks = []project.Check{{Name: "build", Run: "echo built"}}
	spec := func(path string, invariants ...string) project.ModelSpec {
		return project.ModelSpec{Path: path, Invariants: invariants, Seed: "0x2a", Samples: "100", Steps: "7"}
	}
	var _ bool = project.ModelSpec{}.None
	var _ string = spec("p").Path
	var _ []string = spec("p").Invariants
	var _ string = spec("p").Seed
	var _ string = spec("p").Samples
	var _ string = spec("p").Steps

	// The Intent's model passes: typecheck, then one run per invariant with the recorded seed and bounds.
	model := spec(modelPath, "inv")
	p.intent.Model = &model
	in := p.input(1)
	in.Tools = tools
	in.MaintainedModels = []project.ModelSpec{spec("specs/ok.qnt", "keep")}
	pass, err := verify.Verify(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}
	if !pass.Green || pass.Model.State != "ok" || pass.Model.Problems == nil || len(pass.Model.Problems) != 0 {
		t.Errorf("verify.Pass.Model = %+v Green=%v, want ok with no problems", pass.Model, pass.Green)
	}
	if len(pass.MaintainedModels) != 1 || pass.MaintainedModels[0].Path != "specs/ok.qnt" || pass.MaintainedModels[0].Result.State != "ok" {
		t.Errorf("verify.Pass.MaintainedModels = %+v, want specs/ok.qnt ok", pass.MaintainedModels)
	}
	var _ string = pass.MaintainedModels[0].Path
	var _ verify.ModelResult = pass.MaintainedModels[0].Result
	b, _ := os.ReadFile(trace)
	wantTrace := "/q/cli.js typecheck " + modelPath + "\n" +
		"/q/cli.js run " + modelPath + " --backend typescript --invariant inv --seed 0x2a --max-samples 100 --max-steps 7\n" +
		"/q/cli.js typecheck specs/ok.qnt\n" +
		"/q/cli.js run specs/ok.qnt --backend typescript --invariant keep --seed 0x2a --max-samples 100 --max-steps 7\n"
	if string(b) != wantTrace {
		t.Errorf("verify.Input.Tools: quint invocations\n%s\nwant\n%s", b, wantTrace)
	}
	expected, _ := project.ParseGherkin(feature)
	if !pass.Complete(p.cfg, expected, p.intent.Model) {
		t.Errorf("verify.Pass.Complete: a green pass with a passed model is not complete")
	}
	if pass.Complete(p.cfg, expected, nil) {
		t.Errorf("verify.Pass.Complete: a passed model counts as not applicable")
	}
	if got, want := pass.ChecksLine(), "acceptance 2/2, model passed, build passed"; got != want {
		t.Errorf("verify.Pass.ChecksLine = %q, want %q", got, want)
	}

	// A violated invariant fails the model and keeps what came after it from running.
	os.Remove(trace)
	bad := spec(modelPath, "bad")
	p.intent.Model = &bad
	in = p.input(2)
	in.Tools = tools
	in.MaintainedModels = []project.ModelSpec{spec("specs/broken.qnt")}
	pass, err = verify.Verify(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}
	if pass.Green || pass.Model.State != "failed" || len(pass.Model.Problems) != 1 ||
		!strings.HasPrefix(pass.Model.Problems[0], "model.qnt: invariant bad violated (seed 0x2a)\n") || !strings.Contains(pass.Model.Problems[0], "[violation] Invariant violated") {
		t.Errorf("verify.Pass.Model = %+v Green=%v, want a failed model naming the violated invariant and the seed", pass.Model, pass.Green)
	}
	if !slices.Equal(pass.Failing, []string{"model"}) || !slices.Equal(pass.NotRun, []string{"specs/broken.qnt", "build"}) {
		t.Errorf("verify.Pass: Failing=%v NotRun=%v", pass.Failing, pass.NotRun)
	}
	if len(pass.MaintainedModels) != 1 || pass.MaintainedModels[0].Result.State != "not_run" {
		t.Errorf("verify.Pass.MaintainedModels = %+v, want the maintained model not run", pass.MaintainedModels)
	}
	fb := pass.Feedback("run", "finish")
	if !strings.Contains(fb, "\nmodel: failed\n  model.qnt: invariant bad violated (seed 0x2a)\n") {
		t.Errorf("verify.Pass.Feedback = %q, want the failed model with indented problems", fb)
	}

	// A maintained model that fails names its own path.
	good := spec(modelPath)
	p.intent.Model = &good
	in = p.input(3)
	in.Tools = tools
	in.MaintainedModels = []project.ModelSpec{spec("specs/broken.qnt")}
	pass, err = verify.Verify(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}
	if pass.Green || pass.Model.State != "ok" || len(pass.MaintainedModels) != 1 || pass.MaintainedModels[0].Result.State != "failed" ||
		len(pass.MaintainedModels[0].Result.Problems) != 1 || !strings.HasPrefix(pass.MaintainedModels[0].Result.Problems[0], "specs/broken.qnt: quint typecheck failed (status 2)\n") {
		t.Errorf("verify.Pass.MaintainedModels = %+v Model=%+v, want specs/broken.qnt failed with its own path in the problem", pass.MaintainedModels, pass.Model)
	}
	if !slices.Equal(pass.Failing, []string{"specs/broken.qnt"}) || !slices.Equal(pass.NotRun, []string{"build"}) {
		t.Errorf("verify.Pass: Failing=%v NotRun=%v", pass.Failing, pass.NotRun)
	}
	if fb := pass.Feedback("run", "finish"); !strings.Contains(fb, "\nmodel specs/broken.qnt: failed\n  specs/broken.qnt: quint typecheck failed (status 2)\n") {
		t.Errorf("verify.Pass.Feedback = %q, want the failed maintained model with its path", fb)
	}
}
