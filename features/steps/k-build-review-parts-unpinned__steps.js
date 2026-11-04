'use strict';
// Steps of k-build-review-parts-unpinned: kogen-build's review-parts tests run with `go test -json` in
// package build, with the committed go.mod, against a kogen-review version from the module cache, or
// against a scratch copy of one with listed text edits and added files (the 7d005ed6 layout stand-in,
// optionally with one further fault edit). A temporary go.mod passed with -modfile replaces
// kogen-review by that folder, so no checkout file changes. A failure lists each required test whose
// result differs, with its own `--- PASS/FAIL` line and first message, then the command, its exit
// status, the run's `go test -json` rows (other than output) and the tail of what go test printed.
//
// `node features/steps/k-build-review-parts-unpinned__steps.js good|bad` runs the run judge and the
// changed-files judge on known inputs. good prints "accepted" and exits 0 when they accept the real
// passing run and the real failing fault run (both in k-build-review-parts-unpinned__controls.js) and
// a list of test-only changes. bad prints "rejected" and exits 1 when they reject every refused
// variant of those.
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..');
const reviewModule = 'github.com/KogenAI/kogen-review';
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
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-build-review-parts-unpinned-'));
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

// editedCopy copies the module folder src to dest, applies each edit (its text `from` must occur
// exactly `count` times in its file, and every occurrence becomes `to`) and writes each added file,
// which must not exist yet.
function editedCopy(src, dest, edits, added, version) {
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
  for (const [rel, content] of Object.entries(added)) {
    const file = path.join(dest, rel);
    if (fs.existsSync(file)) throw new Error(`${rel} already exists in kogen-review ${version}`);
    fs.writeFileSync(file, content.endsWith('\n') ? content : `${content}\n`);
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

// goTest runs `go test [-modfile=…] -json -count=1 -run '^(name|…)$' ./build` in the checkout.
function goTest(names, mod, against) {
  const pattern = `^(${names.join('|')})$`;
  const args = ['test', ...(mod ? [`-modfile=${mod}`] : []), '-json', '-count=1', '-run', pattern, './build'];
  const r = run('go', args, { timeout: 290 * 1000 });
  const shown = `go test${mod ? ' -modfile=<temporary go.mod>' : ''} -json -count=1 -run '${pattern}' ./build`;
  return parse(r.status, r.stdout, r.stderr, r.error, `${shown} in ${root} with ${against}`);
}

const terminal = ['pass', 'fail', 'skip'];

function results(r) {
  const tests = {};
  let pkgAction;
  for (const { e } of r.rows) {
    if (!terminal.includes(e.Action)) continue;
    if (e.Test) tests[e.Test] = e.Action;
    else if (e.Package === pkg) pkgAction = e.Action;
  }
  return { tests, pkgAction };
}

// judgePass lists why a run does not show every required test and subtest passing, no test failing
// or skipped, go test exiting 0 and package build passing.
function judgePass(r, required) {
  const { tests, pkgAction } = results(r);
  const found = [];
  for (const t of required) {
    if (tests[t] !== 'pass') found.push({ test: t, text: `${t}: ${tests[t] || 'did not run'}, want pass` });
  }
  for (const [t, result] of Object.entries(tests)) {
    if (result !== 'pass' && !required.includes(t)) found.push({ test: t, text: `${t}: ${result}, want pass` });
  }
  if (r.error) found.push({ text: `go test did not finish: ${r.error}` });
  if (r.status !== 0) found.push({ text: `go test exited ${r.status}, want 0` });
  if (pkgAction !== 'pass') found.push({ text: `package ${pkg}: ${pkgAction || 'no result'}, want pass` });
  return found;
}

// judgeFail lists why a run does not show name failing, go test exiting 1 and package build failing.
function judgeFail(r, name) {
  const { tests, pkgAction } = results(r);
  const found = [];
  if (tests[name] !== 'fail') found.push({ test: name, text: `${name}: ${tests[name] || 'did not run'}, want fail` });
  if (r.error) found.push({ text: `go test did not finish: ${r.error}` });
  if (r.status !== 1) found.push({ text: `go test exited ${r.status}, want 1` });
  if (pkgAction !== 'fail') found.push({ text: `package ${pkg}: ${pkgAction || 'no result'}, want fail` });
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
  const shown = [end || 'go test printed no result line for it', ...(at >= 0 ? lines.slice(at, at + 3) : [])];
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

function report(r, found) {
  if (found.length === 0) return;
  const text = found.map((f) => (f.test ? `${f.text} (${evidence(r, f.test)})` : f.text));
  throw new Error(`${text.join('\n')}\n${runShown(r)}`);
}

// productionChanges lists the paths of `git status --porcelain=v1 -z --untracked-files=all` output
// that are kogen-build production: a Go file other than a _test.go file, go.mod or go.sum. Both
// paths of a rename or copy count.
function productionChanges(porcelain) {
  const fields = porcelain.split('\0');
  const paths = [];
  for (let i = 0; i < fields.length; i++) {
    const f = fields[i];
    if (f.length < 4) continue;
    paths.push(f.slice(3));
    if (f[0] === 'R' || f[0] === 'C') paths.push(fields[++i]);
  }
  return paths.filter((p) => p === 'go.mod' || p === 'go.sum' || (p.endsWith('.go') && !p.endsWith('_test.go')));
}

// ---- the controls ----

const reviewPartsTests = ['TestAReviewerThatNeverReadsIsStopped',
  'TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation', 'TestChangesAfterEveryPartWasReadStaysBlocked',
  'TestChangesWhilePartsAreUnreadIsSentBackToRead', 'TestKReviewRetrySeparateAllowances', 'TestKReviewVerdictStrandReset',
  'TestKRevodGatePartsUnchanged', 'TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval'];
const requiredSubtests = [
  'TestKReviewRetrySeparateAllowances/S-DECREL:_unread_rejections,_prose,_then_the_forced_verdict_in_the_same_conversation',
  'TestKReviewRetrySeparateAllowances/reverse_order_in_a_fresh_review:_prose,_then_a_rejected_verdict,_then_a_valid_one',
  'TestKRevodGatePartsUnchanged/empty-source_controls'];
const faultTest = 'TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation';

function control(mode) {
  const runs = require('./k-build-review-parts-unpinned__controls.js');
  const required = [...reviewPartsTests, ...requiredSubtests];
  const variant = (status, lines) => parse(status, `${lines.join('\n')}\n`, '', undefined, 'control');
  const passing = variant(0, runs.passing);
  const failing = variant(1, runs.faultFailing);
  const testOnly = [' M build/review_test.go', 'M  build/k_revod_gate_parts_test.go', ' D build/testdata/k-revod-gate/parts-requests.json',
    ' M README.md', '?? features/steps/k-build-review-parts-unpinned__steps.js', 'R  build/new_test.go', 'build/old_test.go'].join('\0') + '\0';
  if (mode === 'good') {
    const found = [
      ...judgePass(passing, required).map((f) => `passing run: ${f.text}`),
      ...judgePass(passing, [faultTest]).map((f) => `passing run, one test: ${f.text}`),
      ...judgeFail(failing, faultTest).map((f) => `fault run: ${f.text}`),
      ...productionChanges(testOnly).map((p) => `test-only changes: ${p}`),
    ];
    console.log(found.length === 0 ? 'accepted' : `not accepted: ${found.join('; ')}`);
    return found.length === 0 ? 0 : 1;
  }
  const isRow = (l, action, test) => l.includes(`"Action":"${action}"`) &&
    (test === undefined ? !l.includes('"Test":') : l.includes(`"Test":"${test}"`));
  const edit = (lines, f) => lines.flatMap((l) => f(l));
  const heavy = requiredSubtests[0];
  const bad = {
    'a failed test': judgePass(variant(1, edit(runs.passing, (l) => (isRow(l, 'pass', 'TestKReviewVerdictStrandReset') ||
      isRow(l, 'pass') ? [l.replace('"Action":"pass"', '"Action":"fail"')] : [l]))), required),
    'a missing test': judgePass(variant(0, edit(runs.passing, (l) => (l.includes('"Test":"TestAReviewerThatNeverReadsIsStopped"')
      ? [] : [l]))), required),
    'a missing required subtest': judgePass(variant(0, edit(runs.passing, (l) => (l.includes(`"Test":${JSON.stringify(heavy)}`)
      ? [] : [l]))), required),
    'a skipped test': judgePass(variant(0, edit(runs.passing, (l) => (isRow(l, 'pass', 'TestKRevodGatePartsUnchanged')
      ? [l.replace('"Action":"pass"', '"Action":"skip"')] : [l]))), required),
    'a failed extra subtest': judgePass(variant(1, [...runs.passing,
      `{"Action":"run","Package":"${pkg}","Test":"TestKReviewRetrySeparateAllowances/extra"}`,
      `{"Action":"fail","Package":"${pkg}","Test":"TestKReviewRetrySeparateAllowances/extra","Elapsed":0.01}`]), required),
    'a skipped extra subtest': judgePass(variant(0, [...runs.passing,
      `{"Action":"skip","Package":"${pkg}","Test":"TestKReviewVerdictStrandReset/extra","Elapsed":0}`]), required),
    'no test run': judgePass(variant(0, edit(runs.passing, (l) => (l.includes('"Test":') ? [] : [l]))), required),
    'a nonzero exit': judgePass(variant(1, runs.passing), required),
    'no package result': judgePass(variant(0, edit(runs.passing, (l) => (isRow(l, 'pass') ? [] : [l]))), required),
    'a failing run where the test must pass': judgePass(failing, [faultTest]),
    'a passing run where the test must fail': judgeFail(passing, faultTest),
    'a failing run of another test': judgeFail(failing, 'TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval'),
    'a failing run that exited 0': judgeFail(variant(0, runs.faultFailing), faultTest),
    'a failing run without a package result': judgeFail(variant(1, edit(runs.faultFailing, (l) => (isRow(l, 'fail') ? [] : [l]))), faultTest),
    'a changed production file': productionChanges(' M build/material.go\0'),
    'a changed go.mod': productionChanges(' M go.mod\0'),
    'a changed go.sum': productionChanges('M  go.sum\0'),
    'an added production file': productionChanges('?? cmd/kogen-build/extra.go\0'),
    'a production file renamed from a test': productionChanges('R  build/host.go\0build/host_test.go\0'),
    'a production file renamed to a test': productionChanges('R  build/host_test.go\0build/host.go\0'),
  };
  // A judge rejects a variant by finding at least one problem.
  const accepted = Object.keys(bad).filter((k) => bad[k].length === 0);
  console.log(accepted.length === 0 ? 'rejected' : `not rejected: ${accepted.join(', ')}`);
  return accepted.length === 0 ? 1 : 0;
}

if (require.main === module) {
  process.exitCode = control(process.argv[2]);
} else {
  const { Given, When, Then } = require('@cucumber/cucumber');
  const P = 'k-build-review-parts-unpinned: ';
  const timeout = { timeout: 300 * 1000 };
  const state = (world) => {
    world.kbrpu = world.kbrpu || { tests: [], subtests: [], standIn: null };
    return world.kbrpu;
  };
  const standIn = (world, extra) => {
    const s = state(world).standIn;
    if (!s) throw new Error('no 7d005ed6 layout stand-in was defined');
    const dir = scratch();
    const copy = path.join(dir, 'kogen-review');
    const edits = [...s.edits, ...(extra || [])];
    editedCopy(reviewDir(s.version, path.join(dir, 'locate')), copy, edits, s.files, s.version);
    const listed = edits.map((e) => `${e.file}: ${JSON.stringify(e.from)} -> ${JSON.stringify(e.to)}`).join('; ');
    return { mod: modfile(path.join(dir, 'mod'), copy),
      against: `a copy of ${reviewModule} ${s.version} edited (${listed}) with the added ${Object.keys(s.files).join(', ')}` };
  };
  const required = (world) => [...state(world).tests, ...state(world).subtests];

  Given(`${P}the review-parts tests are:`, function (table) {
    state(this).tests = table.hashes().map((row) => row.test);
  });

  Given(`${P}their required subtests are:`, function (table) {
    state(this).subtests = table.hashes().map((row) => row.subtest);
  });

  Given(`${P}the 7d005ed6 layout stand-in is kogen-review {word} with these edits:`, function (version, table) {
    state(this).standIn = { version, edits: table.hashes(), files: {} };
  });

  Given(`${P}the stand-in adds the file {word}:`, function (rel, content) {
    if (!state(this).standIn) throw new Error('no 7d005ed6 layout stand-in was defined');
    state(this).standIn.files[rel] = content;
  });

  When(`${P}go test runs the review-parts tests in package build with the committed go.mod`, timeout, function () {
    state(this).run = goTest(state(this).tests, null, 'the kogen-review version the committed go.mod pins');
  });

  When(`${P}go test runs the review-parts tests in package build against kogen-review {word}`, timeout, function (version) {
    const dir = scratch();
    const mod = modfile(path.join(dir, 'mod'), reviewDir(version, path.join(dir, 'locate')));
    state(this).run = goTest(state(this).tests, mod, `${reviewModule} ${version} from the module cache`);
  });

  When(`${P}go test runs the review-parts tests in package build against the stand-in`, timeout, function () {
    const s = standIn(this);
    state(this).run = goTest(state(this).tests, s.mod, s.against);
  });

  When(`${P}go test runs {word} in package build against the stand-in`, timeout, function (name) {
    const s = standIn(this);
    state(this).run = goTest([name], s.mod, s.against);
  });

  When(`${P}go test runs {word} in package build against the stand-in with this further edit:`, timeout, function (name, table) {
    const s = standIn(this, table.hashes());
    state(this).run = goTest([name], s.mod, s.against);
  });

  Then(`${P}go test reported every review-parts test and required subtest passing, and no test failing or skipped`, function () {
    report(state(this).run, judgePass(state(this).run, required(this)));
  });

  Then(`${P}go test reported {word} passing`, function (name) {
    report(state(this).run, judgePass(state(this).run, [name]));
  });

  Then(`${P}go test reported {word} failing`, function (name) {
    report(state(this).run, judgeFail(state(this).run, name));
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

  Then(`${P}the file {word} does not exist`, function (rel) {
    if (fs.existsSync(path.join(root, rel))) throw new Error(`${rel} exists (${fs.statSync(path.join(root, rel)).size} bytes)`);
  });

  Then(`${P}the file {word} does not contain {string}`, function (rel, text) {
    const body = fs.readFileSync(path.join(root, rel), 'utf8');
    const at = body.indexOf(text);
    if (at >= 0) {
      throw new Error(`${rel} contains ${JSON.stringify(text)} at offset ${at}: ${JSON.stringify(body.slice(Math.max(0, at - 120), at + 120))}`);
    }
  });

  Then(`${P}the candidate changes no non-test Go file, go.mod or go.sum`, function () {
    const r = must(run('git', ['status', '--porcelain=v1', '-z', '--untracked-files=all']), 'listing the changed paths');
    const found = productionChanges(r.stdout);
    if (found.length > 0) {
      throw new Error(`the candidate changes kogen-build production files: ${found.join(', ')}\n` +
        `git status --porcelain=v1:\n${r.stdout.split('\0').join('\n')}`);
    }
  });
}
