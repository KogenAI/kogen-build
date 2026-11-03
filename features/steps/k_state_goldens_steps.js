'use strict';
// Steps of k-state-goldens: the goldens of persisted Build state under build/testdata/state, and
// the build package's new tests run with and without -update in a temporary copy of this
// checkout (harness in k_state_goldens/).
const { Given, When, Then, After } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const { copyCheckout, digests, goTest, relevant, insertSpace, replaceOnce, judges } = require('./k_state_goldens/harness.js');

const root = path.resolve(__dirname, '..', '..');
const goRun = { timeout: 10 * 60 * 1000 };

After(function () {
  if (this.stateCopy) fs.rmSync(this.stateCopy, { recursive: true, force: true });
});

function golden(world, rel) {
  const g = world.stateGoldens[rel];
  if (!g) throw new Error(`${rel} is not one of the state goldens`);
  return g;
}

function checkpoints(world) {
  return Object.keys(world.stateGoldens).filter((g) => g.endsWith('/checkpoint.json'));
}

function reviewRecords(world) {
  return Object.keys(world.stateGoldens).filter((g) => /\/review-\d+\.json$/.test(g));
}

// at is the value at a JSON pointer, or undefined when it is absent.
function at(doc, pointer) {
  let v = doc;
  for (const part of pointer.split('/').slice(1).map((p) => p.replace(/~1/g, '/').replace(/~0/g, '~'))) {
    if (v === null || typeof v !== 'object' || !(part in v)) return undefined;
    v = v[part];
  }
  return v;
}

function fail(problems) {
  if (problems.length > 0) throw new Error(problems.join('\n'));
}

Given('the state goldens of this checkout are:', function (table) {
  this.stateGoldens = {};
  const problems = [];
  for (const [rel] of table.raw()) {
    const file = path.join(root, rel);
    const st = fs.lstatSync(file, { throwIfNoEntry: false });
    if (!st || !st.isFile()) {
      problems.push(`${rel} is ${st ? 'not a regular file' : 'missing'}`);
      continue;
    }
    const raw = fs.readFileSync(file, 'utf8');
    let doc;
    try {
      doc = JSON.parse(raw);
    } catch (err) {
      problems.push(`${rel} is not JSON: ${err.message}`);
      continue;
    }
    this.stateGoldens[rel] = { raw, doc };
  }
  fail(problems);
});

Then('each state golden is one line of JSON that ends with a newline', function () {
  const problems = [];
  for (const [rel, g] of Object.entries(this.stateGoldens)) {
    if (!g.raw.endsWith('\n') || g.raw.indexOf('\n') !== g.raw.length - 1) {
      problems.push(`${rel} has ${g.raw.split('\n').length - 1} newlines, want exactly one, at the end`);
    }
  }
  fail(problems);
});

Then('each checkpoint golden has the top-level keys {string}', function (keys) {
  const want = keys.split(', ');
  fail(checkpoints(this).filter((rel) => Object.keys(golden(this, rel).doc).join(', ') !== want.join(', '))
    .map((rel) => `${rel} has the top-level keys ${Object.keys(golden(this, rel).doc).join(', ')}, want ${want.join(', ')}`));
});

Then('the state of each checkpoint golden starts with the keys:', function (table) {
  const want = table.raw().map((r) => r[0]);
  fail(checkpoints(this).map((rel) => [rel, Object.keys(golden(this, rel).doc.state || {}).slice(0, want.length)])
    .filter(([, got]) => got.join(', ') !== want.join(', '))
    .map(([rel, got]) => `${rel}: the state starts with ${got.join(', ')}, want ${want.join(', ')}`));
});

Then('each review record golden has the keys {string}', function (keys) {
  fail(reviewRecords(this).filter((rel) => Object.keys(golden(this, rel).doc).join(', ') !== keys)
    .map((rel) => `${rel} has the keys ${Object.keys(golden(this, rel).doc).join(', ')}, want ${keys}`));
});

Then('the state goldens hold these values:', function (table) {
  const problems = [];
  for (const row of table.hashes()) {
    const got = at(golden(this, row.golden).doc, row.pointer);
    if (JSON.stringify(got) !== JSON.stringify(JSON.parse(row.value))) {
      problems.push(`${row.golden} ${row.pointer} is ${got === undefined ? 'absent' : JSON.stringify(got)}, want ${row.value}`);
    }
  }
  fail(problems);
});

Then('in each review checkpoint golden the last pass, the last review, the reviewer session and its only attempt name the same tree', function () {
  const problems = [];
  for (const rel of checkpoints(this).filter((g) => g.includes('/review-'))) {
    const s = golden(this, rel).doc.state;
    const attempts = at(s, '/continuation/review/attempts');
    if (!Array.isArray(attempts) || attempts.length !== 1) {
      problems.push(`${rel}: the reviewer session has ${JSON.stringify(attempts)} attempts, want one`);
      continue;
    }
    const trees = [at(s, '/last_pass/tree'), at(s, '/last_review/tree'), at(s, '/continuation/review/tree'), attempts[0].tree];
    if (typeof trees[0] !== 'string' || trees[0] === '' || trees.some((t) => t !== trees[0])) {
      problems.push(`${rel}: last pass, last review, session and attempt trees are ${JSON.stringify(trees)}, want one tree`);
    }
  }
  fail(problems);
});

