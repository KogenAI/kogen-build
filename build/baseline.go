package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-verification/verify"
	"kogen-loop/sys"
)

// The check baseline (AUT-5): every configured check, run in configured order on the
// unchanged tree before the builder edits anything. A nonzero exit is recorded evidence, not a
// refusal and never a waiver: the whole-Intent gate still requires every check to pass before
// landing. Execution errors, interrupts and missing tools keep their own stops.

// Check states of the before/after comparison.
const (
	checkPassed = "passed"
	checkFailed = "failed"
	checkNotRun = "not run"
)

// checkState is one side of a check's comparison: whether it passed, failed or did not run,
// and for a result its actual status and output.
type checkState struct {
	State  string `json:"state"`
	Status *int   `json:"status,omitempty"`
	Output string `json:"output,omitempty"`
}

// checkComparison is one configured check, before the Build changed anything and after the
// last whole-Intent verification pass.
type checkComparison struct {
	Name    string     `json:"name"`
	Command string     `json:"command"`
	Before  checkState `json:"before"`
	After   checkState `json:"after"`
}

func stateOf(r verify.CommandResult) checkState {
	status := r.Status
	if status != 0 {
		return checkState{State: checkFailed, Status: &status, Output: r.Output}
	}
	return checkState{State: checkPassed, Status: &status, Output: r.Output}
}

// baselineRunning is the baseline check in progress: from just before its command event until
// its result returns. It lives on the Attempt only, never in the checkpoint.
type baselineRunning struct {
	name, command string
	started       time.Time
}

// stopRecord is the stop record's account of a Build that ends while a baseline check is in
// progress (nil otherwise), taken when the stop is recorded.
func (b *Attempt) stopRecord() map[string]any {
	r := b.baselineRunning
	if r == nil {
		return nil
	}
	b.baselineRunning = nil
	stopped := time.Now()
	return map[string]any{"name": r.name, "command": r.command, "started_at": isoTime(r.started),
		"stopped_at": isoTime(stopped), "wall_ms": stopped.Sub(r.started).Milliseconds()}
}

// reportBaselineStop keeps the baseline check a stop interrupted for the stop record and prints
// its line.
func (b *Attempt) reportBaselineStop() {
	b.baselineStop = b.stopRecord()
	if m := b.baselineStop; m != nil {
		b.s.Puts(fmt.Sprintf("  stopped during the check baseline: check %s (%s) ran %d ms (started %s)", m["name"], m["command"], m["wall_ms"], m["started_at"]))
	}
}

