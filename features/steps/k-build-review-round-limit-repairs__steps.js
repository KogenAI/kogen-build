'use strict';
// Steps of k-build-review-round-limit-repairs: kogen-build's TestClaudeLineupReviewRoundLimit runs
// with `go test -json` in package build. It runs with the committed go.mod, against a kogen-review
// version from the module cache, or against a scratch copy of one with listed text edits. For the last
// two, a temporary go.mod passed with -modfile replaces kogen-review by that folder, so no checkout
// file changes. The test's function body is checked for required and forbidden texts. A failure
// lists each required test whose result differs, with its own `--- PASS/FAIL` line and first
// message. It then shows the command, its exit status, the run's `go test -json` rows (other than
// output) and the tail of what go test printed.
//
// `node features/steps/k-build-review-round-limit-repairs__steps.js good|bad` runs the run judge and
// the body judge on known inputs. good prints "accepted" and exits 0 when they accept the real passing
// run, the real run against the identity-blind kogen-review (both in
// k-build-review-round-limit-repairs__controls.js) and today's test body. bad prints "rejected" and
// exits 1 when they reject every refused variant of those.
const { spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..');
const reviewModule = 'github.com/KogenAI/kogen-review';
const pkg = 'github.com/KogenAI/kogen-build/build';
const testName = 'TestClaudeLineupReviewRoundLimit';
const testFile = 'build/review_lineup_test.go';
const subtests = ['default', 'one', 'two', 'zero'];

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
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-build-review-round-limit-repairs-'));
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

// reviewDir is the module cache folder of kogen-review at version, located from a temporary module
// in dir that copies the committed go.mod and go.sum.
function reviewDir(version, dir) {
  fs.mkdirSync(dir, { recursive: true });
  fs.copyFileSync(path.join(root, 'go.mod'), path.join(dir, 'go.mod'));
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(dir, 'go.sum'));
  const dl = JSON.parse(must(run('go', ['mod', 'download', '-json', `${reviewModule}@${version}`], { cwd: dir }),
    `locating ${reviewModule}@${version}`).stdout);
  if (dl.Version !== version || !dl.Dir || !fs.existsSync(dl.Dir)) {
    throw new Error(`${reviewModule}@${version} resolved to ${dl.Version} at ${dl.Dir}${dl.Error ? `: ${dl.Error}` : ''}`);
  }
  return dl.Dir;
}

// editedCopy copies the module folder src to dest and applies each edit: its text `from` must occur
// exactly `count` times in its file, and every occurrence becomes `to`.
function editedCopy(src, dest, edits, version) {
  must(run('cp', ['-R', src, dest]), 'copying kogen-review');
  must(run('chmod', ['-R', 'u+w', dest]), 'making the kogen-review copy writable');
  for (const e of edits) {
    const file = path.join(dest, e.file);
    const text = fs.readFileSync(file, 'utf8');
    const n = text.split(e.from).length - 1;
    if (n !== Number(e.count)) {
      throw new Error(`${e.file} of kogen-review ${version}: ${n} occurrences of ${JSON.stringify(e.from)}, want ${e.count}`);
    }
    fs.writeFileSync(file, text.split(e.from).join(e.to));
  }
}

// modfile writes, in dir, a copy of the committed go.mod and go.sum that replaces kogen-review with
// the folder replacement, and returns its path.
function modfile(dir, replacement) {
  fs.mkdirSync(dir, { recursive: true });
  const file = path.join(dir, 'go.mod');
  fs.copyFileSync(path.join(root, 'go.mod'), file);
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(dir, 'go.sum'));
  must(run('go', ['mod', 'edit', `-replace=${reviewModule}=${replacement}`, file]), 'writing the replacing go.mod');
  return file;
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

// goTest runs `go test [-modfile=…] -json -count=1 -run ^name$ ./build` in the checkout.
function goTest(name, mod, against) {
  const args = ['test', ...(mod ? [`-modfile=${mod}`] : []), '-json', '-count=1', '-run', `^${name}$`, './build'];
  const r = run('go', args, { timeout: 240 * 1000 });
  const shown = `go test${mod ? ' -modfile=<temporary go.mod>' : ''} -json -count=1 -run '^${name}$' ./build`;
  return parse(r.status, r.stdout, r.stderr, r.error, `${shown} in ${root} with ${against}`);
}

