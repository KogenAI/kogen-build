'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert');

const DRIVER = path.join('features', 'steps', 'reject-relative-recovery-output__driver', 'acceptance_test.go.txt');
const COPY = path.join('build', 'zz_reject_relative_recovery_output_acceptance_test.go');

let ran;

Given('reject-relative-recovery-output: the kogen-build checkout', function () {
  assert.ok(fs.existsSync(path.join(process.cwd(), 'build', 'recover.go')), 'build/recover.go is missing');
  assert.ok(fs.existsSync(path.join(process.cwd(), DRIVER)), DRIVER + ' is missing');
});

When('reject-relative-recovery-output: the driver test {string} runs in the build package', { timeout: 115000 }, function (name) {
  const copy = path.join(process.cwd(), COPY);
  fs.copyFileSync(path.join(process.cwd(), DRIVER), copy);
  try {
    const r = spawnSync('go', ['test', '-count=1', '-timeout', '100s', '-run', '^' + name + '$', '-v', './build'], {
      cwd: process.cwd(),
      encoding: 'utf8',
      env: { ...process.env, GOFLAGS: '-trimpath' },
      timeout: 112000,
    });
    ran = { name, status: r.status, out: (r.stdout || '') + (r.stderr || '') };
  } finally {
    fs.rmSync(copy, { force: true });
  }
});

function passed(what) {
  assert.ok(ran, 'the driver test did not run');
  const msg = what + ': go test -run ^' + ran.name + '$ ./build exit ' + ran.status + ':\n' + ran.out.slice(0, 4000);
  assert.ok(ran.out.includes('--- PASS: ' + ran.name + ' '), 'test ' + ran.name + ' did not report PASS; ' + msg);
  assert.strictEqual(ran.status, 0, msg);
}

Then('reject-relative-recovery-output: the refusal names the path and nothing was written', function () { passed('relative output'); });
Then('reject-relative-recovery-output: recovery completes and the archive holds the candidate', function () { passed('absolute output'); });
