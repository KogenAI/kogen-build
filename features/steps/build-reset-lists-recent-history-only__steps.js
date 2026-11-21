'use strict';
// Copies the checkout (without .git and node_modules) to a scratch directory, adds the Go acceptance
// test of the reset history to its build package and runs it once, with its subtests, together with
// the existing golden reset test.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { Given, Then } = require('@cucumber/cucumber');

const P = 'build-reset-lists-recent-history-only: ';
const ROOT = process.cwd();
const TEST = 'TestResetHistoryAcceptance';
const GOLDEN = 'TestTheLocalResetEqualsTheElixirReference';
let cached;

function run() {
  if (cached) return cached;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'reset-history-'));
  const copy = spawnSync('sh', ['-c', 'tar --exclude=.git --exclude=node_modules -cf - . | tar -xf - -C "$1"', 'sh', dir], { cwd: ROOT, encoding: 'utf8' });
  assert.equal(copy.status, 0, `copying the checkout failed: ${copy.stderr}`);
  fs.copyFileSync(path.join(ROOT, 'features/steps/build-reset-lists-recent-history-only__tests/reset_history_acceptance.go.tmpl'), path.join(dir, 'build', 'reset_history_acceptance_test.go'));
  const command = `go test ./build -run '^(${TEST}|${GOLDEN})$' -count=1 -v`;
  const r = spawnSync('sh', ['-c', command], { cwd: dir, encoding: 'utf8', timeout: 110000, maxBuffer: 1 << 26 });
  cached = { command, status: r.status, output: (r.stdout || '') + (r.stderr || '') };
  return cached;
}

Given(P + 'the Go acceptance test of the reset history has run in a copy of the checkout', { timeout: 115000 }, function () {
  this.reset = run();
});

Then(P + 'the subtest {string} passed', function (sub) {
  const r = this.reset;
  const line = `--- PASS: ${TEST}/${sub} `;
  assert.ok(r.output.includes(line), `${TEST}/${sub} did not pass (${r.command}, exit ${r.status}):\n${r.output.slice(-3000)}`);
});

Then(P + 'the existing golden reset test passed', function () {
  const r = this.reset;
  const line = `--- PASS: ${GOLDEN} `;
  assert.ok(r.output.includes(line), `${GOLDEN} did not pass (${r.command}, exit ${r.status}):\n${r.output.slice(-3000)}`);
});
