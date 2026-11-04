'use strict';
// Steps of k-build-blocker-claim-grounded. Every Go test runs in its own `go test -json -run`
// process (the acceptance fixture FIXTURE_GO, below, through a -overlay); no step calls a test
// function directly.
// Run with node, this file is the fixture's observer control: `control-good` / `control-bad`.
const {spawnSync} = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const FIXTURE = 'TestKBGAcceptance';
const OVERLAY_TARGET = 'build/zz_kbg_acceptance_test.go';
const FEATURE = '.kogen/intents/k-build-blocker-claim-grounded/acceptance.feature';
// Go commands on this host may queue behind other suites before they start. PWD is the
// working directory Node resolves paths against, so go maps the same overlay target.
const GO_TIMEOUT_MS = 900000;

// FIXTURE_GO is the Go acceptance fixture, compiled into package build's tests as
// OVERLAY_TARGET; it reads KBG_MODE and KBG_EXPECT.
const FIXTURE_GO = String.raw`package build

// Acceptance fixture of k-build-blocker-claim-grounded: one real local Build per mode, run in
// its own go test -run process with KBG_MODE and KBG_EXPECT (the
// feature's expectations, JSON). Every check is collected before the test fails, so one
// failing expectation does not hide the others. Failures are prefixed "kbg-reject: ".

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
)

type kbgExpect struct {
	Outcome  string
	Message  string
	Paths    []string
	Passes   int
	Requests int
}

const (
	kbgReason   = "The protected step requires app.txt to be B while the approved behaviour makes it A."
	kbgEvidence = "Verification failed: AssertionError app.txt must be B, got A at features/steps/integration_steps.js:2"
	kbgSteps    = "const assert = require('node:assert/strict');\nassert.equal(require('node:fs').readFileSync('app.txt', 'utf8'), 'B\\n');\n"
)

type kbgCustody struct{ head, index, approval string }

func kbgCustodyOf(e *env) kbgCustody {
	head, _ := execGit(e, "rev-parse", "--verify", "-q", "HEAD")
	idx, _ := os.ReadFile(filepath.Join(e.root, ".git/index"))
	ap, _ := os.ReadFile(project.ApprovalPath(e.root, testSlug))
	return kbgCustody{strings.TrimSpace(head), string(idx), string(ap)}
}

func kbgSay() handler {
	return sse(map[string]any{"type": "message", "id": "msg-kbg", "role": "assistant",
		"content": []any{map[string]any{"type": "output_text", "text": "The pass is still red; I have nothing more to change."}}})
}

func kbgClaim(f *fakeProvider, paths []string) map[string]any {
	list := make([]any, len(paths))
	for i, p := range paths {
		list[i] = p
	}
	return f.call("report_blocker", map[string]any{"paths": list, "reason": kbgReason, "evidence": kbgEvidence})
}

// kbgOutputFor is the first function_call_output the provider received for callID.
func kbgOutputFor(reqs []map[string]any, callID string) (string, bool) {
	for _, req := range reqs {
		items, _ := req["input"].([]any)
		for _, it := range items {
			m, _ := it.(map[string]any)
			if m["type"] == "function_call_output" && m["call_id"] == callID {
				return fmt.Sprint(m["output"]), true
			}
		}
	}
	return "", false
}

func kbgFile(e *env, rel string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(e.root, rel))
	return string(b), err == nil
}

func TestKBGAcceptance(t *testing.T) {
	mode := os.Getenv("KBG_MODE")
	var want kbgExpect
	if err := json.Unmarshal([]byte(os.Getenv("KBG_EXPECT")), &want); err != nil {
		t.Fatalf("kbg-fixture: KBG_EXPECT is not the expectation JSON: %v", err)
	}
	var problems []string
	switch mode {
	case "first-round", "unnamed", "control-good-first", "control-bad-first", "control-good-unnamed", "control-bad-unnamed":
		problems = kbgContinued(t, mode, want)
	case "backed", "two-paths":
		problems = kbgBacked(t, want)
	default:
		t.Fatalf("kbg-fixture: unknown KBG_MODE %q", mode)
	}
	if len(problems) > 0 {
		t.Fatal("KBG_MODE=" + mode + "\nkbg-reject: " + strings.Join(problems, "\nkbg-reject: "))
	}
}

// kbgContinued: a claim with no verification failure naming its paths must not stop the
// Build. The builder reports in its first round ("first" modes) or after one red pass whose
// failures name no protected step file ("unnamed" modes). The control modes claim the
// unprotected app.txt instead (today's invalid-blocker feedback, which also continues the
// Build); control-good puts the expected answer in place of the real one, control-bad keeps
// the real one.
func kbgContinued(t *testing.T, mode string, want kbgExpect) []string {
	e := newEnv(t)
	f := e.prov
	claimed := want.Paths
	if strings.HasPrefix(mode, "control-") {
		claimed = []string{"app.txt"}
	}
	claim := kbgClaim(f, claimed)
	red := f.calls("write_file", map[string]any{"path": "app.txt", "content": "red\n"}, "finish", map[string]any{"summary": "implemented"})
	if mode == "first-round" || strings.HasSuffix(mode, "-first") {
		f.then(sse(claim), red)
	} else {
		f.then(red, sse(claim))
	}
	f.then(kbgSay(), kbgSay(), kbgSay(), kbgSay())
	steps := e.read(stepsFile)
	before := kbgCustodyOf(e)
	a := e.build()
	id := e.onlyRun()
	res := e.result(id)
	got, answered := kbgOutputFor(f.requests(), claim["call_id"].(string))
	if strings.HasPrefix(mode, "control-good-") && answered {
		got = want.Message
	}
	var p []string
	if a.exit != 1 || a.err != nil {
		p = append(p, fmt.Sprintf("build exit %d err %v, want exit 1 without error", a.exit, a.err))
	}
	if res["outcome"] != want.Outcome {
		p = append(p, fmt.Sprintf("outcome %v, want %s (the claim must not stop the Build)", res["outcome"], want.Outcome))
	}
	if b, ok := res["blocker"]; ok {
		p = append(p, fmt.Sprintf("a blocker was recorded: %v", b))
	}
	if n := len(eventsNamed(e.events(id), "builder_blocker")); n != 0 {
		p = append(p, fmt.Sprintf("%d builder_blocker events, want 0", n))
	}
	if strings.Contains(a.out.String(), "build_blocked") {
		p = append(p, "stdout reports build_blocked:\n"+a.out.String())
	}
	if !answered {
		p = append(p, "blocker tool output: the builder never received an answer to its report_blocker call")
	} else if got != want.Message {
		p = append(p, fmt.Sprintf("blocker tool output:\n got %q\nwant %q", got, want.Message))
	}
	if n := f.count(); n != want.Requests {
		p = append(p, fmt.Sprintf("provider requests %d, want %d (the builder's next round must continue)", n, want.Requests))
	}
	if res["passes"] != any(float64(want.Passes)) {
		p = append(p, fmt.Sprintf("passes %v, want %d", res["passes"], want.Passes))
	}
	if s, _ := kbgFile(e, "app.txt"); s != "red\n" {
		p = append(p, fmt.Sprintf("app.txt %q, want the next round's edit %q", s, "red\n"))
	}
	if s, _ := kbgFile(e, stepsFile); s != steps {
		p = append(p, "the protected step file changed")
	}
	if after := kbgCustodyOf(e); after.head != before.head || after.approval != before.approval {
		p = append(p, "HEAD or approval changed")
	}
	return p
}

// kbgBacked: the protected step file asserts a value the approved behaviour contradicts and
// the red pass's failing step names it; the claim stops the Build exactly as before.
func kbgBacked(t *testing.T, want kbgExpect) []string {
	e := newEnv(t)
	e.write(stepsFile, kbgSteps)
	e.approve()
	feature := project.FeaturePath(testSlug)
	scenarios, err := project.ParseGherkin(testFeature)
	if err != nil {
		t.Fatal(err)
	}
	failure := "AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:\n'A\\n' !== 'B\\n'\n    at World.<anonymous> (" + filepath.Join(e.root, stepsFile) + ":2:8)"
	events := []obj{documentMessage(feature, scenarios)}
	for i, s := range scenarios {
		results := []obj{{"status": "FAILED", "message": failure}}
		for range s.Steps[1:] {
			results = append(results, obj{"status": "SKIPPED"})
		}
		events = append(events, caseMessages(s, i+1, feature, results)...)
	}
	var lines []string
	for _, ev := range events {
		b, err := core.EncodeJSON(ev)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(b))
	}
	named := filepath.Join(e.aux, "named.ndjson")
	writeFile(t, named, strings.Join(lines, "\n")+"\n", 0o644)
	writeFile(t, filepath.Join(e.aux, "accept.sh"), "#!/bin/sh\nif grep -qx A app.txt; then cp '"+named+"' \"$KOGEN_ACCEPTANCE_REPORT\"; echo actual-case-red; exit 1; fi\ncp '"+filepath.Join(e.aux, "green.ndjson")+"' \"$KOGEN_ACCEPTANCE_REPORT\"\n", 0o755)
	f := e.prov
	claim := kbgClaim(f, want.Paths)
	f.then(
		f.calls("write_file", map[string]any{"path": "app.txt", "content": "A\n"}, "finish", map[string]any{"summary": "implemented A"}),
		sse(claim,
			f.call("write_file", map[string]any{"path": "late.txt", "content": "must not run"}),
			f.call("shell", map[string]any{"command": "touch late-shell"}),
			f.call("finish", map[string]any{"summary": "must not verify"})),
	)
	before := kbgCustodyOf(e)
	a := e.build()
	id := e.onlyRun()
	out := a.out.String()
	var p []string
	if a.exit != 1 || a.err != nil {
		p = append(p, fmt.Sprintf("build exit %d err %v, want exit 1 without error\n%s", a.exit, a.err, a.shown()))
	}
	for _, line := range []string{
		"failed fixture: build_blocked (Build " + id + ")\n",
		"candidate/build_blocked: BUILDER-REPORTED protected-baseline contradiction; edits are left in place; this report is not independently proven\n",
		"  paths: " + strings.Join(want.Paths, ", ") + "\n",
		"  reason: " + kbgReason + "\n",
		"  evidence: " + kbgEvidence + "\n",
		"  edited: app.txt\n",
		"  run log: " + e.runPath(id, "log.jsonl") + "\n",
		"  fixture Build " + id + " stopped in building: build_blocked; candidate: " + e.root + "; evidence: " + e.runPath(id, "") + "; index: ",
		"  kogen intent recover " + id + " --output <new-external-directory> --project '" + e.root + "'\n",
		"  Recovery preserves and clears only implementation; the builder-reported claim is unproven. Independently Refine, freshly approve and start a fresh Build; resume cannot override a protected blocker.\n",
		"  kogen intent inspect " + id + " --project '" + e.root + "'\n",
	} {
		if !strings.Contains(out, line) {
			p = append(p, fmt.Sprintf("stdout lacks %q", line))
		}
	}
	if n := strings.Count(out, "\n  paths: "); n != 1 {
		p = append(p, fmt.Sprintf("stdout has %d paths lines, want 1", n))
	}
	res := e.result(id)
	if res["outcome"] != "build_blocked" {
		p = append(p, fmt.Sprintf("outcome %v, want build_blocked", res["outcome"]))
	}
	if res["passes"] != any(float64(want.Passes)) {
		p = append(p, fmt.Sprintf("passes %v, want %d", res["passes"], want.Passes))
	}
	if res["reviews"] != any(float64(0)) {
		p = append(p, fmt.Sprintf("reviews %v, want 0", res["reviews"]))
	}
	paths := make([]any, len(want.Paths))
	for i, s := range want.Paths {
		paths[i] = s
	}
	blocker := map[string]any{"classification": "BUILDER-REPORTED", "paths": paths, "reason": kbgReason, "evidence": kbgEvidence}
	if !reflect.DeepEqual(res["blocker"], any(blocker)) {
		p = append(p, fmt.Sprintf("result blocker %#v, want %#v", res["blocker"], blocker))
	}
	if n := len(eventsNamed(e.events(id), "builder_blocker")); n != 1 {
		p = append(p, fmt.Sprintf("%d builder_blocker events, want 1", n))
	}
	facts := e.jsonFile(filepath.Join(e.root, ".kogen/local/facts", id+".json"))
	if facts["state"] != "build_blocked" || facts["resume_status"] != "unsupported" {
		p = append(p, fmt.Sprintf("facts state %v resume_status %v", facts["state"], facts["resume_status"]))
	}
	if bl, _ := facts["blockers"].([]any); len(bl) != 1 || !reflect.DeepEqual(bl[0].(map[string]any)["paths"], any(paths)) {
		p = append(p, fmt.Sprintf("facts blockers %v", facts["blockers"]))
	}
	if n := f.count(); n != want.Requests {
		p = append(p, fmt.Sprintf("provider requests %d, want %d (no request after the stop)", n, want.Requests))
	}
	for _, rel := range []string{"late.txt", "late-shell"} {
		if _, ok := kbgFile(e, rel); ok {
			p = append(p, rel+" exists: a call after the blocker ran")
		}
	}
	if s, _ := kbgFile(e, "app.txt"); s != "A\n" {
		p = append(p, fmt.Sprintf("app.txt %q, want the candidate %q left in place", s, "A\n"))
	}
	if s, _ := kbgFile(e, stepsFile); s != kbgSteps {
		p = append(p, "the protected step file changed")
	}
	if after := kbgCustodyOf(e); after != before {
		p = append(p, "HEAD, index or approval changed")
	}
	s, rout, rerrs := e.session()
	archive := filepath.Join(physical(t, t.TempDir()), "archive")
	exit, err := Recover(context.Background(), s, RecoverInput{Project: e.root, ID: id, Output: archive})
	if exit != 0 || err != nil {
		p = append(p, fmt.Sprintf("recover exit %d err %v\n%s%s", exit, err, rout.String(), rerrs.String()))
	} else {
		var got any
		raw, rerr := os.ReadFile(filepath.Join(archive, "blocker.json"))
		if rerr == nil {
			rerr = json.Unmarshal(raw, &got)
		}
		if rerr != nil || !reflect.DeepEqual(got, any(blocker)) {
			p = append(p, fmt.Sprintf("recover record blocker.json %s (%v), want %#v", raw, rerr, blocker))
		}
	}
	return p
}
`;

