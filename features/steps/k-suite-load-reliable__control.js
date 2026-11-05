// Runner and judges of k-suite-load-reliable, shared by its steps and run directly as the
// Feasibility observer control:
//
//   node features/steps/k-suite-load-reliable__control.js <passed|failed> <plain|git=SECONDS|mutation=NAME> <TestName> [message]
//
// runs the named Go test of ./build once (go test -count=1 -json) and prints one verdict line:
// "<passed|failed>: <TestName>" with exit 0 when the test had that outcome, otherwise "rejected: ..."
// with exit 1. git=SECONDS puts a git first on PATH that waits SECONDS before every call and then
// runs the real git: a host on which every git process takes longer, as under load; a passing run
// must have made at least one such call. mutation=NAME runs the test against one deliberately broken
// implementation (MUTATIONS); a failing run must show the message in that test's own output and no
// wall-clock wait failure. The checkout is not changed: a mutated kogen-build file reaches the build
// through -overlay, a mutated dependency through a temporary copy and go.mod (-modfile). Requiring
// this file (as cucumber does with every features/steps/*.js) runs nothing.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

// MUTATIONS are the deliberately broken implementations. module is null for a file of this module,
// otherwise the dependency whose pinned copy is edited; from must occur exactly once in file.
const MUTATIONS = {
  // An interrupt during `git commit` (a post-commit hook still running) leaves the commit on HEAD.
  'interrupted-commit-kept': {
    module: 'github.com/KogenAI/kogen-coordinator',
    file: 'git/commit.go',
    from: '\tif err != nil {\n\t\tr.undoIfLanded(spec.Start)\n\t\treturn "", err\n\t}\n',
    to: '\tif err != nil {\n\t\treturn "", err\n\t}\n',
  },
  // Resume no longer refuses an uncertain startup, tool or commit boundary.
  'uncertain-boundary-resumed': {
    module: null,
    file: 'build/resume.go',
    from: '\tif slices.Contains([]string{StageTool, StageCommit, StageStartup}, b.Continuation.Stage) {\n',
    to: '\tif false && slices.Contains([]string{StageTool, StageCommit, StageStartup}, b.Continuation.Stage) {\n',
  },
  // Resume refuses every saved stage as an uncertain boundary, also the resumable ones.
  'every-stage-refused': {
    module: null,
    file: 'build/resume.go',
    from: '\tif slices.Contains([]string{StageTool, StageCommit, StageStartup}, b.Continuation.Stage) {\n',
    to: '\tif b.Continuation.Stage != "" || slices.Contains([]string{StageTool, StageCommit, StageStartup}, b.Continuation.Stage) {\n',
  },
  // A Build that stops while a baseline check runs does not name that check.
  'baseline-stop-unnamed': {
    module: null,
    file: 'build/baseline.go',
    from: '\tb.baselineStop = b.stopRecord()\n',
    to: '\tb.baselineStop = nil\n',
  },
  // A native round interrupted through its session loses the sys.Interrupted (and its signal).
  'native-interrupt-lost': {
    module: null,
    file: 'build/executor_native.go',
    from: '\tif errors.As(err, &interrupted) || ctx.Err() == nil {\n\t\treturn err\n\t}\n',
    to: '\tif errors.As(err, &interrupted) {\n\t\treturn errors.New("the interrupt was lost")\n\t}\n\tif ctx.Err() == nil {\n\t\treturn err\n\t}\n',
  },
  // A native round whose context is cancelled reports success.
  'native-cancel-succeeds': {
    module: null,
    file: 'build/executor_native.go',
    from: '\treturn executor.Fail(executor.ReasonInterrupted, "the round was interrupted; the builder\'s commands were stopped")\n',
    to: '\treturn nil\n',
  },
};

// WALL_CLOCK_FAILURES are failure texts of a wall-clock wait, not of a broken implementation.
const WALL_CLOCK_FAILURES = ['timed out waiting for', 'panic: test timed out'];

// goEnv is this process's environment without its KOGEN_ variables, outside any Go workspace.
function goEnv() {
  const env = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith('KOGEN_')) env[k] = v;
  env.GOWORK = 'off';
  return env;
}

function go(root, args, env) {
  const r = spawnSync('go', args, { cwd: root, encoding: 'utf8', maxBuffer: 256 * 1024 * 1024, env: env || goEnv() });
  if (r.error) throw new Error(`go ${args.join(' ')}: ${r.error.message}`);
  return r;
}

