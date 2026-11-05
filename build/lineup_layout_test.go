package build

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-loop/sys"
)

// lineupMoved reports where the coordinator these tests are compiled against reads the model
// lineup: from `.kogen/models.yaml` (true) or from the models: key of `.kogen/project.yaml`
// (false). The moved coordinator refuses a project.yaml with models:.
var lineupMoved = func() bool {
	_, err := project.ParseConfig("version: 1\nmodels:\n  build: {model: m, effort: low}\n", "f")
	var fail *sys.Error
	return errors.As(err, &fail) && fail.Class == "environment" && fail.Reason == "models_moved"
}()

const (
	projectYAML = ".kogen/project.yaml"
	modelsYAML  = ".kogen/models.yaml"
)

// roleEntryRe finds the entry of role in the models: block of project.yaml: its `  role:` line
// and the indented lines that follow.
func roleEntryRe(role string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(role) + `:.*\n(    .*\n)*`)
}

// movedRoleEntryRe is roleEntryRe for the top level of `.kogen/models.yaml`.
func movedRoleEntryRe(role string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(role) + `:.*\n(  .*\n)*`)
}

// dedent moves the entry of a role in a models: block (`  role: ...`) to the top level of
// `.kogen/models.yaml`.
func dedent(entry string) string {
	lines := strings.SplitAfter(entry, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(l, "  ")
	}
	return strings.Join(lines, "")
}

// setRole makes entry, the role's lines as the models: block of project.yaml writes them
// (`  build:\n    model: m\n    effort: high\n` or `  build: {model: m, effort: high}\n`), the
// role's lineup in the project, and stages the file it wrote. With the pinned coordinator it
// replaces the role in project.yaml's models: block; with the moved one it writes the role to
// `.kogen/models.yaml`, which keeps the other roles it names.
func (e *env) setRole(role, entry string) {
	e.t.Helper()
	if !lineupMoved {
		b, err := os.ReadFile(filepath.Join(e.root, projectYAML))
		if err != nil {
			e.t.Fatal(err)
		}
		re := roleEntryRe(role)
		if !re.Match(b) {
			e.t.Fatalf("no %s role in:\n%s", role, b)
		}
		e.write(projectYAML, re.ReplaceAllLiteralString(string(b), entry))
		e.run("git", "add", projectYAML)
		return
	}
	var text string
	if b, err := os.ReadFile(filepath.Join(e.root, modelsYAML)); err == nil {
		text = string(b)
	} else if !errors.Is(err, os.ErrNotExist) {
		e.t.Fatal(err)
	}
	if re := movedRoleEntryRe(role); re.MatchString(text) {
		text = re.ReplaceAllLiteralString(text, dedent(entry))
	} else {
		text += dedent(entry)
	}
	e.write(modelsYAML, text)
	e.run("git", "add", modelsYAML)
}

// loadLineup loads a configuration whose lineup is the roles in models, written as the
// models: block of project.yaml writes them (`  build: {model: m, effort: low}\n`), through
// the layout of the coordinator in use. Roles it omits keep project.DefaultModels().
func loadLineup(t *testing.T, models string) *project.Config {
	t.Helper()
	if !lineupMoved {
		cfg, err := project.ParseConfig("version: 1\nmodels:\n"+models, "f")
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, projectYAML), "version: 1\n", 0o644)
	writeFile(t, filepath.Join(root, modelsYAML), dedent(models), 0o644)
	cfg, err := project.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// lineupFileForms are the forms in which the Build's prompts name the protected files: the moved
// coordinator lists `.kogen/models.yaml` right after `.kogen/project.yaml`, as a sentence list or
// as list items (also inside JSON text, where the newline is written `\n` or `\\n`).
var lineupFileForms = [][2]string{
	{", .kogen/project.yaml.", ", .kogen/project.yaml, .kogen/models.yaml."},
	{"- .kogen/project.yaml\n", "- .kogen/project.yaml\n- .kogen/models.yaml\n"},
	{`- .kogen/project.yaml\n`, `- .kogen/project.yaml\n- .kogen/models.yaml\n`},
	{`- .kogen/project.yaml\\n`, `- .kogen/project.yaml\\n- .kogen/models.yaml\\n`},
}

// trackedLineupFileForms are the forms of the sorted list of the project's files in a prompt, in
// which a project that holds `.kogen/models.yaml` lists it between `.gitignore` and
// `.kogen/project.yaml`.
var trackedLineupFileForms = [][2]string{
	{".gitignore\n.kogen/project.yaml\n", ".gitignore\n.kogen/models.yaml\n.kogen/project.yaml\n"},
	{`.gitignore\n.kogen/project.yaml\n`, `.gitignore\n.kogen/models.yaml\n.kogen/project.yaml\n`},
	{`.gitignore\\n.kogen/project.yaml\\n`, `.gitignore\\n.kogen/models.yaml\\n.kogen/project.yaml\\n`},
}
