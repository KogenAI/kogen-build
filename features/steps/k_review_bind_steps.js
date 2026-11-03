'use strict';
// Acceptance of k-review-bind. The harness features/steps/k_review_bind/bind_test.go.txt is
// overlaid into ./build as build/zz_k_review_bind_acceptance_test.go (go test -c -vet=off
// -overlay from a temporary folder, so no file of the checkout changes), the test binary is
// built once per run, and the harness test TestKReviewBindAcceptance runs one real Build per
// case and prints one "KRB <json>" observation line, which this file compares with the
// feature's expected values.
//
// Run directly, it qualifies itself:
//   node features/steps/k_review_bind_steps.js --control good|bad
//     judges the recorded observations in k_review_bind/controls/ against the feature's
//     scenarios (good: one recorded run of a prototype of the fix; bad: the unchanged tree and
//     altered observations of plausible wrong implementations). It exits 0 when every scenario
//     accepts the good records, 1 when every bad variant is rejected, 3 otherwise.
//   node features/steps/k_review_bind_steps.js --live <case> <scenario-case>
//     runs the harness case and judges it with the expectations of the scenario of
//     scenario-case: exit 0 accepted, 1 rejected, 2 when the harness cannot run.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const ROOT = path.resolve(__dirname, '..', '..');
const TEST = 'TestKReviewBindAcceptance';
const HARNESS = path.join(ROOT, 'features/steps/k_review_bind/bind_test.go.txt');
const TARGET = path.join(ROOT, 'build/zz_k_review_bind_acceptance_test.go');
const FEATURE = path.join(ROOT, '.kogen/intents/k-review-bind/acceptance.feature');
const CONTROLS = path.join(ROOT, 'features/steps/k_review_bind/controls');
const PREFIX = 'KRB ';

const owned = [];
process.on('exit', () => { for (const d of owned) fs.rmSync(d, {recursive: true, force: true}); });

function env(extra) {
  const out = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith('KOGEN_ACCEPTANCE_')) out[k] = v;
  return {...out, PWD: ROOT, ...extra};
}

let built, buildFailure;
function binary() {
  if (built) return built;
  if (buildFailure) throw buildFailure; // one build per run: later scenarios fail fast with its failure
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-review-bind-'));
  owned.push(dir);
  const overlay = path.join(dir, 'overlay.json');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {[TARGET]: HARNESS}}));
  const bin = path.join(dir, 'build.test');
  const args = ['test', '-c', '-vet=off', '-o', bin, '-overlay', overlay, './build'];
  const r = spawnSync('go', args, {cwd: ROOT, env: env({}), encoding: 'utf8', timeout: 300000, maxBuffer: 16 << 20});
  if (r.status !== 0) {
    buildFailure = new Error(`building the harness of ${TEST}: go ${args.join(' ')} in ${ROOT} exited ${r.status}${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error.message : ''}:\n${r.stdout}${r.stderr}`);
    throw buildFailure;
  }
  built = bin;
  return bin;
}

// observe runs the harness case once and returns its observation: the harness test must have
// passed exactly once and printed exactly one observation line of that case.
function observe(name) {
  const bin = binary();
  const args = ['-test.run', '^' + TEST + '$', '-test.count=1', '-test.v', '-test.timeout=240s'];
  const r = spawnSync(bin, args, {cwd: path.join(ROOT, 'build'), env: env({KREVIEW_BIND_CASE: name}), encoding: 'utf8', timeout: 300000, maxBuffer: 64 << 20});
  const shown = `${TEST} (KREVIEW_BIND_CASE=${name} build.test ${args.join(' ')}, built with go test -c -vet=off -overlay ./build) exited ${r.status}${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error.message : ''}:\n${(r.stdout || '').slice(-20000)}${(r.stderr || '').slice(-20000)}`;
  if (r.error || r.status !== 0) throw new Error(shown);
  const passes = (r.stdout || '').split('\n').filter(l => l.startsWith('--- PASS: ' + TEST + ' ')).length;
  if (passes !== 1) throw new Error(`${TEST} did not pass exactly once (${passes} passes): ${shown}`);
  const lines = (r.stdout || '').split('\n').filter(l => l.startsWith(PREFIX));
  if (lines.length !== 1) throw new Error(`${TEST} printed ${lines.length} observation lines, want 1: ${shown}`);
  const obs = JSON.parse(lines[0].slice(PREFIX.length));
  if (obs.case !== name) throw new Error(`${TEST} observed case ${obs.case}, want ${name}`);
  return obs;
}

// The judgments: each returns the problems, every one {key, message}.

// include: every expected key is in the observation with a semantically equal JSON value.
function include(obs, expected) {
  const problems = [];
  for (const [key, want] of Object.entries(expected)) {
    if (!Object.prototype.hasOwnProperty.call(obs, key)) {
      problems.push({key, message: `the observation has no ${key}`});
      continue;
    }
    try {
      assert.deepStrictEqual(obs[key], want);
    } catch {
      problems.push({key, message: `observation ${key} is ${JSON.stringify(obs[key])}, want ${JSON.stringify(want)}`});
    }
  }
  return problems;
}

