// Steps of k-build-loop-module-path: kogen-build requires every Kogen component at its current
// main by its real module path, and no Go file, package or build-list module uses the bare
// kogen-loop path. Every step reads the checkout or runs the real go tool or grep in it; none
// changes a checkout file (`go mod tidy -diff` only reports).
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const { When, Then } = require('@cucumber/cucumber');

const P = 'k-build-loop-module-path: ';

// run runs a program in the checkout, outside any Go workspace, and returns its result.
function run(cmd, args) {
  const r = spawnSync(cmd, args, {
    encoding: 'utf8',
    timeout: 540000,
    maxBuffer: 64 * 1024 * 1024,
    env: { ...process.env, GOWORK: 'off' },
  });
  if (r.error) throw new Error(`${cmd} ${args.join(' ')}: ${r.error.message}`);
  return r;
}

When(P + 'I read the module requirements of go.mod', function () {
  const r = run('go', ['mod', 'edit', '-json']);
  assert.equal(r.status, 0, `go mod edit -json failed (exit ${r.status}): ${r.stdout}${r.stderr}`);
  this.mod = JSON.parse(r.stdout);
});

Then(P + 'go.mod has no replace directive', function () {
  const replaced = (this.mod.Replace || []).map((r) => `${r.Old.Path}${r.Old.Version ? '@' + r.Old.Version : ''} => ${r.New.Path}${r.New.Version ? '@' + r.New.Version : ''}`);
  assert.deepEqual(replaced, [], `go.mod still has replace directives: ${replaced.join('; ')}`);
});

Then(P + 'go.mod does not require the module {string}', function (module) {
  const found = (this.mod.Require || []).filter((r) => r.Path === module).map((r) => `${r.Path} ${r.Version}`);
  assert.deepEqual(found, [], `go.mod still requires ${found.join(', ')}`);
});

Then(P + 'go.mod requires exactly these modules, none indirect:', function (table) {
  const key = (m) => `${m.path}@${m.version}${m.indirect ? ' // indirect' : ''}`;
  const got = (this.mod.Require || []).map((r) => key({ path: r.Path, version: r.Version, indirect: !!r.Indirect })).sort();
  const want = table.hashes().map((r) => key({ path: r.path, version: r.version, indirect: false })).sort();
  assert.deepEqual(got, want, `go.mod requirements differ:\n got  ${JSON.stringify(got, null, 1)}\n want ${JSON.stringify(want, null, 1)}`);
});

When(P + 'I grep every Go file for an import path starting with {string}', function (prefix) {
  // The request's check: grep -rn "\"kogen-loop/" --include=*.go .
  const args = ['-rn', `"${prefix}`, '--include=*.go', '.'];
  this.grepCommand = `grep ${args.map((a) => (/[\s"*]/.test(a) ? `'${a}'` : a)).join(' ')}`;
  this.grep = run('grep', args);
});

Then(P + 'grep finds nothing', function () {
  const output = `${this.grep.stdout}${this.grep.stderr}`;
  assert.equal(output, '', `${this.grepCommand} found matches or reported errors (exit ${this.grep.status}):\n${output}`);
  assert.equal(this.grep.status, 1, `${this.grepCommand} exited ${this.grep.status}, want 1 (no match)`);
});

When(P + 'I list every package kogen-build and its tests build from', { timeout: 600000 }, function () {
  const args = ['list', '-deps', '-test', '-f', '{{.ImportPath}}', './...'];
  const r = run('go', args);
  assert.equal(r.status, 0, `go ${args.join(' ')} failed (exit ${r.status}):\n${r.stdout}${r.stderr}`);
  // A test variant prints as "pkg [pkg.test]"; its package path is the first field.
  this.packages = [...new Set(r.stdout.split('\n').map((l) => l.trim().split(' ')[0]).filter(Boolean))];
});

Then(P + 'no listed package is {string} or below it', function (bare) {
  const offenders = this.packages.filter((p) => p === bare || p.startsWith(bare + '/'));
  assert.deepEqual(offenders, [], `go list -deps -test ./... still lists bare-path packages: ${offenders.join(', ')}`);
});

Then(P + 'the listed packages include {string}', function (pkg) {
  assert.ok(this.packages.includes(pkg), `go list -deps -test ./... does not list ${pkg}; kogen-loop packages listed: ${this.packages.filter((p) => p.includes('kogen-loop')).join(', ') || 'none'}`);
});

When(P + 'I run go mod tidy -diff', { timeout: 600000 }, function () {
  this.tidy = run('go', ['mod', 'tidy', '-diff']);
});

Then(P + 'go mod tidy reports no difference', function () {
  const output = `${this.tidy.stdout}${this.tidy.stderr}`;
  assert.equal(this.tidy.status, 0, `go mod tidy -diff exited ${this.tidy.status}; go.mod or go.sum is not tidy:\n${output}`);
  assert.equal(output, '', `go mod tidy -diff printed output:\n${output}`);
});

When(P + 'I list the modules of the build list', { timeout: 600000 }, function () {
  const args = ['list', '-m', '-f', '{{.Path}} {{.Version}}', 'all'];
  const r = run('go', args);
  assert.equal(r.status, 0, `go ${args.join(' ')} failed (exit ${r.status}):\n${r.stdout}${r.stderr}`);
  this.modules = r.stdout.split('\n').map((l) => l.trim()).filter(Boolean);
});

Then(P + 'the build list is exactly, in order:', function (table) {
  const want = table.hashes().map((r) => `${r.path} ${r.version}`.trim());
  assert.deepEqual(this.modules, want, `go list -m all differs:\n got  ${JSON.stringify(this.modules, null, 1)}\n want ${JSON.stringify(want, null, 1)}`);
});
