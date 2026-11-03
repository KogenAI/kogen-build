const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const test = 'TestTheReviewHookRunsAfterAGreenPassAndItsApprovalIsCommitted';
const pkg = 'github.com/KogenAI/kogen-build/build';
function passed(result) {
  const events = (result.stdout || '').split('\n').filter(x => x.startsWith('{')).map(x => JSON.parse(x));
  return result.status === 0 && events.filter(e => e.Package === pkg && e.Test === test && e.Action === 'pass').length === 1;
}
// Real pass event from the unchanged pinned-executor run; bad control changes
// only the outcome to ensure exit zero cannot hide an absent passing test.
const good = { status: 0, stdout: '{"Time":"2026-11-04T01:49:32.706026+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestTheReviewHookRunsAfterAGreenPassAndItsApprovalIsCommitted","Elapsed":2.15}\n' };
if (require.main === module) {
  const sample = process.argv[2] === 'good' ? good : { ...good, stdout: good.stdout.replace('"pass"', '"skip"') };
  const ok = passed(sample);
  console.log(ok ? 'accepted' : 'rejected');
  process.exitCode = ok ? 0 : 1;
} else {
  const { When, Then } = require('@cucumber/cucumber');
  When('k-build-note-line-integer: I run the review hook with executor {string}', { timeout: 180000 }, function (executor) {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'note-line-'));
    try {
      const args = ['test', '-json', './build', '-run', `^${test}$`, '-count=1'];
      if (executor === '391e0b6') {
        const mod = path.join(dir, 'go.mod');
        fs.copyFileSync('go.mod', mod);
        fs.copyFileSync('go.sum', path.join(dir, 'go.sum'));
        const edit = spawnSync('go', ['mod', 'edit', `-modfile=${mod}`, '-replace=github.com/KogenAI/kogen-executor=github.com/KogenAI/kogen-executor@v0.0.0-20261103202226-391e0b6ecd4f'], { encoding: 'utf8', timeout: 30000 });
        assert.equal(edit.status, 0, `${test}: go mod edit failed: ${edit.stdout}${edit.stderr}`);
        args.splice(1, 0, '-mod=mod', `-modfile=${mod}`);
      } else assert.equal(executor, 'pinned');
      this.noteCommand = `go ${args.join(' ')}`;
      this.noteResult = spawnSync('go', args, { encoding: 'utf8', timeout: 150000, maxBuffer: 16 * 1024 * 1024 });
    } finally { fs.rmSync(dir, { recursive: true, force: true }); }
  });
  Then('k-build-note-line-integer: the hook passes with integer note output {string} and not {string}', function (expected, rejected) {
    // The existing hook asserts these literals on actual Build output, including
    // the review approval and empty nil line, rather than compiler diagnostics.
    assert.ok(passed(this.noteResult), `${test}: ${this.noteCommand} absent, skipped, failed or ran zero times; expected ${expected}, not ${rejected}; status=${this.noteResult.status}\n${this.noteResult.stdout}\n${this.noteResult.stderr}`);
  });
}
