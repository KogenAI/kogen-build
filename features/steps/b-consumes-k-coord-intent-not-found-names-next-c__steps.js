// Steps of b-consumes-k-coord-intent-not-found-names-next-c: kogen-build follows kogen-coordinator's
// k-coord-intent-not-found-names-next-command contract (the missing-Intent refusal names the slug,
// the searched folder and the next commands). The Go runs go through
// b-consumes-k-coord-intent-not-found-names-next-c__control.js, from the checkout's real path, with
// the go.mod and go.sum the checkout holds; no checkout file changes. The file steps read the
// checkout's pins and Go sources. Every failure message names the tests, the go command and its
// outcome, or the file and what it holds.
'use strict';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./b-consumes-k-coord-intent-not-found-names-next-c__control.js');

const P = 'b-consumes-k-coord-intent-not-found-names-next-c: ';
const LONG = { timeout: 600000 };

function sha256(bytes) {
  return crypto.createHash('sha256').update(bytes).digest('hex');
}

function checkoutPath(rel) {
  return path.join(control.realRoot(), ...rel.split('/'));
}

// filesUnder lists the regular files under the checkout folder rel (recursively, symbolic links
// not followed) by slash-separated path relative to the checkout.
function filesUnder(rel) {
  const files = [];
  (function walk(r) {
    for (const e of fs.readdirSync(checkoutPath(r), { withFileTypes: true })) {
      const p = r + '/' + e.name;
      if (e.isDirectory()) walk(p);
      else if (e.isFile()) files.push(p);
    }
  })(rel);
  return files;
}

When(P + 'I run the Go test {string} of {string}', LONG, async function (test, pkg) {
  this.bcinfTest = test;
  this.bcinfRun = await control.runGo(control.realRoot(), pkg, [test]);
});

Then(P + 'that run passed and no test was skipped', function () {
  const tests = [this.bcinfTest];
  const p = control.passProblem(this.bcinfRun, tests);
  assert.ok(p === '', `${tests.join(', ')}: ${p}\n${control.details(this.bcinfRun, tests)}`);
});

Then(P + 'the build of the checkout uses kogen-coordinator {string} without a replacement', function (want) {
  const c = control.coordinator(control.realRoot());
  assert.ok(c.status === 0, `${c.command} exited ${c.status}: ${c.output}`);
  assert.equal(c.replace, '', `go.mod replaces ${control.COORDINATOR} with ${c.replace} (${c.command}: ${c.output.trim()})`);
  assert.equal(c.version, want, `the go.mod of the checkout makes the build use ${control.COORDINATOR} ${c.version}, want ${want} (${c.command}: ${c.output.trim()})`);
});

Then(P + 'the file {string} has the SHA-256 {string}', function (rel, want) {
  const file = checkoutPath(rel);
  assert.ok(fs.existsSync(file), `${rel} does not exist`);
  const got = sha256(fs.readFileSync(file));
  assert.equal(got, want, `${rel} has the SHA-256 ${got}, want ${want}: its bytes differ`);
});

// The digest of folders: SHA-256 over `<path>\0<sha256 of the file's bytes>\n` for every regular
// file under the comma-separated folders, by slash-separated path relative to the checkout in
// code-unit order, leaving out the listed files.
Then(P + 'the files under {string} other than these have the digest {string}:', function (dirs, want, table) {
  const except = table.hashes().map((r) => r.file);
  assert.ok(except.length > 0 && except.every((f) => f), 'the table names no file in its file column');
  const kept = dirs.split(',').flatMap(filesUnder).filter((f) => !except.includes(f)).sort();
  const lines = kept.map((f) => `${f}\0${sha256(fs.readFileSync(checkoutPath(f)))}\n`).join('');
  const got = sha256(lines);
  assert.equal(got, want, `the ${kept.length} files under ${dirs} other than ${except.join(', ')} have the digest ${got}, want ${want}: a file was added, removed or changed. Files and their SHA-256:\n${lines.replace(/\0/g, ' ')}`);
});

Then(P + 'the Go file {string} contains {string}', function (rel, part) {
  const text = fs.readFileSync(checkoutPath(rel), 'utf8');
  assert.ok(text.includes(part), `${rel} does not contain ${JSON.stringify(part)}`);
});

// The old refusal text, as a Go test states it: "Intent does not exist" closing a quoted string,
// with or without a newline escape (the refusal text, or the refusal's detail).
Then(P + 'no Go test file under {string} states the refusal without the slug', function (dirs) {
  const old = /Intent does not exist(?:\\n)?["`]/;
  const hits = [];
  for (const f of dirs.split(',').flatMap(filesUnder).filter((f) => f.endsWith('_test.go')).sort()) {
    fs.readFileSync(checkoutPath(f), 'utf8').split('\n').forEach((line, i) => {
      if (old.test(line)) hits.push(`${f}:${i + 1}: ${line.trim().slice(0, 200)}`);
    });
  }
  assert.deepEqual(hits, [], `Go test files still state the old refusal text:\n${hits.join('\n')}`);
});
