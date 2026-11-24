const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const { When, Then } = require('@cucumber/cucumber');

const prefix = 'pin-coordinator-strict-build-lock: ';

When(prefix + 'I run go list -m for {string} in the consumer module', { timeout: 35000 }, function (modulePath) {
  this.moduleLine = execFileSync('go', ['list', '-m', '-mod=readonly', modulePath], {
    encoding: 'utf8',
    timeout: 30000,
    env: { ...process.env, GOWORK: 'off', GOFLAGS: '' },
  });
});

Then(prefix + 'the resolved module line is {string}', function (expectedLine) {
  assert.equal(this.moduleLine, `${expectedLine}\n`, 'go list -m must select the exact Coordinator version without a replacement');
});
