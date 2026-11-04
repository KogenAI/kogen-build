'use strict';
// Runs the protected round-limit Build fixture: a driver program (the Go source below, a main
// package overlaid at a path the Build must not create) calls package build's Build and Resume on
// a fresh fixture project with a scripted builder and prints what it observed as one JSON object;
// this file judges that observation against the expectations of the acceptance feature.
const {spawnSync} = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const driverPath = 'cmd/roundlimitdriver/main.go';
const keys = ['redRounds', 'exit', 'outcome', 'passes', 'passEvents', 'rounds', 'reviewedPasses', 'feedbackStarts',
  'stdout', 'stdoutLacks', 'stderrLacks', 'app', 'resume'];
const resumeKeys = ['exit', 'stdout', 'rounds', 'app'];

// observe builds the driver from the checkout and runs it once for mode.
function observe(mode, want) {
  if (fs.existsSync(path.resolve(path.dirname(driverPath)))) {
    return {error: `${path.dirname(driverPath)} exists in the checkout; the round-limit fixture's driver owns that path`};
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'roundlimit-'));
  try {
    const source = path.join(dir, 'main.go');
    fs.writeFileSync(source, DRIVER + '\n');
    const overlay = path.join(dir, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({Replace: {[path.resolve(driverPath)]: source}}));
    const bin = path.join(dir, 'roundlimitdriver');
    const args = ['build', '-overlay', overlay, '-o', bin, './' + path.dirname(driverPath)];
    // Go resolves the overlay's keys against $PWD: make it the directory Node resolved them in.
    const b = spawnSync('go', args, {cwd: process.cwd(), encoding: 'utf8', env: {...process.env, PWD: process.cwd()}, timeout: 600000, maxBuffer: 16 * 1024 * 1024});
    if (b.status !== 0) {
      return {error: `go ${args.join(' ')} exited ${b.status}${b.error ? ' (' + b.error.message + ')' : ''}\n${b.stdout || ''}${b.stderr || ''}`};
    }
    const fixture = path.join(dir, 'fixture');
    fs.mkdirSync(fixture);
    const env = {...process.env, ROUNDLIMIT_MODE: mode, ROUNDLIMIT_DIR: fixture};
    if (want.redRounds !== undefined) env.ROUNDLIMIT_RED_ROUNDS = String(want.redRounds);
    if (want.resume !== undefined) env.ROUNDLIMIT_RESUME = '1';
    const r = spawnSync(bin, [], {cwd: dir, encoding: 'utf8', env, timeout: 600000, maxBuffer: 64 * 1024 * 1024});
    if (r.status !== 0) {
      return {error: `the round-limit driver (mode ${mode}) exited ${r.status}${r.error ? ' (' + r.error.message + ')' : ''}\n${r.stdout || ''}${r.stderr || ''}`};
    }
    try {
      return {obs: JSON.parse(r.stdout)};
    } catch (e) {
      return {error: `the round-limit driver (mode ${mode}) printed no JSON observation: ${e.message}\n${r.stdout}${r.stderr || ''}`};
    }
  } finally {
    fs.rmSync(dir, {recursive: true, force: true});
  }
}

