// Observer of b-consumes-k-coord-intent-not-found-names-next-c: runs named Go tests of one package of this
// module with the real go tool, from the checkout's real path and with the go.mod and go.sum the
// checkout holds (no -modfile, no replace), and judges the go test -json events. It also reads
// which kogen-coordinator that go.mod makes the build use. It runs the go binary of
// `go env GOROOT` directly, so a host wrapper named go adds no waiting. No checkout file changes.
// As a command it is the observer's own control, on real go test -json output of a temporary
// module outside the checkout (a passing test with a subtest, a failing, a skipping and a
// missing test):
//   node b-consumes-k-coord-intent-not-found-names-next-c__control.js good   accepts the passing run:
//                                                                    prints "accepted", exits 0
//   node b-consumes-k-coord-intent-not-found-names-next-c__control.js bad    rejects each of the failing,
//                                                                    skipping and missing runs:
//                                                                    prints "rejected", exits 1
// A judgment the control does not get prints "not accepted: …" or "not rejected: …" and exits 0
// for bad, 1 for good.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

const COORDINATOR = 'github.com/KogenAI/kogen-coordinator';

// goBinary is GOROOT's go, or plain go when GOROOT cannot be read.
function goBinary() {
  const r = spawnSync('go', ['env', 'GOROOT'], { encoding: 'utf8' });
  const bin = r.status === 0 ? path.join(r.stdout.trim(), 'bin', 'go') : '';
  return bin && fs.existsSync(bin) ? bin : 'go';
}

function realRoot() {
  return fs.realpathSync(process.cwd());
}

// goEnv is the environment of every go command: the caller's, without a workspace, with only
// -trimpath as GOFLAGS (as the configured go-test check), so the module's own go.mod decides.
function goEnv() {
  return { ...process.env, GOWORK: 'off', GOFLAGS: '-trimpath' };
}

// coordinator reports the kogen-coordinator module the go.mod of root makes the build use: its
// version and its replacement (empty when there is none), and the go list command and outcome.
function coordinator(root) {
  const go = goBinary();
  const args = ['list', '-m', '-f', '{{.Version}}|{{with .Replace}}{{.Path}} {{.Version}}{{end}}', COORDINATOR];
  const r = spawnSync(go, args, { cwd: root, encoding: 'utf8', env: goEnv() });
  const [version = '', replace = ''] = (r.stdout || '').trim().split('|');
  return { version, replace, status: r.status, command: `go ${args.join(' ')}`, output: (r.stdout || '') + (r.stderr || '') };
}

// runGo runs `go test -count=1 -json -run ^(tests)$ <pkg>` in root and resolves to the run: its
// command, exit status, top-level test results, outputs and own go test -json rows (run and
// result), and the rest of its output.
function runGo(root, pkg, tests) {
  const go = goBinary();
  const args = ['test', '-count=1', '-json', '-run', '^(' + tests.join('|') + ')$', pkg];
  const command = `GOFLAGS=-trimpath go ${args.join(' ')}`;
  return new Promise((resolve) => {
    const child = spawn(go, args, { cwd: root, env: goEnv() });
    let stdout = '';
    let stderr = '';
    let done = false;
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr += d));
    child.on('error', (err) => {
      stderr += `could not run go: ${err.message}\n`;
      finish(null);
    });
    child.on('close', (status) => finish(status));
    function finish(status) {
      if (done) return;
      done = true;
      const results = {};
      const outputs = {};
      const rows = {};
      let raw = '';
      for (const line of stdout.split('\n')) {
        if (!line.startsWith('{')) {
          raw += line ? line + '\n' : '';
          continue;
        }
        let ev;
        try {
          ev = JSON.parse(line);
        } catch {
          raw += line + '\n';
          continue;
        }
        if (!ev.Test) {
          if (ev.Action === 'output' || ev.Action === 'build-output') raw += ev.Output || '';
          continue;
        }
        const top = ev.Test.split('/')[0];
        if (ev.Test === top && ['run', 'pass', 'fail', 'skip'].includes(ev.Action)) (rows[top] = rows[top] || []).push(line);
        if (ev.Action === 'output') outputs[top] = (outputs[top] || '') + ev.Output;
        if (['pass', 'fail', 'skip'].includes(ev.Action) && ev.Test === top) results[top] = ev.Action;
        if (ev.Action === 'skip' && ev.Test !== top) results[ev.Test] = 'skip';
      }
      resolve({ command, status, results, outputs, rows, raw, stderr });
    }
  });
}

