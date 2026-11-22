package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

const expectedForms = "expected build --project <checkout> --slug <slug> or resume --project <checkout> --id <build-id>"

type arguments struct {
	mode, project, slug, id string
}

// parseArgs validates the command line after the program name. It returns a
// non-empty refusal message that names the rejected input, or the parsed arguments.
func parseArgs(argv []string, flagOutput io.Writer) (arguments, string) {
	mode := argv[0]
	if mode != "build" && mode != "resume" {
		return arguments{}, fmt.Sprintf("unknown mode %q; %s", mode, expectedForms)
	}
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(flagOutput)
	a := arguments{mode: mode}
	flags.StringVar(&a.project, "project", ".", "checkout")
	flags.StringVar(&a.slug, "slug", "", "intent slug")
	flags.StringVar(&a.id, "id", "", "build ID")
	if err := flags.Parse(argv[1:]); err != nil {
		return arguments{}, fmt.Sprintf("%v; %s", err, expectedForms)
	}
	if flags.NArg() != 0 {
		return arguments{}, fmt.Sprintf("unexpected argument %q; %s", flags.Arg(0), expectedForms)
	}
	if mode == "build" && a.slug == "" {
		return arguments{}, "missing required --slug for build; " + expectedForms
	}
	if mode == "resume" && a.id == "" {
		return arguments{}, "missing required --id for resume; " + expectedForms
	}
	return a, ""
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
	args, refusal := parseArgs(os.Args[1:], os.Stderr)
	if refusal != "" {
		r.ExitCode = 2
		r.Error = &problem{"usage", "invalid_arguments", refusal}
		return r.ExitCode
	}
	mode, project, slug, id := args.mode, &args.project, &args.slug, &args.id
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
