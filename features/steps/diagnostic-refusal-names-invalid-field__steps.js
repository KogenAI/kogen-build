'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert');

const DRIVER = './features/steps/diagnostic-refusal-names-invalid-field__driver';
let binary;
let observed;

Given('diagnostic-refusal-names-invalid-field: the diagnostic driver is compiled', { timeout: 115000 }, function () {
  if (binary && fs.existsSync(binary)) return;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'diagnostic-refusal-driver-'));
  const out = path.join(dir, 'driver');
  const r = spawnSync('go', ['build', '-o', out, DRIVER], {
    cwd: process.cwd(),
    encoding: 'utf8',
    env: { ...process.env, GOFLAGS: '-trimpath' },
    timeout: 110000,
  });
  assert.strictEqual(r.status, 0, 'go build ' + DRIVER + ' failed:\n' + (r.stdout || '') + (r.stderr || ''));
  binary = out;
});

When('diagnostic-refusal-names-invalid-field: a Build is started in a scratch checkout with the diagnostic case {string}', { timeout: 60000 }, function (name) {
  const r = spawnSync(binary, [name], { cwd: process.cwd(), encoding: 'utf8', timeout: 50000 });
  assert.strictEqual(r.status, 0, 'driver ' + JSON.stringify(name) + ' exited ' + r.status + ': ' + (r.stderr || '').slice(0, 500));
  try {
    observed = JSON.parse(r.stdout);
  } catch (e) {
    assert.fail('driver output is not JSON: ' + JSON.stringify(r.stdout.slice(0, 500)));
  }
});

Then('diagnostic-refusal-names-invalid-field: the Build is refused as intent\\/context_unavailable with exit code 2', function () {
  assert.strictEqual(observed.refused, true, 'Build was not refused: ' + JSON.stringify(observed));
  assert.strictEqual(observed.category, 'intent', JSON.stringify(observed));
  assert.strictEqual(observed.kind, 'context_unavailable', JSON.stringify(observed));
  assert.strictEqual(observed.exit, 2, JSON.stringify(observed));
});

// A detail names a field when it uses that field's name; the former generic detail called the
// source path a "source label" and listed the SHA-256 too, so it names several fields at once.
const FIELDS = {
  'text': (d) => /\btext\b/i.test(d),
  'source path': (d) => /source (path|label)/i.test(d),
  'SHA-256': (d) => /sha-256/i.test(d),
};

Then('diagnostic-refusal-names-invalid-field: the refusal detail names only the {string} field', function (field) {
  assert.ok(FIELDS[field], 'unknown field ' + field);
  const d = observed.detail;
  assert.ok(FIELDS[field](d), 'detail ' + JSON.stringify(d) + ' does not name the ' + field + ' field');
  for (const other of Object.keys(FIELDS)) {
    if (other !== field) assert.ok(!FIELDS[other](d), 'detail ' + JSON.stringify(d) + ' also names the ' + other + ' field');
  }
});

Then('diagnostic-refusal-names-invalid-field: no run or checkout change was made', function () {
  assert.strictEqual(observed.kogen_dir_created, false, 'refusal created .kogen state (a run directory): ' + JSON.stringify(observed));
  assert.strictEqual(observed.head_unchanged, true, 'HEAD changed: ' + JSON.stringify(observed));
  assert.strictEqual(observed.status_unchanged, true, 'checkout status changed: ' + JSON.stringify(observed));
});