// recordBaseline runs every configured check once on the unchanged tree and keeps each
// name, command, exit status and output, in order. A check that exits nonzero does not stop
// the collection. Only an execution error (or an interrupt) does.
func (b *Attempt) recordBaseline(ctx context.Context) error {
	b.CheckBaseline = nil
	began := time.Now()
	timings := []any{}
	// The tree and environment of the baseline's checks, before the first and after the last, and
	// the checks that executed (not reused through the per-base evidence) and exited 0. They
	// enter the ledger only when nothing moved while the checks ran.
	var before checkPoint
	pointKnown := false
	if len(b.Config.Checks) > 0 {
		if p, err := b.checkPoint(); err == nil {
			before, pointKnown = p, true
		}
	}
	var executed []verify.CommandResult
	for i, c := range b.Config.Checks {
		if err := ctx.Err(); err != nil {
			return err
		}
		var digest, dir string
		if c.Run == ":" {
			var err error
			digest, dir, err = baselineIdentity(b.Root, b.Config.Checks)
			if err == nil {
				if e, ok := readBaselineEvidence(dir, digest, i, c.Name); ok {
					if again, _, err := baselineIdentity(b.Root, b.Config.Checks); err == nil && again == digest {
						b.Run.Event(map[string]any{"event": "baseline_reuse", "name": c.Name, "identity": digest, "evidence": e.Reference})
						b.CheckBaseline = append(b.CheckBaseline, verify.CommandResult{Name: c.Name, Command: c.Run})
						continue
					}
				}
			}
		}
		b.s.Progress("kogen: checking " + c.Name + " before the Build changes anything")
		b.baselineRunning = &baselineRunning{name: c.Name, command: c.Run, started: time.Now()}
		b.Run.Event(map[string]any{"event": "command", "name": c.Name, "command": c.Run, "before_change": true})
		started := time.Now()
		b.baselineRunning.started = started
		r, err := verify.RunWatched(b.s, c.Name, c.Run, b.Run, func() (sys.Result, error) {
			if c.Run == ":" {
				return sys.Result{}, nil
			}
			return b.s.Run(c.Run, b.Root, sys.Options{Env: toolEnv(b.deps.Tools), KillGroupOnExit: true})
		})
		if err != nil {
			return err
		}
		ended := time.Now()
		b.baselineRunning = nil
		wall := ended.Sub(started).Milliseconds()
		b.Run.Event(map[string]any{"event": "command_result", "name": c.Name, "status": r.Status, "output": Cap(r.Output),
			"started_at": isoTime(started), "ended_at": isoTime(ended), "wall_ms": wall})
		timings = append(timings, map[string]any{"name": c.Name, "started_at": isoTime(started), "ended_at": isoTime(ended), "wall_ms": wall})
		result := verify.CommandResult{Name: c.Name, Command: c.Run, Status: r.Status, Output: verify.Cap(r.Output)}
		b.CheckBaseline = append(b.CheckBaseline, result)
		executed = append(executed, result)
		if c.Run == ":" && digest != "" && ctx.Err() == nil && b.Run != nil && b.Run.LogErr() == nil {
			if after, _, err := baselineIdentity(b.Root, b.Config.Checks); err == nil && after == digest {
				status, output := 0, ""
				publishBaselineEvidence(dir, baselineEvidence{Version: 2, Complete: true, Command: c.Run, Status: &status, Output: &output, Position: &i, Identity: digest, Index: i, Name: c.Name, Reference: b.Run.LogPath})
			}
		}
	}
	if pointKnown {
		if after, err := b.checkPoint(); err == nil && after.equal(before) {
			b.recordPassed(before, executed)
		}
	}
	if len(b.CheckBaseline) == 0 {
		return nil
	}
	finished := time.Now()
	failing := b.baselineFailures()
	names := make([]string, 0, len(failing))
	for _, r := range failing {
		names = append(names, r.Name)
	}
	b.Run.Event(map[string]any{"event": "baseline_checks", "checks": len(b.CheckBaseline), "failing": core.NonNil(names),
		"started_at": isoTime(began), "ended_at": isoTime(finished), "wall_ms": finished.Sub(began).Milliseconds(), "timings": timings})
	if len(failing) == 0 {
		return nil
	}
	b.s.Puts(fmt.Sprintf("baseline: %d of %d configured check(s) fail before the Build changes anything; recorded as evidence, "+
		"not as a waiver: every configured check must pass before this Build lands", len(failing), len(b.CheckBaseline)))
	for _, r := range failing {
		b.s.Puts(fmt.Sprintf("  check %s (%s) failed (status=%d)", r.Name, r.Command, r.Status))
		for _, line := range strings.Split(verify.Diagnostic(r.Output), "\n") {
			b.s.Puts("    " + line)
		}
	}
	return nil
}

func (b *Attempt) baselineFailures() []verify.CommandResult {
	var out []verify.CommandResult
	for _, r := range b.CheckBaseline {
		if r.Status != 0 {
			out = append(out, r)
		}
	}
	return out
}

// comparison is every configured check in configured order with its baseline and its state in
// the last whole-Intent verification pass (nil: no pass yet). A check without a recorded
// baseline or without a result in that pass is "not run": nothing is inferred as unchanged or
// green. It is nil when no baseline was collected.
func (b *Attempt) comparison() []checkComparison {
	if len(b.CheckBaseline) == 0 {
		return nil
	}
	out := make([]checkComparison, 0, len(b.Config.Checks))
	for i, c := range b.Config.Checks {
		row := checkComparison{Name: c.Name, Command: c.Run, Before: checkState{State: checkNotRun}, After: checkState{State: checkNotRun}}
		if i < len(b.CheckBaseline) && b.CheckBaseline[i].Name == c.Name {
			row.Before = stateOf(b.CheckBaseline[i])
		}
		if r, ok := passCheck(b.LastPass, c.Name); ok {
			row.After = stateOf(r)
		}
		out = append(out, row)
	}
	return out
}

// passCheck is the result of the check called name in a pass, found by name: a pass lists only
// the checks it has a result for.
func passCheck(p *verify.Pass, name string) (verify.CommandResult, bool) {
	if p == nil {
		return verify.CommandResult{}, false
	}
	for _, r := range p.Checks {
		if r.Name == name {
			return r, true
		}
	}
	return verify.CommandResult{}, false
}

func (s checkState) String() string {
	if s.State == checkFailed && s.Status != nil {
		return fmt.Sprintf("%s (status=%d)", s.State, *s.Status)
	}
	return s.State
}

