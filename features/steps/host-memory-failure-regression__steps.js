// Steps of host-memory-failure-regression: the new Go tests of build/env_failure_park_test.go pass
// against the unchanged build/envfailure.go and fail against copies of it with one defect in how
// memory failures are classified or how the host-or-base stop compares them. The runs go through
// host-memory-failure-regression__control.js (go test -overlay; no checkout file changes). Every
// failure message names the command, the test results and the defect.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./host-memory-failure-regression__control.js');

const P = 'host-memory-failure-regression: ';
const LONG = { timeout: 115000 };

When(P + 'I run the new tests of the host-or-base stop against the unchanged environment failure code', LONG, function () {
  this.hmfrRun = control.run(null, 'unchanged');
});

Then(P + 'every one of those tests ran and passed', function () {
  const p = control.unchangedProblem(this.hmfrRun);
  assert.ok(p === '', p);
});

Then(P + 'the new tests fail against each of these environment failure code copies:', LONG, function (table) {
  const rows = table.hashes();
  assert.ok(rows.length > 0 && rows.every((r) => control.DEFECTS[r.copy]), 'the table names an unknown build/envfailure.go copy in its copy column');
  const wrong = [];
  for (const row of rows) {
    const p = control.rejectedProblem(control.run(null, row.copy), row.copy);
    if (p !== '') wrong.push(`${row.copy} (${row.defect}): ${p}`);
  }
  assert.ok(wrong.length === 0, wrong.join('\n'));
});

// build/envfailure.go must still hold the code the defective copies replace, and the working tree's
// changes other than the Intent's own files and acceptance steps may only be the test file. The
// change list is read from git status, which is empty in a clean candidate checkout (such as the
// verification checkout); there only the envfailure.go code check applies, and the scope of the
// change is left to the independent review.
Then(P + 'build\\/envfailure.go keeps its code and no file besides the Intent, its acceptance steps and {string} is changed', function (want) {
  const src = fs.readFileSync(path.join(process.cwd(), 'build', 'envfailure.go'), 'utf8');
  const snippets = new Set();
  for (const d of Object.values(control.DEFECTS)) for (const s of d.snippets) snippets.add(s);
  const missing = [...snippets].filter((s) => !src.includes(s));
  assert.deepEqual(missing, [], `build/envfailure.go no longer contains these lines: ${JSON.stringify(missing)}`);
  const p = spawnSync('git', ['status', '--porcelain', '--untracked-files=all'], { cwd: process.cwd(), encoding: 'utf8' });
  assert.equal(p.status, 0, `git status failed: ${p.stderr}`);
  const files = p.stdout.split('\n').filter(Boolean)
    .map((l) => l.slice(3).split(' -> ').pop().replace(/^"|"$/g, ''))
    .filter((f) => !f.startsWith('.kogen/intents/host-memory-failure-regression/') && !f.startsWith('features/steps/host-memory-failure-regression__'));
  if (files.length > 0) {
    assert.deepEqual(files, [want], `git status --porcelain shows these changed files: ${JSON.stringify(files)}, want only ${want}`);
  }
});
