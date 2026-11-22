'use strict';
const {Then, When} = require('@cucumber/cucumber');
const assert = require('node:assert/strict');
const driver = require('./build-external-stall-round-limit__driver/run.js');
When('build-external-stall-round-limit: the scripted build runs with:', function (doc) {
  this.stallResult = driver.run(JSON.parse(doc));
});
Then('build-external-stall-round-limit: the build satisfies the expected result', function () {
  assert.ok(this.stallResult.ok, this.stallResult.text);
});
