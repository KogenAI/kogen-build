'use strict';
// The k-models-file-build acceptance harness. It runs kogen-build's own Go tests with `go test
// -json`, either with the committed go.mod or against the K-MODELS-FILE coordinator candidate
// 2a4c69a6. That candidate is not fetchable as a module version, so candidateModfile makes it in a
// temporary folder: a copy of the pinned coordinator (c4a49a4, whose Go source 2a4c69a6 changes
// only in project/baseline.go, project/config.go and project/render.go) with
// coordinator-2a4c69a6.patch (that diff without context, indented with spaces) applied and the
// three files formatted by gofmt, checked against the candidate's Git blob ids. The tests then
// run with `-modfile` naming a temporary copy of go.mod that replaces the coordinator with that
// folder, so no file of the checkout changes.
//
// `node harness.js --control good|bad` runs the judges on real `go test -json` output of a
// temporary module and on real file digests: good prints "accepted" and exits 0 when every known
// good input is accepted; bad prints "rejected" and exits 1 when every known bad input is rejected.
const { spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..', '..');
const coordinator = 'github.com/KogenAI/kogen-coordinator';
const pin = 'v0.0.0-20261102193107-c4a49a4fe84f';
// The Git blob ids of the three changed files in kogen-coordinator 2a4c69a6.
const candidateBlobs = {
  'project/baseline.go': '8fb1ed85b880e7e6de1134f8c3ad8bd925c41289',
  'project/config.go': '70062bb963cf0c28450c402a2b823dd25a4d944a',
  'project/render.go': 'deb6c0e36333590ff7c49d4bb838c80653eb2283',
};

function env(extra) {
  return { ...process.env, PWD: root, GOPRIVATE: process.env.GOPRIVATE || 'github.com/KogenAI/*', ...extra };
}

function run(cmd, args, opts) {
  const r = spawnSync(cmd, args, { cwd: root, env: env({}), encoding: 'utf8', maxBuffer: 512 << 20, ...opts });
  return { status: r.status, stdout: r.stdout || '', stderr: r.stderr || '', error: r.error,
    command: `${cmd} ${args.join(' ')}` };
}

function must(r, what) {
  if (r.status !== 0) {
    throw new Error(`${what}: ${r.command} exited ${r.status}${r.error ? ` (${r.error})` : ''}:\n${r.stdout}${r.stderr}`);
  }
  return r;
}

function blobID(file) {
  const b = fs.readFileSync(file);
  return crypto.createHash('sha1').update(`blob ${b.length}\0`).update(b).digest('hex');
}

// goMod is the committed go.mod as `go mod edit -json` reports it.
function goMod() {
  return JSON.parse(must(run('go', ['mod', 'edit', '-json']), 'reading go.mod').stdout);
}

// candidateModfile makes the candidate coordinator and a go.mod that replaces the pinned one with
// it, once per process (so the scenarios share the Go build cache), and returns that go.mod's path.
// The folder is removed when the process exits.
let made = null;
function candidateModfile() {
  if (made) return made;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-models-file-build-'));
  process.on('exit', () => {
    spawnSync('chmod', ['-R', 'u+w', dir]);
    fs.rmSync(dir, { recursive: true, force: true });
  });
  const dl = JSON.parse(must(run('go', ['mod', 'download', '-json', coordinator]), 'locating the pinned coordinator').stdout);
  if (dl.Version !== pin || !dl.Dir) {
    throw new Error(`the pinned coordinator is ${dl.Version} at ${dl.Dir}, want ${pin}`);
  }
  const coord = path.join(dir, 'coordinator');
  must(run('cp', ['-R', dl.Dir, coord]), 'copying the pinned coordinator');
  must(run('chmod', ['-R', 'u+w', coord]), 'making the copy writable');
  const patch = path.join(__dirname, 'coordinator-2a4c69a6.patch');
  must(run('git', ['apply', '--unidiff-zero', '--ignore-whitespace', patch], { cwd: coord, env: env({ PWD: coord, GIT_CEILING_DIRECTORIES: dir }) }),
    'applying the candidate patch');
  must(run('gofmt', ['-w', ...Object.keys(candidateBlobs)], { cwd: coord, env: env({ PWD: coord }) }), 'formatting the patched files');
  for (const [rel, want] of Object.entries(candidateBlobs)) {
    const got = blobID(path.join(coord, rel));
    if (got !== want) {
      throw new Error(`the made coordinator's ${rel} is blob ${got}, kogen-coordinator 2a4c69a6's is ${want}`);
    }
  }
  const modDir = path.join(dir, 'mod');
  fs.mkdirSync(modDir);
  const modfile = path.join(modDir, 'go.mod');
  fs.copyFileSync(path.join(root, 'go.mod'), modfile);
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(modDir, 'go.sum'));
  must(run('go', ['mod', 'edit', `-replace=${coordinator}=${coord}`, modfile]), 'writing the replacing go.mod');
  made = modfile;
  return made;
}

