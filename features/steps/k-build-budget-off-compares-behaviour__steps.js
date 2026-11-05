'use strict';
// Steps of k-build-budget-off-compares-behaviour. TestBuildRequestBudgetOff (package build) runs
// from a test binary compiled once in a scratch copy of this checkout: the files `git ls-files`
// lists (tracked and untracked, not ignored), plus an injection harness. The harness adds two Go
// files to package build and one call line at each of six anchors (three in build/core_run.go,
// three in build/helper_test.go); with KBO_MODE unset every call is a no-op. With KBO_MODE set,
// the second Build of the test process (its second project root, fake provider and env.build
// call) is changed as the mode says, while its first Build, the budget-omitted reference, stays as
// it is (in "noise" mode both Builds get differing noise). The checkout itself is never changed.
//
//   noise                  timing values, host load figures, step durations and a toolchain
//                          probe's status differ in log.jsonl, result.json and checkpoint.json,
//                          and the second Build logs an extra toolchain_probe_timeout event
//   budget-in-checkpoint   checkpoint.json state carries "build_request_budget"
//   budget-in-result       result.json carries {"limits":{"request_budget_left":0}}
//   budget-in-log          log.jsonl carries a build_request_budget event before the outcome
//   extra-builder-request  the builder is scripted one more read_file request first
//   provider-refusal       the provider refuses the builder's first request
//   second-commit          an empty commit "Extra commit" follows the Build
//
// A failure names the test, the mode, the command, its exit status and the tail of its output.
//
// `node features/steps/k-build-budget-off-compares-behaviour__steps.js good|bad` runs the run
// judges on real `-test.v` output of TestBuildRequestBudgetOff: good prints "accepted" and exits 0
// when the real passing run and the real failing run (with a line it printed) are accepted; bad
// prints "rejected" and exits 1 when every refused variant of them is rejected.
const { spawn, spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..');
const TEST = 'TestBuildRequestBudgetOff';
const MODES = ['noise', 'budget-in-checkpoint', 'budget-in-result', 'budget-in-log', 'extra-builder-request', 'provider-refusal', 'second-commit'];

function run(cmd, args, opts) {
  const r = spawnSync(cmd, args, { cwd: root, encoding: 'utf8', maxBuffer: 256 << 20, ...opts });
  return { status: r.status, stdout: r.stdout || '', stderr: r.stderr || '', error: r.error, command: `${cmd} ${args.join(' ')}` };
}

function must(r, what) {
  if (r.status !== 0) {
    throw new Error(`${what}: ${r.command} exited ${r.status}${r.error ? ` (${r.error})` : ''}:\n${(r.stdout + r.stderr).slice(-8000)}`);
  }
  return r;
}

const scratchDirs = [];
function scratch() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-build-budget-off-compares-behaviour-'));
  if (scratchDirs.length === 0) {
    process.on('exit', () => {
      for (const d of scratchDirs) {
        spawnSync('chmod', ['-R', 'u+w', d]);
        fs.rmSync(d, { recursive: true, force: true });
      }
    });
  }
  scratchDirs.push(dir);
  return dir;
}

