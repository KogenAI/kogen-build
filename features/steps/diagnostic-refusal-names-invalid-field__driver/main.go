// Command driver runs one Build with one invalid diagnostic case against a scratch checkout
// and prints what it observed as one JSON object.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KogenAI/kogen-build/build"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-loop/sys"
)

func digestOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func cases() map[string]*build.Diagnostic {
	good := "valid diagnostic"
	return map[string]*build.Diagnostic{
		"blank text":                {Text: " \t\r\n ", SourcePath: "/inaccessible/label", SHA256: digestOf(" \t\r\n ")},
		"invalid UTF-8 text":        {Text: "\xff", SourcePath: "/inaccessible/label", SHA256: digestOf("\xff")},
		"relative source path":      {Text: good, SourcePath: "relative/label", SHA256: digestOf(good)},
		"empty source path":         {Text: good, SourcePath: "", SHA256: digestOf(good)},
		"invalid UTF-8 source path": {Text: good, SourcePath: "/\xff", SHA256: digestOf(good)},
		"mismatched digest":         {Text: good, SourcePath: "/inaccessible/label", SHA256: strings.Repeat("0", 64)},
		"upper-case digest":         {Text: good, SourcePath: "/inaccessible/label", SHA256: strings.ToUpper(digestOf(good))},
	}
}

func git(dir string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git %v: %v\n%s", args, err, out)
		os.Exit(1)
	}
	return strings.TrimSpace(string(out))
}

func main() {
	d, ok := cases()[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown case %q\n", os.Args[1])
		os.Exit(1)
	}
	dir, err := os.MkdirTemp("", "diagnostic-refusal-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	git(dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("scratch\n"), 0o644); err != nil {
		panic(err)
	}
	git(dir, "add", "-A")
	git(dir, "commit", "-q", "-m", "initial")
	head, status := git(dir, "rev-parse", "HEAD"), git(dir, "status", "--porcelain", "--untracked-files=all")

	s := sys.New()
	s.Stdout = os.Stderr
	exit, err := build.Build(context.Background(), s, build.Input{
		Deps:    build.Deps{Client: provider.NewClient("diagnostic-refusal-driver")},
		Project: dir, Slug: "demo", Context: d,
	})
	r := map[string]any{"exit": exit, "category": "", "kind": "", "detail": "", "refused": false}
	if e, ok := sys.AsError(err); ok {
		r["refused"], r["category"], r["kind"], r["detail"] = true, e.Class, e.Reason, e.Detail
	}
	_, statErr := os.Stat(filepath.Join(dir, ".kogen"))
	r["kogen_dir_created"] = statErr == nil
	r["head_unchanged"] = git(dir, "rev-parse", "HEAD") == head
	r["status_unchanged"] = git(dir, "status", "--porcelain", "--untracked-files=all") == status
	_ = json.NewEncoder(os.Stdout).Encode(r)
}
