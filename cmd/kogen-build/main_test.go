package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/KogenAI/kogen-coordinator/git"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseArgsRefusalsNameTheInput(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"unknown mode", []string{"frobnicate", "--project", "."}, "frobnicate"},
		{"unknown flag", []string{"build", "--slug", "demo", "--bogus-flag"}, "-bogus-flag"},
		{"extra positional", []string{"resume", "--id", "demo", "stray-argument"}, "stray-argument"},
		{"missing slug", []string{"build", "--project", "."}, "--slug"},
		{"missing id", []string{"resume", "--project", "."}, "--id"},
		{"build with id", []string{"build", "--slug", "demo", "--id", "x"}, "--id is not valid for build"},
		{"build with empty id", []string{"build", "--slug", "demo", "--id", ""}, "--id is not valid for build"},
		{"build with id and no slug", []string{"build", "--id", "x"}, "--id is not valid for build"},
		{"resume with slug", []string{"resume", "--id", "x", "--slug", "demo"}, "--slug is not valid for resume"},
		{"resume with empty slug", []string{"resume", "--id", "x", "--slug", ""}, "--slug is not valid for resume"},
		{"resume with slug and no id", []string{"resume", "--slug", "demo"}, "--slug is not valid for resume"},
	}
	for _, c := range cases {
		_, msg := parseArgs(c.argv, io.Discard)
		if !strings.Contains(msg, c.want) {
			t.Errorf("%s: message %q does not contain %q", c.name, msg, c.want)
		}
		if strings.HasPrefix(c.name, "build with") && strings.Contains(strings.TrimSuffix(msg, expectedForms), "--slug") {
			t.Errorf("%s: message %q names --slug before the expected forms", c.name, msg)
		}
		if strings.HasPrefix(c.name, "resume with") && strings.Contains(strings.TrimSuffix(msg, expectedForms), "--id") {
			t.Errorf("%s: message %q names --id before the expected forms", c.name, msg)
		}
		if !strings.Contains(msg, expectedForms) {
			t.Errorf("%s: message %q lacks the expected forms", c.name, msg)
		}
	}
}

func TestParseArgsAccepts(t *testing.T) {
	a, msg := parseArgs([]string{"build", "--project", "p", "--slug", "s"}, io.Discard)
	if msg != "" || a.mode != "build" || a.project != "p" || a.slug != "s" {
		t.Fatalf("build: %+v %q", a, msg)
	}
	a, msg = parseArgs([]string{"resume", "--id", "i"}, io.Discard)
	if msg != "" || a.mode != "resume" || a.project != "." || a.id != "i" {
		t.Fatalf("resume: %+v %q", a, msg)
	}
}

func TestParsePrepareContinuation(t *testing.T) {
	a, msg := parseArgs([]string{"prepare-continuation", "--project", "checkout", "--id", "abcdef12", "--binding", "binding.json"}, io.Discard)
	if msg != "" || a.mode != "prepare-continuation" || a.binding != "binding.json" || a.project != "checkout" || a.id != "abcdef12" {
		t.Fatalf("prepare: %+v %q", a, msg)
	}
	for _, tc := range []struct {
		argv []string
		want string
	}{
		{[]string{"prepare-continuation", "--id", "abcdef12"}, "--binding"},
		{[]string{"prepare-continuation", "--binding", "binding.json"}, "--id"},
		{[]string{"prepare-continuation", "--id", "abcdef12", "--binding", "binding.json", "--slug", "s"}, "--slug is not valid"},
		{[]string{"resume", "--id", "abcdef12", "--binding", "binding.json"}, "binding"},
	} {
		_, msg := parseArgs(tc.argv, io.Discard)
		if !strings.Contains(msg, tc.want) {
			t.Fatalf("%v: %q lacks %q", tc.argv, msg, tc.want)
		}
	}
}