// The production half of the harness: Run.Event, Run.Write and Run.PublishCheckpoint pass what
// they are about to write through these functions.
const HARNESS_GO = `package build

import (
	"bytes"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
)

var kboMu sync.Mutex
var kboRoots = map[string]int{}

func kboMode() string { return os.Getenv("KBO_MODE") }

// kboBuild is the ordinal of the run's project root in this process: 1 for the first Build.
func kboBuild(r *Run) int {
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(r.Dir))))
	kboMu.Lock()
	defer kboMu.Unlock()
	n, ok := kboRoots[root]
	if !ok {
		n = len(kboRoots) + 1
		kboRoots[root] = n
	}
	return n
}

func kboLoad() []any { return []any{rand.Float64() * 20, rand.Float64() * 20, rand.Float64() * 20} }

func kboDoc(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		panic(err)
	}
	return m
}

func kboEvent(r *Run, event map[string]any) map[string]any {
	mode := kboMode()
	if mode == "" {
		return event
	}
	n := kboBuild(r)
	name, _ := event["event"].(string)
	switch {
	case mode == "noise":
		out := make(map[string]any, len(event)+4)
		for k, v := range event {
			out[k] = v
		}
		out["clock_skew"] = rand.IntN(100000)
		out["host_load"] = kboLoad()
		switch name {
		case "command", "command_result", "step_sweep":
			out["took"] = rand.IntN(100000)
		case "check_identity":
			if n == 2 {
				r.Event(map[string]any{"event": "toolchain_probe_timeout", "tool": "node", "waited": rand.IntN(100000)})
				out["toolchain_probe"] = "timed_out"
			} else {
				out["toolchain_probe"] = "ok"
			}
		}
		return out
	case mode == "budget-in-log" && n == 2 && name == "outcome":
		r.Event(map[string]any{"event": "build_request_budget", "role": "build", "count": 8, "limit": 0})
	}
	return event
}

func kboWrite(r *Run, name string, data any) any {
	mode := kboMode()
	if name != "result.json" || (mode != "noise" && mode != "budget-in-result") {
		return data
	}
	n := kboBuild(r)
	if mode == "budget-in-result" && n != 2 {
		return data
	}
	m := kboDoc(data)
	if mode == "noise" {
		m["host_load"] = kboLoad()
		m["cleanup_seconds"] = rand.Float64()
	} else {
		m["limits"] = map[string]any{"request_budget_left": 0}
	}
	return m
}

func kboCheckpoint(r *Run, doc any) any {
	mode := kboMode()
	if mode != "noise" && mode != "budget-in-checkpoint" {
		return doc
	}
	n := kboBuild(r)
	if mode == "budget-in-checkpoint" && n != 2 {
		return doc
	}
	m := kboDoc(doc)
	if mode == "noise" {
		m["saved_load"] = kboLoad()
		return m
	}
	state, ok := m["state"].(map[string]any)
	if !ok {
		panic("checkpoint state is not an object")
	}
	state["build_request_budget"] = map[string]any{"limit": 0, "count": 8}
	return m
}
`;

// The test half: the fake providers, their first script and env.build calls are counted.
const HARNESS_TEST_GO = `package build

var kboProviders = map[*fakeProvider]int{}
var kboScripted = map[*fakeProvider]bool{}
var kboBuilds int

func kboProvider(f *fakeProvider) {
	kboMu.Lock()
	defer kboMu.Unlock()
	kboProviders[f] = len(kboProviders) + 1
}

func kboThen(f *fakeProvider, h []handler) []handler {
	kboMu.Lock()
	first := !kboScripted[f]
	kboScripted[f] = true
	n := kboProviders[f]
	kboMu.Unlock()
	if !first || n != 2 {
		return h
	}
	switch kboMode() {
	case "extra-builder-request":
		return append([]handler{f.calls("read_file", map[string]any{"path": "app.txt"})}, h...)
	case "provider-refusal":
		return append([]handler{refused()}, h...)
	}
	return h
}

func kboAfterBuild(e *env) {
	kboMu.Lock()
	kboBuilds++
	n := kboBuilds
	kboMu.Unlock()
	if n == 2 && kboMode() == "second-commit" {
		e.run("git", "commit", "--allow-empty", "-q", "-m", "Extra commit")
	}
}
`;

// Each anchor occurs exactly once; the line is inserted right after it.
const ANCHORS = [
  ['build/core_run.go', 'func (r *Run) Event(event map[string]any) {\n\tif r == nil {\n\t\treturn\n\t}\n', '\tevent = kboEvent(r, event)\n'],
  ['build/core_run.go', 'func (r *Run) Write(name string, data any) error {\n', '\tdata = kboWrite(r, name, data)\n'],
  ['build/core_run.go', 'func (r *Run) PublishCheckpoint(doc any) error {\n', '\tdoc = kboCheckpoint(r, doc)\n'],
  ['build/helper_test.go', 'func newFakeProvider(t *testing.T) *fakeProvider {\n\tf := &fakeProvider{t: t}\n', '\tkboProvider(f)\n'],
  ['build/helper_test.go', 'func (f *fakeProvider) then(h ...handler) *fakeProvider {\n', '\th = kboThen(f, h)\n'],
  ['build/helper_test.go', 'func (e *env) build(mutate ...func(*Input)) *attempt {\n', null],
];
const BUILD_CALL = '\ta.exit, a.err = Build(context.Background(), s, in)\n';

