'use strict';
// Acceptance of k-resetcarry. The step overlays the harness features/steps/k_resetcarry_test.go.txt
// into ./build as an internal test file, builds the build test binary once (go test -c -vet=off
// -overlay) and runs the harness test TestKResetCarryAcceptance once per fixture mode. The
// harness runs a whole Build against the package's scripted provider, forces one working-window
// reset of the native reviewer or builder and prints one KRC observation line; this file compares
// it with the feature's expected values.
//
// Run directly as `node k_resetcarry_steps.js --control good|bad`, it qualifies its own
// comparison: the recorded harness runs in k_resetcarry_controls/ (good: a prototype of the
// feature; bad: the unchanged tree, failed runs and altered observations) against the expected
// values of the feature file.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const TEST = 'TestKResetCarryAcceptance';
const HARNESS = 'features/steps/k_resetcarry_test.go.txt';
const TARGET = 'build/k_resetcarry_acceptance_test.go';
const PREFIX = 'KRC ';
const FEATURE = '.kogen/intents/k-resetcarry/acceptance.feature';
const CONTROLS = 'features/steps/k_resetcarry_controls';

// The fixture each mode's harness builds, as the feature describes it.
const FIXTURES = {
  'review-on': 'the read ledger on, the reviewer\'s first response reads fixtures/pad-1.txt, pad-2.txt and pad-3.txt, and after the reset it reads part 1 again',
  'review-off': 'the read ledger off, the reviewer\'s first response reads fixtures/pad-1.txt, pad-2.txt and pad-3.txt, and after the reset it reads part 1 again',
  'build-on': 'the read ledger on, the builder reads fixtures/pad-1.txt, pad-2.txt and pad-3.txt and writes two 300000-byte scratch files over two responses',
  'build-off': 'the read ledger off, the builder reads fixtures/pad-1.txt, pad-2.txt and pad-3.txt and writes two 300000-byte scratch files over two responses',
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
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'krc-'));
  owned.push(dir);
  const overlay = path.join(dir, 'overlay.json');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {[TARGET]: HARNESS}}));
  const bin = path.join(dir, 'build.test');
  const args = ['test', '-c', '-vet=off', '-o', bin, '-overlay', overlay, './build'];
  const r = spawnSync('go', args, {cwd: root, env: env({}), encoding: 'utf8', timeout: 150000, maxBuffer: 16 * 1024 * 1024});
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
  const args = ['-test.run', '^' + TEST + '$', '-test.count=1', '-test.v', '-test.timeout=120s'];
  const r = spawnSync(bin, args, {cwd: path.join(process.cwd(), 'build'), env: env({KRC_MODE: mode}), encoding: 'utf8', timeout: 150000, maxBuffer: 64 * 1024 * 1024});
  return {status: r.status, signal: r.signal, error: r.error ? r.error.message : null, stdout: r.stdout || '', stderr: r.stderr || '',
    command: `KRC_MODE=${mode} build.test ${args.join(' ')} (built with go test -c -vet=off -overlay ./build)`};
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

function fixture(mode, description) {
  assert.ok(Object.prototype.hasOwnProperty.call(FIXTURES, mode), `unknown reset-carry fixture ${mode}`);
  assert.equal(description, FIXTURES[mode], `the feature describes fixture ${mode} as ${JSON.stringify(description)}; the harness builds ${JSON.stringify(FIXTURES[mode])}`);
}

