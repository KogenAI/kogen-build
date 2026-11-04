package consumes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

const approvalJSON = `{"slug":"fixture","sha256":"deadbeef","base":"abc123","approver":"A <a@example.invalid>",` +
	`"approved_at":"2026-11-03T00:00:00Z","source_provenance":{"path":"/p/current.json","sha256":"d1","contract_sha256":"c1"}}` + "\n"

// build/build.go starts only from an approval (none is intent/not_approved), compares its Base
// with HEAD and its hash with the Baseline's, and hands its source_provenance to ValidateSource;
// build/context.go reads the hash from the approval's own JSON bytes.
func TestReadApproval(t *testing.T) {
	var _ func(string, string) (*project.Approval, error) = project.ReadApproval

	root := t.TempDir()
	got, err := project.ReadApproval(root, fixtureSlug)
	if got != nil || err != nil {
		t.Fatalf("project.ReadApproval without a file = %+v, %v; want nil and no error", got, err)
	}

	writeFile(t, root, ".kogen/local/approvals/fixture.json", approvalJSON)
	approval, err := project.ReadApproval(root, fixtureSlug)
	check(t, err)
	if approval == nil {
		t.Fatal("project.ReadApproval of an approval file is nil")
	}
	var _ string = approval.Base
	var _ string = approval.SHA256
	var _ map[string]any = approval.SourceProvenance
	equal(t, "project.Approval.SHA256", approval.SHA256, "deadbeef")
	equal(t, "project.Approval.Base", approval.Base, "abc123")
	equal(t, "project.Approval.SourceProvenance path", approval.SourceProvenance["path"], any("/p/current.json"))
	equal(t, "project.Approval.SourceProvenance sha256", approval.SourceProvenance["sha256"], any("d1"))
	equal(t, "project.Approval.SourceProvenance contract_sha256", approval.SourceProvenance["contract_sha256"], any("c1"))
	equal(t, "project.ApprovalPath names the file project.ReadApproval read", project.ApprovalPath(root, fixtureSlug),
		filepath.Join(root, ".kogen", "local", "approvals", "fixture.json"))

	// An approval of a legacy Intent has no base and no source provenance.
	writeFile(t, root, ".kogen/local/approvals/legacy.json", `{"slug":"legacy","sha256":"h","approver":"A","approved_at":"t","source_provenance":null}`+"\n")
	legacy, err := project.ReadApproval(root, "legacy")
	check(t, err)
	equal(t, "project.Approval.Base of an approval without one", legacy.Base, "")
	if legacy.SourceProvenance != nil {
		t.Errorf("project.Approval.SourceProvenance of a legacy approval = %v, want nil", legacy.SourceProvenance)
	}

	// build/context.go decodes the approval bytes it kept into a project.Approval.
	var decoded project.Approval
	check(t, json.Unmarshal([]byte(approvalJSON), &decoded))
	equal(t, "project.Approval JSON key sha256", decoded.SHA256, "deadbeef")

	if err := os.WriteFile(project.ApprovalPath(root, fixtureSlug), []byte("not an approval"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := project.ReadApproval(root, fixtureSlug); got != nil || err == nil {
		t.Errorf("project.ReadApproval of a damaged file = %+v, %v; want an error", got, err)
	}
}

// build/build.go, build/resume.go, build/recover.go and build/state.go compare the approval's
// hash with project.Hash of the Baseline: the protected contract in card order, then the
// project configuration and the files `protect` matches.
func TestBaselineAndHash(t *testing.T) {
	var _ func(string, string, *project.Config) ([]string, error) = project.Baseline
	var _ func(string, []string) string = project.Hash

	f := newProject(t)
	f.write(t, ".kogen/intents/fixture/notes.md", "notes\n")
	f.write(t, ".kogen/intents/fixture/b.txt", "b\n")
	f.write(t, "other/x.cfg", "x\n")
	f.write(t, "ignored.cfg", "ignored\n")
	f.write(t, ".gitignore", ".kogen/local/\nignored.cfg\n")
	cfg, err := project.LoadConfig(f.Dir)
	check(t, err)
	cfg.Protect = []string{"guard.cfg", "**/x.cfg", "ignored.cfg"}

	baseline, err := project.Baseline(f.Dir, fixtureSlug, cfg)
	check(t, err)
	equalList(t, "project.Baseline", baseline, []string{
		".kogen/intents/fixture/intent.md", ".kogen/intents/fixture/acceptance.feature",
		"features/steps/fixture_steps.js",
		".kogen/intents/fixture/b.txt", ".kogen/intents/fixture/notes.md",
		".kogen/project.yaml",
		"guard.cfg", "other/x.cfg",
	})

	hash := project.Hash(f.Dir, baseline)
	var lines strings.Builder
	for _, p := range slices.Sorted(slices.Values(baseline)) {
		lines.WriteString(p + "\x00" + sha256Hex([]byte(readFile(t, filepath.Join(f.Dir, p)))) + "\n")
	}
	equal(t, "project.Hash is the digest of the sorted <path>\\0<file digest>\\n lines", hash, sha256Hex([]byte(lines.String())))
	equal(t, "project.Hash is independent of the order of paths", project.Hash(f.Dir, slices.Clone(slices.Sorted(slices.Values(baseline)))), hash)
	equal(t, "project.Hash of a missing file", project.Hash(f.Dir, []string{"gone.txt"}), sha256Hex([]byte("gone.txt\x00missing\n")))
	equal(t, "project.Hash of no paths", project.Hash(f.Dir, nil), sha256Hex(nil))

	// A changed protected file changes the hash an approval holds.
	f.write(t, "guard.cfg", "changed\n")
	if project.Hash(f.Dir, baseline) == hash {
		t.Errorf("project.Hash ignores the content of a protected file")
	}
	// A protected file that did not exist, or one that stops matching, changes the baseline.
	cfg.Protect = []string{"guard.cfg"}
	narrower, err := project.Baseline(f.Dir, fixtureSlug, cfg)
	check(t, err)
	if slices.Contains(narrower, "other/x.cfg") {
		t.Errorf("project.Baseline keeps a file `protect` no longer matches: %q", narrower)
	}
}

// build/facts.go and build/build.go hand the approval's source_provenance to
// project.SourceRefFromMap and ValidateSource; build/state.go and build/legacy.go keep the
// reference in checkpoints; build/builder.go gives the builder project.SourceMessage.
func TestSourceReferences(t *testing.T) {
	var _ func(map[string]any) *project.SourceRef = project.SourceRefFromMap
	var _ func(*project.SourceRef) error = project.ValidateSource
	var _ func(*project.SourceRef) string = project.SourceMessage

	if ref := project.SourceRefFromMap(nil); ref != nil {
		t.Errorf("project.SourceRefFromMap(nil) = %+v, want nil (a legacy approval)", ref)
	}
	ref := project.SourceRefFromMap(map[string]any{"path": "/p/current.json", "sha256": "d1", "contract_sha256": "c1"})
	if ref == nil {
		t.Fatal("project.SourceRefFromMap of an object is nil")
	}
	var _ string = ref.Path
	var _ string = ref.SHA256
	var _ string = ref.ContractSHA256
	equal(t, "project.SourceRef.Path", ref.Path, "/p/current.json")
	equal(t, "project.SourceRef.SHA256", ref.SHA256, "d1")
	equal(t, "project.SourceRef.ContractSHA256", ref.ContractSHA256, "c1")
	partial := project.SourceRefFromMap(map[string]any{"path": "p", "sha256": 3})
	equal(t, "project.SourceRefFromMap of a non-string member", partial.SHA256, "")
	equal(t, "project.SourceRefFromMap of a missing member", partial.ContractSHA256, "")

	// build/state.go keeps the reference as JSON in its checkpoints.
	b, err := json.Marshal(ref)
	check(t, err)
	equal(t, "project.SourceRef JSON", string(b), `{"path":"/p/current.json","sha256":"d1","contract_sha256":"c1"}`)
	b, err = json.Marshal(&project.SourceRef{Path: "p", SHA256: "d"})
	check(t, err)
	equal(t, "project.SourceRef JSON without a contract hash", string(b), `{"path":"p","sha256":"d"}`)
	var back project.SourceRef
	check(t, json.Unmarshal([]byte(`{"path":"p","sha256":"d","contract_sha256":"c"}`), &back))
	equal(t, "project.SourceRef JSON round trip", back, project.SourceRef{Path: "p", SHA256: "d", ContractSHA256: "c"})

	equal(t, "project.SourceMessage of a legacy Intent", project.SourceMessage(nil), "Source provenance: unavailable (legacy)")
	equal(t, "project.SourceMessage", project.SourceMessage(ref),
		"Diagnostic source provenance (not authority): /p/current.json\nManifest sha256: d1; contract: c1. "+
			"Retrieve full originals with read_file. Only the approved Intent authorizes implementation; "+
			"omissions require ordinary Refine and new exact approval. Mapping is an aid, not proof.")
}

// ValidateSource accepts a bound receipt whose custody is intact and refuses any other with
// candidate/source_provenance_changed (exit 1), which build reports before it spends any model
// time; a legacy Intent (nil) has nothing to validate.
func TestValidateSourceCustody(t *testing.T) {
	if err := project.ValidateSource(nil); err != nil {
		t.Errorf("project.ValidateSource(nil) = %v, want nil", err)
	}

	root := t.TempDir()
	const content = `{"version":1}` + "\n"
	for _, dir := range []string{".kogen", ".kogen/local", ".kogen/local/provenance", ".kogen/local/provenance/fixture"} {
		check(t, os.Mkdir(filepath.Join(root, dir), 0o700))
		chmod(t, filepath.Join(root, dir), 0o700)
	}
	path := filepath.Join(root, ".kogen/local/provenance/fixture/current.json")
	check(t, os.WriteFile(path, []byte(content), 0o600))
	chmod(t, path, 0o600)
	ref := &project.SourceRef{Path: path, SHA256: sha256Hex([]byte(content)), ContractSHA256: "c"}
	if err := project.ValidateSource(ref); err != nil {
		t.Fatalf("project.ValidateSource of an intact receipt = %v", err)
	}

	refused := func(what string, ref *project.SourceRef) {
		t.Helper()
		err := project.ValidateSource(ref)
		e := wantSysError(t, "project.ValidateSource "+what, err, "candidate", "source_provenance_changed", 1)
		if !strings.Contains(e.Detail, ref.Path) || !strings.Contains(e.Detail, ref.SHA256) {
			t.Errorf("project.ValidateSource %s: detail %q names neither the path nor the expected digest", what, e.Detail)
		}
	}
	refused("of a different digest", &project.SourceRef{Path: path, SHA256: "0000", ContractSHA256: "c"})
	chmod(t, path, 0o644)
	refused("of a receipt that others can read", ref)
	chmod(t, path, 0o600)
	chmod(t, filepath.Join(root, ".kogen/local/provenance/fixture"), 0o755)
	refused("of a folder others can enter", ref)
	chmod(t, filepath.Join(root, ".kogen/local/provenance/fixture"), 0o700)
	if err := project.ValidateSource(ref); err != nil {
		t.Fatalf("project.ValidateSource after restoring the modes = %v", err)
	}
	check(t, os.WriteFile(path, []byte("tampered\n"), 0o600))
	refused("of changed bytes", ref)
	check(t, os.Remove(path))
	refused("of a missing receipt", ref)
}