// comparisonLines is the human account of the comparison, shown whenever the Build ends:
// one line per check, then the actual output of every failure.
func (b *Attempt) comparisonLines() []string {
	rows := b.comparison()
	if len(rows) == 0 {
		return nil
	}
	after := "no verification pass"
	if b.LastPass != nil {
		after = fmt.Sprintf("verification pass %d", b.LastPass.N)
	}
	lines := []string{fmt.Sprintf("  checks, before the Build → after (%s):", after)}
	for i, r := range rows {
		lines = append(lines, fmt.Sprintf("    %d. %s: %s → %s", i+1, r.Name, r.Before, r.After))
		for _, side := range []struct {
			label string
			state checkState
		}{{"before", r.Before}, {"after", r.After}} {
			if side.state.State != checkFailed {
				continue
			}
			lines = append(lines, "       "+side.label+" output:")
			for _, line := range strings.Split(verify.Diagnostic(side.state.Output), "\n") {
				lines = append(lines, "         "+line)
			}
		}
	}
	return lines
}

// comparisonRecord is the comparison as result.json keeps it.
func (b *Attempt) comparisonRecord() map[string]any {
	rows := b.comparison()
	if len(rows) == 0 {
		return nil
	}
	var afterPass any
	if b.LastPass != nil {
		afterPass = b.LastPass.N
	}
	return map[string]any{"after_pass": afterPass, "checks": rows}
}

// baselineText is the baseline for the repair owners' prompts: the actual before-state of every
// check, in order, and the rule that it waives nothing. Empty when every check passed (or none
// is configured), so that Builds without a baseline failure see exactly what they saw before.
func (b *Attempt) baselineText() string {
	if len(b.baselineFailures()) == 0 {
		return ""
	}
	lines := []string{"# Baseline check results (before any edit)", "",
		"These are the actual results of every configured check, in configured order, on the unchanged tree. " +
			"They are diagnostics, not a waiver and not green verification: every configured check must pass on the final tree " +
			"before this Build can land, whether it failed at the start or not, and a check that passes now must not regress. " +
			"Do not remove, weaken or skip any check; repair what the checks report within the approved Intent.", ""}
	for i, r := range b.CheckBaseline {
		st := stateOf(r)
		lines = append(lines, fmt.Sprintf("%d. %s (%s): %s", i+1, r.Name, r.Command, st))
		if st.State == checkFailed {
			for _, line := range strings.Split(verify.Diagnostic(r.Output), "\n") {
				lines = append(lines, "   "+line)
			}
		}
	}
	return strings.Join(lines, "\n") + "\n\n"
}

// reviewBaselineMessage tells the independent reviewer the baseline and the actual final
// results of the green pass it reviews, check by check. Nil when there is no baseline failure.
func (b *Attempt) reviewBaselineMessage(pass verify.Pass) []any {
	if len(b.baselineFailures()) == 0 {
		return nil
	}
	lines := []string{"Check baseline and actual final results for this independent review (diagnostic evidence, not approval, not a waiver).",
		fmt.Sprintf("The baseline is the unchanged tree before the Build edited anything; the final results are verification pass %d of the candidate you review.", pass.N),
		"Every configured check must pass for the Build to land. Check by check, in configured order:"}
	for i, r := range b.CheckBaseline {
		final := checkState{State: checkNotRun}
		if fr, ok := passCheck(&pass, r.Name); ok {
			final = stateOf(fr)
		}
		lines = append(lines, fmt.Sprintf("%d. %s (%s): baseline %s → final %s", i+1, r.Name, r.Command, stateOf(r), final))
		if st := stateOf(r); st.State == checkFailed {
			for _, line := range strings.Split(verify.Diagnostic(r.Output), "\n") {
				lines = append(lines, "   baseline output: "+line)
			}
		}
		if final.State == checkFailed {
			for _, line := range strings.Split(verify.Diagnostic(final.Output), "\n") {
				lines = append(lines, "   final output: "+line)
			}
		}
	}
	return []any{map[string]any{"role": "user", "content": strings.Join(lines, "\n") + "\n"}}
}

// validBaseline is the checkpoint check of the recorded baseline: at most one result per
// configured check, in configured order, with its command.
func validBaseline(s *State) bool {
	if len(s.CheckBaseline) > len(s.Config.Checks) {
		return false
	}
	for i, r := range s.CheckBaseline {
		c := s.Config.Checks[i]
		if r.Name != c.Name || r.Command != c.Run {
			return false
		}
	}
	return true
}
