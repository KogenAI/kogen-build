const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then} = require('@cucumber/cucumber');

const slug = 'build-use-studio-rule-snapshot';
const helper = `features/steps/${slug}__driver/fixture_test.go.txt`;
let dir = null;
let binary = null;
let compileError = null;

function compile() {
  if (binary || compileError) return;
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'studio-rules-'));
  const overlay = path.join(dir, 'overlay.json');
  const out = path.join(dir, 'build.test');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {
    [path.resolve('build/studio_rules_acceptance_test.go')]: path.resolve(helper)
  }}));
  const args = ['test', '-c', '-overlay', overlay, '-o', out, './build'];
  const built = spawnSync('go', args, {encoding: 'utf8', timeout: 100000});
  if (built.status === 0) binary = out;
  else compileError = `go ${args.join(' ')}: status=${built.status} ${built.error || ''}\n${built.stdout}\n${built.stderr}`;
}

Given(`${slug}: the focused Go fixture is available`, function() {
  assert.ok(fs.statSync(helper).isFile());
});

When(`${slug}: I run the fixture test {string}`, {timeout: 190000}, function(name) {
  compile();
  this.studio = null;
  this.studioDiag = compileError || '';
  if (!binary) return;
  const result = path.join(dir, name + '.json');
  const args = ['-test.run', `^${name}$`, '-test.count=1', '-test.timeout=80s'];
  const p = spawnSync(binary, args, {encoding: 'utf8', timeout: 90000, env: {...process.env, STUDIO_RULES_RESULT: result}});
  this.studioDiag = `${name}: status=${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`;
  if (p.status === 0 && fs.existsSync(result)) this.studio = JSON.parse(fs.readFileSync(result, 'utf8'));
  else this.studioDiag += `\n${name} failed, was absent or skipped`;
});

function lookup(world, field) {
  assert.ok(world.studio, `fixture produced no result: ${world.studioDiag}`);
  let cur = world.studio;
  for (const part of field.split('.')) {
    if (cur === null || cur === undefined) return undefined;
    cur = cur[part];
  }
  return cur;
}

Then(`${slug}: field {string} is {string}`, function(field, want) {
  const got = lookup(this, field);
  if (want === 'non-zero') return assert.ok(typeof got === 'number' && got !== 0, `${field}=${JSON.stringify(got)}\n${this.studioDiag}`);
  assert.equal(String(got), want, `${field}\n${this.studioDiag}`);
});

Then(`${slug}: field {string} is absent`, function(field) {
  assert.equal(lookup(this, field), undefined, field);
});

Then(`${slug}: field {string} contains {string}`, function(field, text) {
  const got = String(lookup(this, field));
  assert.ok(got.includes(text), `${field} does not contain ${JSON.stringify(text)}: ${got}`);
});

Then(`${slug}: field {string} matches {string}`, function(field, re) {
  const got = String(lookup(this, field));
  assert.ok(new RegExp(re).test(got), `${field}=${got} does not match ${re}`);
});

Then(`${slug}: the model request contains {string}`, function(text) {
  const got = String(lookup(this, 'request'));
  assert.ok(got.includes(text), `request lacks ${JSON.stringify(text)}`);
});

Then(`${slug}: the model request does not contain {string}`, function(text) {
  const got = String(lookup(this, 'request'));
  assert.ok(!got.includes(text), `request contains ${JSON.stringify(text)}`);
});
