// Steps of b-consumes-tools-require-executable: kogen-build's TestToolsRequire passes whether
// kogen-verification's Tools.Require wants node to be a regular file (the pinned version) or an
// executable with the new refusal wording, and still rejects defective refusals. The Go runs go
// through b-consumes-tools-require-executable__control.js, from the checkout's real path, against
// the pinned kogen-verification or a temporary copy of it whose Tools.Require is replaced; no
// checkout file changes. The file steps read the checkout's pins and consumer tests. Every
// failure message names the test, the go test command and its outcome, or the files and what they
// hold.
'use strict';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./b-consumes-tools-require-executable__control.js');

const P = 'b-consumes-tools-require-executable: ';
const LONG = { timeout: 600000 };

function sha256(bytes) {
  return crypto.createHash('sha256').update(bytes).digest('hex');
}

// checkoutFile reads a file of the checkout by its slash-separated relative path.
function checkoutFile(rel) {
  return fs.readFileSync(path.join(control.realRoot(), ...rel.split('/')));
}

When(P + 'I run the Go test {string} against the {string} kogen-verification', LONG, async function (test, verification) {
  this.bctreTest = test;
  this.bctreRun = await control.runGo(control.realRoot(), verification, [test]);
});

Then(P + 'that run passed and no test was skipped', function () {
  const tests = [this.bctreTest];
  const p = control.passProblem(this.bctreRun, tests);
  assert.ok(p === '', `${tests.join(', ')}: ${p}\n${control.details(this.bctreRun, tests)}`);
});

Then(P + 'the Go test {string} fails against each of these kogen-verification copies:', LONG, async function (test, table) {
  const rows = table.hashes();
  assert.ok(rows.length > 0 && rows.every((r) => r.copy), 'the table names no kogen-verification copy in its copy column');
  const wrong = [];
  for (const row of rows) {
    const run = await control.runGo(control.realRoot(), row.copy, [test]);
    if (run.status === 0 || run.results[test] !== 'fail') {
      wrong.push(`${test} did not fail against the ${row.copy} kogen-verification (${row.defect}): ${run.command}, exit ${run.status}, ${test}: ${run.results[test] || 'did not run'}\n${control.ranLines(run, [test])}\n${control.clip(run.raw + run.stderr)}`);
    }
  }
  assert.ok(wrong.length === 0, wrong.join('\n'));
});

Then(P + 'the file {string} has the SHA-256 {string}', function (rel, want) {
  const got = sha256(checkoutFile(rel));
  assert.equal(got, want, `${rel} has the SHA-256 ${got}, want ${want}: its bytes changed`);
});

// The digest of a folder: SHA-256 over `<path>\0<sha256 of the file's bytes>\n` for every regular
// file under it (recursively, symbolic links not followed), by slash-separated path relative to
// the checkout in code-unit order, leaving out the listed files.
Then(P + 'the files under {string} other than these have the digest {string}:', function (dir, want, table) {
  const except = table.hashes().map((r) => r.file);
  const root = control.realRoot();
  const files = [];
  (function walk(rel) {
    for (const e of fs.readdirSync(path.join(root, ...rel.split('/')), { withFileTypes: true })) {
      const p = rel + '/' + e.name;
      if (e.isDirectory()) walk(p);
      else if (e.isFile()) files.push(p);
    }
  })(dir);
  files.sort();
  const kept = files.filter((f) => !except.includes(f));
  const lines = kept.map((f) => `${f}\0${sha256(checkoutFile(f))}\n`).join('');
  const got = sha256(lines);
  assert.equal(got, want, `the ${kept.length} files under ${dir} other than ${except.join(', ')} have the digest ${got}, want ${want}: a file was added, removed or changed. Files and their SHA-256:\n${lines.replace(/\0/g, ' ')}`);
});
