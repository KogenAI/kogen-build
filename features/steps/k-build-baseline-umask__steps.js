'use strict';
// Steps of k-build-baseline-umask: TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody in
// package build run with `go test -json -count=1` in the checkout under a given process umask
// (`sh -c 'umask <mask>; exec go test ...'`). To show that the tests still detect the changes they
// check, a run can replace build/baseline_cache.go with a copy in which baselineIdentity is
// changed (a `go test -overlay`, so no checkout file changes). A failure names the tests, the
// umask, the command and its exit status, the `go test -json` rows (other than output) and the
// tail of what go test printed.
//
// `node features/steps/k-build-baseline-umask__steps.js good|bad` runs the two run judges on real
// `go test -json` output under umask 022: good prints "accepted" and exits 0 when a passing run of
// both tests and a run in which both fail (baselineIdentity depending on the checkout path) are
// accepted; bad prints "rejected" and exits 1 when every refused variant is rejected.
const { spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..');
const pkg = 'github.com/KogenAI/kogen-build/build';
const product = path.join(root, 'build', 'baseline_cache.go');
const identityTest = 'TestBaselinePerBaseIdentity';
const custodyTest = 'TestBaselinePerBaseCustody';

// Changes of baselineIdentity used as counterexamples: each replaces one exact line fragment of
// build/baseline_cache.go, which must occur there exactly once.
const mutations = {
  'ignores permission bits': ['put(uint32(st.Mode()))', 'put(uint32(st.Mode().Type()))'],
  'ignores file contents': ['put(hex.EncodeToString(ch.Sum(nil)))', 'put("")'],
  'depends on the checkout path': ['put(common)', 'put(common)\n\t\tput(root)'],
};

// scratch makes a temporary folder that is removed when the process exits.
const scratchDirs = [];
function scratch() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-build-baseline-umask-'));
  if (scratchDirs.length === 0) {
    process.on('exit', () => {
      for (const d of scratchDirs) fs.rmSync(d, { recursive: true, force: true });
    });
  }
  scratchDirs.push(dir);
  return dir;
}

// overlay writes a go overlay file that replaces build/baseline_cache.go with content. The key is
// relative to go's working directory (the checkout), so a symlinked checkout path still matches.
function overlay(content) {
  const dir = scratch();
  const file = path.join(dir, 'baseline_cache.go');
  fs.writeFileSync(file, content);
  const json = path.join(dir, 'overlay.json');
  fs.writeFileSync(json, JSON.stringify({ Replace: { 'build/baseline_cache.go': file } }));
  return json;
}