// goTest runs one test by exact name in its own process and says what happened to it.
function goTest(name, env, overlay) {
  const args = ['test', '-json', '-count=1'];
  if (overlay) args.push('-overlay', overlay);
  args.push('./build', '-run', '^' + name + '$');
  const r = spawnSync('go', args, {encoding: 'utf8', env: {...process.env, PWD: process.cwd(), ...env}, timeout: GO_TIMEOUT_MS, maxBuffer: 64 * 1024 * 1024});
  const rows = (r.stdout || '').split('\n').filter(l => l.startsWith('{')).map(l => {
    try { return JSON.parse(l); } catch { return {}; }
  });
  const count = action => rows.filter(x => x.Test === name && x.Action === action).length;
  const output = rows.filter(x => x.Test === name && x.Action === 'output').map(x => x.Output).join('');
  let outcome;
  if (r.error) outcome = 'did not finish (' + r.error.message + ')';
  else if (count('run') === 0) outcome = 'absent: it ran zero times';
  else if (count('skip') > 0) outcome = 'skipped';
  else if (count('fail') > 0) outcome = 'failed';
  else if (r.status === 0 && count('run') === 1 && count('pass') === 1) outcome = 'passed';
  else outcome = `not a single passing run (exit ${r.status}, runs ${count('run')}, passes ${count('pass')})`;
  return {outcome, output, command: 'go ' + args.join(' '), raw: (r.stdout || '') + (r.stderr || '')};
}

