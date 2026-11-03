'use strict';
// Acceptance of k-review-cost-wide. The step overlays the harness
// features/steps/k_review_cost_wide_test.go.txt into ./build as an internal test file, builds the
// build test binary once (go test -c -vet=off -overlay) and runs the harness test
// TestKReviewCostWideAcceptance once per fixture mode. The harness runs a whole Build with the
// package's scripted Claude Code builder and reviewer and prints one KRCW observation line; this
// file compares it with the feature's expected values.
//
// Run directly as `node k_review_cost_wide_steps.js --control good|bad`, it qualifies its own
// comparison: the recorded harness runs in k_review_cost_wide_controls/ (good: a prototype of the
// feature; bad: the unchanged tree, failed runs and altered observations) against the expected
// values of the feature file.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const TEST = 'TestKReviewCostWideAcceptance';
const HARNESS = 'features/steps/k_review_cost_wide_test.go.txt';
const TARGET = 'build/k_review_cost_wide_acceptance_test.go';
const PREFIX = 'KRCW ';
const FEATURE = '.kogen/intents/k-review-cost-wide/acceptance.feature';
const CONTROLS = 'features/steps/k_review_cost_wide_controls';

// The fixture each mode's harness builds, as the feature describes it.
const FIXTURES = {
  hunk: 'fixtures/golden.json has 15000 lines and the candidate regenerates line 7500',
  rewrite: 'fixtures/a.wire.json and fixtures/b.wire.json have 5000 lines each and the candidate rewrites every line of both',
  custody: 'the "hunk" fixture, and the reviewer of part 1 overwrites review-1-part-1.txt',
  inline: 'fixtures/small.json has 50 lines and the candidate regenerates line 25',
};

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
  const root = process.cwd();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'krcw-'));
  owned.push(dir);
  const overlay = path.join(dir, 'overlay.json');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {[TARGET]: HARNESS}}));
  const bin = path.join(dir, 'build.test');
  const args = ['test', '-c', '-vet=off', '-o', bin, '-overlay', overlay, './build'];
  const r = spawnSync('go', args, {cwd: root, env: env({}), encoding: 'utf8', timeout: 100000, maxBuffer: 16 * 1024 * 1024});
  try {
    assert.equal(r.status, 0, `building the harness of ${TEST}: go ${args.join(' ')} exited ${r.status}${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error.message : ''}:\n${r.stdout}${r.stderr}`);
  } catch (e) {
    buildFailure = e;
    throw e;
  }
  built = bin;
  return bin;
}

function run(mode) {
  const bin = binary();
  const args = ['-test.run', '^' + TEST + '$', '-test.count=1', '-test.v', '-test.timeout=60s'];
  const r = spawnSync(bin, args, {cwd: path.join(process.cwd(), 'build'), env: env({KRCW_MODE: mode}), encoding: 'utf8', timeout: 90000, maxBuffer: 64 * 1024 * 1024});
  return {status: r.status, signal: r.signal, error: r.error ? r.error.message : null, stdout: r.stdout || '', stderr: r.stderr || '',
    command: `KRCW_MODE=${mode} build.test ${args.join(' ')} (built with go test -c -vet=off -overlay ./build)`};
}

// parse is the observation of one harness run: the run must have passed the harness test once
// and printed exactly one observation line of the mode.
function parse(r, mode) {
  const shown = `${TEST} (${r.command}) exited ${r.status}${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error : ''}:\n${r.stdout.slice(0, 20000)}${r.stderr.slice(0, 20000)}`;
  assert.equal(r.error, null, shown);
  assert.equal(r.status, 0, shown);
  assert.equal(r.stdout.split('\n').filter(l => l.startsWith('--- PASS: ' + TEST + ' ')).length, 1, `${TEST} did not pass exactly once: ${shown}`);
  const lines = r.stdout.split('\n').filter(l => l.startsWith(PREFIX));
  assert.equal(lines.length, 1, `${TEST} printed ${lines.length} observation lines, want 1: ${shown}`);
  const obs = JSON.parse(lines[0].slice(PREFIX.length));
  assert.equal(obs.mode, mode, `${TEST} observed mode ${obs.mode}, want ${mode}`);
  return obs;
}