function mutated(name) {
  const m = mutations[name];
  if (!m) throw new Error(`unknown baselineIdentity change "${name}"; known: ${Object.keys(mutations).join(', ')}`);
  const source = fs.readFileSync(product, 'utf8');
  const count = source.split(m[0]).length - 1;
  if (count !== 1) {
    throw new Error(`build/baseline_cache.go contains ${count} occurrences of ${JSON.stringify(m[0])}, want exactly 1 for the change "${name}"`);
  }
  return overlay(source.replace(m[0], m[1]));
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

// goTest runs `go test [-overlay …] -json -count=1 -run <filter> ./build` in the checkout under umask.
function goTest(umask, filter, overlayFile, about) {
  if (!/^0?[0-7]{3}$/.test(umask)) throw new Error(`bad umask ${umask}`);
  const args = ['test', ...(overlayFile ? [`-overlay=${overlayFile}`] : []), '-json', '-count=1', '-run', filter, './build'];
  const r = spawnSync('/bin/sh', ['-c', 'umask "$1" && shift && exec go "$@"', 'sh', umask, ...args], {
    cwd: root, env: { ...process.env, GOWORK: 'off' }, encoding: 'utf8', maxBuffer: 256 << 20, timeout: 100 * 1000,
  });
  const shown = `umask ${umask}; go test${overlayFile ? ' -overlay=<' + about + '>' : ''} -json -count=1 -run '${filter}' ./build`;
  return parse(r.status, r.stdout || '', r.stderr || '', r.error, `${shown} in ${root}`);
}

const bothFilter = `^(${identityTest}|${custodyTest})$`;
const terminal = ['pass', 'fail', 'skip'];

// outcomes is what each named test ended with, in order, and the package result.
function outcomes(r) {
  const ends = {};
  let pkgAction;
  for (const { e } of r.rows) {
    if (!terminal.includes(e.Action)) continue;
    if (e.Test) (ends[e.Test] = ends[e.Test] || []).push(e.Action);
    else if (e.Package === pkg) pkgAction = e.Action;
  }
  return { ends, pkgAction };
}

// judgePassed lists why a run does not show each of names passing exactly once, with go test
// exiting 0 and package build passing.
function judgePassed(r, names) {
  const { ends, pkgAction } = outcomes(r);
  const found = [];
  for (const name of names) {
    const got = ends[name] || [];
    if (got.length === 0) found.push(`${name}: did not run (no result row for it), want one passing run`);
    else if (got.length !== 1 || got[0] !== 'pass') found.push(`${name}: ended ${got.join(', ')}, want one run that passes`);
  }
  if (r.error) found.push(`go test did not finish: ${r.error}`);
  if (r.status !== 0) found.push(`go test exited ${r.status}, want 0`);
  if (pkgAction !== 'pass') found.push(`package ${pkg}: ${pkgAction || 'no result'}, want pass`);
  return found;
}

// judgeFailed lists why a run does not show each of names running and failing.
function judgeFailed(r, names) {
  const { ends } = outcomes(r);
  const found = [];
  for (const name of names) {
    const got = ends[name] || [];
    if (got.length === 0) found.push(`${name}: did not run (no result row for it), want a failing run`);
    else if (!got.includes('fail')) found.push(`${name}: ended ${got.join(', ')}, want a failure`);
  }
  if (r.error) found.push(`go test did not finish: ${r.error}`);
  if (r.status === 0 || r.status === null) found.push(`go test exited ${r.status}, want a failure exit`);
  return found;
}

function report(r, names, found) {
  if (found.length === 0) return;
  const rows = r.rows.filter(({ e }) => e.Action !== 'output').map(({ line }) => line);
  const out = r.rows.filter(({ e }) => e.Action === 'output').map(({ e }) => e.Output).join('') + r.other;
  throw new Error(`${found.join('\n')}\ntests: ${names.join(', ')}\ncommand: ${r.command} (exit ${r.status})\n` +
    `go test -json rows (without output):\n${rows.join('\n')}\nwhat go test printed (last 8000 bytes):\n${out.slice(-8000)}`);
}

// control runs the judges on real go test runs under umask 022 (see the header).
function control(mode) {
  const passing = goTest('022', bothFilter, null);
  const pathDependent = goTest('022', bothFilter, mutated('depends on the checkout path'), 'baselineIdentity that depends on the checkout path');
  const both = [identityTest, custodyTest];
  if (mode === 'good') {
    const problems = [...judgePassed(passing, both), ...judgeFailed(pathDependent, both)];
    if (problems.length > 0) {
      process.stdout.write(`refused a good run:\n${problems.join('\n')}\n`);
      return 2;
    }
    process.stdout.write('accepted\n');
    return 0;
  }
  if (mode === 'bad') {
    const identityOnly = goTest('022', `^${identityTest}$`, null);
    const none = goTest('022', '^$', null);
    const permissions = goTest('022', bothFilter, mutated('ignores permission bits'), 'baselineIdentity that ignores permission bits');
    const broken = goTest('022', bothFilter, overlay('package build\n\nfunc broken(\n'), 'baseline_cache.go that does not compile');
    const variants = [
      ['pass judge, both tests failed', judgePassed(pathDependent, both)],
      ['pass judge, Custody did not run', judgePassed(identityOnly, both)],
      ['pass judge, no test ran', judgePassed(none, both)],
      ['pass judge, Identity failed', judgePassed(permissions, both)],
      ['fail judge, both tests passed', judgeFailed(passing, both)],
      ['fail judge, Custody passed', judgeFailed(permissions, both)],
      ['fail judge, package did not compile', judgeFailed(broken, [identityTest])],
    ];
    const accepted = variants.filter(([, found]) => found.length === 0).map(([name]) => name);
    if (accepted.length > 0) {
      process.stdout.write(`accepted bad variants: ${accepted.join('; ')}\n`);
      return 2;
    }
    process.stdout.write('rejected\n');
    return 1;
  }
  process.stdout.write('usage: k-build-baseline-umask__steps.js good|bad\n');
  return 2;
}

function names(text) {
  return text.split(' and ');
}

if (require.main === module) {
  process.exitCode = control(process.argv[2]);
} else {
  const { When, Then } = require('@cucumber/cucumber');
  const P = 'k-build-baseline-umask: ';

  When(`${P}go test runs TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody in package build under umask {word}`,
    { timeout: 110 * 1000 }, function (umask) {
      this.kbuRun = goTest(umask, bothFilter, null);
    });

  When(`${P}go test runs TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody in package build under umask {word} with baselineIdentity changed so that it {string}`,
    { timeout: 110 * 1000 }, function (umask, change) {
      this.kbuRun = goTest(umask, bothFilter, mutated(change), `baselineIdentity that ${change}`);
    });

  Then(`${P}{string} passed`, function (tests) {
    report(this.kbuRun, names(tests), judgePassed(this.kbuRun, names(tests)));
  });

  Then(`${P}{string} failed`, function (tests) {
    report(this.kbuRun, names(tests), judgeFailed(this.kbuRun, names(tests)));
  });

  Then(`${P}the file {string} has sha256 {word}`, function (file, digest) {
    const got = crypto.createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex');
    if (got !== digest) throw new Error(`${file} has sha256 ${got}, want ${digest} (baselineIdentity's product code must stay unchanged)`);
  });
}
