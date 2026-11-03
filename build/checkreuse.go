package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

// The check ledger: within one Build and its resumes, a configured check that passed is
// recorded under its key (the check, the tree and the environment that can change the result),
// and a later whole-Intent pass on the same key reuses the record instead of running the check
// again. Only an execution that exited 0 is ever recorded, so a check is never skipped on a
// tree it has not passed on.

// checkResult is one ledger record: a passed execution of a configured check.
type checkResult struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Tree        string `json:"tree"`
	Environment string `json:"environment"`
	Key         string `json:"key"`
	Output      string `json:"output"`
}

func hexSum(h hash.Hash) string { return hex.EncodeToString(h.Sum(nil)) }

// checkKey is the key of a check: the SHA-256 of its name, command text, tree and environment.
func checkKey(name, command, tree, environment string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%q\n%q\n%q\n%q\n", name, command, tree, environment)
	return hexSum(h)
}

// toolEnv is the Kogen tool variables a check's command gets: the non-empty ones.
func toolEnv(t verify.Tools) map[string]string {
	env := map[string]string{}
	for k, v := range map[string]string{"KOGEN_NODE": t.Node, "KOGEN_QUINT": t.Quint, "KOGEN_CUCUMBER": t.Cucumber} {
		if v != "" {
			env[k] = v
		}
	}
	return env
}

// commandEnv is the environment a check's command gets, by name: the session's variables (a
// later duplicate wins) plus the tool variables.
func (b *Attempt) commandEnv() map[string]string {
	env := map[string]string{}
	for _, kv := range b.s.Env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	for k, v := range toolEnv(b.deps.Tools) {
		env[k] = v
	}
	return env
}

// toolchainIdentity is parts (a) and (b) of the environment: the variables of the command's
// environment, sorted by name, and the toolchain on its PATH: for each directory in order, each
// entry sorted by name with its mode, size and modification time after following symbolic
// links. An unreadable directory or entry is recorded as unreadable.
func toolchainIdentity(env map[string]string) string {
	h := sha256.New()
	names := make([]string, 0, len(env))
	for k := range env {
		names = append(names, k)
	}
	slices.Sort(names)
	for _, k := range names {
		fmt.Fprintf(h, "env %q=%q\n", k, env[k])
	}
	for _, dir := range filepath.SplitList(env["PATH"]) {
		fmt.Fprintf(h, "dir %q\n", dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintf(h, "unreadable\n")
			continue
		}
		for _, e := range entries {
			st, err := os.Stat(filepath.Join(dir, e.Name()))
			if err != nil {
				fmt.Fprintf(h, "%q unreadable\n", e.Name())
				continue
			}
			fmt.Fprintf(h, "%q %d %d %d\n", e.Name(), uint32(st.Mode()), st.Size(), st.ModTime().UnixNano())
		}
	}
	return hexSum(h)
}

// commandFiles is part (c) of a check's environment: each whitespace-separated word of the
// command that, without surrounding quotes, is the absolute path of a readable regular file,
// with the SHA-256 of the file's content.
func commandFiles(command string) string {
	h := sha256.New()
	for _, word := range strings.Fields(command) {
		path := strings.Trim(word, `'"`)
		if !filepath.IsAbs(path) {
			continue
		}
		st, err := os.Stat(path)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		file := sha256.New()
		_, err = io.Copy(file, f)
		f.Close()
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "file %q %s\n", path, hexSum(file))
	}
	return hexSum(h)
}

// checkEnvironment is a check's environment identity: the SHA-256 over the toolchain identity
// of its command's environment and the files its command names.
func checkEnvironment(toolchain string, c project.Check) string {
	h := sha256.New()
	fmt.Fprintf(h, "toolchain %s\nfiles %s\n", toolchain, commandFiles(c.Run))
	return hexSum(h)
}

// checkPoint is the tree and the environment of every configured check at one moment.
type checkPoint struct {
	tree string
	envs []string // by configured check
}

func (p checkPoint) equal(o checkPoint) bool { return p.tree == o.tree && slices.Equal(p.envs, o.envs) }

// checkRepo is the Build's Git, or the checkout's own for an Attempt made without one.
func (b *Attempt) checkRepo() *git.Repo {
	if b.repo != nil {
		return b.repo
	}
	return git.New(b.Root, b.s.Env)
}

// checkPoint computes the tree and the environment of every configured check.
func (b *Attempt) checkPoint() (checkPoint, error) {
	tree, err := b.checkRepo().Tree(b.Start, b.Excluded)
	if err != nil {
		return checkPoint{}, err
	}
	toolchain := toolchainIdentity(b.commandEnv())
	p := checkPoint{tree: tree, envs: make([]string, len(b.Config.Checks))}
	for i, c := range b.Config.Checks {
		p.envs[i] = checkEnvironment(toolchain, c)
	}
	return p, nil
}

// ledgerFind is the record of a check on exactly this command, tree and environment.
func (b *Attempt) ledgerFind(c project.Check, tree, environment string) (checkResult, bool) {
	for _, r := range b.CheckResults {
		if r.Name == c.Name && r.Command == c.Run && r.Tree == tree && r.Environment == environment {
			return r, true
		}
	}
	return checkResult{}, false
}

