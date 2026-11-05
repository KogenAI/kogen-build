// Steps of k-silent-shell-test-fast: the two silent-shell tests of ./build run on the loop's
// controlled time instead of a real 47-second sleep, still fail when the loop stops a silent command
// at its wait, and still pass when the loop's waits change. Every step runs the real go tool in the
// checkout; a mutated kogen-loop is a temporary copy used through -modfile, so no checkout file
// changes. The runner and the judges are k-silent-shell-test-fast__control.js. Every failure message
// quotes the go test command, its exit status and the runner's own result lines and output.
const assert = require('node:assert/strict');
const { When, Then } = require('@cucumber/cucumber');
const control = require('./k-silent-shell-test-fast__control.js');

const P = 'k-silent-shell-test-fast: ';
const LONG = { timeout: 600000 };

// evidence is what the run shows: its command and exit status, the runner's own RUN/PASS/FAIL/SKIP
// and package result lines, and the tail of its whole output.
function evidence(run) {
  const lines = run.output
    .split('\n')
    .filter((l) => /^\s*(=== RUN|--- (PASS|FAIL|SKIP):)|^(PASS|FAIL)$|^(ok|FAIL)\s/.test(l));
  const out = run.output.length > 6000 ? `...${run.output.slice(-6000)}` : run.output;
  return `${run.command} exited ${run.status}\nresult lines:\n${lines.join('\n')}\noutput:\n${out}`;
}

When(P + 'I run the Go tests {string} and {string} of the build package one at a time', LONG, function (a, b) {
  this.kssfRun = control.goTest(process.cwd(), [a, b]);
});

Then(P + 'the Go tests {string} and {string} both passed', function (a, b) {
  for (const name of [a, b]) {
    const problem = control.judgePassed(this.kssfRun, name);
    assert.ok(problem === null, problem);
  }
});

Then(P + 'the Go test {string} took less than {int} seconds longer than {string}', function (slow, limit, base) {
  const run = this.kssfRun;
  const got = control.outcomes(run);
  for (const name of [slow, base]) {
    assert.ok(got[name], `${name} has no "--- PASS/FAIL/SKIP: ${name}" line\n${evidence(run)}`);
  }
  const extra = got[slow].seconds - got[base].seconds;
  assert.ok(
    extra < limit,
    `${slow} ran and reported ${got[slow].verdict} after ${got[slow].seconds}s; ${base} ran and reported ${got[base].verdict} after ${got[base].seconds}s: ` +
      `${slow} took ${extra.toFixed(2)}s longer, want less than ${limit}s\n${evidence(run)}`,
  );
});

When(P + 'I run the Go test {string} against a kogen-loop whose shell wait stops the command when it elapses', LONG, function (name) {
  this.kssfMutated = control.runMutated(process.cwd(), name, 'stop');
});

When(P + 'I run the Go test {string} against a kogen-loop whose shell waits are 90 and 60 seconds', LONG, function (name) {
  this.kssfMutated = control.runMutated(process.cwd(), name, 'waits');
});

Then(P + 'the Go test {string} failed', function (name) {
  const problem = control.judgeFailed(this.kssfMutated, name);
  assert.ok(problem === null, problem);
});

Then(P + 'the Go test {string} passed', function (name) {
  const problem = control.judgePassed(this.kssfMutated, name);
  assert.ok(problem === null, problem);
});
