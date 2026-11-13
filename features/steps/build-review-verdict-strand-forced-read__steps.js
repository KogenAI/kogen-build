'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { When, Then } = require('@cucumber/cucumber');

const P = 'build-review-verdict-strand-forced-read: ';
const root = () => fs.realpathSync(process.cwd());

When(P + 'I run the Go test {string} in {string}', { timeout: 600000 }, function (test, pkg) {
  const args = ['test', '-count=1', '-json', '-run', `^${test}$`, pkg];
  const r = spawnSync('go', args, { cwd: root(), encoding: 'utf8', env: { ...process.env, GOFLAGS: '-trimpath' }, maxBuffer: 1 << 28 });
  this.bvsRun = { test, command: `go ${args.join(' ')}`, status: r.status, out: (r.stdout || '') + (r.stderr || '') };
});

Then(P + 'the test ran and passed and was not skipped', function () {
  const run = this.bvsRun;
  const events = [];
  for (const line of run.out.split('\n')) {
    try { const e = JSON.parse(line); if (e && e.Test === run.test) events.push(e.Action); } catch (_) { /* not an event */ }
  }
  const where = `${run.test}: ${run.command} exited ${run.status}; actions ${events.join(',') || 'none'}`;
  assert.ok(events.includes('pass') && !events.includes('fail') && !events.includes('skip'), `${where}\n${run.out.slice(-3000)}`);
  assert.equal(run.status, 0, where);
});

Then(P + '{word} checks both {string} and {string} as the forced tool depending on unread parts', function (file, a, b) {
  const text = fs.readFileSync(path.join(root(), file), 'utf8');
  assert.ok(text.includes('unread_parts'), `${file} does not consult the unread_parts state`);
  for (const tool of [a, b]) {
    assert.ok(text.includes(`"${tool}"`), `${file} never names the forced tool "${tool}"`);
  }
  assert.ok(!/eq\(t, "fault tool", faults\[0\]\["tool"\], any\("submit_review"\)\)/.test(text), `${file} still asserts an unconditional forced submit_review`);
});

Then(P + 'every changed file is a test file or a fixture', function () {
  const run = (args) => spawnSync('git', args, { cwd: root(), encoding: 'utf8' }).stdout.split('\n').filter(Boolean);
  const changed = [...run(['diff', '--name-only', 'HEAD']), ...run(['ls-files', '--others', '--exclude-standard'])]
    .filter((f) => !f.startsWith('.kogen/') && !f.startsWith('features/steps/'));
  const bad = changed.filter((f) => !(/_test\.go$/.test(f) || /(^|\/)testdata\//.test(f)));
  assert.deepEqual(bad, [], `non-test files changed: ${bad.join(', ')}`);
});