// judge lists every way the observation differs from the expectations.
function judge(mode, want, obs) {
  const failures = [];
  for (const k of Object.keys(want)) if (!keys.includes(k)) failures.push(`unknown expectation key ${k}`);
  const b = obs.build || {};
  const eq = (label, got, exp) => {
    if (exp !== undefined && JSON.stringify(got) !== JSON.stringify(exp)) failures.push(`${label} ${JSON.stringify(got)}, want ${JSON.stringify(exp)}`);
  };
  eq('exit', b.exit, want.exit);
  eq('outcome', obs.outcome, want.outcome);
  eq('passes', obs.passes, want.passes);
  eq('pass events', obs.passEvents, want.passEvents);
  eq('builder rounds', obs.rounds, want.rounds);
  eq('reviewed passes', obs.reviewedPasses, want.reviewedPasses);
  eq('app.txt', obs.app, want.app);
  const inputs = obs.inputs || [];
  for (const [round, start] of Object.entries(want.feedbackStarts || {})) {
    const got = inputs[Number(round) - 1];
    if (got === undefined) failures.push(`round ${round} did not run, so it cannot have received feedback starting ${JSON.stringify(start)}`);
    else if (!got.startsWith(start)) failures.push(`round ${round} input does not start with ${JSON.stringify(start)}: ${JSON.stringify(got.slice(0, 600))}`);
  }
  for (const s of want.stdout || []) if (!(b.stdout || '').includes(s)) failures.push(`stdout lacks ${JSON.stringify(s)}`);
  for (const s of want.stdoutLacks || []) if ((b.stdout || '').includes(s)) failures.push(`stdout contains ${JSON.stringify(s)}`);
  for (const s of want.stderrLacks || []) if ((b.stderr || '').includes(s)) failures.push(`stderr contains ${JSON.stringify(s)}`);
  if (want.resume !== undefined) {
    const r = obs.resume;
    if (!r) {
      failures.push('no resume ran');
    } else {
      for (const k of Object.keys(want.resume)) if (!resumeKeys.includes(k)) failures.push(`unknown resume expectation key ${k}`);
      eq('resume exit', r.exit, want.resume.exit);
      eq('builder rounds after resume', r.rounds, want.resume.rounds);
      eq('app.txt after resume', r.app, want.resume.app);
      for (const s of want.resume.stdout || []) if (!(r.stdout || '').includes(s)) failures.push(`resume stdout lacks ${JSON.stringify(s)}`);
    }
  }
  return failures.map(f => `mode ${mode}: ${f}`);
}

function report(mode, failures, obs) {
  const b = (obs && obs.build) || {};
  const tail = (b.stderr || '').split('\n').slice(-30).join('\n');
  return `${failures.join('\n')}\n--- Build stdout ---\n${b.stdout || ''}--- Build stderr (last 30 lines) ---\n${tail}`;
}

