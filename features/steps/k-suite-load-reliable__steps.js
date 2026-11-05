// Steps of k-suite-load-reliable: the tests of ./build that wait for a Build's (or a native
// round's) held child wait until it runs or the Build ends, never under a wall-clock deadline, and
// each still fails against a deliberately broken implementation of what it checks. Every step runs
// the real go tool in the checkout through k-suite-load-reliable__control.js: under a git that waits
// before every call (a host on which every git process takes longer, as under load), or against one
// mutation that reaches the build through -overlay or a temporary go.mod, so no checkout file
// changes. The groups of a table run at the same time, one go test each. Every failure message
// quotes the go test command, its exit status and the test's own output.
'use strict';
const assert = require('node:assert/strict');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./k-suite-load-reliable__control.js');

const P = 'k-suite-load-reliable: ';
const LONG = { timeout: 600000 };

// grouped reads the data table's rows (with the named columns) and groups their tests by the
// mode that column key gives them, in first-appearance order.
function grouped(table, columns, modeOf) {
  const rows = table.hashes();
  assert.ok(rows.length > 0, 'the table names no Go test');
  const groups = new Map();
  for (const row of rows) {
    for (const c of columns) assert.ok(typeof row[c] === 'string' && row[c] !== '', `table row ${JSON.stringify(row)} lacks ${c}`);
    const mode = modeOf(row);
    if (!groups.has(mode)) groups.set(mode, []);
    groups.get(mode).push(row.test);
  }
  return { rows, groups: [...groups].map(([mode, names]) => ({ mode, names })) };
}

function report(problems) {
  return problems.map((p) => `${p[0]}\n${p[1]}`).join('\n\n');
}

// runFor is the run of the group whose mode is mode.
function runFor(world, mode) {
  return world.kslrRuns[world.kslrGroups.findIndex((g) => g.mode === mode)];
}

When(P + 'I run these Go tests of the build package at the same time, with every git call delayed:', LONG, async function (table) {
  const { rows, groups } = grouped(table, ['seconds', 'test'], (row) => 'git=' + row.seconds);
  this.kslrRows = rows;
  this.kslrGroups = groups;
  this.kslrRuns = await control.runGroups(process.cwd(), groups);
});

Then(P + 'each of those Go tests passed, and git calls were delayed', function () {
  const problems = this.kslrRows.map((row) => control.passProblem(runFor(this, 'git=' + row.seconds), row.test)).filter(Boolean);
  assert.ok(problems.length === 0, report(problems));
});

When(P + 'I run these Go tests of the build package at the same time, each against its broken implementation:', LONG, async function (table) {
  const { rows, groups } = grouped(table, ['broken implementation', 'test', 'message'], (row) => 'mutation=' + row['broken implementation']);
  this.kslrRows = rows;
  this.kslrGroups = groups;
  this.kslrRuns = await control.runGroups(process.cwd(), groups);
});

Then(P + 'each of those Go tests failed with its own message', function () {
  const problems = this.kslrRows
    .map((row) => control.failProblem(runFor(this, 'mutation=' + row['broken implementation']), row.test, row.message))
    .filter(Boolean);
  assert.ok(problems.length === 0, report(problems));
});
