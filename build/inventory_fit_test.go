package build

import (
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

func TestFittedInventoryGivesToolDiscoveryGuidance(t *testing.T) {
	e := newEnv(t)
	e.write("optional.txt", "example\n")
	intent, err := project.LoadIntent(e.root, testSlug)
	if err != nil {
		t.Fatal(err)
	}
	b := &Attempt{State: State{Root: e.root}, intent: intent, repo: e.gitRepo()}
	input := []any{map[string]any{"role": "user", "content": []any{map[string]any{"text": "authority"}, map[string]any{"text": "# Files\n\n" + strings.Repeat("optional\n", 1000)}}}}
	got, err := b.fitInventory(input, func(items []any) (bool, error) {
		return len(items[0].(map[string]any)["content"].([]any)[1].(map[string]any)["text"].(string)) < 1000, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := got[0].(map[string]any)["content"].([]any)[1].(map[string]any)["text"].(string)
	for _, want := range []string{"shell", "git ls-files", "git grep", "read_file"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	for _, mode := range []string{"guidance", "omit"} {
		fitted, err := b.fitInventory(input, func(items []any) (bool, error) {
			text := items[0].(map[string]any)["content"].([]any)[1].(map[string]any)["text"].(string)
			if mode == "guidance" {
				return strings.Contains(text, "Omitted inventory:"), nil
			}
			return text == "", nil
		})
		if err != nil {
			t.Fatal(err)
		}
		text := fitted[0].(map[string]any)["content"].([]any)[1].(map[string]any)["text"].(string)
		if mode == "guidance" && !strings.Contains(text, "Omitted inventory:") || mode == "omit" && text != "" {
			t.Fatalf("%s: %q", mode, text)
		}
	}

}
