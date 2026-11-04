'use strict';
// Steps of k-build-context-bytes-unpinned: kogen-build's TestContextExternalNativeBytes runs with
// `go test -json` in package build, with the committed go.mod or against a kogen-coordinator
// version taken from the module cache through a temporary go.mod (a directory replace passed with
// -modfile, so no checkout file changes), and the frozen golden is checked by digest. A failure
// lists each required test that did not pass with its own `--- FAIL` line and first message, then
// the command, its exit status, the run's `go test -json` rows (other than output) and the tail of
// what go test printed.
//
// `node features/steps/k-build-context-bytes-unpinned__steps.js good|bad` runs the run judge on
// the real `go test -json` output of TestContextExternalNativeBytes with the pinned coordinator:
// good prints "accepted" and exits 0 when that output is accepted; bad prints "rejected" and exits
// 1 when every refused variant of it (a failed, missing, extra or skipped test, no test run, a
// nonzero exit, no package result) is rejected.
const { spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..');
const coordinator = 'github.com/KogenAI/kogen-coordinator';
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
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-build-context-bytes-unpinned-'));
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

// coordinatorModfile writes, in dir, a copy of the committed go.mod and go.sum that replaces the
// pinned kogen-coordinator with the module cache folder of version, and returns its path.
function coordinatorModfile(version, dir) {
  fs.mkdirSync(dir, { recursive: true });
  const modfile = path.join(dir, 'go.mod');
  fs.copyFileSync(path.join(root, 'go.mod'), modfile);
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(dir, 'go.sum'));
  const dl = JSON.parse(must(run('go', ['mod', 'download', '-json', `${coordinator}@${version}`], { cwd: dir }),
    `locating ${coordinator}@${version}`).stdout);
  if (dl.Version !== version || !dl.Dir || !fs.existsSync(dl.Dir)) {
    throw new Error(`${coordinator}@${version} resolved to ${dl.Version} at ${dl.Dir}${dl.Error ? `: ${dl.Error}` : ''}`);
  }
  must(run('go', ['mod', 'edit', `-replace=${coordinator}=${dl.Dir}`, modfile]), 'writing the replacing go.mod');
  return modfile;
}

