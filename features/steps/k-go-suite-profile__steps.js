// Steps of k-go-suite-profile: the build package's tests cost less per simulated Build and still
// catch the same defects. Every step runs the real go tool on ./build through
// k-go-suite-profile__control.js, from the checkout's real path; deliberately broken
// implementations and the shared acceptance script check reach the build through -overlay, so no
// checkout file changes. Every failure message quotes the go test command, its exit status and
// the test's own output.
'use strict';
const assert = require('node:assert/strict');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./k-go-suite-profile__control.js');

const P = 'k-go-suite-profile: ';
const LONG = { timeout: 600000 };
const SHARED_CHECK = 'TestKGoSuiteProfileSharedAcceptanceScript';

// rows reads the data table's rows and checks that each names the given columns.
function rows(table, columns) {
  const list = table.hashes();
  assert.ok(list.length > 0, 'the table names no Go test');
  for (const row of list) {
    for (const c of columns) assert.ok(typeof row[c] === 'string' && row[c] !== '', `table row ${JSON.stringify(row)} lacks ${c}`);
  }
  return list;
}

function report(problems) {
  return problems.map((p) => `${p[0]}\n${p[1]}`).join('\n\n');
}

When(P + "I run these Go tests of the build package with stand-ins for the host's other toolchains first on PATH:", LONG, async function (table) {
  this.kgspTests = rows(table, ['test']).map((r) => r.test);
  this.kgspRun = await control.runGo(control.realRoot(), this.kgspTests, { standIns: true });
});

Then(P + 'each of those Go tests passed, and no stand-in toolchain ran', function () {
  const problems = [];
  for (const test of this.kgspTests) {
    const p = control.passProblem(this.kgspRun, test, { hermetic: true });
    if (p) problems.push([p, control.details(this.kgspRun, test)]);
  }
  assert.ok(problems.length === 0, report(problems));
});

When(P + 'I run the shared acceptance script check in the build package', LONG, async function () {
  this.kgspRun = await control.runGo(control.realRoot(), [SHARED_CHECK], { fixture: true });
});

Then(P + 'the shared acceptance script check passed', function () {
  const p = control.passProblem(this.kgspRun, SHARED_CHECK);
  assert.ok(p === '', `${p}\n${control.details(this.kgspRun, SHARED_CHECK)}`);
});

When(P + 'I run these Go tests of the build package at the same time, each against its broken implementation:', LONG, async function (table) {
  this.kgspRows = rows(table, ['broken implementation', 'test', 'message']);
  const groups = new Map();
  for (const row of this.kgspRows) {
    const broken = row['broken implementation'];
    assert.ok(control.BROKEN[broken], `unknown broken implementation ${broken}`);
    if (!groups.has(broken)) groups.set(broken, []);
    groups.get(broken).push(row.test);
  }
  const root = control.realRoot();
  const names = [...groups.keys()];
  const runs = await Promise.all(names.map((broken) => control.runGo(root, groups.get(broken), { broken })));
  this.kgspRuns = new Map(names.map((broken, i) => [broken, runs[i]]));
});

Then(P + 'each of those Go tests ran in parallel and failed with its own message', function () {
  const problems = [];
  for (const row of this.kgspRows) {
    const run = this.kgspRuns.get(row['broken implementation']);
    const p = control.failProblem(run, row.test, row.message, { parallel: true });
    if (p) problems.push([`${p} against ${row['broken implementation']}`, control.details(run, row.test)]);
  }
  assert.ok(problems.length === 0, report(problems));
});
