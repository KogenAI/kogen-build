// Observer of build-consumes-executor-claude-fixture: runs the focused Go tests of ./build with the
// real go tool from the checkout's real path and judges the `go test -json` events of the named tests
// and their subtests. The tests run against the kogen-executor go.mod pins, or against a temporary
// copy of that module with one change, reached through a temporary go.mod (-modfile) with a replace
// directive, so no checkout file changes:
//   ignoresmalformed the event stream ignores a line that is not a JSON object instead of refusing it;
//   doubledprompt    the first prompt carries the stable authority twice.
// As a command it judges real `go test -json` output read from stdin:
//   go test -json ... | node build-consumes-executor-claude-fixture__observer.js <test> [<subtest>...]
// It prints one line, the verdict on whether every named test and subtest passed with no test failed
// or skipped, and exits 0 when it accepts and 1 when it rejects.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

const EXECUTOR = 'github.com/KogenAI/kogen-executor';

// The text of the pinned kogen-executor's executor/claudecode files that each copy replaces.
const MALFORMED_FROM = '\t} else if err != nil {\n' +
  '\t\treturn executor.Fail(executor.ReasonProtocol, "line "+strconv.Itoa(r.lines)+" of the event stream is not a JSON object")\n' +
  '\t}';
const PROMPT_FROM = '\treturn constraintsFor(role) + promptSep + brief.Stable + promptSep + brief.Dynamic\n';

const MUTANTS = {
  ignoresmalformed: { file: 'stream.go', from: MALFORMED_FROM, to: '\t} else if err != nil {\n\t\treturn nil\n\t}' },
  doubledprompt: { file: 'session.go', from: PROMPT_FROM, to: '\treturn constraintsFor(role) + promptSep + brief.Stable + promptSep + brief.Stable + promptSep + brief.Dynamic\n' },
};

// goBinary is GOROOT's go, or plain go when GOROOT cannot be read.
function goBinary() {
  const r = spawnSync('go', ['env', 'GOROOT'], { encoding: 'utf8' });
  const bin = r.status === 0 ? path.join(r.stdout.trim(), 'bin', 'go') : '';
  return bin && fs.existsSync(bin) ? bin : 'go';
}

function realRoot() {
  return fs.realpathSync(process.cwd());
}

// moduleDir is the directory of the kogen-executor module go.mod pins, downloaded when needed.
function moduleDir(root, go) {
  const list = () => spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', EXECUTOR], { cwd: root, encoding: 'utf8' });
  let r = list();
  if (r.status === 0 && r.stdout.trim() === '') {
    spawnSync(go, ['mod', 'download', EXECUTOR], { cwd: root, encoding: 'utf8' });
    r = list();
  }
  if (r.status !== 0 || r.stdout.trim() === '') throw new Error(`go list -m -f {{.Dir}} ${EXECUTOR} exited ${r.status}: ${r.stdout}${r.stderr}`);
  return r.stdout.trim();
}

// prepare returns the GOFLAGS of a run: none for the pinned module, otherwise a temporary copy of it
// with the mutant's change, in tmp, used through a temporary go.mod.
function prepare(root, go, mutant, tmp) {
  if (!mutant) return '';
  const v = MUTANTS[mutant];
  if (!v) throw new Error(`unknown kogen-executor copy ${mutant}; want one of ${Object.keys(MUTANTS).join(', ')}`);
  const dir = path.join(tmp, 'kogen-executor');
  fs.cpSync(moduleDir(root, go), dir, { recursive: true });
  spawnSync('chmod', ['-R', 'u+w', dir]);
  const file = path.join(dir, 'executor', 'claudecode', v.file);
  const source = fs.readFileSync(file, 'utf8');
  const n = source.split(v.from).length - 1;
  if (n !== 1) throw new Error(`kogen-executor ${mutant}: executor/claudecode/${v.file} of the pinned kogen-executor holds the replaced text ${n} times, want once`);
  fs.writeFileSync(file, source.replace(v.from, v.to));
  const mod = path.join(tmp, 'go.mod');
  fs.writeFileSync(mod, fs.readFileSync(path.join(root, 'go.mod'), 'utf8') + `\nreplace ${EXECUTOR} => ${dir}\n`);
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(tmp, 'go.sum'));
  const flags = '-modfile=' + mod;
  const used = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', EXECUTOR], { cwd: root, encoding: 'utf8', env: { ...process.env, GOFLAGS: flags } });
  if (used.status !== 0 || used.stdout.trim() !== dir) {
    throw new Error(`kogen-executor ${mutant}: go does not use the changed copy (go list -m exited ${used.status}: ${used.stdout}${used.stderr})`);
  }
  return flags;
}

