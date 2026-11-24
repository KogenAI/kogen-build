'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const {Given, Then} = require('@cucumber/cucumber');
const {verify} = require('./file-index-omission-accounting__observer');
const prefix = 'file-index-omission-accounting: ';
Given(prefix + 'the file-index source and existing regression file are available', function() {
  for (const file of ['build/file_index.go', 'build/file_index_test.go']) {
    assert.ok(fs.statSync(file).isFile(), `${file} must exist`);
  }
});
Then(prefix + 'the regression tests reject overlapping and dropped omission counts', {timeout: 115000}, function() {
  verify('accounting');
});
Then(prefix + 'the regression tests reject input-order-dependent output', {timeout: 115000}, function() {
  verify('order');
});
Then(prefix + 'the regression tests reject required-path loss and oversized output', {timeout: 115000}, function() {
  verify('guards');
});