// The driver program.
const DRIVER = String.raw`// Command roundlimitdriver runs one Build of a fixture project through package build with a
// scripted builder and prints what it observed as one JSON object. ROUNDLIMIT_MODE selects the
// builder's script, ROUNDLIMIT_RED_ROUNDS the number of identical red rounds of a native
// rewriting builder, and ROUNDLIMIT_DIR an empty directory for the fixture.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/KogenAI/kogen-build/build"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

const slug = "fixture"

const feature = "Feature: Round-limit fixture\n  Scenario: First\n    Then app.txt says green\n"

// steps is the fixture's step file: its acceptance passes once app.txt says green, or, when the
// red check is the failing item (checkRed), once app.txt was written at all.
func steps(checkRed bool) string {
	want := "fs.readFileSync('app.txt', 'utf8') !== 'green\\n'"
	if checkRed {
		want = "fs.readFileSync('app.txt', 'utf8') === 'baseline\\n'"
	}
	return "const {Then} = require('@cucumber/cucumber');\nconst fs = require('node:fs');\n" +
		"Then('app.txt says green', function () {\n  if (" + want + ") throw new Error('app.txt is not green');\n});\n"
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "roundlimitdriver: "+format+"\n", args...)
	os.Exit(3)
}

func write(path, content string, mode os.FileMode) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail("%v", err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		fail("%v", err)
	}
}

func gitEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			out = append(out, kv)
		}
	}
	return append(out, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
}

func git(root string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = root, gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		fail("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// fixture makes the repository: one commit with README.md, app.txt ("baseline"), .gitignore and
// the configuration; the Intent "fixture" (one scenario, red unless app.txt says green) approved.
// With checkRed the acceptance passes once app.txt is written and the check "test" fails while
// app.txt says red, without output, as kogen's spec fixture "! grep -q is_even calc.py" does.
func fixture(dir string, external, checkRed bool) (root, home string) {
	root, home = filepath.Join(dir, "checkout"), filepath.Join(dir, "home")
	hooks := filepath.Join(dir, "hooks")
	for _, d := range []string{root, home, hooks} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			fail("%v", err)
		}
	}
	git(root, "init", "-q", "--initial-branch=main")
	git(root, "config", "user.name", "Round Limit Fixture")
	git(root, "config", "user.email", "round-limit@example.invalid")
	git(root, "config", "commit.gpgsign", "false")
	git(root, "config", "core.hooksPath", hooks)
	write(filepath.Join(root, "README.md"), "# Fixture project\n", 0o644)
	write(filepath.Join(root, "app.txt"), "baseline\n", 0o644)
	write(filepath.Join(root, ".gitignore"), ".kogen/local/\n", 0o644)
	node, cucumber := os.Getenv("KOGEN_NODE"), os.Getenv("KOGEN_CUCUMBER")
	if node == "" || cucumber == "" {
		fail("KOGEN_NODE and KOGEN_CUCUMBER must name Kogen's Node and Cucumber tools")
	}
	accept := filepath.Join(dir, "accept.sh")
	write(accept, "#!/bin/sh\nNODE_PATH='"+cucumber+"' exec '"+node+"' '"+cucumber+"/@cucumber/cucumber/bin/cucumber.js' --strict "+
		"--require 'features/steps/*.js' --format \"message:$KOGEN_ACCEPTANCE_REPORT\" \"$KOGEN_ACCEPTANCE_FEATURES\"\n", 0o755)
	cfg := &project.Config{Acceptance: &project.Acceptance{Run: accept, StepsDir: "features/steps"}}
	if checkRed {
		cfg.Checks = []project.Check{{Name: "test", Run: "! grep -qx red app.txt"}}
	}
	config := project.RenderConfig(cfg)
	if external {
		// The build role runs on the claude-code executor; the other roles keep their defaults.
		head, _, ok := strings.Cut(config, "models:\n")
		if !ok {
			fail("the rendered configuration has no models block:\n%s", config)
		}
		config = head + "models:\n  build:\n    executor: claude-code\n    model: claude-sonnet-5-5\n    effort: high\n"
	}
	write(filepath.Join(root, ".kogen/project.yaml"), config, 0o644)
	git(root, "add", "-A")
	git(root, "commit", "-qm", "fixture baseline")
	write(filepath.Join(root, project.IntentMDPath(slug)), "# Keep the fixture\n\nSlug: fixture\nModel: not applicable — no model for this fixture\n\n"+
		"Acceptance:\n- .kogen/intents/fixture/acceptance.feature\n- features/steps/fixture_steps.js\n\n"+
		"## Tests\n- app.txt says green: the acceptance scenario (scenario)\n\n## Source comparison\n- addressed: app.txt says green; source request; contract acceptance.feature.\n\n## Feasibility\n- none\n", 0o644)
	write(filepath.Join(root, project.FeaturePath(slug)), feature, 0o644)
	write(filepath.Join(root, "features/steps/fixture_steps.js"), steps(checkRed), 0o644)
	loaded, err := project.LoadConfig(root)
	if err != nil {
		fail("%v", err)
	}
	baseline, err := project.Baseline(root, slug, loaded)
	if err != nil {
		fail("%v", err)
	}
	if err := project.WriteApproval(root, project.Approval{Slug: slug, SHA256: project.Hash(root, baseline), Base: git(root, "rev-parse", "HEAD"),
		Approver: "Round Limit Fixture <round-limit@example.invalid>", ApprovedAt: "2026-11-04T00:00:00Z"}); err != nil {
		fail("%v", err)
	}
	creds, _ := json.Marshal(map[string]any{"client_id": "oaiapp_fake", "access_token": "fake-access-token", "refresh_token": "fake-refresh-token",
		"expires_at": 4102444800, "scope": "openid", "email": "round-limit@example.invalid"})
	if err := os.MkdirAll(filepath.Join(home, ".kogen", "credentials"), 0o700); err != nil {
		fail("%v", err)
	}
	os.Chmod(filepath.Join(home, ".kogen"), 0o700)
	write(filepath.Join(home, ".kogen", "credentials", "chatgpt.json"), string(creds), 0o600)
	return root, home
}

// ---- the scripted provider (the native builder's model) ----

type scriptedProvider struct {
	mu     sync.Mutex
	ids    int
	script [][]map[string]any // each response's output items; past the end: refusal
	inputs []string           // what the builder received last in each request
}

func (p *scriptedProvider) call(name string, args any) map[string]any {
	b, _ := json.Marshal(args)
	p.ids++
	return map[string]any{"type": "function_call", "id": fmt.Sprintf("function-%d", p.ids), "call_id": fmt.Sprintf("call-%d", p.ids),
		"name": name, "arguments": string(b)}
}

func say(text string) map[string]any {
	return map[string]any{"type": "message", "id": "msg-round-limit", "role": "assistant",
		"content": []any{map[string]any{"type": "output_text", "text": text}}}
}

func lastInput(req map[string]any) string {
	items, _ := req["input"].([]any)
	if len(items) == 0 {
		return ""
	}
	it, _ := items[len(items)-1].(map[string]any)
	if s, ok := it["output"].(string); ok {
		return s
	}
	switch c := it["content"].(type) {
	case string:
		return c
	case []any:
		var parts []string
		for _, part := range c {
			if m, ok := part.(map[string]any); ok {
				if s, ok := m["text"].(string); ok {
					parts = append(parts, s)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func (p *scriptedProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	if r.URL.Path != "/v1/responses" {
		http.NotFound(w, r)
		return
	}
	var req map[string]any
	json.Unmarshal(body, &req)
	p.mu.Lock()
	n := len(p.inputs)
	p.inputs = append(p.inputs, lastInput(req))
	var items []map[string]any
	if n < len(p.script) {
		items = p.script[n]
	}
	p.mu.Unlock()
	if items == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		io.WriteString(w, "{\"error\":{\"code\":\"invalid_request_error\",\"message\":\"scripted provider refusal\",\"type\":\"invalid_request_error\"}}")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	for i, it := range items {
		b, _ := json.Marshal(map[string]any{"type": "response.output_item.done", "output_index": i, "item": it})
		io.WriteString(w, "data: "+string(b)+"\n\n")
	}
	done, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"usage": map[string]any{
		"input_tokens": 1, "input_tokens_details": map[string]any{"cached_tokens": 0},
		"output_tokens": 1, "output_tokens_details": map[string]any{"reasoning_tokens": 0}}}})
	io.WriteString(w, "data: "+string(done)+"\n\n")
}

// ---- the scripted external executor (as claude-code is for the Build) ----

type scriptedExecutor struct {
	mu     sync.Mutex
	root   string
	inputs []string
	round  func(n int, root string, emit executor.Sink) executor.RoundResult
}

func (x *scriptedExecutor) Describe() executor.Descriptor {
	return executor.Descriptor{Name: executor.ClaudeCode, Contract: executor.Contract, Version: "fake-claude 9.9.9",
		Capabilities: executor.Capabilities{Roles: []string{executor.RoleBuild}, Resume: executor.ResumeNone, Continuation: true,
			Usage: executor.UsageHarness, Prevention: []string{executor.PreventPreToolWrite}, MaxRounds: 6}}
}

func (x *scriptedExecutor) Start(ctx context.Context, t executor.Task, emit executor.Sink) (executor.Session, error) {
	x.mu.Lock()
	x.root = t.Root
	x.mu.Unlock()
	emit(map[string]any{"event": "executor_started", "model": t.Model})
	return &scriptedSession{x: x, emit: emit}, nil
}

type scriptedSession struct {
	x    *scriptedExecutor
	emit executor.Sink
	n    int
}

func (s *scriptedSession) Round(ctx context.Context, in executor.Input) (executor.RoundResult, error) {
	s.n++
	s.x.mu.Lock()
	s.x.inputs = append(s.x.inputs, in.Feedback)
	root := s.x.root
	s.x.mu.Unlock()
	res := s.x.round(s.n, root, s.emit)
	res.Handle = s.Handle()
	return res, nil
}

func (s *scriptedSession) Handle() executor.Handle {
	return executor.Handle{Executor: executor.ClaudeCode, Version: "fake-claude 9.9.9", SessionID: "fake-session-round-limit", Resume: executor.ResumeNone}
}

func (s *scriptedSession) Close() error { return nil }

// writeApp writes app.txt with the Write tool and reports the call as the claude-code adapter does.
func writeApp(n int, root string, emit executor.Sink, content string) executor.RoundResult {
	target := filepath.Join(root, "app.txt")
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		fail("%v", err)
	}
	emit(map[string]any{"event": "executor_tool", "round": n, "name": "Write", "tool_use_id": "toolu_" + strconv.Itoa(n) + "_0",
		"denied": false, "arguments": map[string]any{"file_path": target}})
	return executor.RoundResult{Ending: executor.EndFinish, Claim: "Implemented app.txt."}
}

// recordingReviewer records each pass it is asked to review and refuses to review it.
type recordingReviewer struct{ passes *[]int }

func (r recordingReviewer) Review(ctx context.Context, b *build.Attempt, pass verify.Pass) (build.Approved, error) {
	*r.passes = append(*r.passes, pass.N)
	return build.Approved{}, fmt.Errorf("round-limit fixture: review of pass %d was requested", pass.N)
}

func (r recordingReviewer) Resume(ctx context.Context, b *build.Attempt) (build.Approved, error) {
	return build.Approved{}, fmt.Errorf("round-limit fixture: review resume was requested")
}

// invoke runs one Build (or, with resumeID, one resume) and returns its exit code, its stdout
// with a refusal's own text, and its stderr.
func invoke(root, home, url string, deps build.Deps, resumeID string) map[string]any {
	env := append(gitEnv(), "HOME="+home, "KOGEN_PROVIDER_URL="+url, "KOGEN_AUTH_URL="+url)
	s := sys.NewWith(env, root)
	out, errs := &strings.Builder{}, &strings.Builder{}
	s.Stdout, s.Stderr = out, errs
	var code int
	var err error
	if resumeID == "" {
		code, err = build.Build(context.Background(), s, build.Input{Deps: deps, Project: root, Slug: slug})
	} else {
		code, err = build.Resume(context.Background(), s, build.ResumeInput{Deps: deps, Project: root, ID: resumeID})
	}
	text := out.String()
	if e, ok := sys.AsError(err); ok {
		text += e.Render()
	} else if err != nil {
		text += err.Error() + "\n"
	}
	return map[string]any{"exit": code, "stdout": text, "stderr": errs.String()}
}

func main() {
	mode := os.Getenv("ROUNDLIMIT_MODE")
	dir := os.Getenv("ROUNDLIMIT_DIR")
	if dir == "" {
		fail("ROUNDLIMIT_DIR is not set")
	}
	external := strings.HasPrefix(mode, "external-")
	root, home := fixture(dir, external, mode == "external-rewrites")
	p := &scriptedProvider{}
	red := map[string]any{"path": "app.txt", "content": "red\n"}
	var x *scriptedExecutor
	switch mode {
	case "native-rewrites":
		n, err := strconv.Atoi(os.Getenv("ROUNDLIMIT_RED_ROUNDS"))
		if err != nil || n < 1 {
			fail("ROUNDLIMIT_RED_ROUNDS must be a positive integer")
		}
		for i := 0; i < n; i++ {
			p.script = append(p.script, []map[string]any{p.call("write_file", red), p.call("finish", map[string]any{"summary": "Implemented app.txt."})})
		}
	case "native-read-only":
		p.script = append(p.script,
			[]map[string]any{p.call("write_file", red), p.call("finish", map[string]any{"summary": "Implemented app.txt."})},
			[]map[string]any{p.call("read_file", map[string]any{"path": "app.txt"}), p.call("finish", map[string]any{"summary": "Looked again."})})
		for i := 0; i < 8; i++ {
			p.script = append(p.script, []map[string]any{say("I could not complete this; no files were changed, and I am not calling finish.")})
		}
	case "external-rewrites":
		x = &scriptedExecutor{round: func(n int, root string, emit executor.Sink) executor.RoundResult {
			return writeApp(n, root, emit, "red\n")
		}}
	case "external-status-only":
		x = &scriptedExecutor{round: func(n int, root string, emit executor.Sink) executor.RoundResult {
			if n == 1 {
				return writeApp(n, root, emit, "red\n")
			}
			return executor.RoundResult{Ending: executor.EndNoCall, Claim: "I could not complete this; no files were changed."}
		}}
	default:
		fail("unknown ROUNDLIMIT_MODE %q", mode)
	}
	srv := httptest.NewServer(p)
	defer srv.Close()
	var reviewed []int
	deps := build.Deps{Client: provider.NewClient("kogen-build round-limit fixture"), Tools: verify.Tools{Node: os.Getenv("KOGEN_NODE"), Cucumber: os.Getenv("KOGEN_CUCUMBER")},
		Reviewer: recordingReviewer{&reviewed}}
	if x != nil {
		deps.External = func(role project.Role) (executor.Executor, error) { return x, nil }
	}
	inputs := func() []string {
		if x != nil {
			x.mu.Lock()
			defer x.mu.Unlock()
			return append([]string(nil), x.inputs...)
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		return append([]string(nil), p.inputs...)
	}
	obs := map[string]any{"mode": mode}
	first := invoke(root, home, srv.URL, deps, "")
	obs["build"] = first
	entries, _ := os.ReadDir(filepath.Join(root, ".kogen/local/runs"))
	var ids []string
	for _, en := range entries {
		ids = append(ids, en.Name())
	}
	obs["runs"] = len(ids)
	if len(ids) == 1 {
		id := ids[0]
		if b, err := os.ReadFile(filepath.Join(root, ".kogen/local/runs", id, "result.json")); err == nil {
			var res map[string]any
			json.Unmarshal(b, &res)
			obs["outcome"], obs["passes"] = res["outcome"], res["passes"]
		}
		if b, err := os.ReadFile(filepath.Join(root, ".kogen/local/runs", id, "log.jsonl")); err == nil {
			obs["passEvents"] = strings.Count(string(b), "\"event\":\"pass\"")
		}
	}
	in := inputs()
	obs["rounds"] = len(in)
	obs["inputs"] = in
	obs["reviewedPasses"] = append([]int{}, reviewed...)
	if b, err := os.ReadFile(filepath.Join(root, "app.txt")); err == nil {
		obs["app"] = string(b)
	}
	if os.Getenv("ROUNDLIMIT_RESUME") == "1" && len(ids) == 1 {
		r := invoke(root, home, srv.URL, deps, ids[0])
		b, _ := os.ReadFile(filepath.Join(root, "app.txt"))
		r["rounds"], r["app"] = len(inputs()), string(b)
		obs["resume"] = r
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(obs); err != nil {
		fail("%v", err)
	}
}`;

