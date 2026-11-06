package build

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

const (
	projectYAML = ".kogen/project.yaml"
	modelsYAML  = ".kogen/models.yaml"
)

// roleEntryRe finds the entry of role under the root models: key of a models file: its `  role:`
// line and the indented lines that follow.
func roleEntryRe(role string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(role) + `:.*\n(    .*\n)*`)
}

// setRole makes entry, the role's lines as they stand under the root models: key of a models
// file (`  build:\n    model: m\n    effort: high\n` or `  build: {provider: anthropic, model: m,
// effort: high}\n`), the role's lineup in the project, and stages the file it wrote. It writes
// `.kogen/models.yaml`, starting it as `models:\n` when absent, replacing the role's entry if the
// file has one and keeping the other roles. It never writes project.yaml.
func (e *env) setRole(role, entry string) {
	e.t.Helper()
	text := "models:\n"
	if b, err := os.ReadFile(filepath.Join(e.root, modelsYAML)); err == nil {
		text = string(b)
	} else if !errors.Is(err, os.ErrNotExist) {
		e.t.Fatal(err)
	}
	if re := roleEntryRe(role); re.MatchString(text) {
		text = re.ReplaceAllLiteralString(text, entry)
	} else {
		text += entry
	}
	e.write(modelsYAML, text)
	e.run("git", "add", modelsYAML)
}

// loadLineup loads a configuration whose lineup is the roles in models, written as they stand
// under the root models: key of `.kogen/models.yaml` (`  build: {model: m, effort: low}\n`).
// Roles it omits keep project.DefaultModels().
func loadLineup(t *testing.T, models string) *project.Config {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, projectYAML), "version: 1\n", 0o644)
	writeFile(t, filepath.Join(root, modelsYAML), "models:\n"+models, 0o644)
	cfg, err := project.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// lineupFileForms are the forms in which the Build's prompts name the protected files: the
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
