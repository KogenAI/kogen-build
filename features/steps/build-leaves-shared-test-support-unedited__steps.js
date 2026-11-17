const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then} = require('@cucumber/cucumber');

const slug = 'build-leaves-shared-test-support-unedited';
const helper = `features/steps/${slug}__driver/fixture_test.go.txt`;
let dir = null;
let binary = null;

function compile() {
  if (binary) return;
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'shared-support-'));
  const overlay = path.join(dir, 'overlay.json');
  binary = path.join(dir, 'build.test');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {
    [path.resolve('build/shared_support_acceptance_test.go')]: path.resolve(helper)
  }}));
  const args = ['test', '-c', '-overlay', overlay, '-o', binary, './build'];
  const built = spawnSync('go', args, {encoding: 'utf8', timeout: 100000});
  assert.equal(built.status, 0, `go ${args.join(' ')}: status=${built.status} ${built.error || ''}\n${built.stdout}\n${built.stderr}`);
}

Given(`${slug}: the focused Go fixture is available`, function() {
  assert.ok(fs.statSync(helper).isFile());
});

When(`${slug}: I run the fixture test {string}`, {timeout: 115000}, function(name) {
  compile();
  const out = path.join(dir, name + '.dir');
  fs.mkdirSync(out, {recursive: true});
  const args = ['-test.run', `^${name}$`, '-test.count=1', '-test.v', '-test.timeout=100s'];
  const p = spawnSync(binary, args, {encoding: 'utf8', timeout: 110000, cwd: path.resolve('build'), env: {...process.env, SHARED_SUPPORT_RESULT_DIR: out}});
  this.sharedDiag = `${name}: ${binary} ${args.join(' ')}: status=${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`;
  this.sharedStatus = p.status;
  this.sharedRan = fs.readdirSync(out).length;
  this.sharedName = name;
});

Then(`${slug}: the fixture test passed`, function() {
  assert.equal(this.sharedStatus, 0, this.sharedDiag);
  assert.equal(this.sharedRan, 1, `${this.sharedName} was absent, skipped or did not finish:\n${this.sharedDiag}`);
});