// parse reads go test -json output into the results of every test and subtest, outputs per
// top-level test, the go test -json rows of each test, and the rest of the output.
function parse(stdout) {
  const results = {};
  const outputs = {};
  const rows = {};
  let raw = '';
  for (const line of stdout.split('\n')) {
    let ev;
    try {
      ev = line.startsWith('{') ? JSON.parse(line) : null;
    } catch {
      ev = null;
    }
    if (!ev) {
      raw += line ? line + '\n' : '';
      continue;
    }
    if (!ev.Test) {
      if (ev.Action === 'output' || ev.Action === 'build-output') raw += ev.Output || '';
      continue;
    }
    const top = ev.Test.split('/')[0];
    if (['run', 'pass', 'fail', 'skip'].includes(ev.Action)) (rows[ev.Test] = rows[ev.Test] || []).push(line);
    if (ev.Action === 'output') outputs[top] = (outputs[top] || '') + ev.Output;
    if (['pass', 'fail', 'skip'].includes(ev.Action)) results[ev.Test] = ev.Action;
  }
  return { results, outputs, rows, raw };
}

// runGo runs `go test -count=1 -json -run <pattern> [-skip <skip>] ./build` in root, against the
// pinned kogen-executor or a copy of it, and resolves to the run: its command, exit status and
// parse(stdout) with the stderr.
function runGo(root, mutant, pattern, skip) {
  const go = goBinary();
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'bcxc-'));
  let flags;
  try {
    flags = prepare(root, go, mutant, tmp);
  } catch (err) {
    fs.rmSync(tmp, { recursive: true, force: true });
    return Promise.reject(err);
  }
  const args = ['test', '-count=1', '-json', '-run', pattern, ...(skip ? ['-skip', skip] : []), './build'];
  const command = `go ${args.join(' ')}` + (mutant ? ` against the ${mutant} kogen-executor` : '');
  return new Promise((resolve) => {
    const child = spawn(go, args, { cwd: root, env: { ...process.env, GOWORK: 'off', GOFLAGS: flags } });
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
      fs.rmSync(tmp, { recursive: true, force: true });
      resolve({ command, status, stderr, ...parse(stdout) });
    }
  });
}

const sorted = (xs) => xs.slice().sort();

// passProblem is '' when every named test and subtest passed and no test of the run failed or was
// skipped, otherwise the reason, naming the tests and the command.
function passProblem(run, names) {
  const missing = names.filter((t) => !(t in run.results));
  const failed = sorted(Object.keys(run.results).filter((t) => run.results[t] === 'fail'));
  const skipped = sorted(Object.keys(run.results).filter((t) => run.results[t] === 'skip'));
  const problems = [];
  if (missing.length) problems.push(`did not run: ${missing.join(', ')}`);
  if (failed.length) problems.push(`failed: ${failed.join(', ')}`);
  if (skipped.length) problems.push(`skipped: ${skipped.join(', ')}`);
  if (problems.length === 0 && typeof run.status === 'number' && run.status !== 0) problems.push(`go test exited ${run.status}`);
  return problems.length ? `${problems.join('; ')} (${run.command}, exit ${run.status})` : '';
}

// clip shortens text for a failure message: at most 40 lines, each cut to 300 characters.
function clip(text) {
  const lines = text.split('\n');
  const kept = lines.slice(0, 40).map((l) => (l.length > 300 ? `${l.slice(0, 300)}... (${l.length} characters)` : l));
  if (lines.length > 40) kept.push(`... (${lines.length - 40} more lines)`);
  return kept.join('\n');
}

// details is what go test reported for each named test (its go test -json rows, or that it did not
// run), then the clipped output of the top-level tests that did not pass, or of the whole run.
function details(run, names) {
  const own = names.map((t) => (run.rows[t] ? run.rows[t].join('\n') : `${t}: did not run (no go test -json row for it in ${run.command})`));
  const tops = new Set(names.map((t) => t.split('/')[0]).filter((t) => run.results[t] !== 'pass'));
  const parts = [own.join('\n')];
  for (const t of tops) if (run.outputs[t]) parts.push(clip(run.outputs[t]));
  if (parts.length === 1) parts.push(clip(run.raw + (run.stderr || '')));
  return parts.join('\n');
}

module.exports = { MUTANTS, runGo, passProblem, details, clip, realRoot, parse };

if (require.main === module) {
  const names = process.argv.slice(2);
  if (names.length === 0) {
    console.log('usage: go test -json ... | node build-consumes-executor-claude-fixture__observer.js <test> [<subtest>...]');
    process.exit(2);
  }
  let stdin = '';
  process.stdin.setEncoding('utf8');
  process.stdin.on('data', (d) => (stdin += d));
  process.stdin.on('end', () => {
    const p = passProblem({ command: 'go test -json', status: null, ...parse(stdin) }, names);
    if (p) {
      console.log(`rejected: ${names.join(', ')}: ${p.replace(/ \(go test.*$/, '')}`);
      process.exit(1);
    }
    console.log(`passed: ${names.join(', ')}`);
  });
}
