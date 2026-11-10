'use strict';
const {Given,Then} = require('@cucumber/cucumber');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const {run} = require('./run-records-context-spend-events__driver/runner.cjs');
Given('run-records-context-spend-events: the local scripted Build fixture is available', function () {
 assert.ok(fs.existsSync('build/helper_test.go'));
 assert.ok(fs.existsSync('build/context_test.go'));
});
Then('run-records-context-spend-events: request and tool blobs reconstruct the exchange', {timeout:110000}, function (expected) {
 const r=run('custody',{},JSON.parse(expected)); assert.ok(r.ok,r.diagnostic);
});
Then('run-records-context-spend-events: response and call identities link exactly', {timeout:110000}, function (expected) {
 const r=run('identity',{},JSON.parse(expected)); assert.ok(r.ok,r.diagnostic);
});
Then('run-records-context-spend-events: reset and refusal retain measured attribution', {timeout:110000}, function (expected) {
 const r=run('attribution',{},JSON.parse(expected)); assert.ok(r.ok,r.diagnostic);
});