function insertAfter(file, body, anchor, line) {
  const n = body.split(anchor).length - 1;
  if (n !== 1) throw new Error(`${file} has ${n} occurrences of ${JSON.stringify(anchor)}, want 1: the injection harness cannot be placed`);
  const at = body.indexOf(anchor) + anchor.length;
  if (line !== null) return body.slice(0, at) + line + body.slice(at);
  // env.build: after its Build call.
  const call = body.indexOf(BUILD_CALL, at);
  const end = body.indexOf('\n}\n', at);
  if (call < 0 || call > end) throw new Error(`${file}: env.build has no line ${JSON.stringify(BUILD_CALL)}: the injection harness cannot be placed`);
  const after = call + BUILD_CALL.length;
  return body.slice(0, after) + '\tkboAfterBuild(e)\n' + body.slice(after);
}

// listFiles lists the checkout's files: what `git ls-files` lists (tracked and untracked, not
// ignored) or, outside a Git work tree, every file except under .git, node_modules and .kogen/local.
function listFiles() {
  const git = run('git', ['ls-files', '-z', '-c', '-o', '--exclude-standard']);
  if (git.status === 0) return git.stdout.split('\0').filter(Boolean);
  const out = [];
  const walk = (rel) => {
    for (const e of fs.readdirSync(path.join(root, rel), { withFileTypes: true })) {
      const r = path.join(rel, e.name);
      if (e.isDirectory()) {
        if (e.name !== '.git' && e.name !== 'node_modules' && r !== path.join('.kogen', 'local')) walk(r);
      } else out.push(r);
    }
  };
  walk('');
  return out;
}

// prepare copies the checkout, adds the harness and compiles the test binary of package build.
function prepare() {
  const dir = scratch();
  const src = path.join(dir, 'src');
  const listed = listFiles();
  for (const rel of listed) {
    const from = path.join(root, rel);
    let st;
    try { st = fs.lstatSync(from); } catch { continue; }
    if (!st.isFile()) continue;
    fs.mkdirSync(path.dirname(path.join(src, rel)), { recursive: true });
    fs.copyFileSync(from, path.join(src, rel));
    fs.chmodSync(path.join(src, rel), st.mode & 0o777 | 0o200);
  }
  const bodies = {};
  for (const [file, anchor, line] of ANCHORS) {
    if (!(file in bodies)) bodies[file] = fs.readFileSync(path.join(src, file), 'utf8');
    bodies[file] = insertAfter(file, bodies[file], anchor, line);
  }
  for (const [file, body] of Object.entries(bodies)) fs.writeFileSync(path.join(src, file), body);
  fs.writeFileSync(path.join(src, 'build', 'zz_kbo_harness.go'), HARNESS_GO);
  fs.writeFileSync(path.join(src, 'build', 'zz_kbo_harness_test.go'), HARNESS_TEST_GO);
  const binary = path.join(dir, 'build.test');
  must(run('go', ['test', '-c', '-o', binary, './build'], {
    cwd: src, timeout: 100 * 1000, env: { ...process.env, GOWORK: 'off', GOFLAGS: '-trimpath' },
  }), 'compiling the test binary of package build with the injection harness');
  return { binary, pkgDir: path.join(src, 'build') };
}

// prepared_ prepares once; a failure is kept and thrown again rather than retried.
let prepared;
function prepared_() {
  if (!prepared) {
    try {
      prepared = { value: prepare() };
    } catch (error) {
      prepared = { error };
    }
  }
  if (prepared.error) throw prepared.error;
  return prepared.value;
}

