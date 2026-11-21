'use strict';
const {Then, When} = require('@cucumber/cucumber');
const assert = require('node:assert/strict');
const driver = require('./build-classifies-stalled-external-pass__driver/run.js');
When('build-classifies-stalled-external-pass: the scripted build runs with:', function (doc) {
  this.stallResult = driver.run(JSON.parse(doc));
});
Then('build-classifies-stalled-external-pass: the build satisfies the expected result', function () {
  assert.ok(this.stallResult.ok, this.stallResult.text);
});
