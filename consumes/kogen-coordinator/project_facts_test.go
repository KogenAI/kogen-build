package consumes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

// build/context.go, build/recover.go, build/stages.go and build/core_stages.go write Kogen's
// local records with project.EncodeJSON: compact, object keys in byte order, strings escaped
// only for `"`, `\` and control characters, numbers verbatim, and an invalid UTF-8 string an
// error. The bytes are part of persisted records, so they are pinned exactly.
func TestEncodeJSONBytes(t *testing.T) {
	var _ func(any) ([]byte, error) = project.EncodeJSON

	got, err := project.EncodeJSON(map[string]any{"b": 1, "a": "x\n<&>"})
	check(t, err)
	equal(t, "project.EncodeJSON", string(got), `{"a":"x\n<&>","b":1}`)

	got, err = project.EncodeJSON(map[string]any{
		"z": []any{json.Number("1.50"), nil, true, "é\"\\\t"},
		"m": map[string]string{"y": "1", "x": "2"},
		"l": []string{"b", "a"},
		"n": int64(-7),
		"s": []map[string]any{{"k": false}},
		"t": struct {
			B string `json:"b"`
			A int    `json:"a"`
		}{"s", 2},
	})
	check(t, err)
	equal(t, "project.EncodeJSON of nested values", string(got),
		`{"l":["b","a"],"m":{"x":"2","y":"1"},"n":-7,"s":[{"k":false}],"t":{"a":2,"b":"s"},"z":[1.50,null,true,"é\"\\\t"]}`)

	got, err = project.EncodeJSON([]any{})
	check(t, err)
	equal(t, "project.EncodeJSON of an empty list", string(got), `[]`)
	got, err = project.EncodeJSON(nil)
	check(t, err)
	equal(t, "project.EncodeJSON of nil", string(got), `null`)
	if got, err := project.EncodeJSON("\xff"); err == nil {
		t.Errorf("project.EncodeJSON of a string that is not UTF-8 = %q, want an error", got)
	}
}

// build/facts.go names the checkout by the physical Git directory: the id is the lowercase hex
// SHA-256 of its path, the same through a symbolic link; the facts are published as JSON.
func TestCheckoutFacts(t *testing.T) {
	var _ func(string) (project.CheckoutFacts, error) = project.Checkout

	f := newRepo(t)
	facts, err := project.Checkout(f.Dir)
	check(t, err)
	var _ string = facts.ID
	root, err := filepath.EvalSymlinks(f.Dir)
	check(t, err)
	equal(t, "project.CheckoutFacts.ID", facts.ID, sha256Hex([]byte(filepath.Join(root, ".git"))))

	link := filepath.Join(t.TempDir(), "link")
	check(t, os.Symlink(f.Dir, link))
	viaLink, err := project.Checkout(link)
	check(t, err)
	equal(t, "project.Checkout through a symbolic link", viaLink.ID, facts.ID)

	b, err := json.Marshal(facts)
	check(t, err)
	var doc map[string]any
	check(t, json.Unmarshal(b, &doc))
	want := map[string]any{"root": root, "git_dir": filepath.Join(root, ".git"), "id": facts.ID}
	if !reflect.DeepEqual(doc, want) {
		t.Errorf("project.CheckoutFacts JSON = %v, want %v", doc, want)
	}

	if facts, err := project.Checkout(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Errorf("project.Checkout of a missing directory = %+v, want an error", facts)
	}
}

// build/facts.go records the Intent package's static identity at the start of every run; it is
// the file `kogen` lists packages from: the checkout id, the sorted baseline paths, their hash
// and `valid` or `invalid`.
func TestWritePackageFacts(t *testing.T) {
	var _ func(string, string, *project.Config) error = project.WritePackageFacts

	f := newProject(t)
	cfg, err := project.LoadConfig(f.Dir)
	check(t, err)
	path := filepath.Join(f.Dir, ".kogen", "local", "packages", "fixture.json")
	check(t, project.WritePackageFacts(f.Dir, fixtureSlug, cfg))

	baseline, err := project.Baseline(f.Dir, fixtureSlug, cfg)
	check(t, err)
	sorted := slices.Sorted(slices.Values(baseline))
	checkout, err := project.Checkout(f.Dir)
	check(t, err)
	want, err := project.EncodeJSON(map[string]any{
		"checkout_id": checkout.ID, "package_state": "valid", "paths": sorted, "sha256": project.Hash(f.Dir, baseline),
	})
	check(t, err)
	equal(t, "project.WritePackageFacts file", readFile(t, path), string(want)+"\n")

	// An Intent that fails its check is recorded as invalid, with its changed hash.
	earlier := project.Hash(f.Dir, baseline)
	f.write(t, project.IntentMDPath(fixtureSlug), strings.Replace(fixtureIntent, "Slug: fixture", "Slug: other", 1))
	check(t, project.WritePackageFacts(f.Dir, fixtureSlug, cfg))
	var doc map[string]any
	check(t, json.Unmarshal([]byte(readFile(t, path)), &doc))
	equal(t, "project.WritePackageFacts package_state of an invalid Intent", doc["package_state"], any("invalid"))
	equal(t, "project.WritePackageFacts checkout_id", doc["checkout_id"], any(checkout.ID))
	equal(t, "project.WritePackageFacts sha256 of the changed contract", doc["sha256"], any(project.Hash(f.Dir, baseline)))
	if doc["sha256"] == any(earlier) {
		t.Errorf("project.WritePackageFacts keeps the hash of the earlier contract")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("project.WritePackageFacts left its temporary file: %v", err)
	}
}
