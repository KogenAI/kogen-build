'use strict';
// Acceptance of k-build-volatile-offsets. The kogen-loop requirement is read from go.mod (go mod
// edit -json) and go.sum. The two named tests run in uncached go test -count=1 runs. The masking
// is observed by the harness features/steps/k_build_volatile_offsets_test.go.txt, overlaid into
// ./build as an internal test file (go test -overlay) and run once; it prints KBVO lines and this
// file compares them with the feature's expected values.
//
// Run directly as `node k_build_volatile_offsets_steps.js --control good|bad`, it qualifies its own
// comparison: the recorded harness runs in k_build_volatile_offsets_controls/ (good: a prototype of
// the feature against kogen-loop main; bad: the unchanged tree, kogen-loop main with the unchanged
// mask, and a mask that also hides numbers and outcome) against the expected values of the feature.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const TEST = 'TestKBuildVolatileOffsetsAcceptance';
const HARNESS = 'features/steps/k_build_volatile_offsets_test.go.txt';
const TARGET = 'build/k_build_volatile_offsets_acceptance_test.go';
const FEATURE = '.kogen/intents/k-build-volatile-offsets/acceptance.feature';
const CONTROLS = 'features/steps/k_build_volatile_offsets_controls';
const TABLE_STEP = 'gives these observeBuild comparisons:';

function env() {
  const out = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith('KOGEN_ACCEPTANCE_')) out[k] = v;
  return out;
}

function run(cmd, args, opts) {
  const r = spawnSync(cmd, args, {cwd: process.cwd(), env: env(), encoding: 'utf8', maxBuffer: 64 * 1024 * 1024, ...opts});
  const output = (r.stdout || '') + (r.stderr || '');
  const shown = `${cmd} ${args.join(' ')} exited ${r.status}${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error.message : ''}`;
  return {status: r.status, stdout: r.stdout || '', output, shown};
}

function replaceOf(old) {
  const r = run('go', ['mod', 'edit', '-json'], {timeout: 60000});
  assert.equal(r.status, 0, `${r.shown}:\n${r.output}`);
  const mod = JSON.parse(r.stdout);
  const found = (mod.Replace || []).filter(x => x.Old.Path === old);
  assert.equal(found.length, 1, `go.mod has ${found.length} replace directives for ${old}: ${JSON.stringify(mod.Replace)}`);
  return found[0].New;
}

// judgeRecord checks the harness's attempt-record line.
function judgeRecord(stdout) {
  const lines = stdout.split('\n').filter(l => l.startsWith('KBVO attempt record '));
  assert.deepEqual(lines, ['KBVO attempt record request_offset_ms=true load_average=true'],
    `the first provider attempt record does not carry both request_offset_ms and load_average:\n${stdout.slice(0, 20000)}`);
}

// judgeTable checks the harness's comparison lines against the expected rows, exactly and in order.
function judgeTable(stdout, rows) {
  const passed = stdout.split('\n').filter(l => l.startsWith('--- PASS: ' + TEST + ' ')).length;
  assert.equal(passed, 1, `${TEST} did not pass exactly once:\n${stdout.slice(0, 20000)}`);
  const got = stdout.split('\n').filter(l => l.startsWith('KBVO ') && !l.startsWith('KBVO attempt record '));
  const want = rows.map(([rewrite, comparison]) => `KBVO ${rewrite} ${comparison}`);
  assert.deepEqual(got, want, `observeBuild comparisons differ from the feature's:\n${stdout.slice(0, 20000)}`);
}