if (require.main === module) {
  control(process.argv[2] === '--control' ? process.argv[3] : '');
} else {
  const {Given, When, Then, setDefaultTimeout} = require('@cucumber/cucumber');
  setDefaultTimeout(300 * 1000);
  Given(/^the reset-carry fixture "([a-z-]+)": (.+)$/, function (mode, description) {
    fixture(mode, description);
    this.krcMode = mode;
  });
  When('the reset-carry harness builds the fixture', function () {
    this.krcObs = parse(run(this.krcMode), this.krcMode);
  });
  Then('the reset-carry observations include:', function (doc) {
    include(this.krcObs, JSON.parse(doc), this.krcMode);
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
    let m = line.match(/^Given the reset-carry fixture "([a-z-]+)": (.+)$/);
    if (m) { cur.groups.push({mode: m[1], description: m[2], checks: []}); continue; }
    if (!/^(?:Then|And) the reset-carry observations include:$/.test(line)) continue;
    const open = lines[++i];
    const indent = open.indexOf('"""');
    const body = [];
    while (!lines[++i].trim().startsWith('"""')) body.push(lines[i].slice(indent));
    cur.groups[cur.groups.length - 1].checks.push(JSON.parse(body.join('\n')));
  }
  return out;
}

// judge runs one fixture build's expectations on a recorded harness run.
function judge(g, r) {
  fixture(g.mode, g.description);
  const obs = parse(r, g.mode);
  for (const doc of g.checks) include(obs, doc, g.mode);
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

// altered is v changed in its first leaf: a number plus one, a boolean negated, a string extended,
// an array without its last element, an empty array with one element.
function altered(v) {
  if (v === null) return 0;
  if (typeof v === 'number') return v + 1;
  if (typeof v === 'boolean') return !v;
  if (typeof v === 'string') return v + 'x';
  if (Array.isArray(v)) return v.length > 0 ? v.slice(0, -1) : [{}];
  const o = {...v};
  const k = Object.keys(o)[0];
  o[k] = altered(o[k]);
  return o;
}

// leaves are the paths of the leaves of an expected value: object keys and array indexes.
function leaves(v, at = []) {
  if (v !== null && typeof v === 'object' && Object.keys(v).length > 0) {
    return Object.keys(v).flatMap(k => leaves(v[k], [...at, Array.isArray(v) ? Number(k) : k]));
  }
  return [at];
}

// changedAt is obs with the leaf at the path altered (or removed when remove is set).
function changedAt(obs, at, remove) {
  const copy = JSON.parse(JSON.stringify(obs));
  let node = copy;
  for (const k of at.slice(0, -1)) {
    if (node === null || typeof node !== 'object' || !(k in node)) return copy;
    node = node[k];
  }
  const last = at[at.length - 1];
  if (node === null || typeof node !== 'object' || !(last in node)) return copy;
  if (remove) {
    if (Array.isArray(node)) node.splice(last, 1); else delete node[last];
  } else {
    node[last] = altered(node[last]);
  }
  return copy;
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
    assert.equal(list.length, 4, 'the feature has 4 scenarios');
    assert.equal(list.map(sc => sc.groups.map(g => g.mode).join(',')).join(' '), 'review-on review-off build-on build-off', 'the scenarios build review-on, review-off, build-on, build-off');
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
        one("another mode's observation", withObservation(gr, o => { o.mode = g.mode.endsWith('-on') ? g.mode.replace(/-on$/, '-off') : g.mode.replace(/-off$/, '-on'); }));
        one('another fixture description', gr, {...sc, groups: sc.groups.map((x, j) => (j === i ? {...x, description: x.description + ' and more'} : x))});
        // The other mode of the same conversation: an implementation that ignores the option.
        const twin = g.mode.endsWith('-on') ? g.mode.replace(/-on$/, '-off') : g.mode.replace(/-off$/, '-on');
        one(`the ${twin} run`, withObservation(good[twin], o => { o.mode = g.mode; o.option = g.mode.endsWith('-on') ? 'on' : 'off'; }));
        for (const doc of g.checks) {
          for (const at of leaves(doc)) {
            const name = at.join('.');
            one(`${name} altered`, withObservation(gr, o => { Object.assign(o, changedAt(o, at, false)); }));
            one(`${name} missing`, withObservation(gr, o => {
              const changed = changedAt(o, at, true);
              for (const k of Object.keys(o)) delete o[k];
              Object.assign(o, changed);
            }));
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
  console.log('usage: k_resetcarry_steps.js --control good|bad');
  process.exit(64);
}