// runTest runs TestBuildRequestBudgetOff once from the binary, with KBO_MODE set to mode ('' for none).
function runTest(mode) {
  const { binary, pkgDir } = prepared_();
  const args = ['-test.run', `^${TEST}$`, '-test.count=1', '-test.v', '-test.timeout=100s'];
  const command = `cd <scratch>/build && ${mode ? `KBO_MODE=${mode} ` : ''}<build.test> ${args.join(' ')}`;
  return new Promise((resolve) => {
    const env = { ...process.env };
    delete env.KBO_MODE;
    if (mode) env.KBO_MODE = mode;
    const child = spawn(binary, args, { cwd: pkgDir, env, stdio: ['ignore', 'pipe', 'pipe'] });
    let output = '';
    child.stdout.on('data', (d) => { output += d; });
    child.stderr.on('data', (d) => { output += d; });
    const timer = setTimeout(() => child.kill('SIGKILL'), 105 * 1000);
    child.on('error', (error) => { clearTimeout(timer); resolve({ status: null, output, error, mode, command }); });
    child.on('close', (status, signal) => {
      clearTimeout(timer);
      resolve({ status, output, error: signal ? `killed by ${signal}` : undefined, mode, command });
    });
  });
}

// injected runs every injection mode once, at most four at a time, the first time one is needed.
let injectedRuns;
function injected(mode) {
  if (!MODES.includes(mode)) throw new Error(`unknown injection ${JSON.stringify(mode)}; the harness knows ${MODES.join(', ')}`);
  if (!injectedRuns) {
    prepared_();
    injectedRuns = {};
    const queue = [...MODES];
    const lanes = [];
    const waiting = {};
    for (const m of MODES) injectedRuns[m] = new Promise((resolve) => { waiting[m] = resolve; });
    const lane = async () => {
      while (queue.length > 0) {
        const m = queue.shift();
        waiting[m](await runTest(m));
      }
    };
    for (let i = 0; i < 4; i++) lanes.push(lane());
  }
  return injectedRuns[mode];
}

const passLine = new RegExp(`^--- PASS: ${TEST} \\((\\d+(?:\\.\\d+)?)s\\)$`, 'm');
const count = (text, re) => (text.match(new RegExp(re.source, 'gm')) || []).length;

// judgePassed lists why a run does not show the test passing in one run, within maxSeconds when given.
function judgePassed(r, maxSeconds) {
  const found = [];
  if (r.error) found.push(`the test binary did not finish: ${r.error}`);
  if (r.status !== 0) found.push(`the test binary exited ${r.status}, want 0`);
  const passes = count(r.output, new RegExp(`^--- PASS: ${TEST} \\(`));
  if (passes !== 1) found.push(`${TEST}: ${passes} "--- PASS: ${TEST}" lines, want 1 (the test did not run or did not pass)`);
  for (const other of ['FAIL', 'SKIP']) {
    if (count(r.output, new RegExp(`^\\s*--- ${other}: `)) > 0) found.push(`a "--- ${other}:" line was printed`);
  }
  if (!/^PASS$/m.test(r.output)) found.push('no final "PASS" line');
  const m = r.output.match(passLine);
  if (m && maxSeconds !== undefined && Number(m[1]) > maxSeconds) found.push(`${TEST} took ${m[1]}s, want at most ${maxSeconds}s`);
  return found;
}

// judgeFailed lists why a run does not show the test failing in its one run and printing expected.
function judgeFailed(r, expected) {
  const found = [];
  if (r.error) found.push(`the test binary did not finish: ${r.error}`);
  if (r.status === 0 || r.status === null) found.push(`the test binary exited ${r.status}, want a failure`);
  const fails = count(r.output, new RegExp(`^--- FAIL: ${TEST} \\(`));
  if (fails !== 1) found.push(`${TEST}: ${fails} "--- FAIL: ${TEST}" lines, want 1 (the test did not run or did not fail)`);
  if (count(r.output, new RegExp(`^--- PASS: ${TEST} \\(`)) > 0) found.push(`${TEST} passed`);
  if (/^panic: /m.test(r.output)) found.push(`${TEST} panicked instead of reporting`);
  if (!r.output.includes(expected)) found.push(`${TEST} did not report ${JSON.stringify(expected)}`);
  return found;
}