let harness;
function runHarness() {
  if (harness) return harness;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kbvo-'));
  try {
    // Overlay paths are relative to the go command's working directory, the project root.
    const overlay = path.join(dir, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({Replace: {[TARGET]: HARNESS}}));
    const args = ['test', '-vet=off', '-overlay', overlay, '-count=1', '-v', '-timeout=240s', '-run', '^' + TEST + '$', './build'];
    const r = run('go', args, {timeout: 300000});
    harness = {...r, shown: `${r.shown} (harness ${HARNESS} overlaid as ${TARGET})`};
  } finally {
    fs.rmSync(dir, {recursive: true, force: true});
  }
  assert.equal(harness.status, 0, `${harness.shown}:\n${harness.output.slice(0, 20000)}`);
  assert.equal(harness.stdout.split('\n').filter(l => l.startsWith('--- PASS: ' + TEST + ' ')).length, 1,
    `${TEST} did not run and pass exactly once: ${harness.shown}:\n${harness.output.slice(0, 20000)}`);
  return harness;
}

// featureRows reads the expected rows of the comparison table from the feature file.
function featureRows() {
  const lines = fs.readFileSync(FEATURE, 'utf8').split('\n');
  const at = lines.findIndex(l => l.includes(TABLE_STEP));
  assert.ok(at >= 0, `no comparison table in ${FEATURE}`);
  const rows = [];
  for (const l of lines.slice(at + 1)) {
    if (!l.trim().startsWith('|')) break;
    rows.push(l.trim().slice(1, -1).split('|').map(c => c.trim()));
  }
  return rows.slice(1);
}

if (require.main === module) {
  const mode = process.argv[3];
  const names = mode === 'good' ? ['good'] : ['bad-unchanged', 'bad-main-unmasked', 'bad-overmask'];
  let accepted = false;
  for (const name of names) {
    const stdout = fs.readFileSync(path.join(CONTROLS, name + '.txt'), 'utf8');
    try {
      judgeRecord(stdout);
      judgeTable(stdout, featureRows());
      console.log('accepted ' + name);
      accepted = true;
    } catch (e) {
      console.log('rejected ' + name);
    }
  }
  process.exitCode = accepted ? 0 : 1;
} else {
  const {Then} = require('@cucumber/cucumber');

  Then('go.mod replaces {string} with {string} at {string}', {timeout: 60000}, function (old, newPath, version) {
    assert.deepEqual(replaceOf(old), {Path: newPath, Version: version}, `go.mod's replace directive for ${old}`);
  });

  Then('go.sum records {string} and no line for {string}', {timeout: 10000}, function (line, gone) {
    const sum = fs.readFileSync('go.sum', 'utf8').split('\n');
    assert.ok(sum.includes(line), `go.sum lacks the line ${line}`);
    assert.deepEqual(sum.filter(l => l.includes(gone)), [], `go.sum still has lines for ${gone}`);
  });

  Then('{string} and {string} each pass {int} times in a row in .\\/build', {timeout: 1800000}, function (a, b, times) {
    // One uncached go test -count=1 per run: a -count=N result can be replayed from the test cache.
    const args = ['test', '-count=1', '-v', '-timeout=600s', '-run', `^(${a}|${b})$`, './build'];
    for (let n = 1; n <= times; n++) {
      const r = run('go', args, {timeout: 600000});
      const shown = `run ${n} of ${times}: ${r.shown}:\n${r.output.slice(-20000)}`;
      const lines = r.stdout.split('\n');
      for (const name of [a, b]) {
        const passed = lines.filter(l => l.startsWith(`--- PASS: ${name} (`)).length;
        const failed = lines.filter(l => l.startsWith(`--- FAIL: ${name} (`)).length;
        assert.equal(failed, 0, `${name} failed in ${shown}`);
        assert.equal(passed, 1, `${name} ${passed === 0 ? 'did not run' : 'passed ' + passed + ' times'} in ${shown}`);
      }
      assert.equal(r.status, 0, shown);
      assert.ok(!r.output.includes('(cached)'), `a cached result in ${shown}`);
    }
  });

  Then('the first provider attempt record of a Build carries {string} and {string}', {timeout: 320000}, function (offset, load) {
    assert.deepEqual([offset, load], ['request_offset_ms', 'load_average']);
    judgeRecord(runHarness().stdout);
  });

  Then('rewriting one value of a Build\'s first provider attempt record gives these observeBuild comparisons:', {timeout: 320000}, function (table) {
    judgeTable(runHarness().stdout, table.rows());
  });
}
