'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert');

let binary;
let projectDir;
let run;

Given('reject-mode-inappropriate-cli-flags: the kogen-build program is compiled', { timeout: 115000 }, function () {
  if (binary && fs.existsSync(binary)) return;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kogen-build-modeflags-'));
  const out = path.join(dir, 'kogen-build');
  const r = spawnSync('go', ['build', '-o', out, './cmd/kogen-build'], {
    cwd: process.cwd(),
    encoding: 'utf8',
    env: { ...process.env, GOFLAGS: '-trimpath' },
    timeout: 110000,
  });
  assert.strictEqual(r.status, 0, 'go build ./cmd/kogen-build failed:\n' + (r.stdout || '') + (r.stderr || ''));
  binary = out;
  projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'kogen-build-modeflags-project-'));
});

When('reject-mode-inappropriate-cli-flags: it runs with the arguments {string}', { timeout: 60000 }, function (args) {
  const argv = args.split(/\s+/).filter(Boolean).concat(['--project', projectDir]);
  const r = spawnSync(binary, argv, { cwd: projectDir, encoding: 'utf8', timeout: 50000 });
  run = { argv, status: r.status, stdout: r.stdout || '', stderr: r.stderr || '' };
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

// The part of the message before the appended expected-forms text, which itself names both flags.
function leadingMessage(obj) {
  const msg = obj.error && obj.error.message;
  assert.strictEqual(typeof msg, 'string', 'error message missing: ' + run.stdout);
  const i = msg.indexOf('expected build');
  return i >= 0 ? msg.slice(0, i) : msg;
}

Then('reject-mode-inappropriate-cli-flags: stdout is one JSON usage refusal and the exit status is 2', function () {
  const obj = parsed();
  assert.strictEqual(run.status, 2, 'process exit status ' + run.status + ' for ' + run.argv.join(' ') + '; stdout: ' + run.stdout.slice(0, 500));
  assert.strictEqual(obj.exit_code, 2);
  assert.ok(Array.isArray(obj.checks) && obj.checks.length === 0, 'checks must be []');
  assert.ok(obj.error && typeof obj.error === 'object', 'error object missing: ' + run.stdout);
  assert.strictEqual(obj.error.category, 'usage');
  assert.strictEqual(obj.error.kind, 'invalid_arguments');
  assert.ok(typeof obj.error.message === 'string' && obj.error.message.length > 0, 'error message is empty');
});

Then('reject-mode-inappropriate-cli-flags: the refusal message names {string} and not {string}', function (named, other) {
  const lead = leadingMessage(parsed());
  assert.ok(lead.includes(named), 'refusal message ' + JSON.stringify(lead) + ' does not name ' + named);
  assert.ok(!lead.includes(other), 'refusal message ' + JSON.stringify(lead) + ' names ' + other);
});

Then('reject-mode-inappropriate-cli-flags: the program was not refused as a usage error', function () {
  const obj = parsed();
  const cat = obj.error && obj.error.category;
  const kind = obj.error && obj.error.kind;
  assert.ok(!(cat === 'usage' && kind === 'invalid_arguments'), 'arguments ' + run.argv.join(' ') + ' were refused as a usage error: ' + run.stdout.slice(0, 500));
});