// Observer controls. They use builder scripts whose outcome this Intent does not change, so they
// hold before and after the Build: control-good must accept the true expectations of both
// text-only scripts; control-bad must reject an expectation that differs only in the outcome,
// at exactly that assertion.
const NOTICE = 'Round 2 made no edits: the candidate is byte-identical to the tree verification pass 1 checked, so it was not verified again; pass 1 is still red.\n' +
  'Make an edit toward the first named failure: failed: First (line 2).\n' +
  'If a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\n' +
  'Another round without edits stops the Build as no_progress (no edits).';
const STOP = 'candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none; verification pass 1 is still red and was not run again; edits are left in place\n';
function textOnlyWant(extra) {
  return {
    exit: 1, outcome: 'no_progress', passes: 1, passEvents: 1, rounds: 3, reviewedPasses: [],
    feedbackStarts: {'2': 'Verification pass 1 is not green.', '3': NOTICE},
    stdout: [STOP, '  rounds without edits: 2 of 3\n'], stdoutLacks: ['executor_round_limit'], stderrLacks: ['kogen: round 4 '],
    app: 'red\n', ...extra,
  };
}

function check(mode, want) {
  const o = observe(mode, want);
  if (o.error) return {ran: false, failures: [o.error], text: o.error};
  const failures = judge(mode, want, o.obs);
  return {ran: true, failures, text: report(mode, failures, o.obs)};
}