function report(r, found) {
  if (found.length === 0) return;
  throw new Error(`${found.join('\n')}\ntest: ${TEST}, injection: ${r.mode || 'none'}\ncommand: ${r.command} (exit ${r.status})\n` +
    `output (last 8000 bytes):\n${r.output.slice(-8000)}`);
}

// Real -test.v output of TestBuildRequestBudgetOff from this harness on the unchanged tree:
// without injection (passing) and with injection "extra-builder-request" (failing).
const realPassing = [
  '=== RUN   TestBuildRequestBudgetOff',
  '=== PAUSE TestBuildRequestBudgetOff',
  '=== CONT  TestBuildRequestBudgetOff',
  '--- PASS: TestBuildRequestBudgetOff (8.98s)',
  'PASS',
  '',
].join('\n');
const realFailing = [
  '=== RUN   TestBuildRequestBudgetOff',
  '=== PAUSE TestBuildRequestBudgetOff',
  '=== CONT  TestBuildRequestBudgetOff',
  '    build_request_budget_test.go:334: requests 8 and 9, want 8',
  '--- FAIL: TestBuildRequestBudgetOff (8.41s)',
  'FAIL',
  '',
].join('\n');
const realFailingLine = 'requests 8 and 9, want 8';

function control(mode) {
  const ok = (output, status) => ({ status, output, mode: 'control', command: 'control' });
  if (mode === 'good') {
    const found = [...judgePassed(ok(realPassing, 0), 40), ...judgeFailed(ok(realFailing, 1), realFailingLine)];
    console.log(found.length === 0 ? 'accepted' : `not accepted: ${found.join('; ')}`);
    return found.length === 0 ? 0 : 1;
  }
  const passing = {
    'nonzero exit': ok(realPassing, 1),
    'the test failed': ok(realFailing, 1),
    'no test ran': ok('testing: warning: no tests to run\nPASS\n', 0),
    'too slow': ok(realPassing.replace(/\(\d+(?:\.\d+)?s\)/, '(41.20s)'), 0),
    'a subtest failed': ok(realPassing.replace('--- PASS:', '    --- FAIL: TestBuildRequestBudgetOff/x (0.00s)\n--- PASS:'), 0),
    'killed': { ...ok(realPassing, null), error: 'killed by SIGKILL' },
  };
  const failing = {
    'the test passed': [ok(realPassing, 0), realFailingLine],
    'zero exit': [ok(realFailing, 0), realFailingLine],
    'another report': [ok(realFailing, 1), 'request-budget name "build_request_budget" in checkpoint.json of the budget-off Build'],
    'no test ran': [ok('testing: warning: no tests to run\nFAIL\n', 1), realFailingLine],
    'a panic': [ok(realFailing.replace('--- FAIL:', 'panic: boom\n--- FAIL:'), 2), realFailingLine],
  };
  const accepted = [
    ...Object.keys(passing).filter((k) => judgePassed(passing[k], 40).length === 0),
    ...Object.keys(failing).filter((k) => judgeFailed(...failing[k]).length === 0),
  ];
  console.log(accepted.length === 0 ? 'rejected' : `not rejected: ${accepted.join(', ')}`);
  return accepted.length === 0 ? 1 : 0;
}

