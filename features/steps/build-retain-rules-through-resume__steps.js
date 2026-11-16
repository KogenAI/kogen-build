const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then} = require('@cucumber/cucumber');

const slug = 'build-retain-rules-through-resume';
const helper = `features/steps/${slug}__driver/fixture_test.go.txt`;
const coordinator = 'github.com/KogenAI/kogen-coordinator';
let dir = null;
let binary = null;
let compileError = null;

// The newest coordinator source in the module cache that has the rules package. The fixture is
// built against it through a temporary modfile, so the step never depends on go.mod's pin being
// resolvable in a fresh copy and never edits go.mod or go.sum.
function cachedCoordinator() {
  const env = spawnSync('go', ['env', 'GOMODCACHE'], {encoding: 'utf8'});
  const base = path.join(env.stdout.trim(), 'github.com', '!kogen!a!i');
  let names = [];
  try { names = fs.readdirSync(base); } catch (e) { return null; }
  const ok = names.filter(n => n.startsWith('kogen-coordinator@') && fs.existsSync(path.join(base, n, 'rules', 'rules.go'))).sort();
  return ok.length ? path.join(base, ok[ok.length - 1]) : null;
}

function compile() {
  if (binary || compileError) return;
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'retain-rules-'));
  const overlay = path.join(dir, 'overlay.json');
  const out = path.join(dir, 'build.test');
  const modfile = path.join(dir, 'go.mod');
  let mod = fs.readFileSync('go.mod', 'utf8');
  const source = cachedCoordinator();
  if (source) mod += `\nreplace ${coordinator} => ${source}\n`;
  fs.writeFileSync(modfile, mod);
  fs.copyFileSync('go.sum', path.join(dir, 'go.sum'));
  fs.writeFileSync(overlay, JSON.stringify({Replace: {
    [path.resolve('build/studio_rules_resume_acceptance_test.go')]: path.resolve(helper)
  }}));
  const args = ['test', '-c', `-modfile=${modfile}`, '-overlay', overlay, '-o', out, './build'];
  const built = spawnSync('go', args, {encoding: 'utf8', timeout: 100000, env: {...process.env, GOFLAGS: '-mod=mod', GOPROXY: 'off'}});
  if (built.status === 0) binary = out;
  else compileError = `go ${args.join(' ')}: status=${built.status} ${built.error || ''}\n${built.stdout}\n${built.stderr}`;
}

Given(`${slug}: the focused Go fixture is available`, function() {
  assert.ok(fs.statSync(helper).isFile());
});

When(`${slug}: I run the fixture test {string}`, {timeout: 190000}, function(name) {
  compile();
  this.out = null;
  this.diag = compileError || '';
  if (!binary) return;
  const result = path.join(dir, name + '.json');
  const args = ['-test.run', `^${name}$`, '-test.count=1', '-test.timeout=80s'];
  const p = spawnSync(binary, args, {encoding: 'utf8', timeout: 90000, env: {...process.env, RETAIN_RULES_RESULT: result}});
  this.diag = `${name}: status=${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`;
  if (p.status === 0 && fs.existsSync(result)) this.out = JSON.parse(fs.readFileSync(result, 'utf8'));
  else this.diag += `\n${name} failed, was absent or skipped`;
});

function lookup(world, field) {
  assert.ok(world.out, `fixture produced no result: ${world.diag}`);
  let cur = world.out;
  for (const part of field.split('.')) {
    if (cur === null || cur === undefined) return undefined;
    cur = cur[part];
  }
  return cur;
}

Then(`${slug}: field {string} is {string}`, function(field, want) {
  assert.equal(String(lookup(this, field)), want, `${field}\n${this.diag}`);
});

Then(`${slug}: field {string} matches {string}`, function(field, re) {
  const got = String(lookup(this, field));
  assert.ok(new RegExp(re).test(got), `${field}=${got} does not match ${re}`);
});

Then(`${slug}: the model request contains {string}`, function(text) {
  const got = String(lookup(this, 'request'));
  assert.ok(got.includes(text), `request lacks ${JSON.stringify(text)}\n${this.diag}`);
});

Then(`${slug}: the model request does not contain {string}`, function(text) {
  const got = String(lookup(this, 'request'));
  assert.ok(!got.includes(text), `request contains ${JSON.stringify(text)}`);
});
