'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const assert = require('node:assert');

let result;

Given('base-red-kogen-build: the repository checkout', function () {
  assert.ok(process.cwd());
});

When('base-red-kogen-build: the build package tests run with a 100 second timeout', { timeout: 115000 }, function () {
  result = spawnSync('go', ['test', '-count=1', '-timeout', '100s', './build'], {
    cwd: process.cwd(),
    encoding: 'utf8',
    env: { ...process.env, GOFLAGS: '-trimpath' },
    timeout: 112000,
  });
});

Then('base-red-kogen-build: the tests pass without a timeout', function () {
  const out = (result.stdout || '') + (result.stderr || '');
  assert.ok(!/test timed out/.test(out), 'go test ./build timed out:\n' + out.slice(0, 3000));
  assert.strictEqual(result.status, 0, 'go test ./build exit ' + result.status + ':\n' + out.slice(0, 3000));
  assert.match(out, /^ok\s+github\.com\/KogenAI\/kogen-build\/build\s/m);
});
