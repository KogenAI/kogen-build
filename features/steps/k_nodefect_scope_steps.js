'use strict';
// Steps of k-nodefect-scope: a real Build whose reviewer submits one blocking finding, in a
// given review_material mode (harness in k_nodefect_scope/).
const { Given, When, Then } = require('@cucumber/cucumber');
const { runBuild, expectations, describe } = require('./k_nodefect_scope/harness.js');

Given('the project review_material mode is {string}', function (mode) {
  this.reviewMode = mode;
});

When('the reviewer submits a changes verdict whose only finding is this blocker at app.txt line 1:', { timeout: 15 * 60 * 1000 }, function (message) {
  this.reviewRecord = runBuild(this.reviewMode, message);
});

function expect(world, expectation) {
  const r = world.reviewRecord;
  const problems = expectations[expectation](r);
  if (problems.length > 0) {
    throw new Error(`${world.reviewMode} review, expected ${expectation}: ${problems.join('; ')} (${describe(r)})\nBuild output:\n${r.shown}\nreviewer feedback:\n${r.feedback}`);
  }
}

Then('the finding stands: the Build stops review_blocked with that changes verdict and the branch unchanged', function () {
  expect(this, 'blocked');
});

Then('the finding is refused as invalid feedback: the Build stops review_invalid, never review_blocked, and the branch is unchanged', function () {
  expect(this, 'invalid');
});
