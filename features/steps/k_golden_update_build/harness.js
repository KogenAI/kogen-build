'use strict';
// The k-golden-update-build acceptance harness. It copies this checkout (tracked and
// untracked, not ignored, files) to a temporary folder, inserts drift into goldens there and
// runs the build package's golden tests with `go test`, so no file of the checkout changes.
// The judges decide each run from its exit status, the `--- PASS/FAIL/SKIP: <Test>` lines of
// `go test -v` and the SHA-256 digests of the goldens. Run directly, it is the observer control:
//   node features/steps/k_golden_update_build/harness.js judge <record.json>
// judges a recorded drifted, updated and plain run (the controls good.json and bad.json keep,
// of each real `go test -v` output, its result lines) and prints one line: it exits 0 when
// every judge accepts the record, 1 when one refuses it, 2 when the harness itself fails.
const { spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..', '..');

function copyCheckout() {
  const list = spawnSync('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard'], { cwd: root, encoding: 'utf8', maxBuffer: 64 << 20 });
  if (list.status !== 0) {
    throw new Error(`git ls-files in ${root} failed (exit ${list.status}): ${list.stderr || list.error}`);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-golden-update-build-'));
  for (const rel of list.stdout.split('\0').filter(Boolean)) {
    const from = path.join(root, rel);
    const st = fs.lstatSync(from, { throwIfNoEntry: false });
    if (!st || !(st.isFile() || st.isSymbolicLink())) continue;
    const to = path.join(dir, rel);
    fs.mkdirSync(path.dirname(to), { recursive: true });
    fs.copyFileSync(from, to);
    fs.chmodSync(to, st.mode & 0o777);
  }
  return dir;
}

function digests(dir, goldens) {
  const out = {};
  for (const g of goldens) {
    const file = path.join(dir, g);
    out[g] = fs.existsSync(file) ? crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') : 'missing';
  }
  return out;
}

// insertDrift puts text at the start of the first string value of the JSON key in the golden,
// leaving every other byte of the file as it is.
function insertDrift(dir, golden, key, text) {
  const file = path.join(dir, golden);
  const raw = fs.readFileSync(file, 'utf8');
  const escaped = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const m = new RegExp(`(^|[^\\\\])"${escaped}":\\s*"`).exec(raw);
  if (!m) throw new Error(`${golden} has no "${key}" string to drift`);
  const at = m.index + m[0].length;
  fs.writeFileSync(file, raw.slice(0, at) + text + raw.slice(at));
}

function runFilter(tests) {
  return `^(${tests.join('|')})$`;
}

// goTest runs `go <args>` in dir, with <golden-tests> in args replaced by the -run filter of
// the tests, the Node running this harness first on PATH and the Cucumber the acceptance
// command runs (KOGEN_CUCUMBER) or the checkout's own.
function goTest(dir, args, tests, extraEnv) {
  const argv = args.map((a) => (a === '<golden-tests>' ? runFilter(tests) : a));
  const env = {
    ...process.env, PWD: dir,
    PATH: path.dirname(process.execPath) + path.delimiter + (process.env.PATH || ''),
    KOGEN_CUCUMBER: process.env.KOGEN_CUCUMBER || path.join(root, 'testdata', 'tools', 'cucumber', 'node_modules'),
    ...extraEnv,
  };
  const run = spawnSync('go', argv, { cwd: dir, env, encoding: 'utf8', maxBuffer: 256 << 20 });
  const output = `${run.stdout || ''}${run.stderr || ''}${run.error ? String(run.error) : ''}`;
  const shown = Object.keys(extraEnv || {}).map((k) => `${k}=${extraEnv[k]} `).join('') + ['go', ...argv].join(' ');
  return { command: shown, exit: run.status, output };
}

// results are the top-level `--- RESULT: Name (` lines of a `go test -v` run.
function results(output) {
  const out = {};
  for (const m of output.matchAll(/^--- (PASS|FAIL|SKIP): (\S+) \(/gm)) out[m[2]] = m[1];
  return out;
}

function reported(run, tests, allowed) {
  const r = results(run.output);
  const wrong = tests.filter((t) => !allowed.includes(r[t])).map((t) => `${t} ${r[t] || 'did not run'}`);
  return wrong.length > 0 ? [`${wrong.join(', ')}, want ${allowed.join(' or ')}`] : [];
}

function exited(run, want) {
  const ok = want === 0 ? run.exit === 0 : run.exit !== 0;
  return ok ? [] : [`exit ${run.exit}, want ${want === 0 ? '0' : 'nonzero'}`];
}

function differing(got, want, what) {
  return Object.keys(want).filter((g) => got[g] !== want[g]).map((g) => `${g} ${what} ${got[g]}, want ${want[g]}`);
}

// drifted: without -update the run fails, every golden test fails and no golden is written.
function judgeDrifted(exp, rec) {
  const problems = [...exited(rec, 1), ...reported(rec, exp.tests, ['FAIL'])];
  for (const g of Object.keys(exp.digests)) {
    if (rec.before[g] === exp.digests[g]) problems.push(`${g} was not drifted before the run`);
  }
  problems.push(...differing(rec.after, rec.before, 'changed without -update to'));
  return problems;
}

// updated: with -update the run exits 0, every golden test ran and every golden holds again
// exactly its recorded bytes.
function judgeUpdated(exp, rec) {
  return [...exited(rec, 0), ...reported(rec, exp.tests, ['PASS', 'SKIP']), ...differing(rec.after, exp.digests, 'after -update is')];
}

// plain: the follow-up run without -update exits 0 and every golden test passes.
function judgePlain(exp, rec) {
  return [...exited(rec, 0), ...reported(rec, exp.tests, ['PASS'])];
}

const judges = { drifted: judgeDrifted, updated: judgeUpdated, plain: judgePlain };

module.exports = { copyCheckout, digests, insertDrift, goTest, judges };

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
  for (const phase of ['drifted', 'updated', 'plain']) {
    const found = judges[phase](record.expected, record[phase]);
    if (found.length > 0) problems.push(`${phase} (${record[phase].command}): ${found.join('; ')}`);
  }
  if (problems.length > 0) {
    process.stdout.write(`FAILED: ${problems.join('; ')}\n`);
    process.exit(1);
  }
  process.stdout.write(`ok: ${record.expected.tests.length} golden tests, ${Object.keys(record.expected.digests).length} goldens\n`);
}
