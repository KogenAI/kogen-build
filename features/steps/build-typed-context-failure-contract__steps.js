'use strict';
// Acceptance of build-typed-context-failure-contract.
// The Build edits only the test TestDiagnosticDataIsBoundedOriginalUTF8 in build/unit_test.go. The steps
// observe it from outside: each step runs that one test with `go test -overlay`, which swaps in a variant of
// build/context.go from a scratch directory outside the checkout (the checkout is never written), and checks
// whether the test, as it is on disk, passes or fails on the variant. A rejection counts only when the runner
// printed `--- FAIL: TestDiagnosticDataIsBoundedOriginalUTF8`; a compile error is not a rejection.
// Every assertion message starts with what the runner reported for the test (its --- PASS/--- FAIL line and
// the package result line), so a failure shows that the test ran and how it ended.
const {Then} = require('@cucumber/cucumber');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const P = 'build-typed-context-failure-contract: ';
const TEST = 'TestDiagnosticDataIsBoundedOriginalUTF8';
const DETAIL = 'expected bounded nonblank UTF-8 diagnostic data with an absolute source label and matching SHA-256';
const FAIL_LINE = new RegExp('^--- FAIL: ' + TEST + ' \\(', 'm');
const PASS_LINE = new RegExp('^--- PASS: ' + TEST + ' \\(', 'm');

// Each variant maps the text of build/context.go to the text of a variant with one property changed.
const VARIANTS = {
  'a plain error that only spells intent/context_unavailable': (s) =>
    s.replace(/func contextUnavailable\(\) \*sys\.Error \{[\s\S]*?\n\}\n/,
      'func contextUnavailable() error {\n\treturn fmt.Errorf("intent/context_unavailable: %s", "' + DETAIL + '")\n}\n'),
  'failure class build': (s) => s.replace('sys.Fail("intent", "context_unavailable"', 'sys.Fail("build", "context_unavailable"'),
  'failure kind context_missing': (s) => s.replace('sys.Fail("intent", "context_unavailable"', 'sys.Fail("intent", "context_missing"'),
  'exit code 1': (s) => s.replace(DETAIL + '", 2)', DETAIL + '", 1)'),
  'size counted in runes': (s) => s.replace('len(d.Text) <= maxDiagnostic', 'utf8.RuneCountInString(d.Text) <= maxDiagnostic'),
  'text UTF-8 not checked': (s) => s.replace('ok := utf8.ValidString(d.Text) && ', 'ok := '),
  'text containing a carriage return refused': (s) =>
    s.replace('strings.TrimSpace(d.Text) != ""', 'strings.TrimSpace(d.Text) != "" && !strings.Contains(d.Text, "\\r")'),
  'different failure wording': (s) => s.replace(DETAIL, 'diagnostic data is unusable'),
};

const owned = [];
process.on('exit', () => { for (const d of owned) fs.rmSync(d, {recursive: true, force: true}); });

function scratch() {
  const dir = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'btcfc-')));
  owned.push(dir);
  return dir;
}

function goEnv() {
  const out = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith('KOGEN_ACCEPTANCE_')) out[k] = v;
  return out;
}

// runFocused runs the focused test once with context.go replaced by the named variant (or untouched for null).
// `report` is the runner's own result lines, to start every assertion message with.
function runFocused(variant) {
  const root = fs.realpathSync(process.cwd());
  const original = fs.readFileSync(path.join(root, 'build/context.go'), 'utf8');
  const args = ['test', '-vet=off', '-count=1', '-json', '-run', '^' + TEST + '$'];
  if (variant !== null) {
    assert.ok(VARIANTS[variant], `${P}unknown variant ${variant}`);
    const changed = VARIANTS[variant](original);
    assert.notEqual(changed, original, `${P}variant "${variant}" no longer applies to build/context.go`);
    const dir = scratch();
    fs.writeFileSync(path.join(dir, 'context.go'), changed);
    fs.writeFileSync(path.join(dir, 'overlay.json'), JSON.stringify({Replace: {[path.join(root, 'build/context.go')]: path.join(dir, 'context.go')}}));
    args.push('-overlay=' + path.join(dir, 'overlay.json'));
  }
  args.push('./build');
  const r = spawnSync('go', args, {cwd: root, env: goEnv(), encoding: 'utf8', maxBuffer: 64 * 1024 * 1024, timeout: 540000});
  // -json rows carry the -v text in Output; decode it for the PASS/FAIL lines and keep the raw rows in `shown`.
  const text = (r.stdout || '').split('\n').map((l) => {
    try { const row = JSON.parse(l); return typeof row.Output === 'string' ? row.Output : ''; } catch (e) { return l ? l + '\n' : ''; }
  }).join('');
  const out = text + (r.stderr || '');
  const report = out.split('\n').filter((l) => /^(--- (PASS|FAIL)|ok|FAIL|PASS)\b/.test(l)).map((l) => l.trim()).join(' | ') || 'no result line';
  const shown = `go ${args.join(' ')} exited ${r.status}${r.error ? ' ' + r.error.message : ''}:\n${(r.stdout || '').slice(-8000)}${(r.stderr || '').slice(-8000)}`;
  return {status: r.status, out, report, shown};
}

function variants(table) {
  return table.raw().map((row) => row[0]);
}

Then(P + 'the focused test passes on the Build as it is', {timeout: 600000}, function () {
  const r = runFocused(null);
  assert.equal(r.status, 0, `${P}${r.report}: ${TEST} must pass on the unchanged Build\n${r.shown}`);
  assert.match(r.out, PASS_LINE, `${P}${r.report}: ${TEST} did not run and pass\n${r.shown}`);
});

Then(P + 'the focused test rejects each of these Build variants:', {timeout: 600000}, function (table) {
  for (const name of variants(table)) {
    const r = runFocused(name);
    assert.notEqual(r.status, 0, `${P}${r.report}: ${TEST} ran and accepted the variant "${name}"\n${r.shown}`);
    assert.match(r.out, FAIL_LINE, `${P}${r.report}: ${TEST} did not fail as a test on the variant "${name}" (compile error or not run)\n${r.shown}`);
  }
});

Then(P + 'the focused test still passes on each of these Build variants:', {timeout: 600000}, function (table) {
  for (const name of variants(table)) {
    const r = runFocused(name);
    assert.equal(r.status, 0, `${P}${r.report}: ${TEST} failed on the variant "${name}"\n${r.shown}`);
    assert.match(r.out, PASS_LINE, `${P}${r.report}: ${TEST} did not run and pass on the variant "${name}"\n${r.shown}`);
  }
});

// The slash in the file name is escaped: an unescaped `/` separates alternatives in a Cucumber Expression.
Then(P + 'build\\/unit_test.go is the only file changed outside this Intent', function () {
  const root = fs.realpathSync(process.cwd());
  const r = spawnSync('git', ['status', '--porcelain', '--untracked-files=all'], {cwd: root, env: goEnv(), encoding: 'utf8'});
  assert.equal(r.status, 0, `${P}git status failed: ${r.stderr}`);
  const changed = r.stdout.split('\n').filter(Boolean).map((l) => l.slice(3).replace(/^"|"$/g, ''))
    .filter((p) => !p.startsWith('.kogen/') && !p.startsWith('features/steps/build-typed-context-failure-contract__'));
  const extra = changed.filter((p) => p !== 'build/unit_test.go');
  assert.deepEqual(extra, [], `${P}files changed besides build/unit_test.go: ${extra.join(', ')}`);
});
