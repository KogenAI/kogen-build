package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
)

func TestBriefCapsBoundaries(t *testing.T) {
	dir := physical(t, t.TempDir())
	env := isolatedEnv()
	initGit(t, dir, env)
	text := "# Fixture\nSlug: fixture\nModel: not applicable — fixture\n\nAcceptance:\n- .kogen/intents/fixture/acceptance.feature\n\n## Tests\n- none: fixture\n"
	writeFile(t, filepath.Join(dir, ".kogen/intents/fixture/intent.md"), text, 0644)
	writeFile(t, filepath.Join(dir, ".kogen/intents/fixture/acceptance.feature"), "Feature: test\n", 0644)
	readme := "abéz"
	writeFile(t, filepath.Join(dir, "README.md"), readme, 0644)
	writeFile(t, filepath.Join(dir, "a"), "a", 0644)
	in, err := project.LoadIntent(dir, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	b := &Attempt{State: State{Root: dir, Slug: "fixture", Config: &project.Config{}}, intent: in, repo: git.New(dir, env)}
	full, err := b.brief()
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(full.Dynamic, "# Files\n\n") + len("# Files\n\n")
	lines := strings.SplitAfter(full.Dynamic[start:], "\n")
	lines = lines[:len(lines)-1]
	for _, cap := range []int{-1, 0, len(readme) - 1, len(readme), len(readme) + 1, 3} {
		t.Run(fmt.Sprintf("readme-%d", cap), func(t *testing.T) {
			b.deps = Deps{BuildBriefMaxReadmeBytes: cap}
			got, err := b.brief()
			if err != nil {
				t.Fatal(err)
			}
			want := full.Stable
			if cap > 0 && cap < len(readme) {
				want = strings.Replace(want, readme, readme[:cap]+fmt.Sprintf("\n[README.md cut: %d bytes omitted; read_file README.md for the rest.]", len(readme)-cap), 1)
			}
			if got.Stable != want || got.Dynamic != full.Dynamic {
				t.Fatal("readme cap changed unexpected content")
			}
		})
	}
	for _, cap := range []int{-1, 0, len(lines) - 1, len(lines), len(lines) + 1} {
		t.Run(fmt.Sprintf("files-%d", cap), func(t *testing.T) {
			b.deps = Deps{BuildBriefMaxFiles: cap}
			got, err := b.brief()
			if err != nil {
				t.Fatal(err)
			}
			want := full.Dynamic
			if cap > 0 && cap < len(lines) {
				want = full.Dynamic[:start] + strings.Join(lines[:cap], "") + fmt.Sprintf("[File list cut: %d entries omitted; use shell git ls-files --cached --others --exclude-standard for the rest.]", len(lines)-cap)
			}
			if got.Stable != full.Stable || got.Dynamic != want {
				t.Fatal("file cap changed unexpected content")
			}
		})
	}
	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	b.deps = Deps{BuildBriefMaxReadmeBytes: 1}
	missing, err := b.brief()
	if err != nil {
		t.Fatal(err)
	}
	contains(t, "missing README", missing.Stable, "(no README.md)")
	if strings.Contains(missing.Stable, "[README.md cut:") {
		t.Fatal("missing README was capped")
	}
	writeFile(t, filepath.Join(dir, "README.md"), readme, 0644)
	b.ext = &externalBuild{}
	b.deps = Deps{BuildBriefMaxFiles: 1, BuildBriefMaxReadmeBytes: 1}
	got, err := b.brief()
	if err != nil {
		t.Fatal(err)
	}
	if got != full {
		t.Fatal("external brief was capped")
	}
}

func TestBriefCapsReachNativeBuild(t *testing.T) {
	e := newEnv(t)
	e.prov.then(landScript(e.prov)...)
	a := e.build(func(in *Input) { in.BuildBriefMaxFiles = 1; in.BuildBriefMaxReadmeBytes = 1 })
	if a.exit != 0 {
		t.Fatalf("exit %d %s", a.exit, a.shown())
	}
	first := e.prov.requests()[0]["input"].([]any)[0].(map[string]any)
	blocks := first["content"].([]any)
	if len(blocks) != 2 {
		t.Fatalf("%d blocks", len(blocks))
	}
	stable := blocks[0].(map[string]any)["text"].(string)
	dynamic := blocks[1].(map[string]any)["text"].(string)
	contains(t, "stable", stable, "[README.md cut:")
	contains(t, "stable", stable, "# Intent: Preserve the fixture")
	contains(t, "dynamic", dynamic, "[File list cut:")
}