// parse reads `go test -json` output: the last terminal action of every package and test, and
// the output printed for each.
function parse(r, command) {
  const packages = {};
  const tests = {};
  const output = {};
  const other = [];
  for (const line of r.stdout.split('\n')) {
    if (line.trim() === '') continue;
    let e;
    try {
      e = JSON.parse(line);
    } catch {
      other.push(line);
      continue;
    }
    const key = e.Test ? `${e.Test}` : `package ${e.Package || e.ImportPath}`;
    if (e.Action === 'output' || e.Action === 'build-output') {
      output[key] = (output[key] || '') + e.Output;
    } else if (['pass', 'fail', 'skip', 'build-fail'].includes(e.Action)) {
      if (e.Test) tests[e.Test] = e.Action;
      else packages[e.Package || e.ImportPath] = e.Action;
    }
  }
  return { status: r.status, packages, tests, output, other: other.join('\n') + r.stderr, command };
}

// goTest runs `go test -count=1 -json -run ^(<the top-level names>)$ ./build` in the checkout (with
// -modfile when modfile is set) and returns the parsed run.
function goTest(top, modfile, against) {
  const full = ['test', ...(modfile ? [`-modfile=${modfile}`] : []), '-count=1', '-json', '-run', `^(${top.join('|')})$`, './build'];
  const shown = `go test${modfile ? ' -modfile=<temporary go.mod>' : ''} -count=1 -json -run '^(<the ${top.length} top-level tests listed>)$' ./build`;
  return parse(run('go', full), `${shown} in ${root} with ${against}`);
}

// problems lists why the run does not show every package passing and each required test passing.
function problems(result, required) {
  const out = [];
  if (result.status !== 0) out.push(`go test exited ${result.status}, want 0`);
  const pkgs = Object.entries(result.packages);
  if (pkgs.length === 0) out.push('no package reported a result');
  for (const [pkg, action] of pkgs) {
    if (action !== 'pass') out.push(`package ${pkg}: ${action}, want pass`);
  }
  for (const name of required) {
    const action = result.tests[name];
    if (action !== 'pass') out.push(`${name}: ${action ? action : 'did not run'}, want pass`);
  }
  return out;
}

// evidence is what go test printed for one test, on one line: its --- PASS/FAIL/SKIP line (or its
// === RUN line when it did not end) and the first file:line message, or a note that it printed
// nothing.
function evidence(result, name) {
  const lines = (result.output[name] || '').split('\n').map((l) => l.trim());
  const end = lines.find((l) => /^--- (PASS|FAIL|SKIP): /.test(l)) || lines.find((l) => /^=== RUN /.test(l));
  const first = lines.find((l) => /^\S+\.go:\d+: /.test(l));
  const shown = [end, first].filter(Boolean).join('; ');
  return shown ? shown.slice(0, 200) : 'go test printed nothing for it';
}

// report throws when the run does not show each required test passing: one line per required test
// that did not pass, with what go test printed for it, then the command and the run's outcome.
function report(result, required) {
  const found = problems(result, required);
  if (found.length === 0) return;
  const each = required.filter((name) => result.tests[name] !== 'pass')
    .map((name) => `${name}: ${result.tests[name] || 'did not run'}, want pass (${evidence(result, name)})`);
  const others = found.filter((p) => !required.some((name) => p.startsWith(`${name}: `)));
  const pkgOut = Object.keys(result.output).filter((k) => k.startsWith('package ')).map((k) => result.output[k].slice(-1500)).join('\n');
  throw new Error(`${each.join('\n')}\n${others.join('\n')}\ncommand: ${result.command}\n${pkgOut}\n${result.other.slice(-1500)}`);
}

