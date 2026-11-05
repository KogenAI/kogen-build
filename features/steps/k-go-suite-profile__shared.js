// The shared acceptance script check of k-go-suite-profile: Go source of package build, which
// k-go-suite-profile__control.js overlays as build/k_go_suite_profile_acceptance_test.go. This
// module defines no steps.
'use strict';
module.exports = String.raw`package build

// Acceptance of k-go-suite-profile, kept in features/steps/k-go-suite-profile__shared.js.
// k-go-suite-profile__control.js overlays it into package build as
// build/k_go_suite_profile_acceptance_test.go (go test -overlay); it is not a file of the
// repository's own suite. Every failure ends with a t.Fatal whose text
// starts with "kgsp: " and holds no temporary path; details go to t.Log first.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

// TestKGoSuiteProfileSharedAcceptanceScript: two fixture projects made by newEnv run one shared,
// read-only acceptance script, each through its own <aux>/accept.sh, and that script still
// reports red for a red app.txt and green otherwise.
func TestKGoSuiteProfileSharedAcceptanceScript(t *testing.T) {
	a, b := newEnv(t), newEnv(t)
	kgspCheckShared(t, [2]string{a.root, b.root}, [2]string{a.aux, b.aux})
}

// TestKGoSuiteProfileControlShared is the known-good control: two hand-made projects whose
// <aux>/accept.sh are hard links of one read-only script.
func TestKGoSuiteProfileControlShared(t *testing.T) {
	roots, auxes := kgspControlProjects(t)
	shared := filepath.Join(physical(t, t.TempDir()), "accept.sh")
	kgspWriteScript(t, shared)
	for _, aux := range auxes {
		if err := os.Link(shared, filepath.Join(aux, "accept.sh")); err != nil {
			t.Fatal(err)
		}
	}
	kgspCheckShared(t, roots, auxes)
}

// TestKGoSuiteProfileControlSeparate is the known-bad control: the same projects with a script
// of their own each, as every fixture project had before.
func TestKGoSuiteProfileControlSeparate(t *testing.T) {
	roots, auxes := kgspControlProjects(t)
	for _, aux := range auxes {
		kgspWriteScript(t, filepath.Join(aux, "accept.sh"))
	}
	kgspCheckShared(t, roots, auxes)
}

func kgspControlProjects(t *testing.T) ([2]string, [2]string) {
	t.Helper()
	var roots, auxes [2]string
	for i := range roots {
		roots[i], auxes[i] = physical(t, t.TempDir()), physical(t, t.TempDir())
		writeFile(t, filepath.Join(roots[i], "app.txt"), "baseline\n", 0o644)
		cfg := &project.Config{Acceptance: &project.Acceptance{Run: filepath.Join(auxes[i], "accept.sh"), StepsDir: "features/steps"}}
		writeFile(t, filepath.Join(roots[i], ".kogen/project.yaml"), project.RenderConfig(cfg), 0o644)
	}
	return roots, auxes
}

// kgspWriteScript writes a read-only acceptance script at path that behaves like the fixture's,
// with its two reports next to it.
func kgspWriteScript(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	feature := project.FeaturePath(testSlug)
	green, red := filepath.Join(dir, "kgsp-green.ndjson"), filepath.Join(dir, "kgsp-red.ndjson")
	writeFile(t, green, reportFor(t, feature, testFeature, "PASSED"), 0o644)
	writeFile(t, red, reportFor(t, feature, testFeature, "FAILED"), 0o644)
	writeFile(t, path, "#!/bin/sh\nif grep -qx red app.txt; then cp '"+red+"' \"$KOGEN_ACCEPTANCE_REPORT\"; echo actual-case-red; exit 1; fi\ncp '"+green+"' \"$KOGEN_ACCEPTANCE_REPORT\"\n", 0o555)
}

// kgspCheckShared checks the acceptance of two projects (roots[i], with auxiliary folder
// auxes[i]): each runs exactly <aux>/accept.sh, both are one file, that file has no write
// permission, and run in each project it is red for app.txt "red" and green for "baseline".
func kgspCheckShared(t *testing.T, roots, auxes [2]string) {
	t.Helper()
	var infos [2]os.FileInfo
	for i := range roots {
		script := filepath.Join(auxes[i], "accept.sh")
		cfg, err := project.LoadConfig(roots[i])
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Acceptance == nil || cfg.Acceptance.Run != script {
			t.Logf("project %d: acceptance %+v, want the command %s", i+1, cfg.Acceptance, script)
			t.Fatal("kgsp: a fixture project's acceptance command is not its <aux>/accept.sh")
		}
		info, err := os.Stat(script)
		if err != nil {
			t.Logf("project %d: %v", i+1, err)
			t.Fatal("kgsp: a fixture project has no <aux>/accept.sh")
		}
		infos[i] = info
	}
	if !os.SameFile(infos[0], infos[1]) {
		t.Logf("%s and %s", filepath.Join(auxes[0], "accept.sh"), filepath.Join(auxes[1], "accept.sh"))
		t.Fatal("kgsp: the acceptance scripts of two fixture projects are different files")
	}
	if infos[0].Mode().Perm()&0o222 != 0 {
		t.Logf("mode %v", infos[0].Mode())
		t.Fatal("kgsp: the shared acceptance script has write permission")
	}
	for i := range roots {
		kgspRun(t, roots[i], auxes[i], "red\n", 1, "FAILED")
		kgspRun(t, roots[i], auxes[i], "baseline\n", 0, "PASSED")
	}
}

// kgspRun runs <aux>/accept.sh in root with app.txt holding app and checks its exit status
// and that its report carries only steps of status want.
func kgspRun(t *testing.T, root, aux, app string, exit int, want string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "app.txt"), app, 0o644)
	report := filepath.Join(t.TempDir(), "report.ndjson")
	cmd := exec.Command(filepath.Join(aux, "accept.sh"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "KOGEN_ACCEPTANCE_REPORT="+report)
	out, err := cmd.CombinedOutput()
	got := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		got = exitErr.ExitCode()
	} else if err != nil {
		t.Logf("%v", err)
		t.Fatal("kgsp: the shared acceptance script could not run")
	}
	if got != exit {
		t.Logf("app.txt %q: exit %d, output %q", app, got, out)
		t.Fatalf("kgsp: the shared acceptance script exited %d for app.txt %q, want %d", got, app, exit)
	}
	b, err := os.ReadFile(report)
	if err != nil {
		t.Logf("%v", err)
		t.Fatalf("kgsp: the shared acceptance script wrote no report for app.txt %q", app)
	}
	other := map[string]string{"PASSED": "FAILED", "FAILED": "PASSED"}[want]
	if !strings.Contains(string(b), "\"status\":\""+want+"\"") || strings.Contains(string(b), "\"status\":\""+other+"\"") {
		t.Logf("report %s", b)
		t.Fatalf("kgsp: the shared acceptance script's report for app.txt %q is not all %s", app, want)
	}
}
`;
