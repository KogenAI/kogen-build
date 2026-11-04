'use strict';
// Steps of k-build-budget-off-sweep-ms: kogen-build's TestBuildRequestBudgetOff runs with
// `go test -json` in package build, with the committed go.mod or against a kogen-verification
// version taken from the module cache through a temporary go.mod (a directory replace passed with
// -modfile, so no checkout file changes). The counterexample runs it against a scratch copy of
// that version whose step_sweep events carry a `stopped` that counts up across the test process,
// so the two Builds of the test differ in a non-timing field. A failure names the test, what it
// did in each run, the command and its exit status, the run's `go test -json` rows (other than
// output) and the tail of what go test printed.
//
// `node features/steps/k-build-budget-off-sweep-ms__steps.js good|bad` runs the two run judges on
// real `go test -json` output of TestBuildRequestBudgetOff: good prints "accepted" and exits 0 when
// the passing run (-count=5, committed go.mod) and the comparison failure (counterexample producer)
// are accepted; bad prints "rejected" and exits 1 when every refused variant of them is rejected.
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..');
const verification = 'github.com/KogenAI/kogen-verification';
const pkg = 'github.com/KogenAI/kogen-build/build';

function run(cmd, args, opts) {
  const r = spawnSync(cmd, args, {
    cwd: root, env: { ...process.env, GOWORK: 'off' }, encoding: 'utf8', maxBuffer: 256 << 20, ...opts,
  });
  return {
    status: r.status, signal: r.signal, stdout: r.stdout || '', stderr: r.stderr || '', error: r.error,
    command: `${cmd} ${args.join(' ')}`,
  };
}

function must(r, what) {
  if (r.status !== 0) {
    throw new Error(`${what}: ${r.command} exited ${r.status}${r.error ? ` (${r.error})` : ''}:\n${r.stdout}${r.stderr}`);
  }
  return r;
}

// scratch makes a temporary folder that is removed when the process exits.
const scratchDirs = [];
function scratch() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-build-budget-off-sweep-ms-'));
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

// verificationModfile writes, in dir, a copy of the committed go.mod and go.sum that replaces the
// pinned kogen-verification with the module cache folder of version (or, with perturb, with a
// copy of that folder changed by perturb), and returns its path.
function verificationModfile(version, dir, perturb) {
  fs.mkdirSync(dir, { recursive: true });
  const modfile = path.join(dir, 'go.mod');
  fs.copyFileSync(path.join(root, 'go.mod'), modfile);
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(dir, 'go.sum'));
  const dl = JSON.parse(must(run('go', ['mod', 'download', '-json', `${verification}@${version}`], { cwd: dir }),
    `locating ${verification}@${version}`).stdout);
  if (dl.Version !== version || !dl.Dir || !fs.existsSync(dl.Dir)) {
    throw new Error(`${verification}@${version} resolved to ${dl.Version} at ${dl.Dir}${dl.Error ? `: ${dl.Error}` : ''}`);
  }
  let target = dl.Dir;
  if (perturb) {
    target = path.join(dir, 'kogen-verification');
    must(run('cp', ['-R', dl.Dir, target]), `copying ${verification}@${version}`);
    must(run('chmod', ['-R', 'u+w', target]), 'making the copy writable');
    perturb(target);
  }
  must(run('go', ['mod', 'edit', `-replace=${verification}=${target}`, modfile]), 'writing the replacing go.mod');
  return modfile;
}

// countingStopped changes a kogen-verification copy so that the `stopped` of every step_sweep
// event is the real count plus a process-wide sequence number: two Builds in one test process
// then log different `stopped` values, a non-timing difference.
function countingStopped(dir) {
  const from = '"stopped": stopped,';
  const to = '"stopped": stopped + kBudgetOffSweepSeq(),';
  for (const rel of ['verify/env.go', 'verify/concurrent.go']) {
    const file = path.join(dir, rel);
    const body = fs.readFileSync(file, 'utf8');
    const n = body.split(from).length - 1;
    if (n !== 1) throw new Error(`${rel} of the kogen-verification copy has ${n} occurrences of ${JSON.stringify(from)}, want 1`);
    fs.writeFileSync(file, body.replace(from, to));
  }
  fs.writeFileSync(path.join(dir, 'verify', 'k_build_budget_off_sweep_ms_seq.go'), [
    'package verify', '', 'import "sync/atomic"', '', 'var kBudgetOffSweepCount atomic.Int64', '',
    'func kBudgetOffSweepSeq() int { return int(kBudgetOffSweepCount.Add(1)) }', '',
  ].join('\n'));
}

