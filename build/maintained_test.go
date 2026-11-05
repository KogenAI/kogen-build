package build

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

func TestSuccessiveModelIntentsKeepIndependentSpecifications(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	original := e.read(project.IntentMDPath(testSlug))
	model := "module fixture { val Safe = true }\n"
	e.write(project.ModelPath(testSlug), model)
	e.write(project.IntentMDPath(testSlug), strings.Replace(original, "not applicable — no model for this fixture", project.ModelPath(testSlug)+" invariants=Safe,Other seed=007 samples=03 steps=04", 1))
	e.approve()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(e.aux, "models.trace")
	tool := filepath.Join(e.aux, "quint.js")
	writeFile(t, tool, `const fs=require('fs');fs.appendFileSync(process.env.MODEL_TRACE,JSON.stringify(process.argv.slice(2))+'\n');`, 0644)
	e.extraEnv = append(e.extraEnv, "MODEL_TRACE="+trace)
	run := func(slug string) {
		e.prov.then(landScript(e.prov)...)
		a := e.build(func(in *Input) { in.Slug = slug; in.Deps.Tools = verify.Tools{Node: node, Quint: tool} })
		if a.exit != 0 {
			t.Fatalf("%s: %s", slug, a.shown())
		}
	}
	run(testSlug)
	firstRecord := e.read(".kogen/models/fixture/model.json")
	if e.read(".kogen/models/fixture/model.qnt") != model || e.read(project.ModelPath(testSlug)) != model {
		t.Fatal("model bytes changed")
	}
	slug := "later"
	laterModel := "module later { val Good = true }\n"
	e.write(project.ModelPath(slug), laterModel)
	md := strings.ReplaceAll(original, "fixture", slug)
	e.write(project.IntentMDPath(slug), strings.Replace(md, "not applicable — no model for this later", project.ModelPath(slug)+" invariants=Good seed=9 samples=5 steps=6", 1))
	e.write(project.FeaturePath(slug), testFeature)
	green := filepath.Join(e.aux, "later.ndjson")
	writeFile(t, green, reportFor(t, project.FeaturePath(slug), testFeature, "PASSED"), 0644)
	// accept.sh is a link to the read-only script every fixture shares: replace it, never write through it.
	if err := os.Remove(filepath.Join(e.aux, "accept.sh")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(e.aux, "accept.sh"), "#!/bin/sh\ncp '"+green+"' \"$KOGEN_ACCEPTANCE_REPORT\"\n", 0755)
	cfg, err := project.LoadConfig(e.root)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := project.Baseline(e.root, slug, cfg)
	if err != nil {
		t.Fatal(err)
	}
	err = project.WriteApproval(e.root, project.Approval{Slug: slug, SHA256: project.Hash(e.root, baseline), Base: e.head(), Approver: "Kogen Integration <kogen-integration@example.invalid>", ApprovedAt: "2026-10-22T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(trace, nil, 0644); err != nil {
		t.Fatal(err)
	}
	run(slug)
	if e.read(".kogen/models/fixture/model.json") != firstRecord || e.read(".kogen/models/later/model.qnt") != laterModel {
		t.Fatal("maintained models changed")
	}
	specs, err := maintainedModels(e.root)
	if err != nil {
		t.Fatal(err)
	}
	want := []project.ModelSpec{{Path: ".kogen/models/fixture/model.qnt", Invariants: []string{"Safe", "Other"}, Seed: "007", Samples: "03", Steps: "04"}, {Path: ".kogen/models/later/model.qnt", Invariants: []string{"Good"}, Seed: "9", Samples: "5", Steps: "6"}}
	if !reflect.DeepEqual(specs, want) {
		t.Fatalf("specs: %#v", specs)
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var args []string
		if err = json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, args)
	}
	for _, m := range want {
		for _, inv := range m.Invariants {
			expected := []string{"run", m.Path, "--backend", "typescript", "--invariant", inv, "--seed", m.Seed, "--max-samples", m.Samples, "--max-steps", m.Steps}
			found := false
			for _, call := range calls {
				if reflect.DeepEqual(call, expected) {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %v in %v", expected, calls)
			}
		}
	}
	for _, p := range []string{".kogen/models/fixture/model.json", ".kogen/models/later/model.json", ".kogen/models/later/model.qnt"} {
		e.gitOut("show", "HEAD:"+p)
	}
}

func TestModelFreeBuildDoesNotCreateModelStorage(t *testing.T) {
	e := newEnv(t)
	e.prov.then(landScript(e.prov)...)
	a := e.build()
	if a.exit != 0 {
		t.Fatal(a.shown())
	}
	if _, err := os.Stat(filepath.Join(e.root, ".kogen/models")); !os.IsNotExist(err) {
		t.Fatalf("model storage: %v", err)
	}
}

func TestResumeVerifiesMaintainedModel(t *testing.T) {
	e := newEnv(t)
	spec := project.ModelSpec{Path: ".kogen/models/earlier/model.qnt", Invariants: []string{"Safe"}, Seed: "7", Samples: "3", Steps: "4"}
	record, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	e.write(spec.Path, "module earlier { val Safe = true }\n")
	e.write(".kogen/models/earlier/model.json", string(record))
	e.run("git", "add", ".kogen/models")
	e.run("git", "commit", "-qm", "maintained model")
	e.approve()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(e.aux, "resume.trace")
	tool := filepath.Join(e.aux, "resume.js")
	writeFile(t, tool, `const fs=require('fs');fs.appendFileSync(process.env.MODEL_TRACE,JSON.stringify(process.argv.slice(2))+'\n');`, 0644)
	e.extraEnv = append(e.extraEnv, "MODEL_TRACE="+trace)
	id, _ := e.interruptedAtRequest(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}))
	e.prov.replace(e.prov.calls("finish", map[string]any{"summary": "done"}))
	a := e.resume(id, func(in *ResumeInput) { in.Deps.Tools = verify.Tools{Node: node, Quint: tool} })
	if a.exit != 0 {
		t.Fatal(a.shown())
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `["run",".kogen/models/earlier/model.qnt","--backend","typescript","--invariant","Safe","--seed","7","--max-samples","3","--max-steps","4"]`) {
		t.Fatalf("resume trace: %s", data)
	}
}
