'use strict';

const { Given, When, Then, After, AfterAll } = require('@cucumber/cucumber');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { observeUsage, observeNativeHold } = require('./cli-refusal-input-coverage__observer.js');

let buildDir;
let binary;

Given('cli-refusal-input-coverage: the standalone kogen-build binary is built', { timeout: 115000 }, function () {
  if (!binary) {
    buildDir = fs.mkdtempSync(path.join(os.tmpdir(), 'kogen-cli-refusals-build-'));
    const out = path.join(buildDir, 'kogen-build');
    const result = spawnSync('go', ['build', '-o', out, './cmd/kogen-build'], { encoding: 'utf8', timeout: 110000 });
    if (result.error) throw result.error;
    assert.equal(result.status, 0, `go build ./cmd/kogen-build failed: ${result.stdout}${result.stderr}`);
    binary = out;
  }
  this.projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'kogen-cli-refusals-project-'));
});

After(function () {
  if (this.projectDir) fs.rmSync(this.projectDir, { recursive: true, force: true });
});

AfterAll(function () {
  if (buildDir) fs.rmSync(buildDir, { recursive: true, force: true });
});

function runCLI(world, args) {
  const result = spawnSync(binary, args, { cwd: world.projectDir, encoding: 'utf8', timeout: 5000 });
  if (result.error) throw result.error;
  world.refusal = result;
}

When('cli-refusal-input-coverage: I run kogen-build without arguments', function () {
  runCLI(this, []);
});

Then('cli-refusal-input-coverage: I receive a JSON usage refusal listing these supported modes', function (table) {
  observeUsage(this.refusal, { modes: table.raw().flat() });
});

When('cli-refusal-input-coverage: I run prepare-continuation with project {string}, ID {string}, and binding file {string} containing {string}', function (project, id, binding, contents) {
  const target = path.join(this.projectDir, binding);
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, contents);
  this.requestedID = id;
  this.binding = binding;
  runCLI(this, ['prepare-continuation', '--project', project, '--id', id, '--binding', binding]);
});

Then('cli-refusal-input-coverage: I receive a native hold with continuation evidence for the requested ID', function () {
  observeNativeHold(this.refusal, this.requestedID);
});

Then('cli-refusal-input-coverage: I receive a JSON usage refusal identifying the binding path without continuation evidence', function () {
  observeUsage(this.refusal, { binding: this.binding });
});