if (require.main === module) {
  process.exitCode = control(process.argv[2]);
} else {
  const { Given, When, Then } = require('@cucumber/cucumber');
  const P = 'k-build-budget-off-compares-behaviour: ';

  Given(`${P}the test binary of package build is compiled with the injection harness in a scratch copy of this checkout`, { timeout: 110 * 1000 }, function () {
    prepared_();
  });

  When(`${P}{word} runs once without injection`, { timeout: 110 * 1000 }, async function (name) {
    if (name !== TEST) throw new Error(`the harness runs ${TEST}, not ${name}`);
    this.kboRun = await runTest('');
  });

  When(`${P}{word} runs once with injection {string}`, { timeout: 110 * 1000 }, async function (name, mode) {
    if (name !== TEST) throw new Error(`the harness runs ${TEST}, not ${name}`);
    this.kboRun = await injected(mode);
  });

  Then(`${P}{word} passed within {int} seconds`, function (name, seconds) {
    report(this.kboRun, judgePassed(this.kboRun, seconds));
  });

  Then(`${P}{word} passed`, function (name) {
    report(this.kboRun, judgePassed(this.kboRun));
  });

  Then(`${P}{word} failed, reporting {string}`, function (name, expected) {
    report(this.kboRun, judgeFailed(this.kboRun, expected));
  });

  Then(`${P}no test file of package build contains any of:`, function (table) {
    const found = [];
    const dir = path.join(root, 'build');
    const tokens = table.raw().map((row) => row[0]);
    for (const file of fs.readdirSync(dir).filter((f) => f.endsWith('_test.go')).sort()) {
      const lines = fs.readFileSync(path.join(dir, file), 'utf8').split('\n');
      lines.forEach((line, i) => {
        for (const t of tokens) if (line.includes(t)) found.push(`build/${file}:${i + 1}: ${t}: ${line.trim()}`);
      });
    }
    if (found.length > 0) throw new Error(`the masking list or a log-rewriting helper remains in package build's tests:\n${found.join('\n')}`);
  });

  const sha256 = (bytes) => require('node:crypto').createHash('sha256').update(bytes).digest('hex');

  // The table rows are `<sha256> | <path>` of every Go file, go.mod and go.sum of the tree this
  // Intent was shaped on (3dcdb86) except the one file the Intent changes.
  Then(`${P}every Go file, go.mod and go.sum except {word} has the SHA-256 of the shaped tree, and no other Go file exists:`, function (allowed, table) {
    const want = new Map(table.raw().map(([hash, file]) => [file, hash]));
    const found = [];
    for (const [file, hash] of want) {
      let bytes;
      try { bytes = fs.readFileSync(path.join(root, file)); } catch { found.push(`${file}: missing`); continue; }
      const got = sha256(bytes);
      if (got !== hash) found.push(`${file}: changed (SHA-256 ${got}, want ${hash})`);
    }
    for (const file of listFiles().map((f) => f.split(path.sep).join('/'))) {
      if (file.endsWith('.go') && file !== allowed && !want.has(file) && fs.existsSync(path.join(root, file))) found.push(`${file}: a Go file the shaped tree does not have`);
    }
    if (found.length > 0) {
      throw new Error(`product code, configuration, timeouts or limits must not change; only ${allowed} may:\n${found.join('\n')}`);
    }
  });

  // definition is the text of func name in body: its one line, or through the first line "}" after it.
  function definition(body, name, where) {
    const at = body.search(new RegExp(`^func ${name}[\\[(]`, 'm'));
    if (at < 0) throw new Error(`${where} does not define func ${name}`);
    const eol = body.indexOf('\n', at);
    if (body.slice(at, eol).trimEnd().endsWith('}')) return body.slice(at, eol);
    const end = body.indexOf('\n}\n', at);
    return body.slice(at, end + 2);
  }

  Then(`${P}these definitions in {word} have the SHA-256 they have in the shaped tree:`, function (file, table) {
    const now = fs.readFileSync(path.join(root, file), 'utf8');
    const changed = [];
    for (const [name, hash] of table.raw()) {
      const got = sha256(definition(now, name, file));
      if (got !== hash) changed.push(`${name} (SHA-256 ${got}, want ${hash})`);
    }
    if (changed.length > 0) throw new Error(`${file}: definitions changed since the shaped tree: ${changed.join(', ')}`);
  });

  Then(`${P}package build's budget test file still defines {word}`, function (name) {
    const body = fs.readFileSync(path.join(root, 'build', 'build_request_budget_test.go'), 'utf8');
    if (!body.includes(`func ${name}(t *testing.T) {`)) throw new Error(`build/build_request_budget_test.go does not define func ${name}(t *testing.T)`);
  });
}