// parse reads `go test -json` stdout: each JSON row with its original line, and other lines.
function parse(status, stdout, stderr, error, command) {
  const rows = [];
  const other = [];
  for (const line of stdout.split('\n')) {
    if (line.trim() === '') continue;
    try {
      const e = JSON.parse(line);
      if (e && typeof e === 'object' && e.Action) {
        rows.push({ e, line });
        continue;
      }
    } catch { /* not a row */ }
    other.push(line);
  }
  return { status, rows, other: other.join('\n') + stderr, error, command };
}

// goTest runs `go test [-modfile=…] -json -count=n -run ^name$ ./build` in the checkout.
function goTest(name, count, modfile, against) {
  const args = ['test', ...(modfile ? [`-modfile=${modfile}`] : []), '-json', `-count=${count}`, '-run', `^${name}$`, './build'];
  const r = run('go', args, { timeout: 100 * 1000 });
  const shown = `go test${modfile ? ' -modfile=<temporary go.mod>' : ''} -json -count=${count} -run '^${name}$' ./build`;
  return parse(r.status, r.stdout, r.stderr, r.error, `${shown} in ${root} with ${against}`);
}

const terminal = ['pass', 'fail', 'skip'];

// outcomes is what each run of name ended with, in order, and the package result.
function outcomes(r, name) {
  const ends = [];
  const others = [];
  let pkgAction;
  for (const { e } of r.rows) {
    if (!terminal.includes(e.Action)) continue;
    if (e.Test === name) ends.push(e.Action);
    else if (e.Test) others.push(`${e.Test}: ${e.Action}`);
    else if (e.Package === pkg) pkgAction = e.Action;
  }
  return { ends, others, pkgAction };
}

// printed is what go test printed for name.
function printed(r, name) {
  return r.rows.filter(({ e }) => e.Action === 'output' && e.Test === name).map(({ e }) => e.Output).join('');
}

// judgePassed lists why a run does not show name passing in each of count runs, with nothing else
// run, go test exiting 0 and package build passing.
function judgePassed(r, name, count) {
  const { ends, others, pkgAction } = outcomes(r, name);
  const found = [];
  const passes = ends.filter((a) => a === 'pass').length;
  if (ends.length === 0) found.push(`${name}: did not run (no result row for it), want ${count} passing runs`);
  else if (passes !== count || ends.length !== count) found.push(`${name}: runs ended ${ends.join(', ')}, want pass in all ${count} runs`);
  for (const o of others) found.push(`${o}: another test ran`);
  if (r.error) found.push(`go test did not finish: ${r.error}`);
  if (r.status !== 0) found.push(`go test exited ${r.status}, want 0`);
  if (pkgAction !== 'pass') found.push(`package ${pkg}: ${pkgAction || 'no result'}, want pass`);
  return found;
}

// judgeComparisonFailed lists why a run does not show name failing in its one run on its
// comparison of the two Builds: a reported differing line, which shows field.
function judgeComparisonFailed(r, name, field) {
  const { ends, others, pkgAction } = outcomes(r, name);
  const found = [];
  if (ends.length === 0) found.push(`${name}: did not run (no result row for it), want one failing run`);
  else if (ends.length !== 1 || ends[0] !== 'fail') found.push(`${name}: runs ended ${ends.join(', ')}, want one run that fails`);
  const out = printed(r, name);
  if (ends.includes('fail')) {
    if (!/ differs at byte \d+/.test(out)) found.push(`${name}: failed without reporting a differing line ("differs at byte")`);
    if (!out.includes(`"${field}":`)) found.push(`${name}: its failure does not show "${field}"`);
  }
  for (const o of others) found.push(`${o}: another test ran`);
  if (r.error) found.push(`go test did not finish: ${r.error}`);
  if (r.status === 0) found.push('go test exited 0, want a failure');
  if (pkgAction !== 'fail') found.push(`package ${pkg}: ${pkgAction || 'no result'}, want fail`);
  return found;
}

function report(r, name, found) {
  if (found.length === 0) return;
  const rows = r.rows.filter(({ e }) => e.Action !== 'output').map(({ line }) => line);
  const out = r.rows.filter(({ e }) => e.Action === 'output').map(({ e }) => e.Output).join('') + r.other;
  throw new Error(`${found.join('\n')}\ntest: ${name}\ncommand: ${r.command} (exit ${r.status})\n` +
    `go test -json rows (without output):\n${rows.join('\n')}\nwhat go test printed (last 8000 bytes):\n${out.slice(-8000)}`);
}

