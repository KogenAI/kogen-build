// Steps of b-commit-message-file-removed-after-commit: kogen-build's tests and Builds no longer
// depend on the commit message file after kogen-coordinator's Commit returns. Every step runs the
// real go tool through b-commit-message-file-removed-after-commit__control.js, from the
// checkout's real path, against the pinned kogen-coordinator (which leaves the message file
// behind) or a temporary copy of it whose Commit removes the file; no checkout file changes.
// Every failure message names the tests, the go test command and its outcome.
'use strict';
const assert = require('node:assert/strict');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./b-commit-message-file-removed-after-commit__control.js');

const P = 'b-commit-message-file-removed-after-commit: ';
const LONG = { timeout: 600000 };

// names reads the data table's `test` column.
function names(table) {
  const list = table.hashes().map((row) => row.test);
  assert.ok(list.length > 0 && list.every((name) => typeof name === 'string' && name !== ''), 'the table names no Go test in its test column');
  return list;
}

When(P + 'I run every Go test of {string} against the {string} kogen-coordinator', LONG, async function (pkg, coordinator) {
  this.bcmfRun = await control.runGo(control.realRoot(), coordinator, pkg, []);
});

When(P + 'I run these Go tests of {string} against the {string} kogen-coordinator:', LONG, async function (pkg, coordinator, table) {
  this.bcmfRun = await control.runGo(control.realRoot(), coordinator, pkg, names(table));
});

Then(P + 'every test of that run passed and none was skipped, including:', function (table) {
  const tests = names(table);
  const p = control.passProblem(this.bcmfRun, tests);
  assert.ok(p === '', `${tests.join(', ')}: ${p}\n${control.details(this.bcmfRun, tests)}`);
});

// The run's failing tests are judged by their own output, whatever their names.
Then(P + 'that run failed, a failing test reporting the committed message {string} and no failing test reporting a missing file', function (message) {
  const run = this.bcmfRun;
  const ran = Object.keys(run.results);
  const failing = ran.filter((t) => run.results[t] === 'fail');
  const where = `${run.command}, exit ${run.status}; tests that ran: ${ran.join(', ') || 'none'}`;
  const output = failing.map((t) => run.outputs[t] || '').join('\n') || run.raw + run.stderr;
  assert.ok(ran.length > 0, `no test ran (${where})\n${output}`);
  assert.ok(failing.length > 0, `no test failed although the coordinator commits a changed message (${where})`);
  const reporting = failing.filter((t) => (run.outputs[t] || '').includes(message));
  assert.ok(reporting.length > 0, `no failing test reported the committed message ${JSON.stringify(message)} (${where}; failed: ${failing.join(', ')})\n${output}`);
  const missing = failing.filter((t) => (run.outputs[t] || '').includes('no such file or directory'));
  assert.ok(missing.length === 0, `${missing.join(', ')} still read the commit message file after Commit returned (${where})\n${output}`);
});
