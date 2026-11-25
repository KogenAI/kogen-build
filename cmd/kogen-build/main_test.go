package main

import (
	"encoding/json"
	"github.com/KogenAI/kogen-coordinator/git"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