// The real `go test -json -count=5 -run '^TestBuildRequestBudgetOff$' ./build` output (stdout, one
// row per line) with the committed go.mod (kogen-verification v0.0.0-20261103020618-01aa16c01767)
// on the unchanged checkout; go test exited 0.
const realPassing = [
  '{"Time":"2026-11-04T17:49:15.451915+03:00","Action":"start","Package":"github.com/KogenAI/kogen-build/build"}',
  '{"Time":"2026-11-04T17:49:15.76203+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:15.762142+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== RUN   TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:15.762222+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== PAUSE TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:15.762233+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:15.76225+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:15.762258+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== CONT  TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:21.074181+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"--- PASS: TestBuildRequestBudgetOff (5.31s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:21.074212+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Elapsed":5.31}',
  '{"Time":"2026-11-04T17:49:21.074231+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:21.074235+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== RUN   TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:21.074241+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== PAUSE TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:21.074245+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:21.074248+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:21.074251+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== CONT  TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:26.176495+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"--- PASS: TestBuildRequestBudgetOff (5.10s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:26.176521+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Elapsed":5.1}',
  '{"Time":"2026-11-04T17:49:26.176527+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:26.176531+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== RUN   TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:26.176535+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== PAUSE TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:26.176538+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:26.176542+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:26.176544+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== CONT  TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:31.303521+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"--- PASS: TestBuildRequestBudgetOff (5.13s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:31.303551+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Elapsed":5.13}',
  '{"Time":"2026-11-04T17:49:31.303557+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:31.30356+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== RUN   TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:31.303567+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== PAUSE TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:31.30357+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:31.303578+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:31.303581+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== CONT  TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:36.402007+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"--- PASS: TestBuildRequestBudgetOff (5.10s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:36.402033+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Elapsed":5.1}',
  '{"Time":"2026-11-04T17:49:36.402039+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:36.402042+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== RUN   TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:36.402045+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== PAUSE TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:36.402048+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:36.402051+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:49:36.402054+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== CONT  TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:41.455361+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"--- PASS: TestBuildRequestBudgetOff (5.05s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:41.455383+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Elapsed":5.05}',
  '{"Time":"2026-11-04T17:49:41.455388+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Output":"PASS\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:49:41.456055+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Output":"ok  \\tgithub.com/KogenAI/kogen-build/build\\t26.004s\\n"}',
  '{"Time":"2026-11-04T17:49:41.456071+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Elapsed":26.004}',
];

// The real `go test -json -count=1 -run '^TestBuildRequestBudgetOff$' ./build` output against the
// counterexample producer (kogen-verification v0.0.0-20261103201747-3cb7305a5690 changed by
// countingStopped) on the unchanged checkout; go test exited 1.
const realComparisonFailure = [
  '{"Time":"2026-11-04T17:50:22.75023+03:00","Action":"start","Package":"github.com/KogenAI/kogen-build/build"}',
  '{"Time":"2026-11-04T17:50:23.059564+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:50:23.059682+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== RUN   TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:50:23.059765+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== PAUSE TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:50:23.059776+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:50:23.059812+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff"}',
  '{"Time":"2026-11-04T17:50:23.05984+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"=== CONT  TestBuildRequestBudgetOff\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:50:28.285154+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"    build_request_budget_test.go:335: log line 24 differs at byte 45:\\n","OutputType":"error"}',
  '{"Time":"2026-11-04T17:50:28.285188+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"        omitted: ...{\\"at\\":\\"\\u003cvolatile\\u003e\\",\\"event\\":\\"step_sweep\\",\\"ms\\":28,\\"name\\":\\"acceptance\\",\\"stopped\\":1}\\n","OutputType":"error-continue"}',
  '{"Time":"2026-11-04T17:50:28.285213+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"        zero:    ...{\\"at\\":\\"\\u003cvolatile\\u003e\\",\\"event\\":\\"step_sweep\\",\\"ms\\":30,\\"name\\":\\"acceptance\\",\\"stopped\\":4}\\n","OutputType":"error-continue"}',
  '{"Time":"2026-11-04T17:50:28.294286+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Output":"--- FAIL: TestBuildRequestBudgetOff (5.23s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:50:28.294308+03:00","Action":"fail","Package":"github.com/KogenAI/kogen-build/build","Test":"TestBuildRequestBudgetOff","Elapsed":5.23}',
  '{"Time":"2026-11-04T17:50:28.294327+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Output":"FAIL\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:50:28.294974+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Output":"FAIL\\tgithub.com/KogenAI/kogen-build/build\\t5.544s\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T17:50:28.294986+03:00","Action":"fail","Package":"github.com/KogenAI/kogen-build/build","Elapsed":5.545}',
];

