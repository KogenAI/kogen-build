'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { Given, Then } = require('@cucumber/cucumber');

const P = 'simplify-build-single-path: ';

function root() {
  let dir = __dirname;
  while (!fs.existsSync(path.join(dir, 'go.mod'))) {
    const up = path.dirname(dir);
    assert.notEqual(up, dir, 'no go.mod above ' + __dirname);
    dir = up;
  }
  return dir;
}

function goFiles(dir, out) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (e.name === 'node_modules' || e.name === '.git' || e.name === 'testdata' || e.name === '.kogen') continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) goFiles(p, out);
    else if (e.name.endsWith('.go')) out.push(p);
  }
  return out;
}

Given(P + 'the repository checkout', function () {
  this.sbRoot = root();
});

Then(P + 'no Go file mentions {string}, {string}, {string} or {string}', function (a, b, c, d) {
  const hits = [];
  for (const f of goFiles(this.sbRoot, [])) {
    const text = fs.readFileSync(f, 'utf8');
    for (const name of [a, b, c, d]) {
      if (text.includes(name)) hits.push(`${path.relative(this.sbRoot, f)}: ${name}`);
    }
  }
  assert.deepEqual(hits, [], 'removed names still present in Go files');
});

Then(P + 'the file {string} does not exist', function (rel) {
  assert.equal(fs.existsSync(path.join(this.sbRoot, rel)), false, rel + ' exists');
});
