'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert');

const DRIVER = path.join('features', 'steps', 'build-no-edit-stop-quotes-output__driver', 'acceptance_test.go.txt');
const COPY = path.join('build', 'zz_build_no_edit_stop_quotes_output_acceptance_test.go');

let ran;

Given('build-no-edit-stop-quotes-output: the kogen-build checkout', function () {
  assert.ok(fs.existsSync(path.join(process.cwd(), 'build', 'noprogress.go')), 'build/noprogress.go is missing');
  assert.ok(fs.existsSync(path.join(process.cwd(), DRIVER)), DRIVER + ' is missing');
});

When('build-no-edit-stop-quotes-output: the Build test {string} runs in the build package', { timeout: 115000 }, function (name) {
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
  assert.ok(ran, 'the test did not run');
  const msg = what + ': go test -run ^' + ran.name + '$ ./build exit ' + ran.status + ':\n' + ran.out.slice(0, 4000);
  assert.ok(ran.out.includes('--- PASS: ' + ran.name + ' '), 'test ' + ran.name + ' did not report PASS; ' + msg);
  assert.strictEqual(ran.status, 0, msg);
}

Then('build-no-edit-stop-quotes-output: the notice quotes the first nonblank output line right after the sentence naming the first failure', function () { passed('notice quote'); });
Then('build-no-edit-stop-quotes-output: the stop has the output line right after the first failure line and keeps its other lines', function () { passed('stop line'); });
Then('build-no-edit-stop-quotes-output: a 300 character first output line is cut to 200 characters in the notice and in the stop', function () { passed('cut'); });
Then('build-no-edit-stop-quotes-output: the notice and the stop are unchanged when the first failure is an acceptance scenario', function () { passed('acceptance first'); });
Then('build-no-edit-stop-quotes-output: the notice and the stop are unchanged when the failing check has no nonblank output', function () { passed('no output'); });