Then('each review record golden has one attempt, and its tree and the attempt\'s tree are the same 40-hex Git tree id', function () {
  const problems = [];
  for (const rel of reviewRecords(this)) {
    const doc = golden(this, rel).doc;
    if (!Array.isArray(doc.attempts) || doc.attempts.length !== 1) {
      problems.push(`${rel} has attempts ${JSON.stringify(doc.attempts)}, want one`);
    } else if (!/^[0-9a-f]{40}$/.test(doc.tree) || doc.attempts[0].tree !== doc.tree) {
      problems.push(`${rel} has tree ${JSON.stringify(doc.tree)} and attempt tree ${JSON.stringify(doc.attempts[0].tree)}, want the same 40-hex tree id`);
    }
  }
  fail(problems);
});

Then('{string} is {string} resumed: the same envelope, and the same state except {string}', function (resumed, from, except) {
  const r = golden(this, resumed).doc;
  const a = golden(this, from).doc;
  const problems = [];
  for (const k of Object.keys(a).filter((key) => key !== 'state')) {
    if (JSON.stringify(r[k]) !== JSON.stringify(a[k])) problems.push(`${resumed} ${k} is ${JSON.stringify(r[k])}, want ${JSON.stringify(a[k])} as in ${from}`);
  }
  const skip = except.split(', ');
  const without = (state) => Object.fromEntries(Object.entries(state || {}).filter(([k]) => !skip.includes(k)));
  const rs = JSON.stringify(without(r.state));
  const as = JSON.stringify(without(a.state));
  if (rs !== as) problems.push(`${resumed}'s state without ${except} differs from ${from}'s:\n got: ${rs.slice(0, 1500)}\nwant: ${as.slice(0, 1500)}`);
  fail(problems);
});

Then('the parent and the root run of {string} are the run id of {string}, and its own run id differs', function (resumed, from) {
  const r = golden(this, resumed).doc.state || {};
  const id = at(golden(this, from).doc, '/state/run/id');
  const problems = [];
  if (typeof id !== 'string' || id === '') problems.push(`${from} has run id ${JSON.stringify(id)}`);
  if (r.parent !== id) problems.push(`${resumed} parent is ${JSON.stringify(r.parent)}, want ${JSON.stringify(id)}`);
  if (r.root_run !== id) problems.push(`${resumed} root_run is ${JSON.stringify(r.root_run)}, want ${JSON.stringify(id)}`);
  if (at(r, '/run/id') === id) problems.push(`${resumed} has the run id ${id} of ${from}, want a new one`);
  fail(problems);
});

Given('a scratch copy of this checkout for the state goldens', function () {
  const goldens = Object.keys(this.stateGoldens);
  this.stateCopy = copyCheckout();
  this.stateDigests = digests(root, goldens);
  const copied = digests(this.stateCopy, goldens);
  fail(goldens.filter((g) => copied[g] !== this.stateDigests[g]).map((g) => `${g} in the copy has digest ${copied[g]}, want ${this.stateDigests[g]}`));
});

Given('in the copy one space is inserted after the first {string} of each state golden', function (brace) {
  if (brace !== '{') throw new Error(`only "{" is supported, not ${JSON.stringify(brace)}`);
  for (const g of Object.keys(this.stateGoldens)) insertSpace(this.stateCopy, g);
  this.stateDrifted = digests(this.stateCopy, Object.keys(this.stateGoldens));
});

Given('in the copy\'s {string} {string} is replaced by {string}', function (rel, from, to) {
  replaceOnce(this.stateCopy, rel, from, to);
});

When('the new tests of the build package run in the copy with {string}', goRun, function (command) {
  const words = command.split(' ');
  if (words.slice(0, 3).join(' ') !== 'go test ./build') throw new Error(`not a go test ./build command: ${command}`);
  this.stateRun = goTest(this.stateCopy, words.slice(3));
});

function check(world, phase, record) {
  const expected = { goldens: Object.keys(world.stateGoldens), digests: world.stateDigests };
  const problems = judges[phase](expected, record);
  if (problems.length > 0) {
    const lines = record.output.trimEnd().split('\n');
    throw new Error([
      `${record.command} (in ${world.stateCopy}), exit ${record.exit}: ${problems.join('; ')}`,
      'relevant lines of its output:', ...relevant(record.output),
      'last lines of its output:', ...lines.slice(-20).map((l) => (l.length > 400 ? `${l.slice(0, 400)}...` : l)),
    ].join('\n'));
  }
}

Then('the run fails, its output names each state golden, and no state golden in the copy changed', function () {
  check(this, 'drifted', { ...this.stateRun, before: this.stateDrifted, after: digests(this.stateCopy, Object.keys(this.stateGoldens)) });
});

Then('the run exits 0, no test fails, and every state golden in the copy has the checkout\'s bytes again', function () {
  check(this, 'updated', { ...this.stateRun, after: digests(this.stateCopy, Object.keys(this.stateGoldens)) });
});

Then('the run fails and its output names:', function (table) {
  check(this, 'changed', { ...this.stateRun, goldens: table.raw().map((r) => r[0]) });
});

Then('no Go file outside tests, go.mod or go.sum differs from HEAD in this checkout', function () {
  const st = spawnSync('git', ['status', '--porcelain=v1', '-z', '--untracked-files=all'], { cwd: root, encoding: 'utf8' });
  if (st.status !== 0) throw new Error(`git status in ${root} failed (exit ${st.status}): ${st.stderr}`);
  const entries = st.stdout.split('\0').filter(Boolean);
  const changed = [];
  for (let i = 0; i < entries.length; i++) {
    const status = entries[i].slice(0, 2);
    const rel = entries[i].slice(3);
    if (status[0] === 'R' || status[0] === 'C') changed.push(entries[++i]);
    changed.push(rel);
  }
  fail(changed.filter((rel) => (rel.endsWith('.go') && !rel.endsWith('_test.go')) || rel === 'go.mod' || rel === 'go.sum')
    .map((rel) => `${rel} differs from HEAD`));
});