// goAsync runs go like go, without blocking, so that several go test runs proceed at the same time.
function goAsync(root, args, env) {
  return new Promise((resolve, reject) => {
    const child = spawn('go', args, { cwd: root, env });
    const out = [];
    const err = [];
    child.stdout.on('data', (d) => out.push(d));
    child.stderr.on('data', (d) => err.push(d));
    child.on('error', (e) => reject(new Error(`go ${args.join(' ')}: ${e.message}`)));
    child.on('close', (status) => resolve({ status, stdout: Buffer.concat(out).toString('utf8'), stderr: Buffer.concat(err).toString('utf8') }));
  });
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
  if (dir && fs.existsSync(dir)) {
    makeWritable(dir);
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

// realGit is the git a delayed git runs: on macOS the developer directory's own git (the one the
// build package's TestMain would use), otherwise the git found on PATH.
function realGit() {
  if (process.platform === 'darwin') {
    const r = spawnSync('xcode-select', ['-p'], { encoding: 'utf8' });
    const p = r.status === 0 ? path.join(r.stdout.trim(), 'usr', 'bin', 'git') : '';
    if (p && fs.existsSync(p)) return p;
  }
  const r = spawnSync('sh', ['-c', 'command -v git'], { encoding: 'utf8' });
  const p = (r.stdout || '').trim();
  if (r.status !== 0 || !p) throw new Error('no git on PATH');
  return p;
}

function shellQuote(s) {
  return `'${s.replace(/'/g, `'\\''`)}'`;
}

// delayedGit writes dir/git: it records each call in dir/calls, waits seconds and runs the real git.
function delayedGit(dir, seconds) {
  if (!/^\d+(\.\d+)?$/.test(seconds)) throw new Error(`git delay ${JSON.stringify(seconds)} is not a number of seconds`);
  const calls = path.join(dir, 'calls');
  fs.writeFileSync(
    path.join(dir, 'git'),
    `#!/bin/sh\nprintf 'call\\n' >> ${shellQuote(calls)}\nsleep ${seconds}\nexec ${shellQuote(realGit())} "$@"\n`,
    { mode: 0o755 },
  );
  return calls;
}

// mutationArgs prepares the mutation name in dir and returns the go test flags that use it.
function mutationArgs(root, dir, name) {
  const m = MUTATIONS[name];
  if (!m) throw new Error(`unknown mutation ${JSON.stringify(name)}; want one of ${Object.keys(MUTATIONS).join(', ')}`);
  const edit = (file, where) => {
    const src = fs.readFileSync(file, 'utf8');
    const n = src.split(m.from).length - 1;
    if (n !== 1) throw new Error(`the ${name} mutation needs ${JSON.stringify(m.from)} exactly once in ${where}; it occurs ${n} times`);
    return src.replace(m.from, m.to);
  };
  if (m.module === null) {
    const original = path.join(root, m.file);
    const mutated = path.join(dir, path.basename(m.file));
    fs.writeFileSync(mutated, edit(original, m.file));
    const overlay = path.join(dir, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({ Replace: { [original]: mutated } }));
    return ['-overlay=' + overlay];
  }
  const list = go(root, ['list', '-m', '-f', '{{.Dir}}', m.module]);
  const pinned = list.stdout.trim();
  if (list.status !== 0 || !pinned) throw new Error(`go list -m -f {{.Dir}} ${m.module} exited ${list.status}: ${list.stdout}${list.stderr}`);
  const copy = path.join(dir, 'module');
  fs.cpSync(pinned, copy, { recursive: true });
  makeWritable(copy);
  const file = path.join(copy, m.file);
  fs.writeFileSync(file, edit(file, `${pinned}/${m.file}`));
  for (const f of ['go.mod', 'go.sum']) fs.copyFileSync(path.join(root, f), path.join(dir, f));
  const modfile = path.join(dir, 'go.mod');
  const r = go(root, ['mod', 'edit', `-replace=${m.module}=${copy}`, modfile]);
  if (r.status !== 0) throw new Error(`go mod edit -replace=${m.module}=${copy} ${modfile} exited ${r.status}: ${r.stdout}${r.stderr}`);
  return ['-modfile=' + modfile];
}

// runTests runs the named tests of ./build together, once, in mode: "plain", "git=SECONDS" or
// "mutation=NAME", and resolves to the command, its exit status, its go test -json events, its
// whole output and, with a delayed git, how many git calls were delayed.
async function runTests(checkout, names, mode) {
  // The real path: go matches -overlay paths against the directory it resolves itself.
  const root = fs.realpathSync(checkout);
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-suite-load-reliable-'));
  try {
    const env = goEnv();
    env.PWD = root;
    const flags = [];
    let calls = null;
    if (mode.startsWith('git=')) {
      calls = delayedGit(dir, mode.slice(4));
      env.PATH = `${dir}${path.delimiter}${env.PATH || ''}`;
    } else if (mode.startsWith('mutation=')) {
      flags.push(...mutationArgs(root, dir, mode.slice(9)));
    } else if (mode !== 'plain') {
      throw new Error(`unknown mode ${JSON.stringify(mode)}; want plain, git=SECONDS or mutation=NAME`);
    }
    const args = ['test', '-count=1', '-json', ...flags, '-run', `^(${names.join('|')})$`, './build'];
    const r = await goAsync(root, args, env);
    const events = [];
    for (const line of r.stdout.split('\n')) {
      if (!line.startsWith('{')) continue;
      try {
        events.push(JSON.parse(line));
      } catch (_) {
        // a line that is not an event is kept in the output only
      }
    }
    const delayed = calls === null ? null : fs.existsSync(calls) ? fs.readFileSync(calls, 'utf8').split('\n').filter(Boolean).length : 0;
    const prefix = calls === null ? '' : `PATH=<git delayed by ${mode.slice(4)}s>:$PATH `;
    return { command: `${prefix}go ${args.join(' ')}`, status: r.status, events, output: `${r.stdout}${r.stderr}`, delayed };
  } finally {
    removeTree(dir);
  }
}

// runGroups runs every group ({mode, names}) at the same time, one go test each, and resolves to
// the runs in the same order.
function runGroups(root, groups) {
  return Promise.all(groups.map((g) => runTests(root, g.names, g.mode)));
}

// verdictOf is name's verdict in run (PASS, FAIL or SKIP), or "no result line" when it has none.
function verdictOf(run, name) {
  let v = 'no result line';
  for (const e of run.events) {
    if (e.Test === name && ['pass', 'fail', 'skip'].includes(e.Action)) v = e.Action.toUpperCase();
  }
  return v;
}

// testOutput is the output go test attributed to name and its subtests.
function testOutput(run, name) {
  return run.events
    .filter((e) => e.Action === 'output' && (e.Test === name || (e.Test || '').startsWith(name + '/')))
    .map((e) => e.Output)
    .join('');
}

function tail(text) {
  return text.length > 6000 ? `...${text.slice(-6000)}` : text;
}

function evidence(run, name) {
  const own = testOutput(run, name);
  return `${run.command} exited ${run.status}` +
    (run.delayed === null ? '' : `; git calls delayed: ${run.delayed}`) +
    `\n${name}'s output:\n${tail(own) || '(none)'}\nwhole output:\n${tail(run.output)}`;
}

// passProblem is null when name passed in run (and, with a delayed git, git calls were delayed);
// otherwise the one-line reason and the evidence.
function passProblem(run, name) {
  const v = verdictOf(run, name);
  if (v !== 'PASS') return [`${name} did not pass (${v})`, evidence(run, name)];
  if (run.delayed === 0) return [`${name} passed but no git call was delayed`, evidence(run, name)];
  return null;
}

// failProblem is null when name failed in run with message in its own output and no wall-clock
// wait failure; otherwise the one-line reason and the evidence.
function failProblem(run, name, message) {
  const v = verdictOf(run, name);
  if (v !== 'FAIL' || run.status === 0) return [`${name} did not fail (${v})`, evidence(run, name)];
  const own = testOutput(run, name);
  if (message && !own.includes(message)) return [`${name} failed without ${JSON.stringify(message)}`, evidence(run, name)];
  const clock = WALL_CLOCK_FAILURES.find((w) => own.includes(w) || run.output.includes(w));
  if (clock) return [`${name} failed on a wall-clock wait (${JSON.stringify(clock)})`, evidence(run, name)];
  return null;
}

module.exports = { MUTATIONS, runTests, runGroups, verdictOf, testOutput, passProblem, failProblem };

if (require.main === module) {
  const [expect, mode, name, message] = process.argv.slice(2);
  const judge = { passed: passProblem, failed: failProblem }[expect];
  if (!judge || !mode || !name) {
    process.stdout.write('usage: k-suite-load-reliable__control.js <passed|failed> <plain|git=SECONDS|mutation=NAME> <TestName> [message]\n');
    process.exit(2);
  }
  runTests(process.cwd(), [name], mode).then((run) => {
    const problem = judge(run, name, message);
    if (problem === null) {
      process.stdout.write(`${expect}: ${name}\n`);
    } else {
      process.stdout.write(`rejected: ${problem[0]}\n`);
      process.exitCode = 1;
    }
  });
}
