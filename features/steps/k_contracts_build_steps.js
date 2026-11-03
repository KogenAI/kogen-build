'use strict';
const { Then } = require('@cucumber/cucumber');
const { execFileSync } = require('node:child_process');

Then('the producer consumer-contract package passes at {string}', function (pkg) {
  execFileSync('go', ['test', '-count=1', pkg], {
    cwd: process.cwd(),
    env: { ...process.env, GOFLAGS: '-trimpath' },
    stdio: 'pipe',
    encoding: 'utf8'
  });
});
