// Runner and judges of k-silent-shell-test-fast, shared by its steps and run directly as the
// Feasibility observer control:
//
//   node features/steps/k-silent-shell-test-fast__control.js <passed|failed> <stop|waits> <TestName>
//
// runs the named Go test of ./build once against a temporary copy of the pinned kogen-loop with one
// mutation of its shell wait (tools/tools.go) and prints one verdict line: "<passed|failed>: <TestName>"
// with exit 0 when the test had that outcome, otherwise "rejected: ..." with exit 1. The checkout is
// not changed: the mutated copy is used through a temporary go.mod (-modfile). Requiring this file (as
// cucumber does with every features/steps/*.js) runs nothing.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const LOOP = 'github.com/KogenAI/kogen-loop';

// MUTATIONS are the edits of the copied kogen-loop's tools/tools.go; each text must occur exactly once.
const MUTATIONS = {
  // stop: when the shell wait elapses, the loop stops the command instead of yielding it.
  stop: [['\tcase <-elapsed:\n', '\tcase <-elapsed:\n\t\tj.cancel()\n\t\t<-j.cmd.Done()\n']],
  // waits: a new command waits 90 seconds and a retrieval 60, instead of 45 and 30.
  waits: [
    ['wait := 45 * time.Second', 'wait := 90 * time.Second'],
    ['wait = 30 * time.Second', 'wait = 60 * time.Second'],
  ],
};

// goEnv is this process's environment without its KOGEN_ variables, outside any Go workspace.
function goEnv() {
  const env = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith('KOGEN_')) env[k] = v;
  env.GOWORK = 'off';
  return env;
}

function go(root, args) {
  const r = spawnSync('go', args, { cwd: root, encoding: 'utf8', timeout: 540000, maxBuffer: 64 * 1024 * 1024, env: goEnv() });
  if (r.error) throw new Error(`go ${args.join(' ')}: ${r.error.message}`);
  return r;
}

// goTest runs the named tests of ./build once each, one at a time, verbosely; modfile, when given,
// is the go.mod to use instead of the checkout's.
function goTest(root, names, modfile) {
  const args = ['test', '-count=1', '-p', '1', '-parallel', '1', '-timeout', '240s', '-v'];
  if (modfile) args.push('-modfile=' + modfile);
  args.push('-run', `^(${names.join('|')})$`, './build');
  const r = go(root, args);
  return { command: `go ${args.join(' ')}`, status: r.status, output: `${r.stdout}${r.stderr}` };
}

// outcomes maps each test the run reported to its verdict (PASS, FAIL or SKIP) and its seconds.
function outcomes(run) {
  const out = {};
  for (const m of run.output.matchAll(/^\s*--- (PASS|FAIL|SKIP): (\S+) \((\d+(?:\.\d+)?)s\)$/gm)) {
    out[m[2]] = { verdict: m[1], seconds: Number(m[3]) };
  }
  return out;
}

// verdictOf is name's verdict in run, or "no result line" when the run did not report it.
function verdictOf(run, name) {
  const o = outcomes(run)[name];
  return o ? o.verdict : 'no result line';
}

function tail(text) {
  return text.length > 6000 ? `...${text.slice(-6000)}` : text;
}

function report(run, name, want) {
  const o = outcomes(run)[name];
  const got = o
    ? `reported ${o.verdict} after ${o.seconds}s`
    : `has no "--- PASS/FAIL/SKIP: ${name}" line (absent, filtered out, or the package did not build)`;
  return `${name} was expected to ${want} but ${got}; ${run.command} exited ${run.status}\n${tail(run.output)}`;
}

// judgePassed is null when name passed in run, otherwise why not.
function judgePassed(run, name) {
  return verdictOf(run, name) === 'PASS' ? null : report(run, name, 'pass');
}

// judgeFailed is null when name failed in run and the run failed, otherwise why not.
function judgeFailed(run, name) {
  return verdictOf(run, name) === 'FAIL' && run.status !== 0 ? null : report(run, name, 'fail');
}

// makeWritable gives the owner write access below dir (the module cache's files are read-only).
function makeWritable(dir) {
  fs.chmodSync(dir, 0o755);
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) makeWritable(p);
    else if (e.isFile()) fs.chmodSync(p, 0o644);
  }
}

function removeTree(dir) {
  if (fs.existsSync(dir)) {
    makeWritable(dir);
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

// mutatedModfile copies the pinned kogen-loop (go list -m) and the checkout's go.mod and go.sum to
// a new temporary directory, applies the mutation to the copy and points the copied go.mod at it.
function mutatedModfile(root, kind) {
  const edits = MUTATIONS[kind];
  if (!edits) throw new Error(`unknown kogen-loop mutation ${JSON.stringify(kind)}; want one of ${Object.keys(MUTATIONS).join(', ')}`);
  const list = go(root, ['list', '-m', '-f', '{{.Dir}}', LOOP]);
  const pinned = list.stdout.trim();
  if (list.status !== 0 || !pinned) throw new Error(`go list -m -f {{.Dir}} ${LOOP} exited ${list.status}: ${list.stdout}${list.stderr}`);
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-silent-shell-test-fast-'));
  try {
    const loop = path.join(dir, 'kogen-loop');
    fs.cpSync(pinned, loop, { recursive: true });
    makeWritable(loop);
    const file = path.join(loop, 'tools', 'tools.go');
    let src = fs.readFileSync(file, 'utf8');
    for (const [from, to] of edits) {
      const n = src.split(from).length - 1;
      if (n !== 1) throw new Error(`the ${kind} mutation needs ${JSON.stringify(from)} exactly once in ${pinned}/tools/tools.go; it occurs ${n} times`);
      src = src.replace(from, to);
    }
    fs.writeFileSync(file, src);
    for (const f of ['go.mod', 'go.sum']) fs.copyFileSync(path.join(root, f), path.join(dir, f));
    const modfile = path.join(dir, 'go.mod');
    const edit = go(root, ['mod', 'edit', `-replace=${LOOP}=${loop}`, modfile]);
    if (edit.status !== 0) throw new Error(`go mod edit -replace=${LOOP}=${loop} ${modfile} exited ${edit.status}: ${edit.stdout}${edit.stderr}`);
    return { dir, modfile };
  } catch (err) {
    removeTree(dir);
    throw err;
  }
}

// runMutated runs the named test of ./build against the pinned kogen-loop with the mutation kind.
function runMutated(root, name, kind) {
  const { dir, modfile } = mutatedModfile(root, kind);
  try {
    return goTest(root, [name], modfile);
  } finally {
    removeTree(dir);
  }
}

module.exports = { goTest, outcomes, judgePassed, judgeFailed, runMutated };

if (require.main === module) {
  const [expect, kind, name] = process.argv.slice(2);
  const judge = { passed: judgePassed, failed: judgeFailed }[expect];
  if (!judge || !MUTATIONS[kind] || !name) {
    process.stdout.write('usage: k-silent-shell-test-fast__control.js <passed|failed> <stop|waits> <TestName>\n');
    process.exit(2);
  }
  const run = runMutated(process.cwd(), name, kind);
  if (judge(run, name) === null) {
    process.stdout.write(`${expect}: ${name}\n`);
  } else {
    process.stdout.write(`rejected: ${name} did not ${expect === 'passed' ? 'pass' : 'fail'} (${verdictOf(run, name)})\n`);
    if (verdictOf(run, name) === 'no result line') process.stdout.write(tail(run.output));
    process.exitCode = 1;
  }
}
