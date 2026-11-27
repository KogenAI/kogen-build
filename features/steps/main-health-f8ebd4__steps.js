'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then, AfterAll} = require('@cucumber/cucumber');

const P = 'main-health-f8ebd4: ';
const fixture = 'features/steps/build-model-promotion-empty-directory-recovery__driver/fixture_test.go.txt';
const maintained = 'build/maintained.go';
const rename = 'if err := syscall.Rename(stage, dir); err != nil {\n\t\treturn refuse(err)\n\t}';
const direct = 'if err := os.MkdirAll(dir, 0755); err != nil {\n\t\treturn refuse(err)\n\t}\n' +
  '\tif err := os.WriteFile(filepath.Join(dir, "model.qnt"), data, 0644); err != nil {\n\t\treturn refuse(err)\n\t}\n' +
  '\tif err := os.WriteFile(filepath.Join(dir, "model.json"), append(record, 0x0a), 0644); err != nil {\n\t\treturn refuse(err)\n\t}\n' +
  '\t_ = syscall.Getpid';
let dir;
const binaries = {};

function compile(kind) {
  if (binaries[kind]) return binaries[kind];
  if (!dir) dir = fs.mkdtempSync(path.join(os.tmpdir(), 'main-health-'));
  const replace = {[path.resolve('build/zz_main_health_fixture_test.go')]: path.resolve(fixture)};
  if (kind === 'nonatomic') {
    const src = fs.readFileSync(maintained, 'utf8');
    assert.ok(src.includes(rename), `${maintained} no longer publishes the pair by one rename of the staged directory`);
    const copy = path.join(dir, 'maintained_nonatomic.go');
    fs.writeFileSync(copy, src.replace(rename, direct));
    replace[path.resolve(maintained)] = copy;
  }
  const overlay = path.join(dir, kind + '.json');
  fs.writeFileSync(overlay, JSON.stringify({Replace: replace}));
  const target = path.join(dir, kind + '.test');
  const args = ['test', '-c', '-overlay', overlay, '-o', target, './build'];
  const p = spawnSync('go', args, {encoding: 'utf8', timeout: 100000});
  assert.equal(p.status, 0, `go ${args.join(' ')}: exit ${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`);
  binaries[kind] = target;
  return target;
}

function run(world, kind) {
  const binary = compile(kind);
  const args = ['-test.run=^TestBMPEDAtomicPublication$', '-test.count=1', '-test.v', '-test.timeout=100s'];
  const p = spawnSync(binary, args, {cwd: path.resolve('build'), encoding: 'utf8', timeout: 110000});
  world.atomic = {p, kind, command: `${binary} ${args.join(' ')}`, out: (p.stdout || '') + (p.stderr || '')};
}

Given(P + 'the model promotion fixture driver is available', function () {
  assert.ok(fs.statSync(fixture).isFile(), fixture + ' is missing');
  assert.ok(fs.existsSync(maintained), maintained + ' is missing');
});

When(P + 'I run the atomic publication fixture', {timeout: 118000}, function () {
  run(this, 'current');
});

When(P + 'I run the atomic publication fixture against a non-atomic publication', {timeout: 118000}, function () {
  run(this, 'nonatomic');
});

Then(P + 'the fixture passed with an observer count independent of the host CPU count', function () {
  const {p, command, out} = this.atomic;
  const tail = out.length > 4000 ? out.slice(-4000) : out;
  assert.ok(out.includes('--- PASS: TestBMPEDAtomicPublication ('),
    `TestBMPEDAtomicPublication did not pass (absent, skipped or failed); ${command}; exit ${p.status}\n${tail}`);
  assert.equal(p.status, 0, `${command}: exit ${p.status}\n${tail}`);
  const src = fs.readFileSync(fixture, 'utf8');
  const start = src.indexOf('func bmAtomicAttempt(');
  assert.ok(start >= 0, `${fixture} has no bmAtomicAttempt function`);
  const next = src.indexOf('\nfunc ', start + 1);
  const body = src.slice(start, next < 0 ? undefined : next);
  assert.ok(!body.includes('runtime.NumCPU'),
    `bmAtomicAttempt in ${fixture} still starts one busy observer per host CPU (runtime.NumCPU)`);
});

Then(P + 'the fixture rejected the publication as partial', function () {
  const {p, command, out} = this.atomic;
  const tail = out.length > 4000 ? out.slice(-4000) : out;
  assert.notEqual(p.status, 0, `TestBMPEDAtomicPublication passed against a non-atomic publication; ${command}\n${tail}`);
  assert.ok(out.includes('--- FAIL: TestBMPEDAtomicPublication'), `TestBMPEDAtomicPublication did not run and fail; ${command}\n${tail}`);
  assert.ok(out.includes('a partial model was observable during publication'), `the failure was not the partial-model rejection; ${command}\n${tail}`);
});

AfterAll(function () { if (dir) fs.rmSync(dir, {recursive: true, force: true}); });
