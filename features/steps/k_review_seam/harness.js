'use strict';
// The k-review-seam acceptance harness. It runs the go command in this checkout, or in a
// temporary copy of it (tracked and untracked, not ignored, files) where it changes code, so no
// file of the checkout changes. The judges decide each run from its exit status and output:
// the kogen-build packages `go list -deps -test` names, the `ok`/`FAIL`/`--- FAIL`/`--- PASS`
// lines of `go test` and the names and paths its output carries. Run directly, it is the
// observer control:
//   node features/steps/k_review_seam/harness.js judge <record.json>
// judges each recorded run of the record (the controls good.json and bad.json hold real outputs
// of these commands, of a `go test -v` output only the lines the judges read; a namesFile is a
// file of this folder with one name per line) and prints one line: it exits 0 when every judge
// accepts its run, 1 when one refuses it, 2 when the harness fails.
const { spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..', '..');
const MODULE = 'github.com/KogenAI/kogen-build';

function copyCheckout() {
  const list = spawnSync('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard'], { cwd: root, encoding: 'utf8', maxBuffer: 64 << 20 });
  if (list.status !== 0) {
    throw new Error(`git ls-files in ${root} failed (exit ${list.status}): ${list.stderr || list.error}`);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-review-seam-'));
  for (const rel of list.stdout.split('\0').filter(Boolean)) {
    const from = path.join(root, rel);
    const st = fs.lstatSync(from, { throwIfNoEntry: false });
    if (!st || !st.isFile()) continue;
    const to = path.join(dir, rel);
    fs.mkdirSync(path.dirname(to), { recursive: true });
    fs.copyFileSync(from, to);
    fs.chmodSync(to, st.mode & 0o777);
  }
  return dir;
}

// goRun runs `go <args>` in dir, with the Node running this harness first on PATH and the
// Cucumber the acceptance command runs (KOGEN_CUCUMBER) or the checkout's own.
function goRun(dir, args) {
  const env = {
    ...process.env, PWD: dir,
    PATH: path.dirname(process.execPath) + path.delimiter + (process.env.PATH || ''),
    KOGEN_CUCUMBER: process.env.KOGEN_CUCUMBER || path.join(root, 'testdata', 'tools', 'cucumber', 'node_modules'),
  };
  const run = spawnSync('go', args, { cwd: dir, env, encoding: 'utf8', maxBuffer: 256 << 20 });
  const output = `${run.stdout || ''}${run.stderr || ''}${run.error ? String(run.error) : ''}`;
  return { command: `go ${args.join(' ')}`, exit: run.status, output };
}

// goFiles are the non-test Go files of dir (relative to root), of its subdirectories too when
// deep.
function goFiles(base, rel, deep) {
  const dir = path.join(base, rel);
  if (!fs.existsSync(dir)) return [];
  const out = [];
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const child = path.join(rel, e.name);
    if (e.isDirectory() && deep && e.name !== 'testdata') out.push(...goFiles(base, child, deep));
    if (e.isFile() && e.name.endsWith('.go') && !e.name.endsWith('_test.go')) out.push(child);
  }
  return out.sort();
}

// packageName is the package clause of the first non-test Go file directly in rel.
function packageName(base, rel) {
  for (const f of goFiles(base, rel, false)) {
    const m = fs.readFileSync(path.join(base, f), 'utf8').match(/^package\s+(\w+)/m);
    if (m) return m[1];
  }
  throw new Error(`${rel} has no non-test Go file with a package clause`);
}

// replaceAll replaces every occurrence of from in the non-test Go files under rel; it returns
// how many it replaced.
function replaceAll(base, rel, from, to) {
  let n = 0;
  for (const f of goFiles(base, rel, true)) {
    const file = path.join(base, f);
    const raw = fs.readFileSync(file, 'utf8');
    const k = raw.split(from).length - 1;
    if (k > 0) {
      fs.writeFileSync(file, raw.split(from).join(to));
      n += k;
    }
  }
  return n;
}

function digest(file) {
  return fs.existsSync(file) ? crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') : 'missing';
}

// kogenPackages are the packages of this module that a `go list` output names, test variants
// ("p [p.test]", "p.test", "p_test") folded into their package.
function kogenPackages(output) {
  const out = new Set();
  for (let line of output.split('\n')) {
    line = line.trim().replace(/\s+\[.*\]$/, '');
    if (line !== MODULE && !line.startsWith(`${MODULE}/`)) continue;
    out.add(line.replace(/\.test$/, '').replace(/_test$/, ''));
  }
  return [...out].sort();
}

function exited(rec, want) {
  const ok = want === 0 ? rec.exit === 0 : rec.exit !== 0 && rec.exit !== null;
  return ok ? [] : [`exit ${rec.exit}, want ${want === 0 ? '0' : 'nonzero'}`];
}

function testFailures(output) {
  return [...output.matchAll(/^\s*--- FAIL: (\S+)/gm)].map((m) => m[1]);
}

