'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then} = require('@cucumber/cucumber');

const slug = 'run-write-path-confinement';
const fixture = `features/steps/${slug}__driver/run_write_acceptance_test.go.txt`;
let dir = null;
let binary = null;

function compile() {
  if (binary) return;
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'run-write-'));
  const overlay = path.join(dir, 'overlay.json');
  const out = path.join(dir, 'build.test');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {
    [path.resolve('build/run_write_acceptance_test.go')]: path.resolve(fixture)
  }}));
  const args = ['test', '-c', '-overlay', overlay, '-o', out, './build'];
  const built = spawnSync('go', args, {encoding: 'utf8', timeout: 100000});
  assert.equal(built.status, 0, `go ${args.join(' ')}: status=${built.status} ${built.error || ''}\n${built.stdout}\n${built.stderr}`);
  binary = out;
}

Given(`${slug}: the Go fixture is available`, function() {
  assert.ok(fs.statSync(fixture).isFile());
});

When(`${slug}: I run the fixture test {string}`, {timeout: 115000}, function(name) {
  compile();
  const args = ['-test.run', `^${name}$`, '-test.count=1', '-test.v', '-test.timeout=60s'];
  const p = spawnSync(binary, args, {encoding: 'utf8', timeout: 70000, cwd: path.resolve('build')});
  this.rwName = name;
  this.rwStatus = p.status;
  this.rwOut = `${name}: ${binary} ${args.join(' ')}: status=${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`;
});

Then(`${slug}: the fixture test passed`, function() {
  assert.equal(this.rwStatus, 0, this.rwOut);
  assert.match(this.rwOut, new RegExp(`^--- PASS: ${this.rwName} `, 'm'), `${this.rwName} was absent, skipped or did not run:\n${this.rwOut}`);
});