// ledgerRecord keeps a passed execution of a configured check, replacing that check's previous
// record.
func (b *Attempt) ledgerRecord(c project.Check, p checkPoint, i int, output string) {
	r := checkResult{Name: c.Name, Command: c.Run, Tree: p.tree, Environment: p.envs[i],
		Key: checkKey(c.Name, c.Run, p.tree, p.envs[i]), Output: verify.Cap(output)}
	for j := range b.CheckResults {
		if b.CheckResults[j].Name == c.Name {
			b.CheckResults[j] = r
			return
		}
	}
	b.CheckResults = append(b.CheckResults, r)
}

// recordPassed records every configured check of results that exited 0.
func (b *Attempt) recordPassed(p checkPoint, results []verify.CommandResult) {
	for _, r := range results {
		if r.Status != 0 {
			continue
		}
		for i, c := range b.Config.Checks {
			if c.Name == r.Name && c.Run == r.Command {
				b.ledgerRecord(c, p, i, r.Output)
			}
		}
	}
}

// reuseDecision is what a whole-Intent pass decided before it ran: the point it was decided at,
// the checks reused (by configured index) and the checks to run, targeted first.
type reuseDecision struct {
	point  checkPoint
	reused map[int]checkResult
	run    []project.Check
}

// decideReuse reuses every check the ledger holds a record of on the pass's tree and
// environment, logs each reuse and orders the rest: the checks the last whole-Intent pass
// failed first, then the others, each in configured order.
func (b *Attempt) decideReuse() (reuseDecision, error) {
	p, err := b.checkPoint()
	if err != nil {
		return reuseDecision{}, err
	}
	d := reuseDecision{point: p, reused: map[int]checkResult{}}
	for i, c := range b.Config.Checks {
		if r, ok := b.ledgerFind(c, p.tree, p.envs[i]); ok {
			d.reused[i] = r
			key := checkKey(c.Name, c.Run, p.tree, p.envs[i])
			message := "baseline reused: " + c.Name + " " + key
			b.Run.Event(map[string]any{"event": "check_reuse", "name": c.Name, "command": c.Run, "key": key, "tree": p.tree,
				"pass": b.Pass, "message": message})
			b.s.Progress("kogen: " + message)
		}
	}
	failed := map[string]bool{}
	if b.LastPass != nil {
		for _, r := range b.LastPass.Checks {
			if r.Status != 0 {
				failed[r.Name] = true
			}
		}
	}
	for _, targeted := range []bool{true, false} {
		for i, c := range b.Config.Checks {
			if _, ok := d.reused[i]; !ok && failed[c.Name] == targeted {
				d.run = append(d.run, c)
			}
		}
	}
	return d, nil
}

// account is the pass the Build keeps for a pass verify made over the checks of d.run: every
// configured check in configured order (a reused one as its recorded pass, an executed one as
// its result, one that neither ran nor was reused absent), and the NotRun, Failing and Green
// that follow.
func (b *Attempt) account(d reuseDecision, pass verify.Pass) verify.Pass {
	configured := map[string]bool{}
	for _, c := range b.Config.Checks {
		configured[c.Name] = true
	}
	reused := d.reused
	if pass.Tree != d.point.tree {
		reused = nil
	}
	notRun, failing := []string{}, []string{}
	for _, n := range pass.NotRun {
		if !configured[n] {
			notRun = append(notRun, n)
		}
	}
	for _, n := range pass.Failing {
		if !configured[n] {
			failing = append(failing, n)
		}
	}
	var checks []verify.CommandResult
	var absent, red []string
	for i, c := range b.Config.Checks {
		if r, ok := reused[i]; ok {
			checks = append(checks, verify.CommandResult{Name: c.Name, Command: c.Run, Output: r.Output})
			continue
		}
		at := slices.IndexFunc(pass.Checks, func(r verify.CommandResult) bool { return r.Name == c.Name })
		if at < 0 {
			absent = append(absent, c.Name)
			continue
		}
		checks = append(checks, pass.Checks[at])
		if pass.Checks[at].Status != 0 {
			red = append(red, c.Name)
		}
	}
	out := pass
	out.Checks = append([]verify.CommandResult{}, checks...)
	out.NotRun = append(notRun, absent...)
	out.Failing = append(failing, red...)
	out.Green = pass.Green && pass.Tree == d.point.tree && len(absent) == 0
	return out
}

// verifyReusing runs verify over the checks the ledger cannot answer, targeted first, and
// accounts for the whole configured suite.
func (b *Attempt) verifyReusing(ctx context.Context, in verify.Input) (verify.Pass, error) {
	d, err := b.decideReuse()
	if err != nil {
		return verify.Pass{}, err
	}
	cfg := *in.Config
	cfg.Checks = d.run
	in.Config = &cfg
	pass, err := verify.Verify(ctx, b.s, in)
	if err != nil {
		return verify.Pass{}, err
	}
	if pass.Tree == d.point.tree && len(pass.Moved) == 0 {
		b.recordPassed(d.point, pass.Checks)
	}
	return b.account(d, pass), nil
}