// copyCheckout copies the checkout's tracked and untracked, not ignored files to dest.
function copyCheckout(dest) {
  const listed = must(run('git', ['ls-files', '-z', '-co', '--exclude-standard']), 'listing the checkout files').stdout;
  for (const rel of new Set(listed.split('\0').filter(Boolean))) {
    const src = path.join(root, rel);
    let st;
    try {
      st = fs.lstatSync(src);
    } catch {
      continue;
    }
    const dst = path.join(dest, rel);
    fs.mkdirSync(path.dirname(dst), { recursive: true });
    if (st.isSymbolicLink()) {
      fs.symlinkSync(fs.readlinkSync(src), dst);
    } else if (st.isFile()) {
      fs.copyFileSync(src, dst);
      fs.chmodSync(dst, st.mode & 0o777);
    }
  }
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

// goTest runs `go test [-modfile=…] -json -count=1 -run ^name$ ./build [extra…]` in cwd.
function goTest(name, cwd, modfile, against, extra) {
  const args = ['test', ...(modfile ? [`-modfile=${modfile}`] : []), '-json', '-count=1', '-run', `^${name}$`, './build', ...(extra || [])];
  const r = run('go', args, { cwd, timeout: 105 * 1000 });
  const shown = `go test${modfile ? ' -modfile=<temporary go.mod>' : ''} -json -count=1 -run '^${name}$' ./build${extra ? ` ${extra.join(' ')}` : ''}`;
  return parse(r.status, r.stdout, r.stderr, r.error, `${shown} in ${cwd} with ${against}`);
}

const terminal = ['pass', 'fail', 'skip'];

// judge lists why a run does not show name and exactly the given subtests of it passing, with
// go test exiting 0 and package build passing.
function judge(r, name, subtests) {
  const tests = {};
  let pkgAction;
  for (const { e } of r.rows) {
    if (!terminal.includes(e.Action)) continue;
    if (e.Test) tests[e.Test] = e.Action;
    else if (e.Package === pkg) pkgAction = e.Action;
  }
  const found = [];
  const required = [name, ...subtests.map((s) => `${name}/${s}`)];
  for (const t of required) {
    if (tests[t] !== 'pass') found.push({ test: t, text: `${t}: ${tests[t] || 'did not run'}, want pass` });
  }
  for (const t of Object.keys(tests)) {
    if (t.startsWith(`${name}/`) && !required.includes(t)) found.push({ text: `${t}: ${tests[t]}, not a listed subtest` });
  }
  if (r.error) found.push({ text: `go test did not finish: ${r.error}` });
  if (r.status !== 0) found.push({ text: `go test exited ${r.status}, want 0` });
  if (pkgAction !== 'pass') found.push({ text: `package ${pkg}: ${pkgAction || 'no result'}, want pass` });
  return found;
}

// printed is what go test printed for one test (or, with test "", for the package).
function printed(r, test) {
  return r.rows.filter(({ e }) => e.Action === 'output' && (e.Test || '') === test).map(({ e }) => e.Output).join('');
}

// evidence is the test's own `--- PASS/FAIL/SKIP` line and the first file:line message it printed.
function evidence(r, test) {
  const lines = printed(r, test).split('\n').map((l) => l.trim());
  const end = lines.find((l) => /^--- (PASS|FAIL|SKIP): /.test(l));
  const at = lines.findIndex((l) => /^\S+\.go:\d+: /.test(l));
  const shown = [end || 'go test printed no result line for it', ...(at >= 0 ? lines.slice(at, at + 3) : [])];
  return shown.join('; ').slice(0, 900);
}

function report(r, name, subtests) {
  const found = judge(r, name, subtests);
  if (found.length === 0) return;
  const text = found.map((f) => (f.test ? `${f.text} (${evidence(r, f.test)})` : f.text));
  const rows = r.rows.filter(({ e }) => e.Action !== 'output').map(({ line }) => line);
  const out = r.rows.filter(({ e }) => e.Action === 'output').map(({ e }) => e.Output).join('') + r.other;
  throw new Error(`${text.join('\n')}\ncommand: ${r.command} (exit ${r.status})\n` +
    `go test -json rows (without output):\n${rows.join('\n')}\nwhat go test printed (last 8000 bytes):\n${out.slice(-8000)}`);
}

function digest(file) {
  try {
    return crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
  } catch {
    return 'missing';
  }
}

// The real `go test -json -count=1 -run '^TestContextExternalNativeBytes$' ./build` output (stdout,
// one row per line) with the coordinator go.mod pins (v0.0.0-20261103113131-ffca7072fe15) on the
// unchanged checkout; go test exited 0.
const realPassing = [
  '{"Time":"2026-11-04T09:24:36.370194+03:00","Action":"start","Package":"github.com/KogenAI/kogen-build/build"}',
  '{"Time":"2026-11-04T09:24:36.926008+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes"}',
  '{"Time":"2026-11-04T09:24:36.926107+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes","Output":"=== RUN   TestContextExternalNativeBytes\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.926142+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes","Output":"=== PAUSE TestContextExternalNativeBytes\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.926146+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes"}',
  '{"Time":"2026-11-04T09:24:36.926186+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes"}',
  '{"Time":"2026-11-04T09:24:36.926192+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes","Output":"=== CONT  TestContextExternalNativeBytes\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927339+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_without_context"}',
  '{"Time":"2026-11-04T09:24:36.927357+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_without_context","Output":"=== RUN   TestContextExternalNativeBytes/native/native_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927421+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_without_context","Output":"=== PAUSE TestContextExternalNativeBytes/native/native_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927426+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_without_context"}',
  '{"Time":"2026-11-04T09:24:36.927431+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_with_context"}',
  '{"Time":"2026-11-04T09:24:36.927437+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_with_context","Output":"=== RUN   TestContextExternalNativeBytes/native/native_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927441+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_with_context","Output":"=== PAUSE TestContextExternalNativeBytes/native/native_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927444+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_with_context"}',
  '{"Time":"2026-11-04T09:24:36.92745+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_without_context"}',
  '{"Time":"2026-11-04T09:24:36.927453+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_without_context","Output":"=== RUN   TestContextExternalNativeBytes/native/external_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927464+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_without_context","Output":"=== PAUSE TestContextExternalNativeBytes/native/external_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927468+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_without_context"}',
  '{"Time":"2026-11-04T09:24:36.92747+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_with_context"}',
  '{"Time":"2026-11-04T09:24:36.92749+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_with_context","Output":"=== RUN   TestContextExternalNativeBytes/native/external_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927493+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_with_context","Output":"=== PAUSE TestContextExternalNativeBytes/native/external_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927495+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_with_context"}',
  '{"Time":"2026-11-04T09:24:36.927499+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_without_context"}',
  '{"Time":"2026-11-04T09:24:36.927501+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_without_context","Output":"=== RUN   TestContextExternalNativeBytes/external/native_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927504+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_without_context","Output":"=== PAUSE TestContextExternalNativeBytes/external/native_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927506+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_without_context"}',
  '{"Time":"2026-11-04T09:24:36.927508+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_without_context"}',
  '{"Time":"2026-11-04T09:24:36.92751+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_without_context","Output":"=== RUN   TestContextExternalNativeBytes/external/external_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927519+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_without_context","Output":"=== PAUSE TestContextExternalNativeBytes/external/external_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927522+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_without_context"}',
  '{"Time":"2026-11-04T09:24:36.927527+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_with_context"}',
  '{"Time":"2026-11-04T09:24:36.927529+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_with_context","Output":"=== RUN   TestContextExternalNativeBytes/external/native_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927532+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_with_context","Output":"=== PAUSE TestContextExternalNativeBytes/external/native_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927534+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_with_context"}',
  '{"Time":"2026-11-04T09:24:36.927538+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_with_context"}',
  '{"Time":"2026-11-04T09:24:36.927541+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_with_context","Output":"=== RUN   TestContextExternalNativeBytes/external/external_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927549+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_with_context","Output":"=== PAUSE TestContextExternalNativeBytes/external/external_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927552+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_with_context"}',
  '{"Time":"2026-11-04T09:24:36.927556+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_without_context"}',
  '{"Time":"2026-11-04T09:24:36.927558+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_without_context","Output":"=== CONT  TestContextExternalNativeBytes/native/native_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927561+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_without_context"}',
  '{"Time":"2026-11-04T09:24:36.927563+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_without_context","Output":"=== CONT  TestContextExternalNativeBytes/external/native_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927581+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_with_context"}',
  '{"Time":"2026-11-04T09:24:36.927584+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_with_context","Output":"=== CONT  TestContextExternalNativeBytes/external/native_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:36.927744+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_with_context"}',
  '{"Time":"2026-11-04T09:24:36.927776+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_with_context","Output":"=== CONT  TestContextExternalNativeBytes/external/external_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:39.064052+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_with_context","Output":"--- PASS: TestContextExternalNativeBytes/external/native_with_context (2.14s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:39.064079+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_with_context","Elapsed":2.14}',
  '{"Time":"2026-11-04T09:24:39.064105+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_without_context"}',
  '{"Time":"2026-11-04T09:24:39.06411+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_without_context","Output":"=== CONT  TestContextExternalNativeBytes/external/external_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:39.538296+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_without_context","Output":"--- PASS: TestContextExternalNativeBytes/external/native_without_context (2.61s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:39.538321+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/native_without_context","Elapsed":2.61}',
  '{"Time":"2026-11-04T09:24:39.538329+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_without_context"}',
  '{"Time":"2026-11-04T09:24:39.538333+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_without_context","Output":"=== CONT  TestContextExternalNativeBytes/native/external_without_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:39.881388+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_with_context","Output":"--- PASS: TestContextExternalNativeBytes/external/external_with_context (2.95s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:39.881415+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_with_context","Elapsed":2.95}',
  '{"Time":"2026-11-04T09:24:39.881422+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_with_context"}',
  '{"Time":"2026-11-04T09:24:39.881425+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_with_context","Output":"=== CONT  TestContextExternalNativeBytes/native/external_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:40.431368+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_without_context","Output":"--- PASS: TestContextExternalNativeBytes/native/native_without_context (3.50s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:40.431393+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_without_context","Elapsed":3.5}',
  '{"Time":"2026-11-04T09:24:40.431401+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_with_context"}',
  '{"Time":"2026-11-04T09:24:40.431405+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_with_context","Output":"=== CONT  TestContextExternalNativeBytes/native/native_with_context\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:40.761694+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_without_context","Output":"--- PASS: TestContextExternalNativeBytes/external/external_without_context (1.70s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:41.189779+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/external/external_without_context","Elapsed":1.7}',
  '{"Time":"2026-11-04T09:24:41.189806+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_without_context","Output":"--- PASS: TestContextExternalNativeBytes/native/external_without_context (1.65s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:41.696171+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_without_context","Elapsed":1.65}',
  '{"Time":"2026-11-04T09:24:41.696198+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_with_context","Output":"--- PASS: TestContextExternalNativeBytes/native/external_with_context (1.81s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:42.229513+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/external_with_context","Elapsed":1.81}',
  '{"Time":"2026-11-04T09:24:42.229537+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_with_context","Output":"--- PASS: TestContextExternalNativeBytes/native/native_with_context (1.80s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:42.229545+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes/native/native_with_context","Elapsed":1.8}',
  '{"Time":"2026-11-04T09:24:42.229548+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes","Output":"--- PASS: TestContextExternalNativeBytes (0.00s)\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:42.229559+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestContextExternalNativeBytes","Elapsed":0}',
  '{"Time":"2026-11-04T09:24:42.229562+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Output":"PASS\\n","OutputType":"frame"}',
  '{"Time":"2026-11-04T09:24:42.230332+03:00","Action":"output","Package":"github.com/KogenAI/kogen-build/build","Output":"ok  \\tgithub.com/KogenAI/kogen-build/build\\t5.860s\\n"}',
  '{"Time":"2026-11-04T09:24:42.230397+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Elapsed":5.86}',
];

function control(mode) {
  const name = 'TestContextExternalNativeBytes';
  const subtests = ['native/native_without_context', 'native/native_with_context', 'native/external_without_context',
    'native/external_with_context', 'external/native_without_context', 'external/external_without_context',
    'external/native_with_context', 'external/external_with_context'];
  const variant = (status, edit) => parse(status, edit(realPassing.join('\n')) + '\n', '', undefined, 'control');
  const good = variant(0, (s) => s);
  if (mode === 'good') {
    const found = judge(good, name, subtests);
    console.log(found.length === 0 ? 'accepted' : `not accepted: ${found.map((f) => f.text).join('; ')}`);
    return found.length === 0 ? 0 : 1;
  }
  const lines = (s) => s.split('\n');
  const sub = `${name}/native/native_with_context`;
  const isRow = (l, action, test) => l.includes(`"Action":"${action}"`) &&
    (test === undefined ? !l.includes('"Test":') : l.includes(`"Test":"${test}"`));
  const bad = {
    'failed subtest': variant(1, (s) => lines(s).map((l) => {
      if (isRow(l, 'pass', sub) || isRow(l, 'pass', name) || isRow(l, 'pass')) return l.replace('"Action":"pass"', '"Action":"fail"');
      return l;
    }).join('\n')),
    'missing subtest': variant(0, (s) => lines(s).filter((l) => !l.includes(`"Test":"${name}/external/external_with_context"`)).join('\n')),
    'extra subtest': variant(0, (s) => `${s}\n{"Action":"run","Package":"${pkg}","Test":"${name}/native/extra"}\n` +
      `{"Action":"pass","Package":"${pkg}","Test":"${name}/native/extra","Elapsed":0.01}`),
    'skipped test': variant(0, (s) => lines(s).map((l) => (isRow(l, 'pass', name) ? l.replace('"Action":"pass"', '"Action":"skip"') : l)).join('\n')),
    'no test run': variant(0, (s) => lines(s).filter((l) => !l.includes('"Test":')).join('\n')),
    'nonzero exit': variant(1, (s) => s),
    'no package result': variant(0, (s) => lines(s).filter((l) => !isRow(l, 'pass')).join('\n')),
  };
  const accepted = Object.keys(bad).filter((k) => judge(bad[k], name, subtests).length === 0);
  console.log(accepted.length === 0 ? 'rejected' : `not rejected: ${accepted.join(', ')}`);
  return accepted.length === 0 ? 1 : 0;
}

if (require.main === module) {
  process.exitCode = control(process.argv[2]);
} else {
  const { When, Then } = require('@cucumber/cucumber');
  const P = 'k-build-context-bytes-unpinned: ';

  When(`${P}go test runs {word} in package build with the committed go.mod`, { timeout: 115 * 1000 }, function (name) {
    this.kbcbuRun = goTest(name, root, null, 'the kogen-coordinator version the committed go.mod pins');
  });

  When(`${P}go test runs {word} in package build against kogen-coordinator {word}`, { timeout: 115 * 1000 }, function (name, version) {
    const modfile = coordinatorModfile(version, scratch());
    this.kbcbuRun = goTest(name, root, modfile, `${coordinator} ${version} from the module cache`);
  });

  When(`${P}go test -update runs {word} in package build of a scratch copy of the checkout against kogen-coordinator {word}`,
    { timeout: 115 * 1000 }, function (name, version) {
      const dir = scratch();
      const copy = path.join(dir, 'checkout');
      copyCheckout(copy);
      const modfile = coordinatorModfile(version, path.join(dir, 'mod'));
      this.kbcbuCopy = copy;
      this.kbcbuRun = goTest(name, copy, modfile, `${coordinator} ${version} from the module cache`, ['-update']);
    });

  Then(`${P}{word} passed with exactly these subtests passing:`, function (name, table) {
    report(this.kbcbuRun, name, table.hashes().map((row) => row.subtest));
  });

  Then(`${P}go.mod requires {word} at {word} without replacing it`, function (module, version) {
    const mod = JSON.parse(must(run('go', ['mod', 'edit', '-json']), 'reading go.mod').stdout);
    const req = (mod.Require || []).find((r) => r.Path === module);
    if (!req || req.Version !== version) {
      throw new Error(`go.mod requires ${module} ${req ? req.Version : '(nothing)'}, want ${version}`);
    }
    const replaced = (mod.Replace || []).filter((r) => r.Old.Path === module);
    if (replaced.length > 0) throw new Error(`go.mod replaces ${module}: ${JSON.stringify(replaced)}`);
  });

  Then(`${P}the file {word} has SHA-256 {word}`, function (rel, want) {
    const got = digest(path.join(root, rel));
    if (got !== want) throw new Error(`${rel}: sha256 ${got}, want ${want}`);
  });

  Then(`${P}in the scratch copy, the file {word} has SHA-256 {word}`, function (rel, want) {
    const got = digest(path.join(this.kbcbuCopy, rel));
    if (got !== want) {
      let named = '';
      try {
        named = fs.readFileSync(path.join(this.kbcbuCopy, rel), 'utf8').includes('.kogen/models.yaml')
          ? ' (it names .kogen/models.yaml)' : '';
      } catch { /* missing */ }
      throw new Error(`${rel} written by ${this.kbcbuRun.command}: sha256 ${got}${named}, want ${want}`);
    }
  });

  Then(`${P}the file {word} does not contain {string}`, function (rel, text) {
    const body = fs.readFileSync(path.join(root, rel), 'utf8');
    const at = body.indexOf(text);
    if (at >= 0) throw new Error(`${rel} contains ${JSON.stringify(text)} at offset ${at}: ${JSON.stringify(body.slice(Math.max(0, at - 80), at + 80))}`);
  });
}
