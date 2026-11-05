// Steps of b-consumes-baseline-optional-models: kogen-build's tests pass whether
// kogen-coordinator's project.Baseline always lists .kogen/models.yaml (the pinned version) or
// leaves it out when the project has no such file. The Go runs go through
// b-consumes-baseline-optional-models__control.js, from the checkout's real path, against the
// pinned kogen-coordinator or a temporary copy of it whose project.Baseline is changed; no
// checkout file changes. The file steps read the checkout's goldens and pins. Every failure
// message names the tests, the go test command and its outcome, or the file and what it holds.
'use strict';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./b-consumes-baseline-optional-models__control.js');

const P = 'b-consumes-baseline-optional-models: ';
const LONG = { timeout: 600000 };

// names reads the data table's `test` column.
function names(table) {
  const list = table.hashes().map((row) => row.test);
  assert.ok(list.length > 0 && list.every((name) => typeof name === 'string' && name !== ''), 'the table names no Go test in its test column');
  return list;
}

function sha256(bytes) {
  return crypto.createHash('sha256').update(bytes).digest('hex');
}

// checkoutFile reads a file of the checkout by its slash-separated relative path.
function checkoutFile(rel) {
  return fs.readFileSync(path.join(control.realRoot(), ...rel.split('/')));
}

When(P + 'I run these Go tests of {string} against the {string} kogen-coordinator:', LONG, async function (pkg, coordinator, table) {
  this.bcbomRun = await control.runGo(control.realRoot(), coordinator, pkg, names(table));
});

Then(P + 'every test of that run passed and none was skipped, including:', function (table) {
  const tests = names(table);
  const p = control.passProblem(this.bcbomRun, tests);
  assert.ok(p === '', `${tests.join(', ')}: ${p}\n${control.details(this.bcbomRun, tests)}`);
});

Then(P + 'that run failed, with each of these tests failing:', function (table) {
  const tests = names(table);
  const run = this.bcbomRun;
  const where = `${run.command}, exit ${run.status}`;
  const results = tests.map((t) => `${t}: ${run.results[t] || 'did not run'}`);
  assert.ok(run.status !== 0, `go test exited 0 although the coordinator lists .kogen/models.yaml in the wrong place (${where}; ${results.join(', ')})\n${control.ranLines(run, tests)}`);
  const notFailed = tests.filter((t) => run.results[t] !== 'fail');
  assert.ok(notFailed.length === 0, `${notFailed.join(', ')} did not fail although the coordinator lists .kogen/models.yaml in the wrong place (${where}; ${results.join(', ')})\n${control.ranLines(run, tests)}\n${control.clip(run.raw + run.stderr)}`);
});

Then(P + 'the file {string} has the SHA-256 {string}', function (rel, want) {
  const got = sha256(checkoutFile(rel));
  assert.equal(got, want, `${rel} has the SHA-256 ${got}, want ${want}: its bytes changed`);
});

// The digest of a folder: SHA-256 over `<path>\0<sha256 of the file's bytes>\n` for every regular
// file under it (recursively, symbolic links not followed), by slash-separated path relative to
// the checkout in code-unit order, leaving out the named file.
Then(P + 'the files under {string} other than {string} have the digest {string}', function (dir, except, want) {
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
  const kept = files.filter((f) => f !== except);
  assert.ok(kept.length < files.length, `${except} is not a file under ${dir}`);
  const lines = kept.map((f) => `${f}\0${sha256(checkoutFile(f))}\n`).join('');
  const got = sha256(lines);
  assert.equal(got, want, `the ${kept.length} files under ${dir} other than ${except} have the digest ${got}, want ${want}: a file was added, removed or changed. Files and their SHA-256:\n${lines.replace(/\0/g, ' ')}`);
});

// The project's file list of a recorded request: the text item of request.input[0].content that
// starts with "# Files\n\n", which must end with a newline after the last file.
Then(P + 'the recorded request of {string} lists exactly these project files:', function (rel, doc) {
  const golden = JSON.parse(checkoutFile(rel).toString('utf8'));
  const content = golden && golden.request && Array.isArray(golden.request.input) && golden.request.input[0] && golden.request.input[0].content;
  assert.ok(Array.isArray(content), `${rel} holds no request.input[0].content list`);
  const lists = content.filter((c) => c && typeof c.text === 'string' && c.text.startsWith('# Files\n\n'));
  assert.equal(lists.length, 1, `${rel}: request.input[0].content holds ${lists.length} items starting "# Files", want 1`);
  assert.equal(lists[0].text, doc + '\n', `${rel}: the recorded request's project file list differs`);
});
