package consumes

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

// completePass is a pass of cfg that proves every gate for the scenarios.
func completePass(cfg *project.Config, expected []project.Scenario, model string) verify.Pass {
	a := verify.Acceptance{Passed: len(expected), Scenarios: []verify.ScenarioResult{}, Problems: []string{}}
	for _, e := range expected {
		a.Scenarios = append(a.Scenarios, verify.ScenarioResult{Name: e.Name, Line: e.Line, Status: "passed"})
	}
	p := verify.Pass{N: 1, Clean: true, Green: true, Tree: "abc", Acceptance: &a, Model: verify.ModelResult{State: model, Problems: []string{}},
		Setup: []verify.CommandResult{}, Checks: []verify.CommandResult{}, NotRun: []string{}, Moved: []string{}, Failing: []string{}}
	for i, c := range cfg.Setup {
		p.Setup = append(p.Setup, verify.CommandResult{Name: "setup-" + itoa(i+1), Command: c})
	}
	for _, c := range cfg.Checks {
		p.Checks = append(p.Checks, verify.CommandResult{Name: c.Name, Command: c.Run})
	}
	return p
}

func TestPassComplete(t *testing.T) {
	var _ func(verify.Pass, *project.Config, []project.Scenario, *project.ModelSpec) bool = verify.Pass.Complete

	cfg := &project.Config{Setup: []string{"npm ci"}, Checks: []project.Check{{Name: "build", Run: "go build"}, {Name: "test", Run: "go test"}}}
	expected, err := project.ParseGherkin(feature)
	if err != nil {
		t.Fatal(err)
	}
	none := &project.ModelSpec{None: true}
	withModel := &project.ModelSpec{Path: "model.qnt"}

	if p := completePass(cfg, expected, "none"); !p.Complete(cfg, expected, nil) || !p.Complete(cfg, expected, none) {
		t.Errorf("verify.Pass.Complete: a pass proving every gate, model not applicable, is not complete")
	}
	if p := completePass(cfg, expected, "ok"); !p.Complete(cfg, expected, withModel) {
		t.Errorf("verify.Pass.Complete: a pass proving every gate and a passed model is not complete")
	}
	// A model that passed does not count for an Intent with none, and the other way round.
	if p := completePass(cfg, expected, "ok"); p.Complete(cfg, expected, nil) {
		t.Errorf("verify.Pass.Complete: model ok accepted where the Intent has no model")
	}
	if p := completePass(cfg, expected, "none"); p.Complete(cfg, expected, withModel) {
		t.Errorf("verify.Pass.Complete: model none accepted where the Intent has a model")
	}

	for name, mutate := range map[string]func(p *verify.Pass){
		"not clean":           func(p *verify.Pass) { p.Clean = false },
		"not green":           func(p *verify.Pass) { p.Green = false },
		"moved":               func(p *verify.Pass) { p.Moved = []string{"a"} },
		"failing":             func(p *verify.Pass) { p.Failing = []string{"build"} },
		"not run":             func(p *verify.Pass) { p.NotRun = []string{"test"} },
		"acceptance not run":  func(p *verify.Pass) { p.Acceptance = nil },
		"acceptance status":   func(p *verify.Pass) { p.Acceptance.Status = 1 },
		"acceptance problems": func(p *verify.Pass) { p.Acceptance.Problems = []string{"x"} },
		"scenario missing":    func(p *verify.Pass) { p.Acceptance.Scenarios = p.Acceptance.Scenarios[:1] },
		"scenario failed":     func(p *verify.Pass) { p.Acceptance.Scenarios[1].Status = "failed" },
		"scenario name":       func(p *verify.Pass) { p.Acceptance.Scenarios[0].Name = "Other" },
		"scenario line":       func(p *verify.Pass) { p.Acceptance.Scenarios[0].Line = 99 },
		"passed count":        func(p *verify.Pass) { p.Acceptance.Passed = 1 },
		"model failed":        func(p *verify.Pass) { p.Model = verify.ModelResult{State: "failed", Problems: []string{"x"}} },
		"model not run":       func(p *verify.Pass) { p.Model = verify.ModelResult{State: "not_run"} },
		"check missing":       func(p *verify.Pass) { p.Checks = p.Checks[:1] },
		"check name":          func(p *verify.Pass) { p.Checks[0].Name = "other" },
		"check command":       func(p *verify.Pass) { p.Checks[1].Command = "go test -short" },
		"check status":        func(p *verify.Pass) { p.Checks[1].Status = 1 },
		"check order":         func(p *verify.Pass) { p.Checks[0], p.Checks[1] = p.Checks[1], p.Checks[0] },
		"setup missing":       func(p *verify.Pass) { p.Setup = nil },
		"setup name":          func(p *verify.Pass) { p.Setup[0].Name = "setup-2" },
		"setup command":       func(p *verify.Pass) { p.Setup[0].Command = "npm install" },
		"setup status":        func(p *verify.Pass) { p.Setup[0].Status = 1 },
	} {
		p := completePass(cfg, expected, "none")
		mutate(&p)
		if p.Complete(cfg, expected, nil) {
			t.Errorf("verify.Pass.Complete: a pass with %s is complete", name)
		}
	}

	// A pass recorded before setup and not_run existed stays reusable without them.
	legacy := completePass(cfg, expected, "none")
	legacy.Legacy, legacy.Setup, legacy.NotRun = true, nil, nil
	if !legacy.Complete(cfg, expected, nil) {
		t.Errorf("verify.Pass.Legacy: a legacy pass without setup is not complete")
	}
	legacy.Legacy = false
	if legacy.Complete(cfg, expected, nil) {
		t.Errorf("verify.Pass.Legacy: a pass without setup commands is complete although it is not legacy")
	}
	// Other expectations.
	if p := completePass(cfg, expected, "none"); p.Complete(&project.Config{Setup: cfg.Setup, Checks: cfg.Checks[:1]}, expected, nil) || p.Complete(cfg, expected[:1], nil) {
		t.Errorf("verify.Pass.Complete: a pass is complete for another configuration or another feature")
	}
}

