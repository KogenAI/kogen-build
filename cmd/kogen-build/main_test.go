package main

import (
	"io"
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
