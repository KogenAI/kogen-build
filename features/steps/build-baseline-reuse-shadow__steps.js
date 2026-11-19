'use strict';
// Copies the checkout (without .git and node_modules) to a scratch directory, adds the Go acceptance
// test of the shadow to its build package and runs it once, with its subtests, under `go test -v`.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { Given, Then } = require('@cucumber/cucumber');

const P = 'build-baseline-reuse-shadow: ';
const ROOT = process.cwd();
const TEST = 'TestReuseShadowAcceptance';
const OLD_PIN = 'v0.0.0-20261111114253-4a379220bdb1';
let cached;

function run() {
  if (cached) return cached;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'reuse-shadow-'));
  const copy = spawnSync('sh', ['-c', 'tar --exclude=.git --exclude=node_modules -cf - . | tar -xf - -C "$1"', 'sh', dir], { cwd: ROOT, encoding: 'utf8' });
  assert.equal(copy.status, 0, `copying the checkout failed: ${copy.stderr}`);
  fs.copyFileSync(path.join(ROOT, 'features/steps/build-baseline-reuse-shadow__tests/shadow_acceptance.go.tmpl'), path.join(dir, 'build', 'shadow_acceptance_test.go'));
  const command = `go test ./build -run '^${TEST}$' -count=1 -v`;
  const r = spawnSync('sh', ['-c', command], { cwd: dir, encoding: 'utf8', timeout: 110000, maxBuffer: 1 << 26 });
  cached = { command, status: r.status, output: (r.stdout || '') + (r.stderr || '') };
  return cached;
}

Given(P + 'the Go acceptance test of the shadow has run in a copy of the checkout', { timeout: 115000 }, function () {
  this.shadow = run();
});

Then(P + 'the subtest {string} passed', function (sub) {
  const r = this.shadow;
  const line = `--- PASS: ${TEST}/${sub} `;
  assert.ok(r.output.includes(line), `${TEST}/${sub} did not pass (${r.command}, exit ${r.status}):\n${r.output.slice(-3000)}`);
});

Then(P + 'go.mod pins another kogen-verification version than the previous one', function () {
  const mod = fs.readFileSync(path.join(ROOT, 'go.mod'), 'utf8');
  const m = mod.match(/github\.com\/KogenAI\/kogen-verification (\S+)/);
  assert.ok(m, 'go.mod has no kogen-verification requirement');
  assert.notEqual(m[1], OLD_PIN, `go.mod still pins kogen-verification ${OLD_PIN}`);
});

Then(P + 'the kogen-verification consumes suite mentions the reuse key or snapshot', function () {
  const dir = path.join(ROOT, 'consumes', 'kogen-verification');
  const files = fs.readdirSync(dir).filter((f) => f.endsWith('.go'));
  const hit = files.some((f) => /reuse|snapshot/i.test(fs.readFileSync(path.join(dir, f), 'utf8')));
  assert.ok(hit, `no .go file in consumes/kogen-verification (${files.join(', ')}) mentions reuse or snapshot`);
});
