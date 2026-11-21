// Steps of build-restore-used-survives-resume: they copy the Intent's Go driver into build/ as a
// temporary _test.go file, run it by name with the go tool and remove it again.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { When, Then } = require('@cucumber/cucumber');

const P = 'build-restore-used-survives-resume: ';
const LONG = { timeout: 280000 };
const ROOT = path.resolve(__dirname, '..', '..');
const DRIVER = path.join(__dirname, 'build-restore-used-survives-resume__driver', 'restore_used.go.txt');

When(P + 'I run the driver test {string}', LONG, function (name) {
  const target = path.join(ROOT, 'build', 'zz_restore_used_driver_test.go');
  fs.copyFileSync(DRIVER, target);
  try {
    const args = ['test', '-count=1', '-v', '-run', `^${name}$`, './build'];
    const r = spawnSync('go', args, { cwd: ROOT, encoding: 'utf8', timeout: 270000, env: { ...process.env, GOFLAGS: '-trimpath' } });
    this.restoreRun = { name, command: `go ${args.join(' ')}`, status: r.status, output: (r.stdout || '') + (r.stderr || '') };
  } finally {
    fs.rmSync(target, { force: true });
  }
});

Then(P + 'that driver test ran and passed', function () {
  const run = this.restoreRun;
  const where = `${run.command}, exit ${run.status}`;
  assert.ok(run.output.includes(`--- PASS: ${run.name} `), `${run.name} did not pass (${where})\n${run.output.slice(-3000)}`);
  assert.equal(run.status, 0, `${run.name}: non-zero exit (${where})\n${run.output.slice(-3000)}`);
});

Then(P + 'the persisted state type declares the JSON key {string}', function (key) {
  const src = fs.readFileSync(path.join(ROOT, 'build', 'core_state.go'), 'utf8');
  assert.ok(src.includes('`json:"' + key + '"`'), `build/core_state.go declares no field with the exact JSON tag ${key} (without omitempty)`);
});