const terminal = ['pass', 'fail', 'skip'];

// judge lists why a run does not show name with result parent and exactly the given subtests with
// their results, go test exiting 0 for pass and 1 for fail, and package build with result parent.
function judge(r, name, parent, expected) {
  const tests = {};
  let pkgAction;
  for (const { e } of r.rows) {
    if (!terminal.includes(e.Action)) continue;
    if (e.Test) tests[e.Test] = e.Action;
    else if (e.Package === pkg) pkgAction = e.Action;
  }
  const want = { [name]: parent };
  for (const [s, result] of Object.entries(expected)) want[`${name}/${s}`] = result;
  const found = [];
  for (const [t, result] of Object.entries(want)) {
    if (tests[t] !== result) found.push({ test: t, text: `${t}: ${tests[t] || 'did not run'}, want ${result}` });
  }
  for (const t of Object.keys(tests)) {
    if (t.startsWith(`${name}/`) && !(t in want)) found.push({ text: `${t}: ${tests[t]}, not a listed subtest` });
  }
  if (r.error) found.push({ text: `go test did not finish: ${r.error}` });
  const exit = parent === 'pass' ? 0 : 1;
  if (r.status !== exit) found.push({ text: `go test exited ${r.status}, want ${exit}` });
  if (pkgAction !== parent) found.push({ text: `package ${pkg}: ${pkgAction || 'no result'}, want ${parent}` });
  return found;
}

// printed is what go test printed for one test.
function printed(r, test) {
  return r.rows.filter(({ e }) => e.Action === 'output' && (e.Test || '') === test).map(({ e }) => e.Output).join('');
}

// evidence is the test's own `--- PASS/FAIL/SKIP` line and the first file:line message it printed.
function evidence(r, test) {
  const lines = printed(r, test).split('\n').map((l) => l.trim());
  const end = lines.find((l) => /^--- (PASS|FAIL|SKIP): /.test(l));
  const at = lines.findIndex((l) => /^\S+\.go:\d+: /.test(l));
  const shown = [end || 'go test printed no result line for it', ...(at >= 0 ? lines.slice(at, at + 4) : [])];
  return shown.join('; ').slice(0, 900);
}

// runShown is the command, its exit status, the run's `go test -json` rows other than output and
// the tail of what go test printed.
function runShown(r) {
  const rows = r.rows.filter(({ e }) => e.Action !== 'output').map(({ line }) => line);
  const out = r.rows.filter(({ e }) => e.Action === 'output').map(({ e }) => e.Output).join('') + r.other;
  return `command: ${r.command} (exit ${r.status})\n` +
    `go test -json rows (without output):\n${rows.join('\n')}\nwhat go test printed (last 8000 bytes):\n${out.slice(-8000)}`;
}

function report(r, name, parent, expected) {
  const found = judge(r, name, parent, expected);
  if (found.length === 0) return;
  const text = found.map((f) => (f.test ? `${f.text} (${evidence(r, f.test)})` : f.text));
  throw new Error(`${text.join('\n')}\n${runShown(r)}`);
}

// body is the source of func name in text, from its `func` line to its closing `}` line, or null.
function body(text, name) {
  const start = text.indexOf(`\nfunc ${name}(`);
  if (start < 0) return null;
  const end = text.indexOf('\n}\n', start);
  return text.slice(start + 1, end < 0 ? text.length : end + 3);
}

// bodyProblems lists why the body b of func name in rel lacks a required text or holds a forbidden one.
function bodyProblems(b, name, rel, required, forbidden) {
  if (b === null) return [`${rel} has no func ${name}(`];
  const found = [];
  for (const t of required) {
    if (!b.includes(t)) found.push(`func ${name} in ${rel} lacks ${JSON.stringify(t)}`);
  }
  for (const t of forbidden) {
    const at = b.indexOf(t);
    if (at >= 0) {
      found.push(`func ${name} in ${rel} contains ${JSON.stringify(t)}: ${JSON.stringify(b.slice(Math.max(0, at - 80), at + 80))}`);
    }
  }
  return found;
}

