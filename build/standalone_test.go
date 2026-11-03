package build

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"kogen-loop/sys"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
)

func TestStandaloneBuildEvidence(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "kogen-build")
	cmd := exec.Command("go", "build", "-o", bin, "../cmd/kogen-build")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	for _, stopped := range []bool{false, true} {
		t.Run(map[bool]string{false: "landed", true: "stopped"}[stopped], func(t *testing.T) {
			e := newEnv(t, func(c *project.Config) {
				c.Checks = []project.Check{{Name: "first", Run: "true"}, {Name: "second", Run: "true"}}
			})
			e.credentials()
			e.prov.then(landScript(e.prov)...)
			if stopped {
				e.prov.then(refused())
			} else {
				e.prov.then(submit(e.prov, "approve"))
			}
			// An unrelated older run must not supply evidence for this invocation.
			decoy := filepath.Join(e.root, ".kogen/local/runs/00000000")
			writeFile(t, filepath.Join(decoy, "checkpoint.json"), `{"candidate":"decoy","state":{"last_pass":{"checks":[{"name":"decoy"}]}}}`, 0600)
			cmd := exec.Command(bin, "build", "--project", e.root, "--slug", testSlug)
			cmd.Env = isolatedEnv("HOME="+e.home, "KOGEN_PROVIDER_URL="+e.prov.srv.URL, "KOGEN_AUTH_URL="+e.prov.srv.URL)
			stdout, err := cmd.Output()
			if !stopped && err != nil {
				t.Fatalf("run: %v %s", err, cmd.Stderr)
			}
			if stopped && err == nil {
				t.Fatal("expected stop")
			}
			var result struct {
				Exit   int    `json:"exit_code"`
				Tree   string `json:"tree"`
				Checks []any  `json:"checks"`
			}
			if err := json.Unmarshal(stdout, &result); err != nil {
				t.Fatalf("json: %v %s stderr %s", err, stdout, cmd.Stderr)
			}
			dirs, err := os.ReadDir(filepath.Join(e.root, ".kogen/local/runs"))
			if err != nil {
				t.Fatal(err)
			}
			var id string
			for _, d := range dirs {
				if d.Name() != "00000000" {
					id = d.Name()
				}
			}
			if id == "" {
				t.Fatalf("no run: %s", cmd.Stderr)
			}
			doc, state := e.checkpoint(id)
			pass := state["last_pass"].(map[string]any)
			if result.Tree != doc["candidate"] || len(result.Checks) != 2 || !reflect.DeepEqual(any(result.Checks), pass["checks"]) {
				t.Fatalf("result %s checkpoint checks %v", stdout, pass["checks"])
			}
			if !stopped && (result.Exit != 0 || result.Tree != e.gitOut("rev-parse", "HEAD^{tree}") || e.jsonFile(e.runPath(id, "review-1.json"))["verdict"] != "approve") {
				t.Fatalf("not landed: %s", stdout)
			}
			if stopped && result.Exit == 0 {
				t.Fatalf("not stopped: %s", stdout)
			}
		})
	}
}

func TestStandaloneOutcomeUsesReportedStop(t *testing.T) {
	// The integration above exercises ordinary stops; context exhaustion is translated
	// by the phase before reporting and must retain that same structured error.
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	repo, err := git.Open(dir, sys.New().Env)
	if err != nil {
		t.Fatal(err)
	}
	b := &Attempt{repo: repo, State: State{Start: repo.Head(), Config: &project.Config{Models: map[string]project.Role{"build": {Model: "test"}}}}}
	var outcome Outcome
	recordOutcome(&outcome, b, sys.Fail("provider", "context_too_long", "too long", 1))
	if outcome.Error == nil || outcome.Error.Class != "candidate" || outcome.Error.Reason != "context_exhausted" {
		t.Fatalf("reported stop: %#v", outcome.Error)
	}
}
