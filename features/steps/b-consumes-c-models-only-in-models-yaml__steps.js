// Steps of b-consumes-c-models-only-in-models-yaml: kogen-build follows kogen-coordinator's
// c-models-only-in-models-yaml contract (model roles only in .kogen/models.yaml and
// .kogen/local/models.yaml; a models: key in .kogen/project.yaml is refused). The Go runs go
// through b-consumes-c-models-only-in-models-yaml__control.js, from the checkout's real path, with
// the go.mod and go.sum the checkout holds; no checkout file changes. The file steps read the
// checkout's pins, goldens and Go sources. Every failure message names the tests, the go command
// and its outcome, or the file and what it holds.
'use strict';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./b-consumes-c-models-only-in-models-yaml__control.js');

const P = 'b-consumes-c-models-only-in-models-yaml: ';
const LONG = { timeout: 600000 };

// column reads a data table's column, which must name at least one nonblank value.
function column(table, name) {
  const list = table.hashes().map((row) => row[name]);
  assert.ok(list.length > 0 && list.every((v) => typeof v === 'string' && v !== ''), `the table names nothing in its ${name} column`);
  return list;
}

function sha256(bytes) {
  return crypto.createHash('sha256').update(bytes).digest('hex');
}

function checkoutPath(rel) {
  return path.join(control.realRoot(), ...rel.split('/'));
}

// filesUnder lists the regular files under the checkout folder rel (recursively, symbolic links
// not followed) by slash-separated path relative to the checkout, in code-unit order.
function filesUnder(rel) {
  const files = [];
  (function walk(r) {
    for (const e of fs.readdirSync(checkoutPath(r), { withFileTypes: true })) {
      const p = r + '/' + e.name;
      if (e.isDirectory()) walk(p);
      else if (e.isFile()) files.push(p);
    }
  })(rel);
  return files.sort();
}

When(P + 'I run these Go tests of {string}:', LONG, async function (pkg, table) {
  this.bcmoyRun = await control.runGo(control.realRoot(), pkg, column(table, 'test'));
});

Then(P + 'every test of that run passed and none was skipped, including:', function (table) {
  const tests = column(table, 'test');
  const p = control.passProblem(this.bcmoyRun, tests);
  assert.ok(p === '', `${tests.join(', ')}: ${p}\n${control.details(this.bcmoyRun, tests)}`);
});

Then(P + 'that run used kogen-coordinator {string} without a replacement', function (want) {
  const c = control.coordinator(control.realRoot());
  assert.ok(c.status === 0, `${c.command} exited ${c.status}: ${c.output}`);
  assert.equal(c.replace, '', `go.mod replaces ${control.COORDINATOR} with ${c.replace} (${c.command}: ${c.output.trim()})`);
  assert.equal(c.version, want, `the go.mod of the checkout makes the build use ${control.COORDINATOR} ${c.version}, want ${want} (${c.command}: ${c.output.trim()}); the run above ran against ${c.version}`);
});

Then(P + 'the file {string} has the SHA-256 {string}', function (rel, want) {
  const file = checkoutPath(rel);
  assert.ok(fs.existsSync(file), `${rel} does not exist`);
  const got = sha256(fs.readFileSync(file));
  assert.equal(got, want, `${rel} has the SHA-256 ${got}, want ${want}: its bytes differ`);
});

Then(P + 'these files do not exist:', function (table) {
  const present = column(table, 'file').filter((rel) => fs.existsSync(checkoutPath(rel)));
  assert.deepEqual(present, [], `these files still exist: ${present.join(', ')}`);
});

// The digest of a folder: SHA-256 over `<path>\0<sha256 of the file's bytes>\n` for every regular
// file under it, by slash-separated path relative to the checkout in code-unit order, leaving out
// the files the table names.
Then(P + 'the files under {string} other than these have the digest {string}:', function (dir, want, table) {
  const except = column(table, 'file');
  const kept = filesUnder(dir).filter((f) => !except.includes(f));
  const lines = kept.map((f) => `${f}\0${sha256(fs.readFileSync(checkoutPath(f)))}\n`).join('');
  const got = sha256(lines);
  assert.equal(got, want, `the ${kept.length} files under ${dir} other than ${except.join(', ')} have the digest ${got}, want ${want}: a file was added, removed or changed. Files and their SHA-256:\n${lines.replace(/\0/g, ' ')}`);
});

Then(P + 'no Go file under these folders contains {string}:', function (word, table) {
  const hits = [];
  for (const dir of column(table, 'folder')) {
    for (const f of filesUnder(dir).filter((f) => f.endsWith('.go'))) {
      fs.readFileSync(checkoutPath(f), 'utf8').split('\n').forEach((line, i) => {
        if (line.includes(word)) hits.push(`${f}:${i + 1}: ${line.trim().slice(0, 200)}`);
      });
    }
  }
  assert.deepEqual(hits, [], `Go files still hold ${word}:\n${hits.join('\n')}`);
});
