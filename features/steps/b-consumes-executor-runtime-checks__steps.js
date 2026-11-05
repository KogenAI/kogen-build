// Steps of b-consumes-executor-runtime-checks: kogen-build's TestContextExternalFirstPrompt passes
// whether kogen-executor accepts any regular, non-empty, executable Claude Code runtime (the pinned
// version) or also checks its pinned size, owner and mode (kogen-runs-unverified-claude-code-runtime),
// and still proves what the first prompt contains with context on and off. The Go runs go through
// b-consumes-executor-runtime-checks__control.js, from the checkout's real path, against the pinned
// kogen-executor or a temporary copy of it with one change; no checkout file changes. The file
// steps read the checkout's pins and other tests. Every failure message names the test, the go
// test command and its outcome, or the files and what they hold.
'use strict';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./b-consumes-executor-runtime-checks__control.js');

const P = 'b-consumes-executor-runtime-checks: ';
const LONG = { timeout: 600000 };

function sha256(bytes) {
  return crypto.createHash('sha256').update(bytes).digest('hex');
}

// checkoutFile reads a file of the checkout by its slash-separated relative path.
function checkoutFile(rel) {
  return fs.readFileSync(path.join(control.realRoot(), ...rel.split('/')));
}

async function run(world, test, executor, umask) {
  world.bcertTest = test;
  world.bcertRun = await control.runGo(control.realRoot(), executor, umask, [test]);
}

When(P + 'I run the Go test {string} against the {string} kogen-executor', LONG, async function (test, executor) {
  await run(this, test, executor, '');
});

When(P + 'I run the Go test {string} against the {string} kogen-executor with umask {string}', LONG, async function (test, executor, umask) {
  await run(this, test, executor, umask);
});

Then(P + 'that run passed these subtests and no test was skipped:', function (table) {
  const subtests = table.hashes().map((r) => r.subtest);
  assert.ok(subtests.length > 0 && subtests.every(Boolean), 'the table names no subtest in its subtest column');
  const names = [this.bcertTest, ...subtests];
  const p = control.passProblem(this.bcertRun, names);
  assert.ok(p === '', `${names.join(', ')}: ${p}\n${control.details(this.bcertRun, names)}`);
});

Then(P + 'the Go test {string} fails against each of these kogen-executor copies:', LONG, async function (test, table) {
  const rows = table.hashes();
  assert.ok(rows.length > 0 && rows.every((r) => r.copy), 'the table names no kogen-executor copy in its copy column');
  const wrong = [];
  for (const row of rows) {
    const r = await control.runGo(control.realRoot(), row.copy, '', [test]);
    if (r.status === 0 || r.results[test] !== 'fail') {
      wrong.push(`${test} did not fail against the ${row.copy} kogen-executor (${row.defect}): ${r.command}, exit ${r.status}, ${test}: ${r.results[test] || 'did not run'}\n${control.ranLines(r, [test])}\n${control.clip(r.raw + r.stderr)}`);
    }
  }
  assert.ok(wrong.length === 0, wrong.join('\n'));
});

Then(P + 'the file {string} has the SHA-256 {string}', function (rel, want) {
  const got = sha256(checkoutFile(rel));
  assert.equal(got, want, `${rel} has the SHA-256 ${got}, want ${want}: its bytes changed`);
});

// The digest of folders: SHA-256 over `<path>\0<sha256 of the file's bytes>\n` for every regular
// file under them (recursively, symbolic links not followed), by slash-separated path relative to
// the checkout in code-unit order, leaving out the one named file.
Then(P + 'the files under these folders other than {string} have the digest {string}:', function (except, want, table) {
  const dirs = table.hashes().map((r) => r.folder);
  assert.ok(dirs.length > 0 && dirs.every(Boolean), 'the table names no folder in its folder column');
  const root = control.realRoot();
  const files = [];
  function walk(rel) {
    for (const e of fs.readdirSync(path.join(root, ...rel.split('/')), { withFileTypes: true })) {
      const p = rel + '/' + e.name;
      if (e.isDirectory()) walk(p);
      else if (e.isFile()) files.push(p);
    }
  }
  dirs.forEach(walk);
  files.sort();
  const kept = files.filter((f) => f !== except);
  const lines = kept.map((f) => `${f}\0${sha256(checkoutFile(f))}\n`).join('');
  const got = sha256(lines);
  assert.equal(got, want, `the ${kept.length} files under ${dirs.join(', ')} other than ${except} have the digest ${got}, want ${want}: a file was added, removed or changed. Files and their SHA-256:\n${lines.replace(/\0/g, ' ')}`);
});
