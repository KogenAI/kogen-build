'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {Given,Then} = require('@cucumber/cucumber');
const {run} = require('./native-continuation-worker-adapter__driver/run.cjs');
const P = 'native-continuation-worker-adapter: ';
Given(P+'the native fixture harness is available', function () {
  assert.ok(fs.existsSync(path.join(__dirname,'../../build/resume.go')));
  assert.ok(fs.existsSync(path.join(__dirname,'native-continuation-worker-adapter__driver/fixture_test.go.txt')));
});
Then(P+'the adapter satisfies {string} with this evidence', {timeout:115000}, function (mode, text) {
  const spec = JSON.parse(text);
  const result = run(mode,spec);
  assert.ok(result.ok, `TestNCWA (${mode}) did not complete and pass; ${result.command}; exit ${result.status}\n${result.output}`);
});