function readBody(rel, name) {
  return body(fs.readFileSync(path.join(root, rel), 'utf8'), name);
}

const requiredTexts = ['wantStop(t, a, "candidate/executor_round_limit")', 'len(f.rounds(executor.RoleReview)); got != tc.max',
  'len(f.starts[executor.RoleReview]) != 1', 'len(f.starts[executor.RoleBuild]) != 1', 'len(f.rounds(executor.RoleBuild)) != 1',
  'eq(t, "HEAD", e.head(), start)', '"preserved work"', 'int(x["rounds"].(float64)) != tc.max'];
const forbiddenTexts = ['review_blocked', 'repeated identical', 'hard cap', 'FailureStreak', 'failure_streak'];

const todayDigest = 'fc9ea4ebbc01d8b245fe1b90339d1d1a56f00b034ef2dfcb1fd6be4cf008be6d';
const todayBlob = 'b3b1aea32e2d6102ab2308517dbe93f32a6a8c1d';

function todayFile() {
  const file = path.join(root, testFile);
  if (fs.existsSync(file) && crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') === todayDigest) {
    return fs.readFileSync(file, 'utf8');
  }
  return must(run('git', ['cat-file', 'blob', todayBlob]), `reading ${testFile} of kogen-build 32c65a7`).stdout;
}

function control(mode) {
  const runs = require('./k-build-review-round-limit-repairs__controls.js');
  const all = (result) => Object.fromEntries(subtests.map((s) => [s, result]));
  const capSixFail = { default: 'fail', one: 'pass', two: 'pass', zero: 'fail' };
  const variant = (status, lines) => parse(status, `${lines.join('\n')}\n`, '', undefined, 'control');
  const passing = variant(0, runs.passing);
  const blind = variant(1, runs.identityBlind);
  const today = todayFile();
  const judgeBody = (text) => bodyProblems(body(text, testName), testName, testFile, requiredTexts, forbiddenTexts);
  if (mode === 'good') {
    const found = [
      ...judge(passing, testName, 'pass', all('pass')).map((f) => `passing run: ${f.text}`),
      ...judge(blind, testName, 'fail', capSixFail).map((f) => `identity-blind run: ${f.text}`),
      ...judgeBody(today).map((f) => `today's body: ${f}`),
    ];
    console.log(found.length === 0 ? 'accepted' : `not accepted: ${found.join('; ')}`);
    return found.length === 0 ? 0 : 1;
  }
  const isRow = (l, action, test) => l.includes(`"Action":"${action}"`) &&
    (test === undefined ? !l.includes('"Test":') : l.includes(`"Test":"${test}"`));
  const edit = (lines, f) => lines.flatMap((l) => f(l));
  const once = (text, from, to) => {
    if (text.split(from).length !== 2) throw new Error(`control edit: ${JSON.stringify(from)} does not occur once`);
    return text.replace(from, to);
  };
  const bad = {
    'failed subtest': judge(variant(1, edit(runs.passing, (l) => (isRow(l, 'pass', `${testName}/default`) ||
      isRow(l, 'pass', testName) || isRow(l, 'pass') ? [l.replace('"Action":"pass"', '"Action":"fail"')] : [l]))),
    testName, 'pass', all('pass')),
    'missing subtest': judge(variant(0, edit(runs.passing, (l) => (l.includes(`"Test":"${testName}/zero"`) ? [] : [l]))),
      testName, 'pass', all('pass')),
    'extra subtest': judge(variant(0, [...runs.passing, `{"Action":"run","Package":"${pkg}","Test":"${testName}/three"}`,
      `{"Action":"pass","Package":"${pkg}","Test":"${testName}/three","Elapsed":0.01}`]), testName, 'pass', all('pass')),
    'skipped test': judge(variant(0, edit(runs.passing, (l) => (isRow(l, 'pass', testName)
      ? [l.replace('"Action":"pass"', '"Action":"skip"')] : [l]))), testName, 'pass', all('pass')),
    'no test run': judge(variant(0, edit(runs.passing, (l) => (l.includes('"Test":') ? [] : [l]))), testName, 'pass', all('pass')),
    'nonzero exit': judge(variant(1, runs.passing), testName, 'pass', all('pass')),
    'no package result': judge(variant(0, edit(runs.passing, (l) => (isRow(l, 'pass') ? [] : [l]))), testName, 'pass', all('pass')),
    'passing run where the cap-6 subtests must fail': judge(passing, testName, 'fail', capSixFail),
    'failing run where all must pass': judge(blind, testName, 'pass', all('pass')),
    'failing run where only default must fail': judge(blind, testName, 'fail', { ...all('pass'), default: 'fail' }),
    'failing run that exited 0': judge(variant(0, runs.identityBlind), testName, 'fail', capSixFail),
    'body accepting the review_blocked stop': judgeBody(once(today, 'wantStop(t, a, "candidate/executor_round_limit")',
      'wantStop(t, a, "candidate/review_blocked")')),
    'body without the recorded-rounds assertion': judgeBody(once(today, 'int(x["rounds"].(float64)) != tc.max', 'false')),
    'body naming repeated identical failures': judgeBody(once(today, '\t\t\tf.reviewMax = tc.caps\n',
      '\t\t\tf.reviewMax = tc.caps\n\t\t\t// before three repeated identical final failures\n')),
    'file without the test': judgeBody(once(today, `func ${testName}(`, `func ${testName}Renamed(`)),
  };
  const accepted = Object.keys(bad).filter((k) => bad[k].length === 0);
  console.log(accepted.length === 0 ? 'rejected' : `not rejected: ${accepted.join(', ')}`);
  return accepted.length === 0 ? 1 : 0;
}