// include checks every expected key against the observation: JSON values compared semantically,
// arrays in order.
function include(obs, expected, mode) {
  for (const [key, want] of Object.entries(expected)) {
    assert.ok(Object.prototype.hasOwnProperty.call(obs, key), `mode ${mode}: the observation has no ${key}: ${JSON.stringify(obs).slice(0, 4000)}`);
    assert.deepStrictEqual(obs[key], want, `mode ${mode}: observation ${key} is ${JSON.stringify(obs[key])}, want ${JSON.stringify(want)}`);
  }
}

function containsText(obs, key, text, mode) {
  assert.equal(typeof obs[key], 'string', `mode ${mode}: observation ${key} is not a string: ${JSON.stringify(obs[key])}`);
  assert.ok(obs[key].includes(text), `mode ${mode}: observation ${key} lacks ${JSON.stringify(text)}; it is ${JSON.stringify(obs[key])}`);
}

function fixture(mode, description) {
  assert.ok(Object.prototype.hasOwnProperty.call(FIXTURES, mode), `unknown review-cost fixture ${mode}`);
  assert.equal(description, FIXTURES[mode], `the feature describes fixture ${mode} as ${JSON.stringify(description)}; the harness builds ${JSON.stringify(FIXTURES[mode])}`);
}

if (require.main === module) {
  control(process.argv[2] === '--control' ? process.argv[3] : '');
} else {
  const {Given, When, Then, setDefaultTimeout} = require('@cucumber/cucumber');
  setDefaultTimeout(200 * 1000);
  Given(/^the review-cost fixture "([a-z]+)": (.+)$/, function (mode, description) {
    fixture(mode, description);
    this.krcwMode = mode;
  });
  When('the review-cost harness builds the fixture', function () {
    this.krcwObs = parse(run(this.krcwMode), this.krcwMode);
  });
  Then('the review-cost observations include:', function (doc) {
    include(this.krcwObs, JSON.parse(doc), this.krcwMode);
  });
  Then('the review-cost observation {string} contains:', function (key, doc) {
    containsText(this.krcwObs, key, doc, this.krcwMode);
  });
}

// scenarios reads the feature's scenarios: per scenario its fixture builds in order, each with its
// mode and its expectations.
function scenarios() {
  const lines = fs.readFileSync(FEATURE, 'utf8').split('\n');
  const out = [];
  let cur = null;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (line.startsWith('Scenario:')) { cur = {name: line, groups: []}; out.push(cur); continue; }
    if (!cur) continue;
    let m = line.match(/^Given the review-cost fixture "([a-z]+)": (.+)$/);
    if (m) { cur.groups.push({mode: m[1], description: m[2], checks: []}); continue; }
    m = line.match(/^(?:Then|And) the review-cost (observations include:|observation "([a-z_]+)" contains:)$/);
    if (!m) continue;
    const open = lines[++i];
    const indent = open.indexOf('"""');
    const body = [];
    while (!lines[++i].trim().startsWith('"""')) body.push(lines[i].slice(indent));
    const doc = body.join('\n');
    cur.groups[cur.groups.length - 1].checks.push(m[2] ? {kind: 'contains', key: m[2], doc} : {kind: 'include', doc: JSON.parse(doc)});
  }
  return out;
}

// judge runs one fixture build's expectations on a recorded harness run.
function judge(g, r) {
  fixture(g.mode, g.description);
  const obs = parse(r, g.mode);
  for (const c of g.checks) {
    if (c.kind === 'include') include(obs, c.doc, g.mode);
    else containsText(obs, c.key, c.doc, g.mode);
  }
}

