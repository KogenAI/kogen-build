'use strict';
// Steps of k-golden-update-build: the build package's golden tests run with and without the
// test-only -update flag in a temporary copy of this checkout (harness in k_golden_update_build/).
const { Given, When, Then, After } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const { copyCheckout, digests, insertDrift, goTest, judges } = require('./k_golden_update_build/harness.js');

const root = path.resolve(__dirname, '..', '..');
const goRun = { timeout: 10 * 60 * 1000 };

After(function () {
  if (this.goldenCopy) fs.rmSync(this.goldenCopy, { recursive: true, force: true });
});

function expected(world) {
  return { tests: world.goldenTests, digests: world.goldenDigests };
}

// testResults is the result each golden test reported in a `go test -v` run (the top-level
// `--- PASS/FAIL/SKIP: <Test> (` line), or "did not run" when the output has none.
function testResults(world, output) {
  return world.goldenTests.map((t) => {
    const m = new RegExp(`^--- (PASS|FAIL|SKIP): ${t} \\(`, 'm').exec(output);
    return `  ${t}: ${m ? `ran, ${m[1]}` : 'did not run'}`;
  });
}

function check(world, phase, record) {
  const problems = judges[phase](expected(world), record);
  if (problems.length > 0) {
    const lines = record.output.trimEnd().split('\n');
    const runLines = lines.filter((l) => /^=== RUN\s+Test\w+$/.test(l));
    const resultLines = lines.filter((l) => /^\s*--- (PASS|FAIL|SKIP): |^(ok|FAIL|PASS)\b/.test(l));
    throw new Error([
      `${record.command} (in ${world.goldenCopy}), exit ${record.exit}: ${problems.join('; ')}`,
      'result of each golden test in this run:', ...testResults(world, record.output),
      'test runs started in its output:', ...runLines,
      'result lines of its output:', ...resultLines,
      'last lines of its output:', ...lines.slice(-15).map((l) => (l.length > 400 ? `${l.slice(0, 400)}...` : l)),
    ].join('\n'));
  }
}

Given('a scratch copy of this checkout, whose goldens have these SHA-256 digests:', function (table) {
  this.goldenDigests = Object.fromEntries(table.hashes().map((r) => [r.golden, r.sha256]));
  this.goldenCopy = copyCheckout();
  const got = digests(this.goldenCopy, Object.keys(this.goldenDigests));
  const wrong = Object.keys(this.goldenDigests).filter((g) => got[g] !== this.goldenDigests[g]);
  if (wrong.length > 0) {
    throw new Error(wrong.map((g) => `${g} has digest ${got[g]}, want ${this.goldenDigests[g]}`).join('\n'));
  }
});

Given('the golden tests are:', function (table) {
  this.goldenTests = table.raw().map((r) => r[0]);
});

Given('in the copy {string} is inserted at the start of the first string value of each golden\'s key:', function (text, table) {
  const rows = table.hashes();
  const missing = Object.keys(this.goldenDigests).filter((g) => !rows.some((r) => r.golden === g));
  if (missing.length > 0) throw new Error(`no drift for ${missing.join(', ')}`);
  for (const r of rows) insertDrift(this.goldenCopy, r.golden, r.key, text);
  this.drifted = digests(this.goldenCopy, Object.keys(this.goldenDigests));
});

function run(world, command, env) {
  const words = command.split(' ');
  if (words[0] !== 'go' || words[1] !== 'test') throw new Error(`not a go test command: ${command}`);
  world.goldenRun = goTest(world.goldenCopy, words.slice(1), world.goldenTests, env);
}

When('{string} runs in the copy', goRun, function (command) {
  run(this, command, {});
});

When('{string} runs in the copy with {word} set to {string}', goRun, function (command, name, value) {
  run(this, command, { [name]: value });
});

Then('the run fails, each golden test reports FAIL, and no golden in the copy changed', function () {
  check(this, 'drifted', { ...this.goldenRun, before: this.drifted, after: digests(this.goldenCopy, Object.keys(this.goldenDigests)) });
});

Then('the run exits 0, each golden test reports PASS or SKIP, and every golden in the copy has its recorded digest again', function () {
  check(this, 'updated', { ...this.goldenRun, after: digests(this.goldenCopy, Object.keys(this.goldenDigests)) });
});

Then('the run exits 0 and each golden test reports PASS', function () {
  check(this, 'plain', this.goldenRun);
});

Then('the non-test code of the module imports no {string} package and no non-test Go file contains any of:', goRun, function (pkg, table) {
  const problems = [];
  const list = spawnSync('go', ['list', '-f', '{{.ImportPath}}: {{join .Imports " "}}', './...'], { cwd: root, encoding: 'utf8' });
  if (list.status !== 0) throw new Error(`go list ./... in ${root} failed (exit ${list.status}):\n${list.stdout}${list.stderr}`);
  for (const line of list.stdout.trim().split('\n')) {
    const [p, imports] = line.split(': ');
    if ((imports || '').split(' ').includes(pkg)) problems.push(`${p} imports ${pkg} outside its tests`);
  }
  const texts = table.raw().map((r) => r[0]);
  const files = spawnSync('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard', '*.go'], { cwd: root, encoding: 'utf8' });
  if (files.status !== 0) throw new Error(`git ls-files in ${root} failed (exit ${files.status}): ${files.stderr}`);
  for (const rel of files.stdout.split('\0').filter(Boolean)) {
    if (rel.endsWith('_test.go') || rel.split('/').includes('testdata') || !fs.existsSync(path.join(root, rel))) continue;
    const body = fs.readFileSync(path.join(root, rel), 'utf8');
    for (const t of texts) if (body.includes(t)) problems.push(`${rel} contains ${t}`);
  }
  if (problems.length > 0) throw new Error(problems.join('\n'));
});

Then('README.md documents {string}', function (text) {
  const readme = fs.readFileSync(path.join(root, 'README.md'), 'utf8');
  if (!readme.includes(text)) throw new Error(`README.md does not contain ${JSON.stringify(text)}`);
});
