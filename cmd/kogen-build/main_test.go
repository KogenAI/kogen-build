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
	}
	for _, c := range cases {
		_, msg := parseArgs(c.argv, io.Discard)
		if !strings.Contains(msg, c.want) {
			t.Errorf("%s: message %q does not contain %q", c.name, msg, c.want)
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
