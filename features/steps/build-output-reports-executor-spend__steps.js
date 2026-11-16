const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then} = require('@cucumber/cucumber');

const slug = 'build-output-reports-executor-spend';
const helper = `features/steps/${slug}__driver/fixture_test.go.txt`;
let dir = null;
let binary = null;

function compile() {
  if (binary) return;
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'spend-report-'));
  const overlay = path.join(dir, 'overlay.json');
  binary = path.join(dir, 'build.test');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {
    [path.resolve('build/spend_report_acceptance_test.go')]: path.resolve(helper)
  }}));
  const args = ['test', '-c', '-overlay', overlay, '-o', binary, './build'];
  const built = spawnSync('go', args, {encoding: 'utf8', timeout: 100000});
  assert.equal(built.status, 0, `go ${args.join(' ')}: status=${built.status} ${built.error || ''}\n${built.stdout}\n${built.stderr}`);
}

function runTest(name) {
  compile();
  const result = path.join(dir, name + '.out');
  const args = ['-test.run', `^${name}$`, '-test.count=1', '-test.timeout=60s'];
  const p = spawnSync(binary, args, {encoding: 'utf8', timeout: 70000, env: {...process.env, SPEND_REPORT_RESULT: result}});
  const diag = `${name}: ${binary} ${args.join(' ')}: status=${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`;
  assert.equal(p.status, 0, diag);
  assert.ok(fs.existsSync(result), `${diag}\n${name} was absent, skipped or wrote no output`);
  return fs.readFileSync(result, 'utf8');
}

function lines(world) {
  return world.spendOutput.split('\n');
}

Given(`${slug}: the focused Go fixture is available`, function() {
  assert.ok(fs.statSync(helper).isFile());
});

When(`${slug}: I run the fixture test {string}`, {timeout: 190000}, function(name) {
  this.spendOutput = runTest(name);
});

Then(`${slug}: the output has the exact line {string}`, function(line) {
  assert.ok(lines(this).includes(line), `no line equals ${JSON.stringify(line)} in:\n${this.spendOutput}`);
});

Then(`${slug}: the output has no line containing {string}`, function(text) {
  const hit = lines(this).filter(l => l.includes(text));
  assert.deepEqual(hit, [], `lines containing ${JSON.stringify(text)} in:\n${this.spendOutput}`);
});

Then(`${slug}: the output has a line matching {string}`, function(re) {
  const rx = new RegExp(re);
  assert.ok(lines(this).some(l => rx.test(l)), `no line matches ${re} in:\n${this.spendOutput}`);
});

Then(`${slug}: the output has no line matching {string}`, function(re) {
  const rx = new RegExp(re);
  const hit = lines(this).filter(l => rx.test(l));
  assert.deepEqual(hit, [], `lines matching ${re} in:\n${this.spendOutput}`);
});

function adjacent(world, want, match) {
  const got = lines(world);
  for (let i = 0; i + want.length <= got.length; i++) {
    if (want.every((w, j) => match(w, got[i + j]))) return;
  }
  assert.fail(`the lines ${JSON.stringify(want)} are not adjacent and in order in:\n${world.spendOutput}`);
}

Then(`${slug}: the output has these adjacent lines, each a regular expression:`, function(text) {
  adjacent(this, text.split('\n'), (w, g) => new RegExp(w).test(g));
});

Then(`${slug}: the output has these adjacent exact lines:`, function(text) {
  adjacent(this, text.split('\n'), (w, g) => w === g);
});