// withObservation is a recorded run whose observation line is replaced by change(observation).
function withObservation(r, change) {
  const stdout = r.stdout.split('\n').map(l => {
    if (!l.startsWith(PREFIX)) return l;
    const obs = JSON.parse(l.slice(PREFIX.length));
    change(obs);
    return PREFIX + JSON.stringify(obs);
  }).join('\n');
  return {...r, stdout};
}

function altered(v) {
  if (typeof v === 'number') return v + 1;
  if (typeof v === 'boolean') return !v;
  if (typeof v === 'string') return v + 'x';
  if (Array.isArray(v)) return v.slice(0, -1);
  const o = {...v};
  const k = Object.keys(o)[0];
  o[k] = altered(o[k]);
  return o;
}

function control(kind) {
  const good = JSON.parse(fs.readFileSync(path.join(CONTROLS, 'good.json'), 'utf8'));
  const bad = JSON.parse(fs.readFileSync(path.join(CONTROLS, 'bad.json'), 'utf8'));
  const list = scenarios();
  // A scenario passes when every fixture build of it passes with its run (runs[i] for group i).
  const passes = (sc, runs) => sc.groups.every((g, i) => {
    try { judge(g, runs[i]); return true; } catch (e) { if (e instanceof assert.AssertionError) return false; throw e; }
  });
  if (kind === 'good') {
    assert.equal(list.length, 3, 'the feature has 3 scenarios');
    assert.equal(list.map(sc => sc.groups.map(g => g.mode).join(',')).join(' '), 'inline,hunk rewrite custody', 'the scenarios build inline then hunk, rewrite, custody');
    for (const sc of list) assert.ok(passes(sc, sc.groups.map(g => good[g.mode])), `the good control of ${sc.name} was rejected`);
    console.log('accepted');
    process.exit(0);
  }
  if (kind === 'bad') {
    const cases = [];
    for (const sc of list) {
      const goods = sc.groups.map(g => good[g.mode]);
      cases.push([`${sc.name}: the unchanged tree`, sc, sc.groups.map(g => bad[g.mode])]);
      sc.groups.forEach((g, i) => {
        const gr = good[g.mode];
        const one = (name, r, scen = sc) => cases.push([`${sc.name} / ${g.mode}: ${name}`, scen, goods.map((x, j) => (j === i ? r : x))]);
        one('a failed run', {...gr, status: 1});
        one('a run that did not start', {...gr, error: 'spawn ENOENT'});
        one('no observation line', {...gr, stdout: gr.stdout.split('\n').filter(l => !l.startsWith(PREFIX)).join('\n')});
        one('two observation lines', {...gr, stdout: gr.stdout + '\n' + gr.stdout.split('\n').find(l => l.startsWith(PREFIX))});
        one('no passing harness test', {...gr, stdout: gr.stdout.replace('--- PASS: ' + TEST + ' ', '--- FAIL: ' + TEST + ' ')});
        one("another mode's observation", withObservation(gr, o => { o.mode = g.mode === 'inline' ? 'hunk' : 'inline'; }));
        one('another fixture description', gr, {...sc, groups: sc.groups.map((x, j) => (j === i ? {...x, description: x.description + ' and more'} : x))});
        for (const c of g.checks) {
          if (c.kind === 'include') {
            for (const key of Object.keys(c.doc)) {
              one(`${key} altered`, withObservation(gr, o => { o[key] = altered(o[key]); }));
              one(`${key} missing`, withObservation(gr, o => { delete o[key]; }));
            }
          } else {
            one(`${c.key} without the expected text`, withObservation(gr, o => { o[c.key] = o[c.key].replace(c.doc.slice(0, 40), ''); }));
          }
        }
      });
    }
    const accepted = cases.filter(([, sc, runs]) => passes(sc, runs)).map(([name]) => name);
    if (accepted.length > 0) {
      console.log('accepted bad controls: ' + accepted.join('; '));
      process.exit(2);
    }
    console.log('rejected');
    process.exit(1);
  }
  console.log('usage: k_review_cost_wide_steps.js --control good|bad');
  process.exit(64);
}
