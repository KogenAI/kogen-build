'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert');

let binary;
let run;

Given('cli-usage-refusal-names-input: the kogen-build program is compiled', { timeout: 115000 }, function () {
  if (binary && fs.existsSync(binary)) return;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kogen-build-usage-'));
  const out = path.join(dir, 'kogen-build');
  const r = spawnSync('go', ['build', '-o', out, './cmd/kogen-build'], {
    cwd: process.cwd(),
    encoding: 'utf8',
    env: { ...process.env, GOFLAGS: '-trimpath' },
    timeout: 110000,
  });
  assert.strictEqual(r.status, 0, 'go build ./cmd/kogen-build failed:\n' + (r.stdout || '') + (r.stderr || ''));
  binary = out;
});

When('cli-usage-refusal-names-input: it runs with the arguments {string}', { timeout: 60000 }, function (args) {
  const r = spawnSync(binary, args.split(/\s+/).filter(Boolean), {
    cwd: process.cwd(),
    encoding: 'utf8',
    timeout: 50000,
  });
  run = { status: r.status, stdout: r.stdout || '', stderr: r.stderr || '' };
});

function parsed() {
  let obj;
  try {
    obj = JSON.parse(run.stdout);
  } catch (e) {
    assert.fail('stdout is not a single JSON object: ' + JSON.stringify(run.stdout.slice(0, 500)));
  }
  assert.ok(obj && typeof obj === 'object' && !Array.isArray(obj), 'stdout JSON is not an object');
  return obj;
}

Then('cli-usage-refusal-names-input: stdout is one JSON usage refusal and the exit status is 2', function () {
  const obj = parsed();
  assert.strictEqual(run.status, 2, 'process exit status ' + run.status + '; stderr: ' + run.stderr.slice(0, 500));
  assert.strictEqual(obj.exit_code, 2);
  assert.ok(Array.isArray(obj.checks) && obj.checks.length === 0, 'checks must be []');
  assert.ok(obj.error && typeof obj.error === 'object', 'error object missing: ' + run.stdout);
  assert.strictEqual(obj.error.category, 'usage');
  assert.strictEqual(obj.error.kind, 'invalid_arguments');
  assert.strictEqual(typeof obj.error.message, 'string');
  assert.ok(obj.error.message.length > 0, 'error message is empty');
});

Then('cli-usage-refusal-names-input: the error message contains {string}', function (text) {
  const obj = parsed();
  const msg = obj.error && obj.error.message;
  assert.ok(typeof msg === 'string' && msg.includes(text), 'error message ' + JSON.stringify(msg) + ' does not contain ' + JSON.stringify(text));
});
