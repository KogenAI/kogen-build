'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert');

const DRIVER = path.join('features', 'steps', 'build-env-red-no-edit-host-stop__driver', 'acceptance_test.go.txt');
const COPY = path.join('build', 'zz_build_env_red_no_edit_host_stop_acceptance_test.go');

let ran;

Given('build-env-red-no-edit-host-stop: the kogen-build checkout', function () {
  assert.ok(fs.existsSync(path.join(process.cwd(), 'build', 'noprogress.go')), 'build/noprogress.go is missing');
  assert.ok(fs.existsSync(path.join(process.cwd(), DRIVER)), DRIVER + ' is missing');
});

When('build-env-red-no-edit-host-stop: the Build test {string} runs in the build package', { timeout: 115000 }, function (name) {
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

Then('build-env-red-no-edit-host-stop: the Build stops as environment host_or_base_failure with exit 3 after two passes and no no-edit notice', function () { passed('timeout stop'); });
Then('build-env-red-no-edit-host-stop: the Build gives the ordinary feedback of pass 2 and stops for neither the host nor no_progress', function () { passed('ordinary second pass'); });
Then('build-env-red-no-edit-host-stop: the first no-edit round gets the notice and the second stops as candidate no_progress without another pass', function () { passed('ordinary red'); });
Then('build-env-red-no-edit-host-stop: the Build stops as environment host_or_base_failure naming check test with exit 3 after two passes and no no-edit notice', function () { passed('disk stop'); });
Then('build-env-red-no-edit-host-stop: a round 2 without edits after pass 1 failing check vet ordinarily and check test with a disk failure is verified again without the notice', function () { passed('mixed failing checks'); });
Then('build-env-red-no-edit-host-stop: the Build stops as environment host_or_base_failure naming the stage check with exit 3 after two passes and no no-edit notice', function () { passed('stage gate stop'); });