if (require.main === module) {
  const control = process.argv[2];
  if (control === 'control-good') {
    const results = [check('native-read-only', textOnlyWant({})), check('external-status-only', textOnlyWant({}))];
    const failed = results.filter(r => !r.ran || r.failures.length > 0);
    if (failed.length === 0) {
      console.log('observer accepted');
    } else {
      console.log(failed.map(r => r.text).join('\n'));
      process.exitCode = 2;
    }
  } else if (control === 'control-bad') {
    const r = check('native-read-only', textOnlyWant({outcome: 'landed'}));
    if (r.ran && r.failures.length === 1 && r.failures[0] === 'mode native-read-only: outcome "no_progress", want "landed"') {
      console.log('observer rejected');
      process.exitCode = 1;
    } else {
      console.log(r.text);
      process.exitCode = 2;
    }
  } else {
    console.log('usage: k-build-round-limit-outcome__steps.js control-good|control-bad');
    process.exitCode = 2;
  }
} else {
  const {Then} = require('@cucumber/cucumber');
  Then('k-build-round-limit-outcome: the round-limit Build fixture {string} ends as specified', {timeout: 1260000}, function (mode, text) {
    const r = check(mode, JSON.parse(text));
    if (!r.ran || r.failures.length > 0) throw new Error(r.text);
  });
}
