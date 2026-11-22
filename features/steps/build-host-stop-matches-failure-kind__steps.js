'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert');

const DRIVER = path.join('features', 'steps', 'build-host-stop-matches-failure-kind__driver', 'acceptance_test.go.txt');
const COPY = path.join('build', 'zz_build_host_stop_matches_failure_kind_acceptance_test.go');

let ran;

Given('build-host-stop-matches-failure-kind: the kogen-build checkout', function () {
  assert.ok(fs.existsSync(path.join(process.cwd(), 'build', 'envfailure.go')), 'build/envfailure.go is missing');
  assert.ok(fs.existsSync(path.join(process.cwd(), DRIVER)), DRIVER + ' is missing');
});

When('build-host-stop-matches-failure-kind: the Build test {string} runs in the build package', { timeout: 115000 }, function (name) {
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

Then('build-host-stop-matches-failure-kind: the Build stops after pass 2 quoting the second disk line and asks for no further round', function () { passed('stop with different disk lines'); });
Then('build-host-stop-matches-failure-kind: the Build does not stop for the host or base after a disk failure then a memory failure', function () { passed('different kinds'); });
Then('build-host-stop-matches-failure-kind: the Build does not stop for the host or base after an ordinary failure then a disk failure', function () { passed('ordinary then disk'); });
Then('build-host-stop-matches-failure-kind: the Build does not stop for the host or base when the disk failures are in different checks', function () { passed('different checks'); });
Then('build-host-stop-matches-failure-kind: the Build still stops with the existing message and event when the disk lines are identical', function () { passed('identical lines'); });
