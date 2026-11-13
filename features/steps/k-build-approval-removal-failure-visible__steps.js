// Steps of k-build-approval-removal-failure-visible: copy the Go test data file of the Intent into
// a scratch copy of the checkout's build package and run it with the real go tool.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { When, Then } = require('@cucumber/cucumber');

const P = 'k-build-approval-removal-failure-visible: ';
const SLUG = 'k-build-approval-removal-failure-visible';

When(P + 'I run the landing scenarios of the Build', { timeout: 600000 }, function () {
  const root = fs.realpathSync(process.cwd());
  const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'kbarfv-'));
  const copy = path.join(scratch, 'src');
  const cp = spawnSync('rsync', ['-a', '--exclude=.git', '--exclude=node_modules', root + '/', copy + '/'], { encoding: 'utf8' });
  assert.equal(cp.status, 0, 'copy of the checkout failed: ' + cp.stderr);
  fs.copyFileSync(path.join(root, '.kogen/intents', SLUG, 'approval_removal.go.txt'), path.join(copy, 'build/zz_approval_removal_test.go'));
  const cmd = 'go test -count=1 -v -run ^TestApprovalRemoval$ ./build';
  const r = spawnSync('go', ['test', '-count=1', '-v', '-run', '^TestApprovalRemoval$', './build'], { cwd: copy, encoding: 'utf8', timeout: 500000 });
  this.kbarfv = { cmd, out: (r.stdout || '') + (r.stderr || ''), status: r.status };
});

Then(P + 'the subtest {string} passed', function (name) {
  const r = this.kbarfv;
  assert.ok(r.out.includes('--- PASS: TestApprovalRemoval/' + name), `${r.cmd}: subtest ${name} did not pass (exit ${r.status})\n${r.out}`);
});
