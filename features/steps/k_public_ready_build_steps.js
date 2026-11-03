'use strict';
const { Then } = require('@cucumber/cucumber');
const fs = require('fs');
const cp = require('child_process');
const assert = require('assert/strict');

Then('the public README contains these literals:', function (table) {
  const text = fs.readFileSync('README.md', 'utf8');
  for (const { literal } of table.hashes()) assert.ok(text.includes(literal), `README.md lacks ${literal}`);
});
Then('the README contains the inquiry address {string} exactly {int} time', function (address, count) {
  assert.equal(fs.readFileSync('README.md', 'utf8').split(address).length - 1, count, 'README inquiry-address count');
});
Then('the public README does not match {string}', function (pattern) {
  assert.ok(!new RegExp(pattern, 'i').test(fs.readFileSync('README.md', 'utf8')), `README.md matches forbidden copy: ${pattern}`);
});
Then('these public policy files are absent:', function (table) {
  for (const { path } of table.hashes()) assert.ok(!fs.existsSync(path), `Forbidden public policy file: ${path}`);
});
Then('tracked documentation has no real home paths or workspace references', function () {
  const paths = cp.execFileSync('git', ['ls-files', '-z'], { encoding: 'utf8' }).split('\0').filter(p => /\.(md|rst|adoc)$/i.test(p));
  const forbidden = new RegExp('/' + '(Users|home)/[^\\s/]+/|work\\.noindex', 'i');
  for (const path of paths) assert.ok(!forbidden.test(fs.readFileSync(path, 'utf8')), `Machine-specific reference in ${path}`);
});
Then('{string} contains these report fields:', function (path, table) {
  const text = fs.readFileSync(path, 'utf8');
  for (const { literal } of table.hashes()) assert.ok(text.toLowerCase().includes(literal.toLowerCase()), `${path} lacks report field ${literal}`);
  assert.match(text, /\b[0-9a-f]{7,40}\b/, `${path} must identify actual historical commits`);
});