func TestPassValid(t *testing.T) {
	var _ func(verify.Pass) bool = verify.Pass.Valid

	ok := verify.Pass{N: 1, Clean: true, Tree: "abc", Model: verify.ModelResult{State: "none"}}
	if !ok.Valid() {
		t.Errorf("verify.Pass.Valid: clean, with a tree and a model, acceptance not run")
	}
	withAcceptance := ok
	withAcceptance.Acceptance = &verify.Acceptance{Scenarios: []verify.ScenarioResult{}}
	if !withAcceptance.Valid() {
		t.Errorf("verify.Pass.Valid: with well-formed acceptance")
	}
	for name, mutate := range map[string]func(p *verify.Pass){
		"not clean":                    func(p *verify.Pass) { p.Clean = false },
		"no tree":                      func(p *verify.Pass) { p.Tree = "" },
		"no model":                     func(p *verify.Pass) { p.Model = verify.ModelResult{} },
		"acceptance without scenarios": func(p *verify.Pass) { p.Acceptance = &verify.Acceptance{} },
	} {
		p := ok
		mutate(&p)
		if p.Valid() {
			t.Errorf("verify.Pass.Valid: a pass with %s is valid", name)
		}
	}
}

func TestPassJSON(t *testing.T) {
	// build keeps a pass in its checkpoints as this JSON.
	p := verify.Pass{N: 2, Clean: true, Green: false, Tree: "abc", Model: verify.ModelResult{State: "failed", Problems: []string{"p1"}},
		Setup:  []verify.CommandResult{{Name: "setup-1", Command: "x", Status: 0, Output: "o"}},
		Checks: []verify.CommandResult{{Name: "build", Command: "go build", Status: 1, Output: "bad"}}, NotRun: []string{"test"}, Failing: []string{"build", "model"}, Moved: []string{"a.txt"},
		MaintainedModels: []verify.MaintainedModelResult{{Path: "specs/m.qnt", Result: verify.ModelResult{State: "ok", Problems: []string{}}}}}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"n":2,"clean":true,"green":false,"tree":"abc","setup":[{"name":"setup-1","command":"x","status":0,"output":"o"}],"acceptance":"not_run","model":{"failed":["p1"]},` +
		`"maintained_models":[{"path":"specs/m.qnt","result":{"state":"ok","problems":[]}}],"checks":[{"name":"build","command":"go build","status":1,"output":"bad"}],"not_run":["test"],"moved":["a.txt"],"failing":["build","model"]}`
	if string(b) != want {
		t.Errorf("verify.Pass: JSON\n%s\nwant\n%s", b, want)
	}
	var back verify.Pass
	if err := json.Unmarshal(b, &back); err != nil || !reflect.DeepEqual(back, p) {
		t.Errorf("verify.Pass: decoded %+v (%v), want %+v", back, err, p)
	}
	if back.Legacy {
		t.Errorf("verify.Pass.Legacy: a record with setup and not_run is legacy")
	}

	// Model values and empty lists.
	for _, c := range []struct {
		model verify.ModelResult
		want  string
	}{
		{verify.ModelResult{State: "none"}, `"model":"none"`},
		{verify.ModelResult{State: ""}, `"model":"not_run"`},
		{verify.ModelResult{State: "not_run"}, `"model":"not_run"`},
		{verify.ModelResult{State: "ok"}, `"model":{"ok":[]}`},
	} {
		b, _ := json.Marshal(verify.Pass{Model: c.model})
		for _, w := range []string{c.want, `"checks":[]`, `"setup":[]`, `"not_run":[]`, `"moved":[]`, `"failing":[]`} {
			if !strings.Contains(string(b), w) {
				t.Errorf("verify.Pass: JSON of model %q = %s, want %s", c.model.State, b, w)
			}
		}
	}

	// A record from before setup and not_run existed is a legacy pass and stays that way.
	old := `{"n":1,"clean":true,"green":true,"tree":"t","acceptance":"not_run","model":"none","checks":[],"moved":[],"failing":[]}`
	var legacy verify.Pass
	if err := json.Unmarshal([]byte(old), &legacy); err != nil || !legacy.Legacy || legacy.N != 1 || !legacy.Green || legacy.Acceptance != nil || legacy.Model.State != "none" {
		t.Errorf("verify.Pass.Legacy: decoded %+v (%v), want a legacy pass", legacy, err)
	}
	if b, _ := json.Marshal(legacy); strings.Contains(string(b), `"setup"`) || strings.Contains(string(b), `"not_run":[`) {
		t.Errorf("verify.Pass.Legacy: a legacy pass encodes setup or not_run: %s", b)
	}
	if err := json.Unmarshal([]byte(`{"n":1,"model":"none","acceptance":"not_run","setup":[]}`), &legacy); err != nil || legacy.Legacy {
		t.Errorf("verify.Pass.Legacy: a record with a setup key is legacy (%v)", err)
	}
	if err := json.Unmarshal([]byte(`{"n":1,"model":{"weird":[]}}`), &legacy); err == nil {
		t.Errorf("verify.Pass: an unknown model state decoded")
	}
}

func TestPassFeedbackSections(t *testing.T) {
	p := verify.Pass{N: 7,
		Acceptance: &verify.Acceptance{Status: 1, Problems: []string{"failed: First (line 2)\ndetail"}},
		Model:      verify.ModelResult{State: "failed", Problems: []string{"model.qnt: bad"}},
		MaintainedModels: []verify.MaintainedModelResult{
			{Path: "specs/a.qnt", Result: verify.ModelResult{State: "ok", Problems: []string{}}},
			{Path: "specs/b.qnt", Result: verify.ModelResult{State: "failed", Problems: []string{"specs/b.qnt: bad"}}},
		},
		Setup:  []verify.CommandResult{{Name: "setup-1", Command: "npm ci", Status: 4, Output: "npm failed\n"}},
		Checks: []verify.CommandResult{{Name: "lint", Command: "golint", Status: 0, Output: "fine"}, {Name: "vet", Command: "go vet", Status: 2, Output: "vet bad\n\n"}},
		Moved:  []string{"a.txt", "b.txt"}, NotRun: []string{"test", "e2e"}}
	got := p.Feedback("./accept.sh", "")
	want := strings.Join([]string{
		"Verification pass 7 is not green. Fix the implementation (the approved acceptance files cannot change), then call finish again.",
		"acceptance (./accept.sh): failed, exit status 1",
		"  failed: First (line 2)",
		"  detail",
		"model: failed",
		"  model.qnt: bad",
		"model specs/b.qnt: failed",
		"  specs/b.qnt: bad",
		"check setup-1 (npm ci): failed, exit status 4",
		"  npm failed",
		"check vet (go vet): failed, exit status 2",
		"  vet bad",
		"files changed while checks ran: a.txt, b.txt",
		"not run because an earlier item failed: test, e2e",
	}, "\n")
	if got != want {
		t.Errorf("verify.Pass.Feedback =\n%s\nwant\n%s", got, want)
	}
}

func TestPassChecksLine(t *testing.T) {
	var _ func(verify.Pass) string = verify.Pass.ChecksLine

	if got := (verify.Pass{Model: verify.ModelResult{State: "none"}}).ChecksLine(); got != "acceptance 0/0, model not applicable" {
		t.Errorf("verify.Pass.ChecksLine: acceptance not run = %q", got)
	}
	p := verify.Pass{Model: verify.ModelResult{State: "ok"}, Acceptance: &verify.Acceptance{Passed: 2, Scenarios: make([]verify.ScenarioResult, 3)},
		Checks: []verify.CommandResult{{Name: "a"}, {Name: "b"}}}
	if got, want := p.ChecksLine(), "acceptance 2/3, model passed, a passed, b passed"; got != want {
		t.Errorf("verify.Pass.ChecksLine = %q, want %q", got, want)
	}
}
