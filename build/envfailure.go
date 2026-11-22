package build

import (
	"strings"

	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

// envFailure is one check's environment failure: its kind and the first line verify.EnvFailure returned.
type envFailure struct {
	kind string
	line string
}

// envMemory is the environment failures (check name to kind and first line) of the previous
// builder pass of this Build attempt, and that pass's number. Like noEditWatch it is not part of
// the State: a resumed Build starts without it.
type envMemory struct {
	pass     int
	failures map[string]envFailure
}

// envFailureKind classifies a line verify.EnvFailure returned by the phrases that function
// recognises: timeout, disk, memory or the file limit.
func envFailureKind(line string) string {
	switch {
	case strings.HasPrefix(line, "panic: test timed out after "):
		return "timeout"
	case strings.Contains(line, "no space left on device"):
		return "disk"
	case strings.Contains(line, "cannot allocate memory"),
		strings.Contains(line, "out of memory"),
		strings.Contains(line, "Out of memory: Killed process"):
		return "memory"
	case strings.Contains(line, "too many open files"):
		return "files"
	}
	return "other"
}

// hostOrBaseStop is the host-or-base rule (LOOP-7) for a builder pass: the whole-Intent pass
// after a builder round and a stage gate. A check of the pass that fails with an environment
// failure (verify.EnvFailure) of the same kind as the previous builder pass's same check
// stops the Build, whether or not the first lines match exactly, before any other decision about
// the pass. Every builder pass replaces the memory.
func (b *Attempt) hostOrBaseStop(pass verify.Pass) error {
	previous := b.env
	b.env = envMemory{pass: pass.N}
	var stop *verify.CommandResult
	var failure string
	for i, r := range pass.Checks {
		text, ok := verify.EnvFailure(r)
		if !ok {
			continue
		}
		if b.env.failures == nil {
			b.env.failures = map[string]envFailure{}
		}
		kind := envFailureKind(text)
		b.env.failures[r.Name] = envFailure{kind: kind, line: text}
		if prev, seen := previous.failures[r.Name]; stop == nil && seen && prev.kind == kind {
			stop, failure = &pass.Checks[i], text
		}
	}
	if stop == nil {
		return nil
	}
	passes := []int{previous.pass, pass.N}
	b.Run.Event(map[string]any{"event": "host_or_base_failure", "check": stop.Name, "failure": failure, "passes": passes})
	e := sys.Fail("environment", "host_or_base_failure",
		"check "+stop.Name+" failed the same way in verification passes "+itoa(passes[0])+" and "+itoa(passes[1])+": "+failure+
			"; this failure is in the host or the base, not in the candidate, so no further builder round was requested; edits are left in place", 3)
	e.Lines = []string{
		"check: " + stop.Name + " (" + stop.Command + ")",
		"failure: " + failure,
		"evidence: " + stop.Name + " output of verification passes " + itoa(passes[0]) + " and " + itoa(passes[1]) + " in " + b.Run.LogPath,
		"candidate tree: " + pass.Tree,
		"No further builder round was requested. Fix the host or the base (for example, run the check when no other test suite is running), " +
			"then `kogen intent resume " + b.Run.ID + "` verifies this candidate again, or start a fresh Build.",
	}
	return e
}
