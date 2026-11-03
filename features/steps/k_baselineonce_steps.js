'use strict';
// Acceptance of k-baselineonce. The step overlays the harness features/steps/k_baselineonce_test.go.txt
// into ./build as build/k_baselineonce_acceptance_test.go, builds the build package's test binary once
// (go test -c -vet=off -overlay) and runs the harness test TestKBaselineOnceAcceptance once per mode
// (KBO_MODE). The harness runs whole Builds and resumes against the package's scripted provider and
// prints one observation line, "KBO <json>"; this file compares that observation, as a JSON value, with
// the one the feature expects.
//
// Run directly as `node features/steps/k_baselineonce_steps.js --control good|bad`, it qualifies its own
// comparison with recorded harness observations in k_baselineonce_controls/: good.json holds one
// observation per mode recorded from a prototype of the feature, and every one must be accepted; bad.json
// holds observations recorded on the unchanged tree and altered observations of plausible wrong
// implementations, and every one must be rejected.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const TEST = 'TestKBaselineOnceAcceptance';
const HARNESS = 'features/steps/k_baselineonce_test.go.txt';
const TARGET = 'build/k_baselineonce_acceptance_test.go';
const PREFIX = 'KBO ';
const FEATURE = '.kogen/intents/k-baselineonce/acceptance.feature';
const CONTROLS = 'features/steps/k_baselineonce_controls';

const owned = [];
process.on('exit', () => { for (const d of owned) fs.rmSync(d, {recursive: true, force: true}); });

function env(extra) {
  const out = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith('KOGEN_ACCEPTANCE_')) out[k] = v;
  return {...out, ...extra};
}

let built, buildFailure;
function binary() {
  if (built) return built;
  if (buildFailure) throw buildFailure; // one build per run: later scenarios fail fast with its failure
  const root = fs.realpathSync(process.cwd());
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kbo-'));
  owned.push(dir);
  const overlay = path.join(dir, 'overlay.json');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {[path.join(root, TARGET)]: path.join(root, HARNESS)}}));
  const bin = path.join(dir, 'build.test');
  const args = ['test', '-c', '-vet=off', '-o', bin, '-overlay', overlay, './build'];
  const r = spawnSync('go', args, {cwd: root, env: env({PWD: root}), encoding: 'utf8', timeout: 600000, maxBuffer: 16 * 1024 * 1024});
  try {
    assert.equal(r.status, 0, `building the harness of ${TEST}: go ${args.join(' ')} exited ${r.status}${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error.message : ''}:\n${r.stdout}${r.stderr}`);
  } catch (e) {
    buildFailure = e;
    throw e;
  }
  built = bin;
  return bin;
}

// observe runs the harness in one mode: the run must pass the harness test exactly once and print
// exactly one observation line of that mode.
function observe(mode) {
  const bin = binary();
  const root = fs.realpathSync(process.cwd());
  const args = ['-test.run', '^' + TEST + '$', '-test.count=1', '-test.v', '-test.timeout=300s'];
  const r = spawnSync(bin, args, {cwd: path.join(root, 'build'), env: env({KBO_MODE: mode}), encoding: 'utf8', timeout: 330000, maxBuffer: 64 * 1024 * 1024});
  const stdout = r.stdout || '', stderr = r.stderr || '';
  const shown = `${TEST} (KBO_MODE=${mode} build.test ${args.join(' ')}, built with go test -c -vet=off -overlay ./build) exited ${r.status}` +
    `${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error.message : ''}:\n${stdout.slice(-20000)}${stderr.slice(-20000)}`;
  assert.equal(r.error, undefined, shown);
  assert.equal(r.status, 0, shown);
  assert.equal(stdout.split('\n').filter(l => l.startsWith('--- PASS: ' + TEST + ' ')).length, 1, `${TEST} did not pass exactly once: ${shown}`);
  const lines = stdout.split('\n').filter(l => l.startsWith(PREFIX));
  assert.equal(lines.length, 1, `${TEST} printed ${lines.length} observation lines, want 1: ${shown}`);
  const got = JSON.parse(lines[0].slice(PREFIX.length));
  assert.equal(got.mode, mode, `${TEST} observed mode ${got.mode}, want ${mode}: ${shown}`);
  return got;
}

// compare judges an observation against the feature's expected value: equal JSON values, with
// object keys in any order and arrays in order.
function compare(mode, got, want) {
  assert.deepStrictEqual(got, want, `mode ${mode}: observed\n${JSON.stringify(got, null, 2)}\nwant\n${JSON.stringify(want, null, 2)}`);
}

// expectations are the feature's expected observations by mode.
function expectations() {
  const text = fs.readFileSync(FEATURE, 'utf8');
  const re = /the baseline-once harness in mode "([^"]+)" observes:\s*\n\s*"""\n([\s\S]*?)\n\s*"""/g;
  const out = {};
  let m;
  while ((m = re.exec(text)) !== null) out[m[1]] = JSON.parse(m[2]);
  return out;
}

function control(kind) {
  const want = expectations();
  const records = JSON.parse(fs.readFileSync(path.join(CONTROLS, kind + '.json'), 'utf8'));
  let accepted = 0, rejected = 0;
  for (const rec of records) {
    assert.ok(want[rec.mode], `control ${kind}: the feature has no expectation for mode ${rec.mode}`);
    try {
      compare(rec.mode, rec.observation, want[rec.mode]);
      accepted++;
      if (kind === 'bad') console.error(`known-bad observation accepted: ${rec.mode}: ${rec.why}`);
    } catch (e) {
      if (!(e instanceof assert.AssertionError)) throw e;
      rejected++;
      if (kind === 'good') console.error(`known-good observation rejected: ${rec.mode}: ${e.message}`);
    }
  }
  if (kind === 'good') {
    assert.deepEqual(records.map(r => r.mode).sort(), Object.keys(want).sort(), 'control good: one recorded observation per expected mode');
    return rejected === 0;
  }
  return accepted > 0;
}

if (require.main === module) {
  const kind = process.argv[2] === '--control' ? process.argv[3] : '';
  if (kind !== 'good' && kind !== 'bad') {
    console.log('usage: node features/steps/k_baselineonce_steps.js --control good|bad');
    process.exit(2);
  }
  let ok;
  try {
    ok = control(kind);
  } catch (e) {
    console.log('control error: ' + e.message);
    process.exit(2);
  }
  console.log(ok ? 'observer accepted' : 'observer rejected');
  process.exitCode = ok ? 0 : 1;
} else {
  const {Then} = require('@cucumber/cucumber');
  Then('the baseline-once harness in mode {string} observes:', {timeout: 1200000}, function (mode, doc) {
    compare(mode, observe(mode), JSON.parse(doc));
  });
}
