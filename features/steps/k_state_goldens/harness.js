'use strict';
// The k-state-goldens acceptance harness. It copies this checkout (tracked and untracked, not
// ignored, files) to a temporary folder, changes goldens or production code there and runs only
// the build package's tests that are not in existing-tests.txt (the top-level tests of c088534,
// skipped with `go test -skip`), so no file of the checkout changes. The judges decide each run
// from its exit status, the `=== RUN` / `--- FAIL` lines of `go test -v`, the golden paths its
// output names and the SHA-256 digests of the goldens. Run directly, it is the observer control:
//   node features/steps/k_state_goldens/harness.js judge <record.json>
// judges a recorded drifted, updated, format and review run (the controls good.json and
// bad.json keep, of each real `go test -v` output, its relevant lines) and prints one line: it
// exits 0 when every judge accepts the record, 1 when one refuses it, 2 when the harness fails.
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
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-state-goldens-'));
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

// digests are the SHA-256 digests of the goldens in dir ("missing" for an absent one).
function digests(dir, goldens) {
  const out = {};
  for (const g of goldens) {
    const file = path.join(dir, g);
    out[g] = fs.existsSync(file) ? crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') : 'missing';
  }
  return out;
}

// packagePath is a golden's path relative to the build package, as its test names it.
function packagePath(golden) {
  return golden.replace(/^build\//, '');
}

function existingTests() {
  return fs.readFileSync(path.join(__dirname, 'existing-tests.txt'), 'utf8').split('\n').filter(Boolean);
}

// goTest runs `go test ./build <args> -skip <existing tests>` in dir, with the Node running this
// harness first on PATH and the Cucumber the acceptance command runs (KOGEN_CUCUMBER) or the
// checkout's own.
function goTest(dir, args) {
  const skip = `^(${existingTests().join('|')})$`;
  const argv = ['test', './build', ...args, '-skip', skip];
  const env = {
    ...process.env, PWD: dir,
    PATH: path.dirname(process.execPath) + path.delimiter + (process.env.PATH || ''),
    KOGEN_CUCUMBER: process.env.KOGEN_CUCUMBER || path.join(root, 'testdata', 'tools', 'cucumber', 'node_modules'),
  };
  const run = spawnSync('go', argv, { cwd: dir, env, encoding: 'utf8', maxBuffer: 256 << 20 });
  const output = `${run.stdout || ''}${run.stderr || ''}${run.error ? String(run.error) : ''}`;
  return { command: `go test ./build ${args.join(' ')} -skip <existing-tests.txt>`, exit: run.status, output };
}

// relevant are the lines of a `go test -v` output the judges read.
function relevant(output) {
  return output.split('\n').filter((l) => /^(=== RUN|\s*--- (PASS|FAIL|SKIP):|ok\s|FAIL\b|PASS$)/.test(l) || l.includes('testdata/state/'));
}

// insertSpace puts one space after the first "{" of the golden: the same JSON, other bytes.
function insertSpace(dir, golden) {
  const file = path.join(dir, golden);
  const raw = fs.readFileSync(file, 'utf8');
  const at = raw.indexOf('{');
  if (at < 0) throw new Error(`${golden} has no "{"`);
  fs.writeFileSync(file, `${raw.slice(0, at + 1)} ${raw.slice(at + 1)}`);
}

// replaceOnce replaces the only occurrence of from in the copy's file.
function replaceOnce(dir, rel, from, to) {
  const file = path.join(dir, rel);
  const raw = fs.readFileSync(file, 'utf8');
  const n = raw.split(from).length - 1;
  if (n !== 1) throw new Error(`${rel} contains ${JSON.stringify(from)} ${n} times, want once`);
  fs.writeFileSync(file, raw.split(from).join(to));
}

function ran(output) {
  return [...output.matchAll(/^=== RUN\s+(Test[^\s/]*)$/gm)].map((m) => m[1]);
}

function failed(output) {
  return [...output.matchAll(/^--- FAIL: (\S+) \(/gm)].map((m) => m[1]);
}

function exited(rec, want) {
  const ok = want === 0 ? rec.exit === 0 : rec.exit !== 0 && rec.exit !== null;
  return ok ? [] : [`exit ${rec.exit}, want ${want === 0 ? '0' : 'nonzero'}`];
}

function someTestRan(rec) {
  return ran(rec.output).length > 0 ? [] : ['no test outside existing-tests.txt ran'];
}

function failing(rec) {
  return failed(rec.output).length > 0 ? [] : ['no top-level test reported FAIL'];
}

function naming(rec, goldens) {
  return goldens.filter((g) => !rec.output.includes(packagePath(g))).map((g) => `the output does not name ${packagePath(g)}`);
}

function differing(got, want, what) {
  return Object.keys(want).filter((g) => got[g] !== want[g]).map((g) => `${g} ${what} ${got[g]}, want ${want[g]}`);
}

// drifted: without -update the run fails naming every drifted golden, and writes none.
function judgeDrifted(exp, rec) {
  const problems = [...exited(rec, 1), ...someTestRan(rec), ...failing(rec), ...naming(rec, exp.goldens)];
  for (const g of exp.goldens) {
    if (rec.before[g] === exp.digests[g]) problems.push(`${g} was not drifted before the run`);
  }
  problems.push(...differing(rec.after, rec.before, 'changed without -update to'));
  return problems;
}

// updated: with -update the run exits 0, no test fails and every golden holds again exactly
// the bytes of the checkout.
function judgeUpdated(exp, rec) {
  const fails = failed(rec.output);
  return [...exited(rec, 0), ...someTestRan(rec), ...(fails.length > 0 ? [`${fails.join(', ')} FAIL`] : []),
    ...differing(rec.after, exp.digests, 'after -update is')];
}

// changed: with a production format change the run fails naming every golden it changes
// (rec.goldens).
function judgeChanged(exp, rec) {
  return [...exited(rec, 1), ...someTestRan(rec), ...failing(rec), ...naming(rec, rec.goldens)];
}

const judges = { drifted: judgeDrifted, updated: judgeUpdated, changed: judgeChanged };

// The runs of a control record and their judges.
const PHASES = [['drifted', 'drifted'], ['updated', 'updated'], ['format', 'changed'], ['review', 'changed']];

module.exports = { root, copyCheckout, digests, packagePath, goTest, relevant, insertSpace, replaceOnce, judges };

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
  for (const [phase, judge] of PHASES) {
    const found = judges[judge](record.expected, record[phase]);
    if (found.length > 0) problems.push(`${phase} (${record[phase].command}): ${found.join('; ')}`);
  }
  if (problems.length > 0) {
    process.stdout.write(`FAILED: ${problems.join('; ')}\n`);
    process.exit(1);
  }
  process.stdout.write(`ok: ${record.expected.goldens.length} state goldens, ${PHASES.length} runs\n`);
}
