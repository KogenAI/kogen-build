package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	ExitCode       int                     `json:"exit_code"`
	Branch         string                  `json:"branch"`
	Tree           string                  `json:"tree"`
	Checks         []verify.CommandResult  `json:"checks"`
	Continuation   *build.ContinuationPlan `json:"continuation,omitempty"`
	Reconstruction *build.Reconstruction   `json:"reconstruction,omitempty"`
	Error          *problem                `json:"error"`
}

func main() {
	sys.MaybeSupervise(os.Args[1:])
	sys.EnableSupervisor()
	os.Exit(run())
}

const expectedForms = "expected build --project <checkout> --slug <slug> or resume --project <checkout> --id <build-id> or prepare-continuation --project <checkout> --id <build-id> --binding <file> or reconstruct --project <checkout> --evidence <file> --archive <absolute-new-directory> [--stop-after preserved|fenced] or reconstruction --project <checkout> --item <queue-item-id>"

type arguments struct {
	mode, project, slug, id, binding string
	evidence, archive, stopAfter     string
	item                             string
}

// parseArgs validates the command line after the program name. It returns a
// non-empty refusal message that names the rejected input, or the parsed arguments.
func parseArgs(argv []string, flagOutput io.Writer) (arguments, string) {
	mode := argv[0]
	if mode != "build" && mode != "resume" && mode != "prepare-continuation" && mode != "reconstruct" && mode != "reconstruction" {
		return arguments{}, fmt.Sprintf("unknown mode %q; %s", mode, expectedForms)
	}
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(flagOutput)
	a := arguments{mode: mode}
	flags.StringVar(&a.project, "project", ".", "checkout")
	flags.StringVar(&a.slug, "slug", "", "intent slug")
	flags.StringVar(&a.id, "id", "", "build ID")
	switch mode {
	case "prepare-continuation":
		flags.StringVar(&a.binding, "binding", "", "dispatch binding JSON file")
	case "reconstruct":
		flags.StringVar(&a.evidence, "evidence", "", "reconstruction evidence JSON file")
		flags.StringVar(&a.archive, "archive", "", "new absolute archive directory")
		flags.StringVar(&a.stopAfter, "stop-after", "", "stop after the named phase: preserved or fenced")
	case "reconstruction":
		flags.StringVar(&a.item, "item", "", "queue item ID")
	}
	if err := flags.Parse(argv[1:]); err != nil {
		return arguments{}, fmt.Sprintf("%v; %s", err, expectedForms)
	}
	if flags.NArg() != 0 {
		return arguments{}, fmt.Sprintf("unexpected argument %q; %s", flags.Arg(0), expectedForms)
	}
	supplied := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { supplied[f.Name] = true })
	if (mode == "build" || mode == "reconstruct" || mode == "reconstruction") && supplied["id"] {
		return arguments{}, "--id is not valid for " + mode + "; " + expectedForms
	}
	if mode != "build" && supplied["slug"] {
		return arguments{}, "--slug is not valid for " + mode + "; " + expectedForms
	}
	if mode == "build" && a.slug == "" {
		return arguments{}, "missing required --slug for build; " + expectedForms
	}
	if (mode == "resume" || mode == "prepare-continuation") && a.id == "" {
		return arguments{}, "missing required --id for " + mode + "; " + expectedForms
	}
	if mode == "prepare-continuation" && a.binding == "" {
		return arguments{}, "missing required --binding for prepare-continuation"
	}
	if mode == "reconstruct" {
		if a.evidence == "" {
			return arguments{}, "missing required --evidence for reconstruct; " + expectedForms
		}
		if a.archive == "" {
			return arguments{}, "missing required --archive for reconstruct; " + expectedForms
		}
		if !filepath.IsAbs(a.archive) {
			return arguments{}, fmt.Sprintf("--archive %q must be an absolute new directory; %s", a.archive, expectedForms)
		}
		if supplied["stop-after"] && a.stopAfter != "preserved" && a.stopAfter != "fenced" {
			return arguments{}, fmt.Sprintf("--stop-after %q must be preserved or fenced; %s", a.stopAfter, expectedForms)
		}
	}
	if mode == "reconstruction" && a.item == "" {
		return arguments{}, "missing required --item for reconstruction; " + expectedForms
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
		r.Error = &problem{"usage", "invalid_arguments", expectedForms}
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
	if repo, e := git.OpenReadOnly(*project, s.Env); e == nil {
		if mode == "prepare-continuation" {
			r.Tree = observedTree(repo)
		} else {
			r.Tree, _ = repo.Tree(repo.Head(), nil)
		}
	}
	if mode == "reconstruct" || mode == "reconstruction" {
		var rec build.Reconstruction
		var p *build.ReconstructionProblem
		if mode == "reconstruct" {
			rec, r.ExitCode, p = build.Reconstruct(s, build.ReconstructInput{Project: *project, Evidence: args.evidence, Archive: args.archive, StopAfter: args.stopAfter})
		} else {
			rec, r.ExitCode, p = build.ReadReconstruction(*project, args.item, s.Env)
		}
		r.Reconstruction = &rec
		if p != nil {
			r.Error = &problem{p.Category, p.Kind, p.Message}
			fmt.Fprintln(os.Stderr, p.Message)
		}
		return r.ExitCode
	}
	out := build.Outcome{}
	deps := build.Deps{Client: provider.NewClient("kogen-build"), Tools: verify.Tools{Node: os.Getenv("KOGEN_NODE"), Quint: os.Getenv("KOGEN_QUINT"), Cucumber: os.Getenv("KOGEN_CUCUMBER")}, Outcome: &out}
	var err error
	if mode == "build" {
		r.ExitCode, err = build.Build(s.Context(), s, build.Input{Deps: deps, Project: *project, Slug: *slug})
	} else if mode == "prepare-continuation" {
		var binding build.ContinuationBinding
		raw, e := os.ReadFile(args.binding)
		if e == nil {
			e = json.Unmarshal(raw, &binding)
			if e != nil {
				e = fmt.Errorf("%s: %w", args.binding, e)
			}
		}
		if e != nil {
			r.ExitCode = 2
			r.Error = &problem{"usage", "invalid_arguments", "--binding: " + e.Error()}
			return r.ExitCode
		}
		plan, e := (build.ResumeInput{Deps: deps, Project: *project, ID: *id}).PrepareContinuation(s.Context(), s, binding)
		r.Continuation = &plan
		err = e
		if err != nil {
			r.ExitCode = 1
			errors.As(err, &out.Error)
		}
	} else {
		r.ExitCode, err = build.Resume(s.Context(), s, build.ResumeInput{Deps: deps, Project: *project, ID: *id})
	}
	if out.Tree != "" {
		r.Tree = out.Tree
	}
	if out.Checks != nil {
		r.Checks = out.Checks
	}
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

// observedTree keeps the standalone readback's Git objects outside the checkout.
func observedTree(repo *git.Repo) string {
	objects, err := repo.OK("rev-parse", "--git-path", "objects")
	if err != nil {
		return ""
	}
	objects = strings.TrimSpace(objects)
	if !filepath.IsAbs(objects) {
		objects = filepath.Join(repo.Dir, objects)
	}
	dir, err := os.MkdirTemp("", "kogen-readback-objects-")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(dir)
	index := filepath.Join(dir, "index")
	env := []string{"GIT_OBJECT_DIRECTORY=" + dir, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + objects, "GIT_INDEX_FILE=" + index}
	if _, err := repo.OKEnv(env, "read-tree", repo.Head()); err != nil {
		return ""
	}
	if _, err := repo.OKEnv(env, "add", "-A", "--", "."); err != nil {
		return ""
	}
	tree, _ := repo.OKEnv(env, "write-tree")
	return strings.TrimSpace(tree)
}
