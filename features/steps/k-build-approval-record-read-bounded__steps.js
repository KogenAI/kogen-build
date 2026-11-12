'use strict';
// Steps of k-build-approval-record-read-bounded: a driver Go file (overlaid into package build for
// one `go test -c` compile, so no checkout file changes) is compiled into a program that runs
// Attempt.approvalIntact and Attempt.indexIntact on prepared files and writes each outcome (error
// text, "" for equal) as JSON. The step runs that compiled program, not a test of the checkout.
const { Given, When, Then } = require('@cucumber/cucumber');
const assert = require('node:assert');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..');
const driver = path.join(__dirname, 'k-build-approval-record-read-bounded__driver', 'driver_test.go.txt');
const P = 'k-build-approval-record-read-bounded: ';

Given(P + 'the driver is available', function () {
  assert.ok(fs.existsSync(driver), 'missing driver ' + driver);
});

When(P + 'the comparisons run on prepared files', function () {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kbarr-'));
  const ov = path.join(dir, 'overlay.json');
  const outFile = path.join(dir, 'out.json');
  const bin = path.join(dir, 'driver.bin');
  fs.writeFileSync(ov, JSON.stringify({ Replace: { 'build/zz_kbarr_driver_test.go': driver } }));
  const compile = ['go', 'test', '-overlay', ov, '-c', '-o', bin, './build'];
  const c = spawnSync(compile[0], compile.slice(1), { cwd: root, encoding: 'utf8', timeout: 240000 });
  assert.ok(fs.existsSync(bin), `driver did not compile; ran ${compile.join(' ')} exit ${c.status}:\n${(c.stdout || '') + (c.stderr || '')}`.slice(0, 3000));
  const r = spawnSync(bin, ['-test.run', '^TestKBARRDriver$', '-test.count=1'], { cwd: path.join(root, 'build'), encoding: 'utf8', timeout: 120000, env: { ...process.env, KBARR_OUT: outFile } });
  assert.ok(fs.existsSync(outFile), `driver wrote no result; ran ${bin} exit ${r.status}:\n${(r.stdout || '') + (r.stderr || '')}`.slice(0, 3000));
  this.kbarr = JSON.parse(fs.readFileSync(outFile, 'utf8'));
  fs.rmSync(dir, { recursive: true, force: true });
});

function outcome(w, name) {
  assert.ok(name in w.kbarr, 'no outcome for ' + name);
  return w.kbarr[name];
}

Then(P + '{word} reports equal', function (name) {
  assert.strictEqual(outcome(this, name), '', name + ' should compare equal');
});

Then(P + '{word} reports the approval changed', function (name) {
  assert.strictEqual(outcome(this, name), 'the approval changed');
});

Then(P + '{word} is refused as {word} naming the path', function (name, kind) {
  const msg = outcome(this, name);
  const words = { oversized: 'oversized', irregular: 'not a regular file', unreadable: 'unreadable' };
  assert.ok(words[kind], 'unknown kind ' + kind);
  assert.notStrictEqual(msg, 'BLOCKED', name + ' blocked instead of being refused');
  assert.ok(msg.toLowerCase().includes(words[kind]), `${name}: expected "${words[kind]}" in: ${msg}`);
  assert.ok(!/changed/.test(msg), `${name}: must not say changed: ${msg}`);
  const base = name.startsWith('approval') ? 'approval' : 'index';
  assert.ok(msg.includes(base), `${name}: expected the path (containing "${base}") in: ${msg}`);
});
