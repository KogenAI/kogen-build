package build

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
)

// Reference data written by testdata/gen/*.exs from the Elixir implementation (run with
// `mise exec -- mix run --no-start cli/build/testdata/gen/<script>.exs` in the repository root).
// `go test ./... -update` rewrites it from the current Go code's output.

// TestOpeningMessageEqualsTheElixirReference compares the builder's opening message (the stable
// and the dynamic block) of each case of testdata/context-golden.json with the golden's recorded
// blocks. The golden holds the case inputs and the two blocks the Elixir implementation produced;
// with -update the blocks are rewritten from the current output and the case inputs stay.
func TestOpeningMessageEqualsTheElixirReference(t *testing.T) {
	t.Parallel()
	// ExUnit: lib/kogen/build.ex context/1 (the builder's opening message, LOOP-2), replayed
	// through the reference code
	raw, err := os.ReadFile("testdata/context-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name          string     `json:"name"`
		ModelLine     string     `json:"model_line"`
		ModelQnt      *string    `json:"model_qnt"`
		Readme        *string    `json:"readme"`
		Setup         []string   `json:"setup"`
		Fix           []string   `json:"fix"`
		Checks        [][]string `json:"checks"`
		AcceptanceRun *string    `json:"acceptance_run"`
		Baseline      []string   `json:"baseline"`
		Stable        string     `json:"stable"`
		Dynamic       string     `json:"dynamic"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	type blocksOf struct{ stable, dynamic string }
	actual := make([]*blocksOf, len(cases))
	if *update {
		updateGoldenAfter(t, func() {
			updateGoldenJSON(t, "testdata/context-golden.json", compactGolden, func(doc any) any {
				list := goldenList(t, doc)
				for i, got := range actual {
					if got == nil {
						// A case that did not run (a -run filter) keeps its recorded blocks.
						continue
					}
					c := goldenObject(t, list[i])
					c["stable"], c["dynamic"] = got.stable, got.dynamic
				}
				return list
			})
		})
	}
	for i, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			dir := physical(t, t.TempDir())
			env := isolatedEnv()
			cmd := exec.Command("git", "init", "-q")
			cmd.Dir, cmd.Env = dir, env
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v %s", err, out)
			}
			intentMD := "# Add a \"greeting\" 2 \\ é\nSlug: fixture\nModel: " + c.ModelLine + "\n\nAcceptance:\n- .kogen/intents/fixture/acceptance.feature\n- features/steps/s.js\n\n## Tests\n- one: unit\n"
			writeFile(t, filepath.Join(dir, ".kogen/intents/fixture/intent.md"), intentMD, 0o644)
			writeFile(t, filepath.Join(dir, ".kogen/intents/fixture/acceptance.feature"), "Feature: F\n  Scenario: S\n    Given a thing\n", 0o644)
			if c.ModelQnt != nil {
				writeFile(t, filepath.Join(dir, ".kogen/intents/fixture/model.qnt"), *c.ModelQnt, 0o644)
			}
			if c.Readme != nil {
				writeFile(t, filepath.Join(dir, "README.md"), *c.Readme, 0o644)
			}
			writeFile(t, filepath.Join(dir, "a.txt"), "a\n", 0o644)
			cfg := &project.Config{Setup: c.Setup, Fix: c.Fix}
			for _, ch := range c.Checks {
				cfg.Checks = append(cfg.Checks, project.Check{Name: ch[0], Run: ch[1]})
			}
			if c.AcceptanceRun != nil {
				cfg.Acceptance = &project.Acceptance{Run: *c.AcceptanceRun, StepsDir: "features/steps"}
			}
			intent, err := project.LoadIntent(dir, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			b := &Attempt{State: State{Root: dir, Slug: "fixture", Config: cfg, Baseline: c.Baseline}, intent: intent, repo: git.New(dir, env)}
			blocks, err := b.openingMessage()
			if err != nil {
				t.Fatal(err)
			}
			if len(blocks) != 2 {
				t.Fatalf("%d blocks", len(blocks))
			}
			stable := blocks[0].(map[string]any)["text"].(string)
			dynamic := blocks[1].(map[string]any)["text"].(string)
			if *update {
				actual[i] = &blocksOf{stable, dynamic}
				return
			}
			if stable != c.Stable {
				t.Fatalf("stable block differs\n got: %q\nwant: %q", stable, c.Stable)
			}
			if dynamic != c.Dynamic {
				t.Fatalf("dynamic block differs\n got: %q\nwant: %q", dynamic, c.Dynamic)
			}
		})
	}
}
