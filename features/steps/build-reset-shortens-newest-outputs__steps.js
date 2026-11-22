const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then} = require('@cucumber/cucumber');

const slug = 'build-reset-shortens-newest-outputs';
const helper = `features/steps/${slug}__driver/shorten_test.go.txt`;
let dir = null;
let binary = null;

function compile() {
  if (binary) return;
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'reset-shortens-'));
  const overlay = path.join(dir, 'overlay.json');
  binary = path.join(dir, 'build.test');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {
    [path.resolve('build/zz_shorten_acceptance_test.go')]: path.resolve(helper)
  }}));
  const args = ['test', '-c', '-overlay', overlay, '-o', binary, './build'];
  const built = spawnSync('go', args, {encoding: 'utf8', timeout: 100000});
  assert.equal(built.status, 0, `go ${args.join(' ')}: status=${built.status} ${built.error || ''}\n${built.stdout}\n${built.stderr}`);
}

Given(`${slug}: the focused Go fixture is available`, function() {
  assert.ok(fs.statSync(helper).isFile(), `${helper} is missing`);
});

When(`${slug}: I run the focused Go test {string}`, {timeout: 115000}, function(name) {
  compile();
  const args = ['-test.run', `^${name}$`, '-test.count=1', '-test.v', '-test.timeout=100s'];
  const p = spawnSync(binary, args, {encoding: 'utf8', timeout: 110000, cwd: path.resolve('build')});
  this.shortenName = name;
  this.shortenStatus = p.status;
  this.shortenOut = p.stdout || '';
  this.shortenDiag = `${name}: ${binary} ${args.join(' ')}: status=${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`;
});

Then(`${slug}: the focused Go test passed`, function() {
  assert.equal(this.shortenStatus, 0, this.shortenDiag);
  assert.ok(this.shortenOut.includes(`--- PASS: ${this.shortenName} `), `${this.shortenName} was absent, skipped or did not run:\n${this.shortenDiag}`);
});