func TestPrepareReadbackDoesNotWriteGitObjects(t *testing.T) {
	dir := t.TempDir()
	gitCmd := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	gitCmd("init", "-q")
	gitCmd("config", "user.email", "x@example.com")
	gitCmd("config", "user.name", "x")
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCmd("add", "a")
	gitCmd("commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	objects := filepath.Join(dir, ".git", "objects")
	count := func() int {
		n := 0
		err := filepath.WalkDir(objects, func(_ string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				n++
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	repo, err := git.OpenReadOnly(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if observedTree(repo) == "" {
		t.Fatal("missing checkout tree")
	}
	if after := count(); after != before {
		t.Fatalf("readback wrote Git objects: %d -> %d", before, after)
	}
}

func TestStandaloneUsageRefusals(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "kogen-build")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	binding := filepath.Join(dir, "inputs", "binding.json")
	if err := os.MkdirAll(filepath.Dir(binding), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binding, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"no mode", nil, []string{"build", "resume", "prepare-continuation"}},
		{"malformed binding", []string{"prepare-continuation", "--project", dir, "--id", "abcdef12", "--binding", binding}, []string{"--binding", binding, "unexpected end of JSON input"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := exec.Command(binary, tc.args...).Output()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit: %v, output: %s", err, out)
			}
			var r result
			if err := json.Unmarshal(out, &r); err != nil {
				t.Fatalf("JSON: %v: %s", err, out)
			}
			if r.ExitCode != 2 || r.Error == nil || r.Error.Category != "usage" || r.Error.Kind != "invalid_arguments" || r.Continuation != nil {
				t.Fatalf("refusal: %+v", r)
			}
			for _, want := range tc.want {
				if !strings.Contains(r.Error.Message, want) {
					t.Errorf("message %q lacks %q", r.Error.Message, want)
				}
			}
		})
	}
}

func TestParseReconstructionModes(t *testing.T) {
	a, msg := parseArgs([]string{"reconstruct", "--project", "c", "--evidence", "e.json", "--archive", "/abs/new", "--stop-after", "fenced"}, io.Discard)
	if msg != "" || a.mode != "reconstruct" || a.evidence != "e.json" || a.archive != "/abs/new" || a.stopAfter != "fenced" {
		t.Fatalf("reconstruct: %+v %q", a, msg)
	}
	a, msg = parseArgs([]string{"reconstruction", "--item", "q1"}, io.Discard)
	if msg != "" || a.mode != "reconstruction" || a.item != "q1" || a.project != "." {
		t.Fatalf("reconstruction: %+v %q", a, msg)
	}
	for _, tc := range []struct {
		argv []string
		want string
	}{
		{[]string{"reconstruct", "--archive", "/a"}, "--evidence"},
		{[]string{"reconstruct", "--evidence", "e"}, "--archive"},
		{[]string{"reconstruct", "--evidence", "e", "--archive", "relative"}, "--archive"},
		{[]string{"reconstruct", "--evidence", "e", "--archive", "/a", "--stop-after", "created"}, "--stop-after"},
		{[]string{"reconstruct", "--evidence", "e", "--archive", "/a", "--id", "x"}, "--id is not valid for reconstruct"},
		{[]string{"reconstruct", "--evidence", "e", "--archive", "/a", "--item", "x"}, "-item"},
		{[]string{"reconstruction", "--item", "q", "--evidence", "e"}, "-evidence"},
		{[]string{"reconstruction", "--item", "q", "--slug", "s"}, "--slug is not valid for reconstruction"},
		{[]string{"reconstruction"}, "--item"},
		{[]string{"build", "--slug", "s", "--archive", "/a"}, "-archive"},
		{[]string{"resume", "--id", "i", "--item", "q"}, "-item"},
	} {
		_, msg := parseArgs(tc.argv, io.Discard)
		if !strings.Contains(msg, tc.want) || !strings.Contains(msg, "reconstruct") {
			t.Errorf("%v: %q lacks %q", tc.argv, msg, tc.want)
		}
	}
}

func TestStandaloneReconstruction(t *testing.T) {
	const item = "bb13338f-045e-4853-8cc4-90ec80dc8b4f"
	const requestSHA = "ee87cf09b602ba3eec64112ecf8e85bbf1d539864308a2ab542ac467a9202cb4"
	dir := t.TempDir()
	binary := filepath.Join(dir, "kogen-build")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	for _, mode := range []string{"absent", "preserved", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			checkout := filepath.Join(root, "checkout")
			if err := os.Mkdir(checkout, 0700); err != nil {
				t.Fatal(err)
			}
			git := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", checkout, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			git("init", "-q")
			git("commit", "-q", "--allow-empty", "-m", "baseline")
			builds := []any{}
			for i, id := range []string{"b97d96270b9f4801aa09a7ba00e64f84", "c3b576c985bb4299b1b443666eed172a"} {
				body := []byte(fmt.Sprintf("{\"receipt\":%d}\n", i+1))
				path := filepath.Join(root, id+".json")
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
				builds = append(builds, map[string]any{"id": id, "receipt_sha256": strings.Repeat("a", 63) + fmt.Sprint(i), "candidate_tree": strings.Repeat("b", 39) + fmt.Sprint(i), "path": path, "copy_sha256": fmt.Sprintf("%x", sha256.Sum256(body))})
			}
			evidence := filepath.Join(root, "evidence.json")
			archive := filepath.Join(root, "archive")
			ev := map[string]any{
				"queue_item": map[string]any{"id": item, "version": 18}, "obligation": "Problem20", "owner": "fleet",
				"request":         map[string]any{"document": "queue-request-" + item, "revision": 3, "sha256": requestSHA},
				"current_request": map[string]any{"revision": 3, "sha256": requestSHA}, "baseline": map[string]any{"head": git("rev-parse", "HEAD")},
				"prior":   map[string]any{"attempts": []string{"577e7c601aab41a39eb0820fc5fee5fb"}, "builds": builds, "spend": map[string]any{"units": 42}},
				"effects": map[string]any{"publication": "none", "external": "none"}, "owners": []any{}, "custody": map[string]any{"generation": "gen-1", "revoked": true}, "claims": map[string]any{},
			}
			data, err := json.Marshal(ev)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(evidence, data, 0600); err != nil {
				t.Fatal(err)
			}
			snapshot := func() map[string]string {
				t.Helper()
				files := map[string]string{}
				err := filepath.WalkDir(checkout, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					rel, err := filepath.Rel(checkout, path)
					if err != nil {
						return err
					}
					if rel == ".git" {
						return filepath.SkipDir
					}
					if rel == "." {
						return nil
					}
					if d.IsDir() {
						files[rel] = "directory"
						return nil
					}
					b, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					files[rel] = "file:" + string(b)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				return files
			}
			before := snapshot()
			args := []string{"reconstruction", "--project", checkout, "--item", item}
			wantExit := 1
			if mode != "absent" {
				stop := "preserved"
				if mode == "invalid" {
					stop = "not-a-phase"
					wantExit = 2
				}
				args = []string{"reconstruct", "--project", checkout, "--evidence", evidence, "--archive", archive, "--stop-after", stop}
			}
			out, runErr := exec.Command(binary, args...).Output()
			exit, ok := runErr.(*exec.ExitError)
			if !ok || exit.ExitCode() != wantExit {
				t.Fatalf("exit: %v, stdout: %s", runErr, out)
			}
			var r result
			if err := json.Unmarshal(out, &r); err != nil {
				t.Fatalf("JSON: %v: %s", err, out)
			}
			if r.ExitCode != exit.ExitCode() || r.Error == nil {
				t.Fatalf("envelope: %+v", r)
			}
			var raw struct {
				Reconstruction map[string]json.RawMessage `json:"reconstruction"`
			}
			if err := json.Unmarshal(out, &raw); err != nil {
				t.Fatal(err)
			}
			if mode == "invalid" {
				if r.Reconstruction != nil || raw.Reconstruction != nil || r.Error.Category != "usage" || r.Error.Kind != "invalid_arguments" || !strings.Contains(r.Error.Message, "--stop-after") || !strings.Contains(r.Error.Message, "not-a-phase") {
					t.Fatalf("invalid refusal: %+v", r)
				}
				if _, err := os.Stat(archive); !os.IsNotExist(err) {
					t.Fatalf("archive exists or cannot be checked: %v", err)
				}
			} else {
				state := "absent"
				if mode == "preserved" {
					state = "interrupted"
				}
				if r.Reconstruction == nil || r.Reconstruction.State != state {
					t.Fatalf("reconstruction: %+v", r.Reconstruction)
				}
				if string(raw.Reconstruction["attempts"]) != "[]" || string(raw.Reconstruction["wakes"]) != "[]" {
					t.Fatalf("attempts/wakes: %s", out)
				}
				if mode == "absent" && len(raw.Reconstruction) != 3 {
					t.Fatalf("absent fields: %s", out)
				}
				if mode == "preserved" {
					if _, err := os.Stat(filepath.Join(archive, "manifest.json")); err != nil {
						t.Fatalf("manifest: %v", err)
					}
				}
			}
			if mode != "preserved" {
				if after := snapshot(); !reflect.DeepEqual(before, after) {
					t.Fatalf("checkout changed: before %v after %v", before, after)
				}
			}
		})
	}
}
