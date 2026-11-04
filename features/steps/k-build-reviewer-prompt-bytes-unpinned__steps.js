'use strict';
// Steps of k-build-reviewer-prompt-bytes-unpinned: kogen-build's TestContextExternalNativeBytes runs
// with `go test -json` in package build, with the committed go.mod or against a kogen-review version
// from the module cache, or against a scratch copy of one with listed text edits. Either way a
// temporary go.mod passed with -modfile replaces kogen-review by that folder, so no checkout file
// changes. The golden is checked part by part. A failure lists each required test whose result
// differs, with its own `--- PASS/FAIL` line and first message, then the command, its exit status,
// the run's `go test -json` rows (other than output) and the tail of what go test printed. A golden
// that `-update` wrote differently is reported with the same command, status and rows.
//
// `node features/steps/k-build-reviewer-prompt-bytes-unpinned__steps.js good|bad` runs the run judge
// and the golden judge on known inputs. good prints "accepted" and exits 0 when they accept the real
// passing run, the real run against the diagnostic-dropping kogen-review (both in
// k-build-reviewer-prompt-bytes-unpinned__controls.js) and today's golden rewritten as the Intent
// specifies. bad prints "rejected" and exits 1 when they reject every refused variant of those.
const { spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..');
const reviewModule = 'github.com/KogenAI/kogen-review';
const pkg = 'github.com/KogenAI/kogen-build/build';
const testName = 'TestContextExternalNativeBytes';
const goldenPath = 'build/testdata/context-external-golden.json';
const subtests = ['native/native_without_context', 'native/native_with_context', 'native/external_without_context',
  'native/external_with_context', 'external/native_without_context', 'external/external_without_context',
  'external/native_with_context', 'external/external_with_context'];

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
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-build-reviewer-prompt-bytes-unpinned-'));
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

// copyCheckout copies the checkout's tracked and untracked, not ignored files to dest.
function copyCheckout(dest) {
  const listed = must(run('git', ['ls-files', '-z', '-co', '--exclude-standard']), 'listing the checkout files').stdout;
  for (const rel of new Set(listed.split('\0').filter(Boolean))) {
    const src = path.join(root, rel);
    let st;
    try {
      st = fs.lstatSync(src);
    } catch {
      continue;
    }
    const dst = path.join(dest, rel);
    fs.mkdirSync(path.dirname(dst), { recursive: true });
    if (st.isSymbolicLink()) {
      fs.symlinkSync(fs.readlinkSync(src), dst);
    } else if (st.isFile()) {
      fs.copyFileSync(src, dst);
      fs.chmodSync(dst, st.mode & 0o777);
    }
  }
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

// goTest runs `go test [-modfile=…] -json -count=1 -run ^name$ ./build [extra…]` in cwd.
function goTest(name, cwd, mod, against, extra) {
  const args = ['test', ...(mod ? [`-modfile=${mod}`] : []), '-json', '-count=1', '-run', `^${name}$`, './build', ...(extra || [])];
  const r = run('go', args, { cwd, timeout: 240 * 1000 });
  const shown = `go test${mod ? ' -modfile=<temporary go.mod>' : ''} -json -count=1 -run '^${name}$' ./build${extra ? ` ${extra.join(' ')}` : ''}`;
  return parse(r.status, r.stdout, r.stderr, r.error, `${shown} in ${cwd} with ${against}`);
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

function report(r, name, parent, expected) {
  const found = judge(r, name, parent, expected);
  if (found.length === 0) return;
  const text = found.map((f) => (f.test ? `${f.text} (${evidence(r, f.test)})` : f.text));
  throw new Error(`${text.join('\n')}\n${runShown(r)}`);
}

function sha256(data) {
  return crypto.createHash('sha256').update(data).digest('hex');
}

// canonical is v as JSON with the keys of every object sorted; array order is kept.
function canonical(v) {
  if (Array.isArray(v)) return `[${v.map(canonical).join(',')}]`;
  if (v && typeof v === 'object') {
    return `{${Object.keys(v).sort().map((k) => `${JSON.stringify(k)}:${canonical(v[k])}`).join(',')}}`;
  }
  return JSON.stringify(v);
}

// fingerprint is the SHA-256 of a part's text or, for a reviewer part, of its JSON with sorted keys.
function fingerprint(part, value) {
  if (typeof value !== 'string') return `not a string: ${JSON.stringify(value)}`.slice(0, 200);
  if (part !== 'reviewer') return sha256(value);
  try {
    return sha256(canonical(JSON.parse(value)));
  } catch (err) {
    return `not JSON: ${err.message}`;
  }
}

// reviewerSummary shows what a reviewer part's instructions and submit_review entry hold.
function reviewerSummary(value) {
  try {
    const r = JSON.parse(value);
    const tool = (r.tools || []).find((t) => t && (t.name === 'submit_review' || t.review_tool_definition === 'submit_review'));
    return `instructions ${JSON.stringify(String(r.instructions).slice(0, 80))}, submit_review entry ${JSON.stringify(tool).slice(0, 160)}`;
  } catch {
    return 'not JSON';
  }
}

// partsProblems lists how the golden text differs from exactly the expected parts, given as rows
// {entry, part, sha256}.
function partsProblems(text, expected) {
  let golden;
  try {
    golden = JSON.parse(text);
  } catch (err) {
    return [`the golden is not JSON: ${err.message}`];
  }
  const found = [];
  const want = {};
  for (const row of expected) (want[row.entry] = want[row.entry] || {})[row.part] = row.sha256;
  for (const entry of Object.keys(golden)) if (!(entry in want)) found.push(`entry ${JSON.stringify(entry)}: not expected`);
  for (const [entry, parts] of Object.entries(want)) {
    const got = golden[entry];
    if (!got || typeof got !== 'object') {
      found.push(`entry ${JSON.stringify(entry)}: missing`);
      continue;
    }
    for (const part of Object.keys(got)) if (!(part in parts)) found.push(`${entry} ${part}: not an expected part`);
    for (const [part, sum] of Object.entries(parts)) {
      if (!(part in got)) {
        found.push(`${entry} ${part}: missing`);
        continue;
      }
      const f = fingerprint(part, got[part]);
      if (f !== sum) {
        found.push(`${entry} ${part}: fingerprint ${f}, want ${sum}${part === 'reviewer' ? ` (${reviewerSummary(got[part])})` : ''}`);
      }
    }
  }
  return found;
}

// forbiddenProblems lists each of texts that body contains, with where.
function forbiddenProblems(body, texts) {
  const found = [];
  for (const text of texts) {
    const at = body.indexOf(text);
    if (at >= 0) found.push(`contains ${JSON.stringify(text)} at offset ${at}: ${JSON.stringify(body.slice(Math.max(0, at - 80), at + 80))}`);
  }
  return found;
}

// ---- the controls ----

// The golden of kogen-build 36ce493 (git blob a3db31e…), before this change.
const todayDigest = '9b753a918827ef61e9072fea1e8af6ec132aed43b64f3a5a921e104cfe698790';
const todayBlob = 'a3db31e16ed35670739db3d538318beaecc01278';

function todayGolden() {
  const file = path.join(root, goldenPath);
  if (fs.existsSync(file) && sha256(fs.readFileSync(file)) === todayDigest) return fs.readFileSync(file, 'utf8');
  return must(run('git', ['cat-file', 'blob', todayBlob]), 'reading the golden of kogen-build 36ce493').stdout;
}

// rewritten is a golden as the Intent specifies it: an external reviewer's parts dropped, and in a
// native reviewer part the instructions and the submit_review definition replaced by placeholders.
function rewritten(text, keep) {
  const out = {};
  for (const [entry, parts] of Object.entries(JSON.parse(text))) {
    out[entry] = {};
    for (const [part, value] of Object.entries(parts)) {
      if (part.startsWith('reviewer.') && !(keep && keep.stable)) continue;
      if (part !== 'reviewer') {
        out[entry][part] = value;
        continue;
      }
      const r = JSON.parse(value);
      if (!(keep && keep.instructions)) r.instructions = '<review.ReviewerInstructions()>';
      if (!(keep && keep.definition)) {
        r.tools = r.tools.map((t) => (t && t.name === 'submit_review' ? { review_tool_definition: 'submit_review' } : t));
      }
      out[entry][part] = JSON.stringify(r);
    }
  }
  return out;
}

const show = (golden) => `${JSON.stringify(golden, null, 2)}\n`;

// The parts table and the reviewer wording of the acceptance's golden scenario.
const expectedParts = [
  ['external/external without context', 'builder.dynamic', 'a19da629adb2062c7086e21e30e53dc61a34543baeb165599aaafebed9760a2b'],
  ['external/external without context', 'builder.stable', '9b7646d93a41bc3e74ba32e1e69f2f5f71a3491192c6de185477bb0da7ea718b'],
  ['external/native without context', 'builder.dynamic', 'a19da629adb2062c7086e21e30e53dc61a34543baeb165599aaafebed9760a2b'],
  ['external/native without context', 'builder.stable', '9b7646d93a41bc3e74ba32e1e69f2f5f71a3491192c6de185477bb0da7ea718b'],
  ['external/native without context', 'reviewer', '4c384b391441b9e29e106298d9184484dc94e9ee1be167f81b368ab84638d686'],
  ['native/external with context', 'builder', 'ab61779f5f078b833420282712ec96264c7e840e744512d89c1b3add4f60d615'],
  ['native/external without context', 'builder', '4f5a54d8659fa59b4099e356389ee0afc28ada34bd327d380176b02545a17b02'],
  ['native/native with context', 'builder', 'ab61779f5f078b833420282712ec96264c7e840e744512d89c1b3add4f60d615'],
  ['native/native with context', 'reviewer', '6927575fff3d00419edf95d7e5b04dfe44220d2f56293f7cbc13541f27407e96'],
  ['native/native without context', 'builder', '4f5a54d8659fa59b4099e356389ee0afc28ada34bd327d380176b02545a17b02'],
  ['native/native without context', 'reviewer', '4c384b391441b9e29e106298d9184484dc94e9ee1be167f81b368ab84638d686'],
].map(([entry, part, sum]) => ({ entry, part, sha256: sum }));
const reviewerWording = ['You review code that another model wrote', 'run your focused probes', 'Limit raises:',
  'Requirements: before any verdict', 'Submit the review:', '# Review task', 'Finish with the structured verdict'];

function control(mode) {
  const runs = require('./k-build-reviewer-prompt-bytes-unpinned__controls.js');
  const all = (result) => Object.fromEntries(subtests.map((s) => [s, result]));
  const dropping = { ...all('pass'), 'native/external_with_context': 'fail' };
  const variant = (status, lines) => parse(status, `${lines.join('\n')}\n`, '', undefined, 'control');
  const passing = variant(0, runs.passing);
  const dropped = variant(1, runs.dropDiagnostic);
  const today = todayGolden();
  const good = show(rewritten(today));
  const golden = (text) => [...partsProblems(text, expectedParts), ...forbiddenProblems(text, reviewerWording)];
  if (mode === 'good') {
    const found = [
      ...judge(passing, testName, 'pass', all('pass')).map((f) => `passing run: ${f.text}`),
      ...judge(dropped, testName, 'fail', dropping).map((f) => `dropped-diagnostic run: ${f.text}`),
      ...golden(good).map((f) => `rewritten golden: ${f}`),
    ];
    console.log(found.length === 0 ? 'accepted' : `not accepted: ${found.join('; ')}`);
    return found.length === 0 ? 0 : 1;
  }
  const isRow = (l, action, test) => l.includes(`"Action":"${action}"`) &&
    (test === undefined ? !l.includes('"Test":') : l.includes(`"Test":"${test}"`));
  const sub = `${testName}/native/native_with_context`;
  const edit = (lines, f) => lines.flatMap((l) => f(l));
  const changed = (entry, part, f) => {
    const g = rewritten(today);
    g[entry][part] = f(g[entry][part]);
    return show(g);
  };
  const bad = {
    'failed subtest': judge(variant(1, edit(runs.passing, (l) => (isRow(l, 'pass', sub) || isRow(l, 'pass', testName) ||
      isRow(l, 'pass') ? [l.replace('"Action":"pass"', '"Action":"fail"')] : [l]))), testName, 'pass', all('pass')),
    'missing subtest': judge(variant(0, edit(runs.passing, (l) => (l.includes(`"Test":"${testName}/external/external_with_context"`)
      ? [] : [l]))), testName, 'pass', all('pass')),
    'extra subtest': judge(variant(0, [...runs.passing, `{"Action":"run","Package":"${pkg}","Test":"${testName}/native/extra"}`,
      `{"Action":"pass","Package":"${pkg}","Test":"${testName}/native/extra","Elapsed":0.01}`]), testName, 'pass', all('pass')),
    'skipped test': judge(variant(0, edit(runs.passing, (l) => (isRow(l, 'pass', testName)
      ? [l.replace('"Action":"pass"', '"Action":"skip"')] : [l]))), testName, 'pass', all('pass')),
    'no test run': judge(variant(0, edit(runs.passing, (l) => (l.includes('"Test":') ? [] : [l]))), testName, 'pass', all('pass')),
    'nonzero exit': judge(variant(1, runs.passing), testName, 'pass', all('pass')),
    'no package result': judge(variant(0, edit(runs.passing, (l) => (isRow(l, 'pass') ? [] : [l]))), testName, 'pass', all('pass')),
    'passing run where one subtest must fail': judge(passing, testName, 'fail', dropping),
    'failing run where all must pass': judge(dropped, testName, 'pass', all('pass')),
    'failing run where another subtest must fail': judge(dropped, testName, 'fail',
      { ...all('pass'), 'native/native_with_context': 'fail' }),
    'failing run that exited 0': judge(variant(0, runs.dropDiagnostic), testName, 'fail', dropping),
    "today's golden": golden(today),
    'golden with the reviewer instructions': golden(show(rewritten(today, { instructions: true }))),
    'golden with the submit_review definition': golden(show(rewritten(today, { definition: true }))),
    "golden with the external reviewer's parts": golden(show(rewritten(today, { stable: true }))),
    'golden with a builder byte changed': golden(changed('native/native with context', 'builder', (s) => s.replace('candidate', 'Candidate'))),
    'golden with an extra part': golden(changed('native/external with context', 'reviewer.dynamic', () => 'source')),
    'golden without an entry': golden(show(Object.fromEntries(Object.entries(rewritten(today)).slice(1)))),
    'golden naming reviewer wording outside the parts': forbiddenProblems(`${good}Limit raises:\n`, reviewerWording),
  };
  const accepted = Object.keys(bad).filter((k) => bad[k].length === 0);
  console.log(accepted.length === 0 ? 'rejected' : `not rejected: ${accepted.join(', ')}`);
  return accepted.length === 0 ? 1 : 0;
}

if (require.main === module) {
  process.exitCode = control(process.argv[2]);
} else {
  const { When, Then } = require('@cucumber/cucumber');
  const P = 'k-build-reviewer-prompt-bytes-unpinned: ';
  const timeout = { timeout: 250 * 1000 };

  When(`${P}go test runs {word} in package build with the committed go.mod`, timeout, function (name) {
    this.kbrpbuRun = goTest(name, root, null, 'the kogen-review version the committed go.mod pins');
  });

  When(`${P}go test runs {word} in package build against kogen-review {word}`, timeout, function (name, version) {
    const dir = scratch();
    const mod = modfile(path.join(dir, 'mod'), reviewDir(version, path.join(dir, 'locate')));
    this.kbrpbuRun = goTest(name, root, mod, `${reviewModule} ${version} from the module cache`);
  });

  When(`${P}go test runs {word} in package build against kogen-review {word} with these edits:`, timeout,
    function (name, version, table) {
      const dir = scratch();
      const edits = table.hashes();
      const copy = path.join(dir, 'kogen-review');
      editedCopy(reviewDir(version, path.join(dir, 'locate')), copy, edits, version);
      const mod = modfile(path.join(dir, 'mod'), copy);
      const listed = edits.map((e) => `${e.file}: ${JSON.stringify(e.from)} -> ${JSON.stringify(e.to)}`).join('; ');
      this.kbrpbuRun = goTest(name, root, mod, `a copy of ${reviewModule} ${version} edited (${listed})`);
    });

  When(`${P}go test -update runs {word} in package build of a scratch copy of the checkout against kogen-review {word}`,
    timeout, function (name, version) {
      const dir = scratch();
      const copy = path.join(dir, 'checkout');
      copyCheckout(copy);
      const mod = modfile(path.join(dir, 'mod'), reviewDir(version, path.join(dir, 'locate')));
      this.kbrpbuCopy = copy;
      this.kbrpbuRun = goTest(name, copy, mod, `${reviewModule} ${version} from the module cache`, ['-update']);
    });

  Then(`${P}go test reported {word} as {word} with these subtest results:`, function (name, parent, table) {
    report(this.kbrpbuRun, name, parent, Object.fromEntries(table.hashes().map((row) => [row.subtest, row.result])));
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

  Then(`${P}the golden {word} holds exactly these parts, fingerprinted by the SHA-256 of their text or, for a reviewer part, of its JSON with sorted keys:`,
    function (rel, table) {
      const found = partsProblems(fs.readFileSync(path.join(root, rel), 'utf8'), table.hashes());
      if (found.length > 0) throw new Error(`${rel}:\n${found.join('\n')}`);
    });

  Then(`${P}the file {word} contains none of these texts:`, function (rel, table) {
    const found = forbiddenProblems(fs.readFileSync(path.join(root, rel), 'utf8'), table.hashes().map((row) => row.text));
    if (found.length > 0) throw new Error(`${rel}:\n${found.join('\n')}`);
  });

  Then(`${P}in the scratch copy, the file {word} has the same bytes as in the checkout`, function (rel) {
    const r = this.kbrpbuRun;
    const want = fs.readFileSync(path.join(root, rel));
    let got;
    try {
      got = fs.readFileSync(path.join(this.kbrpbuCopy, rel));
    } catch {
      throw new Error(`${rel} is missing in the scratch copy after ${testName} ran with -update\n${runShown(r)}`);
    }
    if (Buffer.compare(got, want) !== 0) {
      let i = 0;
      while (i < got.length && i < want.length && got[i] === want[i]) i++;
      throw new Error(`${rel} written by ${testName} with -update differs from the checkout's at byte ${i} ` +
        `(${got.length} bytes, want ${want.length}):\n got ...${JSON.stringify(got.subarray(Math.max(0, i - 80), i + 80).toString())}\n` +
        `want ...${JSON.stringify(want.subarray(Math.max(0, i - 80), i + 80).toString())}\n${runShown(r)}`);
    }
  });
}