// passProblem is '' when every named test passed and no test of the run failed or was skipped,
// otherwise the reason, naming the tests and the command.
function passProblem(run, tests) {
  const missing = tests.filter((t) => !(t in run.results));
  const failed = Object.keys(run.results).filter((t) => run.results[t] === 'fail');
  const skipped = Object.keys(run.results).filter((t) => run.results[t] === 'skip');
  const problems = [];
  if (missing.length) problems.push(`did not run: ${missing.join(', ')}`);
  if (failed.length) problems.push(`failed: ${failed.join(', ')}`);
  if (skipped.length) problems.push(`skipped: ${skipped.join(', ')}`);
  if (problems.length === 0 && run.status !== 0) problems.push(`go test exited ${run.status}`);
  return problems.length ? `${problems.join('; ')} (${run.command}, exit ${run.status})` : '';
}

// clip shortens text for a failure message: at most 40 lines, each cut to 300 characters.
function clip(text) {
  const lines = text.split('\n');
  const kept = lines.slice(0, 40).map((l) => (l.length > 300 ? `${l.slice(0, 300)}... (${l.length} characters)` : l));
  if (lines.length > 40) kept.push(`... (${lines.length - 40} more lines)`);
  return kept.join('\n');
}

// ranLines is, for each named test, what go test reported for it: its own top-level go test
// -json rows (its run row and its pass, fail or skip row), or that it did not run.
function ranLines(run, tests) {
  return tests.map((t) => (run.rows[t] ? run.rows[t].join('\n') : `${t}: did not run (no go test -json row for it in ${run.command})`)).join('\n');
}

// details is, first, what go test reported for each named test (ranLines), then the clipped
// output of the run's failing, skipped or missing tests, or its clipped raw output.
function details(run, tests) {
  const names = new Set([...tests.filter((t) => run.results[t] !== 'pass'), ...Object.keys(run.results).filter((t) => run.results[t] !== 'pass')]);
  const parts = [ranLines(run, tests)];
  for (const t of names) if (run.outputs[t]) parts.push(clip(run.outputs[t]));
  if (parts.length === 1) parts.push(clip(run.raw + run.stderr));
  return parts.join('\n');
}

module.exports = { runGo, passProblem, details, ranLines, clip, realRoot, coordinator, COORDINATOR };

// The control module: one package with a passing test (with a subtest), a failing and a skipping
// test, written to a temporary folder outside the checkout.
const CONTROL_TEST = `package control

import "testing"

func TestPasses(t *testing.T) {
	t.Run("child", func(t *testing.T) {})
}

func TestFails(t *testing.T) { t.Fatal("this control test fails") }

func TestSkips(t *testing.T) { t.Skip("this control test skips") }
`;

async function main(mode) {
  if (mode !== 'good' && mode !== 'bad') {
    console.log('usage: good|bad');
    return 2;
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'bcinf-control-'));
  try {
    fs.writeFileSync(path.join(dir, 'go.mod'), 'module bcinfcontrol\n\ngo 1.27.2\n');
    fs.writeFileSync(path.join(dir, 'control_test.go'), CONTROL_TEST);
    if (mode === 'good') {
      const run = await runGo(dir, '.', ['TestPasses']);
      const p = passProblem(run, ['TestPasses']);
      if (p !== '' || run.results['TestPasses'] !== 'pass') {
        console.log(`not accepted: TestPasses: ${p || 'no pass row'}`);
        return 1;
      }
      console.log('accepted');
      return 0;
    }
    const cases = [['TestPasses', 'TestFails'], ['TestPasses', 'TestSkips'], ['TestPasses', 'TestMissing']];
    for (const tests of cases) {
      const run = await runGo(dir, '.', tests);
      if (passProblem(run, tests) === '') {
        console.log(`not rejected: ${tests.join(', ')}`);
        return 0;
      }
    }
    console.log('rejected');
    return 1;
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

if (require.main === module) {
  main(process.argv[2]).then(
    (code) => process.exit(code),
    (err) => {
      console.log(`error: ${err.message}`);
      process.exit(2);
    },
  );
}