// contains: the observation's text value contains text.
function contains(obs, key, text) {
  if (typeof obs[key] !== 'string') return [{key, message: `observation ${key} is not a string: ${JSON.stringify(obs[key])}`}];
  if (!obs[key].includes(text)) return [{key, message: `observation ${key} lacks ${JSON.stringify(text)}; it is ${JSON.stringify(obs[key])}`}];
  return [];
}

function fail(name, problems) {
  if (problems.length > 0) throw new Error(`case ${name}: ${problems.map(p => p.message).join('; ')}`);
}

// scenarios reads the feature: per scenario its name, its case and its checks in order.
function scenarios() {
  const lines = fs.readFileSync(FEATURE, 'utf8').split('\n');
  const out = [];
  let cur = null;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    let m = line.match(/^Scenario: (.+)$/);
    if (m) { cur = {name: m[1], checks: []}; out.push(cur); continue; }
    if (!cur) continue;
    m = line.match(/^When the review-bind harness runs the case "([a-z-]+)"$/);
    if (m) { cur.case = m[1]; continue; }
    m = line.match(/^(?:Then|And) the review-bind (observations include:|observation "([a-z_]+)" contains:)$/);
    if (!m) continue;
    const open = lines[++i];
    const indent = open.indexOf('"""');
    const body = [];
    while (!lines[++i].trim().startsWith('"""')) body.push(lines[i].slice(indent));
    const doc = body.join('\n');
    cur.checks.push(m[2] ? {kind: 'contains', key: m[2], doc} : {kind: 'include', doc: JSON.parse(doc)});
  }
  return out;
}

function judge(scenario, obs) {
  const problems = [];
  for (const c of scenario.checks) {
    problems.push(...(c.kind === 'include' ? include(obs, c.doc) : contains(obs, c.key, c.doc)));
  }
  return problems;
}

function control(kind) {
  const all = scenarios();
  if (kind === 'good') {
    const good = JSON.parse(fs.readFileSync(path.join(CONTROLS, 'good.json'), 'utf8'));
    const lines = [];
    for (const s of all) {
      const problems = s.case in good ? judge(s, good[s.case]) : [{key: 'case', message: 'no recorded observation'}];
      if (problems.length > 0) lines.push(`scenario "${s.name}": rejected at ${problems[0].key}`);
    }
    if (lines.length > 0) {
      process.stdout.write(`control good: ${lines.length} of ${all.length} scenarios reject the good records\n${lines.join('\n')}\n`);
      process.exit(3);
    }
    process.stdout.write(`control good: all ${all.length} scenarios accept the good records\n`);
    process.exit(0);
  }
  if (kind === 'bad') {
    const variants = JSON.parse(fs.readFileSync(path.join(CONTROLS, 'bad.json'), 'utf8'));
    const lines = [];
    let accepted = 0;
    for (const v of variants) {
      let verdict = 'ACCEPTED';
      for (const s of all) {
        if (!(s.case in v.observations)) continue;
        const problems = judge(s, v.observations[s.case]);
        if (problems.length > 0) { verdict = `rejected by "${s.name}" at ${problems[0].key}`; break; }
      }
      if (verdict === 'ACCEPTED') accepted++;
      lines.push(`variant ${v.variant}: ${verdict}`);
    }
    process.stdout.write(lines.join('\n') + '\n');
    process.exit(accepted === 0 ? 1 : 3);
  }
  process.stdout.write('usage: k_review_bind_steps.js --control good|bad | --live <case> <scenario-case>\n');
  process.exit(2);
}

function live(name, as) {
  const s = scenarios().find(x => x.case === as);
  if (!s || !name) {
    process.stdout.write(`no scenario runs the case ${as}\n`);
    process.exit(2);
  }
  let obs;
  try {
    obs = observe(name);
  } catch (e) {
    process.stdout.write(`${e.message}\n`);
    process.exit(2);
  }
  const problems = judge(s, obs);
  if (problems.length > 0) {
    process.stdout.write(`live ${name} judged as "${s.name}": rejected at ${problems[0].key}\n`);
    process.exit(1);
  }
  process.stdout.write(`live ${name} judged as "${s.name}": accepted\n`);
  process.exit(0);
}

if (require.main === module) {
  const [flag, a, b] = process.argv.slice(2);
  if (flag === '--live') live(a, b);
  else control(flag === '--control' ? a : '');
} else {
  const {When, Then} = require('@cucumber/cucumber');
  When('the review-bind harness runs the case {string}', {timeout: 600 * 1000}, function (name) {
    this.krbCase = name;
    this.krbObs = observe(name);
  });
  Then('the review-bind observations include:', function (doc) {
    fail(this.krbCase, include(this.krbObs, JSON.parse(doc)));
  });
  Then('the review-bind observation {string} contains:', function (key, doc) {
    fail(this.krbCase, contains(this.krbObs, key, doc));
  });
}
