'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then, AfterAll} = require('@cucumber/cucumber');

const P = 'build-model-promotion-empty-directory-recovery: ';
const helper = 'features/steps/build-model-promotion-empty-directory-recovery__driver/fixture_test.go.txt';
let dir, binary;

function compile() {
  if (binary) return;
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'model-promotion-'));
  const overlay = path.join(dir, 'overlay.json');
  const target = path.join(dir, 'build.test');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {
    [path.resolve('build/zz_model_promotion_acceptance_test.go')]: path.resolve(helper)
  }}));
  const args = ['test', '-c', '-overlay', overlay, '-o', target, './build'];
  const p = spawnSync('go', args, {encoding: 'utf8', timeout: 100000});
  assert.equal(p.status, 0, `go ${args.join(' ')}: exit ${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`);
  binary = target;
}

Given(P + 'the disposable Build fixture driver is available', function () {
  assert.ok(fs.statSync(helper).isFile(), helper + ' is missing');
  assert.ok(fs.existsSync('build/maintained.go'), 'build/maintained.go is missing');
});

When(P + 'I run the fixture tests {string}', {timeout: 118000}, function (names) {
  compile();
  const list = names.split(',');
  const run = '^TestBMPED(' + list.join('|') + ')$';
  const args = ['-test.run=' + run, '-test.count=1', '-test.v', '-test.timeout=100s'];
  const p = spawnSync(binary, args, {cwd: path.resolve('build'), encoding: 'utf8', timeout: 110000});
  this.promotion = {list, p, command: `${binary} ${args.join(' ')}`};
});

Then(P + 'the fixture tests {string} ran and passed', function (names) {
  const {list, p, command} = this.promotion;
  const out = (p.stdout || '') + (p.stderr || '');
  const tail = out.length > 6000 ? out.slice(-6000) : out;
  for (const name of names.split(',')) {
    assert.ok(list.includes(name), `TestBMPED${name} was not run by the preceding step`);
    assert.ok(out.includes(`--- PASS: TestBMPED${name} (`),
      `TestBMPED${name} did not pass (it was absent, skipped or failed); ${command}; exit ${p.status}\n${tail}`);
    assert.ok(!out.includes(`--- FAIL: TestBMPED${name} `), `TestBMPED${name} failed; ${command}\n${tail}`);
  }
  assert.equal(p.status, 0, `${command}: exit ${p.status} ${p.error || ''}\n${tail}`);
});

AfterAll(function () { if (dir) fs.rmSync(dir, {recursive: true, force: true}); });