// digest is the SHA-256 of a checkout file (path relative to the checkout), or "missing".
function digest(rel, base) {
  try {
    return crypto.createHash('sha256').update(fs.readFileSync(path.join(base || root, rel))).digest('hex');
  } catch {
    return 'missing';
  }
}

// digestProblems lists every file whose digest is not the wanted one.
function digestProblems(rows, base) {
  return rows.filter((r) => digest(r.path, base) !== r.sha256)
    .map((r) => `${r.path}: sha256 ${digest(r.path, base)}, want ${r.sha256}`);
}

// control runs the judges on real inputs: a temporary module whose tests pass, fail and skip, and
// a temporary file with a known digest.
function control(mode) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-models-file-build-control-'));
  try {
    fs.writeFileSync(path.join(dir, 'go.mod'), 'module kmfbcontrol\n\ngo 1.21\n');
    fs.writeFileSync(path.join(dir, 'c_test.go'), 'package kmfbcontrol\n\nimport "testing"\n\n' +
      'func TestPasses(t *testing.T) {}\n\n' +
      'func TestParent(t *testing.T) {\n\tt.Run("child", func(t *testing.T) {})\n}\n\n' +
      'func TestFails(t *testing.T) { t.Fatal("control failure") }\n\n' +
      'func TestSkips(t *testing.T) { t.Skip("control skip") }\n');
    fs.writeFileSync(path.join(dir, 'abc.txt'), 'abc\n');
    const opts = { cwd: dir, env: env({ PWD: dir, GOWORK: 'off', GOFLAGS: '' }) };
    const go = (args) => parse(run('go', ['test', '-count=1', '-json', ...args, '.'], opts), `go test ${args.join(' ')} (control)`);
    const passing = go(['-run', '^(TestPasses|TestParent)$']);
    const all = go([]);
    const abc = 'edeaaff3f1774ad2888673770c6d64097e391bc362d7d6fb34982ddf0efd18cb';
    const good = [
      ['a passing run', problems(passing, ['TestPasses', 'TestParent', 'TestParent/child'])],
      ['a matching digest', digestProblems([{ path: 'abc.txt', sha256: abc }], dir)],
      ['a missing file wanted missing', digestProblems([{ path: 'none.txt', sha256: 'missing' }], dir)],
    ];
    const bad = [
      ['a failed run', problems(all, ['TestPasses'])],
      // The failed run's own events with its exit status and package result read as passing, so
      // that only the judgment of the required test itself can reject it.
      ['a failed required test', problems({ ...all, status: 0, packages: { kmfbcontrol: 'pass' } }, ['TestFails'])],
      ['a skipped required test', problems(go(['-run', '^TestSkips$']), ['TestSkips'])],
      ['a test that did not run', problems(passing, ['TestPasses', 'TestAbsent'])],
      ['a run of no package', problems(parse({ status: 0, stdout: '', stderr: '' }, 'empty'), [])],
      ['a wrong digest', digestProblems([{ path: 'abc.txt', sha256: abc.replace(/^e/, 'f') }], dir)],
      ['a missing file', digestProblems([{ path: 'none.txt', sha256: abc }], dir)],
    ];
    if (mode === 'good') {
      const wrong = good.filter(([, p]) => p.length > 0).map(([what, p]) => `${what}: ${p.join('; ')}`);
      if (wrong.length > 0) {
        process.stdout.write(`not accepted: ${wrong.join(' | ')}\n`);
        return 2;
      }
      process.stdout.write('accepted\n');
      return 0;
    }
    const missed = bad.filter(([, p]) => p.length === 0).map(([what]) => what);
    if (missed.length > 0) {
      process.stdout.write(`not rejected: ${missed.join(', ')}\n`);
      return 0;
    }
    process.stdout.write('rejected\n');
    return 1;
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

if (require.main === module) {
  const i = process.argv.indexOf('--control');
  const mode = i >= 0 ? process.argv[i + 1] : '';
  if (mode !== 'good' && mode !== 'bad') {
    process.stdout.write('usage: harness.js --control good|bad\n');
    process.exit(2);
  }
  process.exit(control(mode));
}

module.exports = { goMod, candidateModfile, goTest, report, digestProblems, coordinator, pin };