function runFixture(mode, expect) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kbg-'));
  try {
    const overlay = path.join(dir, 'overlay.json');
    const source = path.join(dir, 'zz_kbg_acceptance_test.go');
    fs.writeFileSync(source, FIXTURE_GO);
    fs.writeFileSync(overlay, JSON.stringify({Replace: {[path.resolve(OVERLAY_TARGET)]: source}}));
    return goTest(FIXTURE, {KBG_MODE: mode, KBG_EXPECT: expect}, overlay);
  } finally {
    fs.rmSync(dir, {recursive: true, force: true});
  }
}

function describe(r, what) {
  return `${what}: ${r.outcome}; command: ${r.command}\n${r.output || r.raw}`;
}

// expectationOf is the doc string of the feature's step for the fixture mode.
function expectationOf(mode) {
  const lines = fs.readFileSync(FEATURE, 'utf8').split('\n');
  const at = lines.findIndex(l => l.includes(`the Build fixture "${mode}" `));
  const open = lines.findIndex((l, i) => i > at && l.trim() === '"""');
  const close = lines.findIndex((l, i) => i > open && l.trim() === '"""');
  if (at < 0 || open < 0 || close < 0) throw new Error(`no expectations for ${mode} in ${FEATURE}`);
  const indent = lines[open].indexOf('"""');
  return lines.slice(open + 1, close).map(l => l.slice(indent)).join('\n');
}

