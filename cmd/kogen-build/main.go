package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/KogenAI/kogen-build/build"
	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

type problem struct {
	Category string `json:"category"`
	Kind     string `json:"kind"`
	Message  string `json:"message"`
}
type result struct {
	ExitCode int                    `json:"exit_code"`
	Branch   string                 `json:"branch"`
	Tree     string                 `json:"tree"`
	Checks   []verify.CommandResult `json:"checks"`
	Error    *problem               `json:"error"`
}

func main() {
	sys.MaybeSupervise(os.Args[1:])
	sys.EnableSupervisor()
	os.Exit(run())
}
func run() int {
	s := sys.New()
	s.Stdout = os.Stderr
	stop := s.WatchSignals()
	defer stop()
	r := result{Checks: []verify.CommandResult{}}
	defer func() { _ = json.NewEncoder(os.Stdout).Encode(r) }()
	if len(os.Args) < 2 {
		r.ExitCode = 2
		r.Error = &problem{"usage", "invalid_arguments", "expected build or resume"}
		return r.ExitCode
	}
	mode := os.Args[1]
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	project := flags.String("project", ".", "checkout")
	slug := flags.String("slug", "", "intent slug")
	id := flags.String("id", "", "build ID")
	if flags.Parse(os.Args[2:]) != nil || flags.NArg() != 0 || (mode != "build" && mode != "resume") || (mode == "build" && *slug == "") || (mode == "resume" && *id == "") {
		r.ExitCode = 2
		r.Error = &problem{"usage", "invalid_arguments", "expected build --project <checkout> --slug <slug> or resume --project <checkout> --id <build-id>"}
		return r.ExitCode
	}
	cmd := exec.Command("git", "-C", *project, "symbolic-ref", "--quiet", "--short", "HEAD")
	b, _ := cmd.Output()
	r.Branch = strings.TrimSpace(string(b))
	if repo, e := git.Open(*project, s.Env); e == nil {
		r.Tree, _ = repo.Tree(repo.Head(), nil)
	}
	out := build.Outcome{}
	deps := build.Deps{Client: provider.NewClient("kogen-build"), Tools: verify.Tools{Node: os.Getenv("KOGEN_NODE"), Quint: os.Getenv("KOGEN_QUINT"), Cucumber: os.Getenv("KOGEN_CUCUMBER")}, Outcome: &out}
	var err error
	if mode == "build" {
		r.ExitCode, err = build.Build(s.Context(), s, build.Input{Deps: deps, Project: *project, Slug: *slug})
	} else {
		r.ExitCode, err = build.Resume(s.Context(), s, build.ResumeInput{Deps: deps, Project: *project, ID: *id})
	}
	if out.Tree != "" {
		r.Tree = out.Tree
	}
	r.Checks = out.Checks
	if out.Error != nil {
		r.Error = &problem{out.Error.Class, out.Error.Reason, out.Error.Detail}
	} else if err != nil {
		r.Error = &problem{"internal", "error", err.Error()}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return r.ExitCode
}