function buildFailures(output) {
  return output.split('\n').filter((l) => /\[(build|setup) failed\]/.test(l) || /^# /.test(l));
}

// deps: `go list -deps -test` exits 0 and the kogen-build packages it names are exactly allowed.
function judgeDeps(exp, rec) {
  const got = kogenPackages(rec.output);
  const problems = exited(rec, 0);
  if (rec.exit === 0 && got.join(' ') !== [...exp.allowed].sort().join(' ')) {
    problems.push(`the kogen-build packages are ${got.join(', ') || 'none'}, want exactly ${[...exp.allowed].sort().join(', ')}`);
  }
  return problems;
}

// imports: `go list -f {{join .Imports "\n"}}` exits 0 and names every wanted import.
function judgeImports(exp, rec) {
  const lines = rec.output.split('\n').map((l) => l.trim());
  return [...exited(rec, 0), ...exp.imports.filter((p) => !lines.includes(p)).map((p) => `${p} is not imported`)];
}

// passed: `go test` exits 0, nothing failed, and a test of one of the packages ran (an `ok` line).
function judgePassed(exp, rec) {
  const problems = exited(rec, 0);
  const fails = testFailures(rec.output);
  if (fails.length > 0) problems.push(`${fails.join(', ')} FAIL`);
  if (/^FAIL\b/m.test(rec.output)) problems.push('a package reported FAIL');
  const ran = exp.packages.filter((p) => new RegExp(`^ok\\s+${p.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&')}(/\\S*)?\\s`, 'm').test(rec.output));
  if (ran.length === 0) problems.push(`no test of ${exp.packages.join(' or ')} ran (no ok line)`);
  if (/flag provided but not defined/.test(rec.output)) problems.push('a test binary does not define a flag it was given');
  return problems;
}

// refused: `go test` fails in a test (not in compiling or vetting) whose output names exp.names.
function judgeRefused(exp, rec) {
  const problems = exited(rec, 1);
  const built = buildFailures(rec.output);
  if (built.length > 0) problems.push(`the run failed to build: ${built.slice(0, 3).join(' | ')}`);
  if (testFailures(rec.output).length === 0) problems.push('no test reported --- FAIL');
  problems.push(...exp.names.filter((n) => !rec.output.includes(n)).map((n) => `the output does not name ${n}`));
  return problems;
}

// tests: `go test -v` exits 0 and every named test reports --- PASS.
function judgeTests(exp, rec) {
  const passed = new Set([...rec.output.matchAll(/^\s*--- PASS: (\S+)/gm)].map((m) => m[1]));
  return [...exited(rec, 0), ...exp.tests.filter((t) => !passed.has(t)).map((t) => `${t} did not report PASS`)];
}

// listed: `go test -list` exits 0 and lists every name.
function judgeListed(exp, rec) {
  const lines = new Set(rec.output.split('\n').map((l) => l.trim()));
  const missing = exp.names.filter((n) => !lines.has(n));
  return [...exited(rec, 0), ...(missing.length > 0 ? [`${missing.length} existing test(s) not listed: ${missing.slice(0, 20).join(', ')}`] : [])];
}

// compiled: the go command exits 0 with no diagnostics.
function judgeCompiled(exp, rec) {
  return [...exited(rec, 0), ...(rec.output.trim() !== '' ? [`it printed: ${rec.output.trim().split('\n').slice(0, 10).join(' | ')}`] : [])];
}

const judges = {
  deps: judgeDeps, imports: judgeImports, passed: judgePassed, refused: judgeRefused,
  tests: judgeTests, listed: judgeListed, compiled: judgeCompiled,
};

// relevant are the lines of a go output worth showing on a failure.
function relevant(output) {
  const lines = output.split('\n');
  const keep = lines.filter((l) => /^(\s*--- (PASS|FAIL|SKIP):|ok\s|FAIL\b|PASS$|panic:|# )/.test(l) || /\.go:\d+/.test(l) || /testdata\//.test(l));
  return [...keep.slice(0, 60), '...last lines:', ...lines.slice(-20)].map((l) => (l.length > 400 ? `${l.slice(0, 400)}...` : l));
}

module.exports = { root, MODULE, copyCheckout, goRun, goFiles, packageName, replaceAll, digest, kogenPackages, judges, relevant };

if (require.main === module) {
  const [mode, file] = process.argv.slice(2);
  if (mode !== 'judge' || !file) {
    process.stdout.write('usage: harness.js judge <record.json>\n');
    process.exit(2);
  }
  let record;
  try {
    record = JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch (err) {
    process.stdout.write(`${err.message}\n`);
    process.exit(2);
  }
  const problems = [];
  for (const run of record.runs) {
    const judge = judges[run.judge];
    if (!judge) {
      process.stdout.write(`unknown judge ${run.judge}\n`);
      process.exit(2);
    }
    const expected = { ...run.expected };
    if (expected.namesFile) expected.names = fs.readFileSync(path.join(__dirname, expected.namesFile), 'utf8').split('\n').filter(Boolean);
    const found = judge(expected, run.record);
    if (found.length > 0) problems.push(`${run.name} (${run.record.command}): ${found.join('; ')}`);
  }
  if (problems.length > 0) {
    process.stdout.write(`FAILED: ${problems.join('; ')}\n`);
    process.exit(1);
  }
  process.stdout.write(`ok: ${record.runs.length} runs\n`);
}