const CONTROLS = [['first', 'first-round'], ['unnamed', 'unnamed']];

function control(kind) {
  const results = CONTROLS.map(([suffix, mode]) => runFixture(`control-${kind}-${suffix}`, expectationOf(mode)));
  if (kind === 'good') {
    if (results.every(r => r.outcome === 'passed')) return console.log('observer accepted');
    results.forEach((r, i) => console.error(describe(r, `${FIXTURE} control-good-${CONTROLS[i][0]}`)));
    console.log('observer rejected');
    process.exitCode = 1;
    return;
  }
  if (results.every(r => r.outcome === 'failed' && r.output.includes('kbg-reject: blocker tool output:'))) {
    console.log('observer rejected');
    process.exitCode = 1;
  } else if (results.some(r => r.outcome === 'passed')) {
    console.log('observer accepted');
  } else {
    results.forEach((r, i) => console.error(describe(r, `${FIXTURE} control-bad-${CONTROLS[i][0]}`)));
    console.log('observer broken');
    process.exitCode = 2;
  }
}

if (require.main === module) {
  const kind = {'control-good': 'good', 'control-bad': 'bad'}[process.argv[2]];
  if (!kind) {
    console.log('usage: node ' + path.basename(__filename) + ' control-good|control-bad');
    process.exitCode = 2;
  } else {
    control(kind);
  }
} else {
  const {Then} = require('@cucumber/cucumber');

  Then('k-build-blocker-claim-grounded: the Build fixture {string} meets these expectations', {timeout: GO_TIMEOUT_MS + 60000}, function (mode, expect) {
    JSON.parse(expect);
    const r = runFixture(mode, expect);
    if (r.outcome !== 'passed') throw new Error(describe(r, `${FIXTURE} with KBG_MODE=${mode}`));
  });

  Then('k-build-blocker-claim-grounded: the existing test {string} passes in its own go test run', {timeout: GO_TIMEOUT_MS + 60000}, function (name) {
    const r = goTest(name, {}, null);
    if (r.outcome !== 'passed') throw new Error(describe(r, `existing test ${name}`));
  });

  Then('k-build-blocker-claim-grounded: these golden files keep their bytes', function (list) {
    const changed = [];
    for (const line of list.split('\n').filter(l => l.trim() !== '')) {
      const [want, file] = line.trim().split(/\s+/);
      let got;
      try {
        got = crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
      } catch (e) {
        got = 'unreadable (' + e.message + ')';
      }
      if (got !== want) changed.push(`${file}: sha256 ${got}, want ${want}`);
    }
    if (changed.length) throw new Error('golden files changed; Builds without a blocker report must keep their requests and results:\n' + changed.join('\n'));
  });
}