if (require.main === module) {
  process.exitCode = control(process.argv[2]);
} else {
  const { When, Then } = require('@cucumber/cucumber');
  const P = 'k-build-review-round-limit-repairs: ';
  const timeout = { timeout: 250 * 1000 };

  When(`${P}go test runs {word} in package build with the committed go.mod`, timeout, function (name) {
    this.kbrlrRun = goTest(name, null, 'the kogen-review version the committed go.mod pins');
  });

  When(`${P}go test runs {word} in package build against kogen-review {word}`, timeout, function (name, version) {
    const dir = scratch();
    const mod = modfile(path.join(dir, 'mod'), reviewDir(version, path.join(dir, 'locate')));
    this.kbrlrRun = goTest(name, mod, `${reviewModule} ${version} from the module cache`);
  });

  When(`${P}go test runs {word} in package build against kogen-review {word} with these edits:`, timeout,
    function (name, version, table) {
      const dir = scratch();
      const edits = table.hashes();
      const copy = path.join(dir, 'kogen-review');
      editedCopy(reviewDir(version, path.join(dir, 'locate')), copy, edits, version);
      const mod = modfile(path.join(dir, 'mod'), copy);
      const listed = edits.map((e) => `${e.file}: ${JSON.stringify(e.from)} -> ${JSON.stringify(e.to)}`).join('; ');
      this.kbrlrRun = goTest(name, mod, `a copy of ${reviewModule} ${version} edited (${listed})`);
    });

  Then(`${P}go test reported {word} as {word} with these subtest results:`, function (name, parent, table) {
    report(this.kbrlrRun, name, parent, Object.fromEntries(table.hashes().map((row) => [row.subtest, row.result])));
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

  Then(`${P}the body of {word} in {word} contains each of these texts:`, function (name, rel, table) {
    const found = bodyProblems(readBody(rel, name), name, rel, table.hashes().map((row) => row.text), []);
    if (found.length > 0) throw new Error(found.join('\n'));
  });

  Then(`${P}the body of {word} in {word} contains none of these texts:`, function (name, rel, table) {
    const found = bodyProblems(readBody(rel, name), name, rel, [], table.hashes().map((row) => row.text));
    if (found.length > 0) throw new Error(found.join('\n'));
  });
}