function control(mode) {
  const name = 'TestBuildRequestBudgetOff';
  const variant = (rows, status, edit) => parse(status, edit(rows.join('\n')) + '\n', '', undefined, 'control');
  const same = (s) => s;
  if (mode === 'good') {
    const found = [...judgePassed(variant(realPassing, 0, same), name, 5),
      ...judgeComparisonFailed(variant(realComparisonFailure, 1, same), name, 'stopped')];
    console.log(found.length === 0 ? 'accepted' : `not accepted: ${found.join('; ')}`);
    return found.length === 0 ? 0 : 1;
  }
  const lines = (s) => s.split('\n');
  const testRow = (l, action) => l.includes(`"Action":"${action}"`) && l.includes(`"Test":"${name}"`);
  const pkgRow = (l, action) => l.includes(`"Action":"${action}"`) && !l.includes('"Test":');
  let seen = 0;
  const third = (l) => testRow(l, 'pass') && ++seen === 3;
  const passing = {
    'one run failed': variant(realPassing, 1, (s) => lines(s).map((l) => (third(l) || pkgRow(l, 'pass')
      ? l.replace('"Action":"pass"', '"Action":"fail"') : l)).join('\n')),
    'four runs': variant(realPassing, 0, (s) => {
      const ls = lines(s);
      const last = ls.map((l) => testRow(l, 'pass')).lastIndexOf(true);
      return ls.filter((_, i) => i !== last).join('\n');
    }),
    'a skipped run': variant(realPassing, 0, (s) => lines(s).map((l, i, all) => (testRow(l, 'pass') && all.slice(0, i).some((p) => testRow(p, 'pass'))
      && !all.slice(i + 1).some((p) => testRow(p, 'pass')) ? l.replace('"Action":"pass"', '"Action":"skip"') : l)).join('\n')),
    'nonzero exit': variant(realPassing, 1, same),
    'no package result': variant(realPassing, 0, (s) => lines(s).filter((l) => !pkgRow(l, 'pass')).join('\n')),
    'no test run': variant(realPassing, 0, (s) => lines(s).filter((l) => !l.includes('"Test":')).join('\n')),
  };
  const failing = {
    'the test passed': variant(realPassing, 0, same),
    'a failure without a differing line': variant(realComparisonFailure, 1, (s) => lines(s).map((l) => (l.includes('differs at byte')
      ? l.replace(/log line 24 differs at byte 45:/, 'exit 1: verification refused') : l))
      .filter((l) => !l.includes('omitted:') && !l.includes('zero:')).join('\n')),
    'a difference that does not show the field': variant(realComparisonFailure, 1, (s) => s.split('stopped').join('count')),
    'not built': variant(realComparisonFailure, 1, (s) => lines(s).filter((l) => !l.includes('"Test":')).join('\n')),
    'zero exit': variant(realComparisonFailure, 0, same),
  };
  const accepted = [
    ...Object.keys(passing).filter((k) => judgePassed(passing[k], name, 5).length === 0),
    ...Object.keys(failing).filter((k) => judgeComparisonFailed(failing[k], name, 'stopped').length === 0),
  ];
  console.log(accepted.length === 0 ? 'rejected' : `not rejected: ${accepted.join(', ')}`);
  return accepted.length === 0 ? 1 : 0;
}

if (require.main === module) {
  process.exitCode = control(process.argv[2]);
} else {
  const { Given, When, Then } = require('@cucumber/cucumber');
  const P = 'k-build-budget-off-sweep-ms: ';

  Given(`${P}go.mod requires {word} at {word} without replacing it`, function (module, version) {
    const mod = JSON.parse(must(run('go', ['mod', 'edit', '-json']), 'reading go.mod').stdout);
    const req = (mod.Require || []).find((r) => r.Path === module);
    if (!req || req.Version !== version) {
      throw new Error(`go.mod requires ${module} ${req ? req.Version : '(nothing)'}, want ${version}`);
    }
    const replaced = (mod.Replace || []).filter((r) => r.Old.Path === module);
    if (replaced.length > 0) throw new Error(`go.mod replaces ${module}: ${JSON.stringify(replaced)}`);
  });

  When(`${P}go test -count={int} runs {word} in package build with the committed go.mod`, { timeout: 110 * 1000 }, function (count, name) {
    this.kbosRun = goTest(name, count, null, `the kogen-verification version the committed go.mod pins`);
  });

  When(`${P}go test -count={int} runs {word} in package build against kogen-verification {word}`, { timeout: 110 * 1000 }, function (count, name, version) {
    const modfile = verificationModfile(version, scratch());
    this.kbosRun = goTest(name, count, modfile, `${verification} ${version} from the module cache`);
  });

  When(`${P}go test -count={int} runs {word} in package build against kogen-verification {word} changed so that every step_sweep event's stopped counts up across the test process`,
    { timeout: 110 * 1000 }, function (count, name, version) {
      const modfile = verificationModfile(version, scratch(), countingStopped);
      this.kbosRun = goTest(name, count, modfile, `a copy of ${verification} ${version} whose step_sweep stopped counts up across the process`);
    });

  Then(`${P}{word} passed in all {int} runs and failed in none`, function (name, count) {
    report(this.kbosRun, name, judgePassed(this.kbosRun, name, count));
  });

  Then(`${P}{word} failed in its one run on its comparison, reporting a differing line that shows {string}`, function (name, field) {
    report(this.kbosRun, name, judgeComparisonFailed(this.kbosRun, name, field));
  });
}
