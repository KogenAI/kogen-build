// Steps of k-build-accept-canonical-loop-path: kogen-build requires and imports kogen-loop only by
// its canonical module path, github.com/KogenAI/kogen-loop, and compiles with kogen-verification
// 7ecd05a. Every step reads the checkout or runs the real go tool in it; the build with another
// requirement uses a temporary copy of go.mod and go.sum (-modfile), so no checkout file changes.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { When, Then } = require('@cucumber/cucumber');

const P = 'k-build-accept-canonical-loop-path: ';

// go runs the go tool in the checkout, outside any workspace, and returns its result.
function go(args) {
  const r = spawnSync('go', args, {
    encoding: 'utf8',
    timeout: 540000,
    maxBuffer: 64 * 1024 * 1024,
    env: { ...process.env, GOWORK: 'off' },
  });
  if (r.error) throw new Error(`go ${args.join(' ')}: ${r.error.message}`);
  return r;
}

// goFiles lists every .go file under dir, relative to it, leaving out .git and node_modules.
function goFiles(dir, rel = '') {
  const out = [];
  for (const e of fs.readdirSync(path.join(dir, rel), { withFileTypes: true })) {
    const p = rel ? `${rel}/${e.name}` : e.name;
    if (e.isDirectory()) {
      if (e.name !== '.git' && e.name !== 'node_modules') out.push(...goFiles(dir, p));
    } else if (e.isFile() && e.name.endsWith('.go')) out.push(p);
  }
  return out;
}

function escape(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

When(P + 'I read the module requirements of go.mod', function () {
  const r = go(['mod', 'edit', '-json']);
  assert.equal(r.status, 0, `go mod edit -json failed: ${r.stdout}${r.stderr}`);
  this.mod = JSON.parse(r.stdout);
});

Then(P + 'go.mod requires exactly these modules, none indirect:', function (table) {
  const key = (m) => `${m.path}@${m.version}${m.indirect ? ' // indirect' : ''}`;
  const got = (this.mod.Require || []).map((r) => key({ path: r.Path, version: r.Version, indirect: !!r.Indirect })).sort();
  const want = table.hashes().map((r) => key({ path: r.path, version: r.version, indirect: false })).sort();
  assert.deepEqual(got, want, `go.mod requirements differ:\n got  ${JSON.stringify(got, null, 1)}\n want ${JSON.stringify(want, null, 1)}`);
});

Then(P + 'go.mod has no replace directive', function () {
  const replaced = (this.mod.Replace || []).map((r) => `${r.Old.Path}${r.Old.Version ? '@' + r.Old.Version : ''} => ${r.New.Path}${r.New.Version ? '@' + r.New.Version : ''}`);
  assert.deepEqual(replaced, [], `go.mod still has replace directives: ${replaced.join('; ')}`);
});

When(P + 'I list every package kogen-build and its tests build from', { timeout: 600000 }, function () {
  const args = ['list', '-deps', '-test', '-f', '{{.ImportPath}}', './...'];
  const r = go(args);
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

Then(P + 'no Go file of the checkout imports {string} or a package below it', function (bare) {
  const spec = new RegExp(`^\\s*(?:import\\s+)?(?:[A-Za-z_][A-Za-z0-9_]*\\s+|[._]\\s+)?"${escape(bare)}(?:/[^"]*)?"`, 'gm');
  const root = process.cwd();
  const offenders = [];
  for (const f of goFiles(root)) {
    const text = fs.readFileSync(path.join(root, f), 'utf8');
    for (const m of text.matchAll(spec)) {
      const line = text.slice(0, m.index).split('\n').length;
      offenders.push(`${f}:${line}: ${m[0].trim()}`);
    }
  }
  assert.deepEqual(offenders, [], `Go files still import the bare ${bare} path:\n${offenders.join('\n')}`);
});

When(P + 'I build every package with {string} required at {string}', { timeout: 600000 }, function (module, version) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'canonical-loop-'));
  try {
    const mod = path.join(dir, 'go.mod');
    fs.copyFileSync('go.mod', mod);
    fs.copyFileSync('go.sum', path.join(dir, 'go.sum'));
    const edit = go(['mod', 'edit', `-modfile=${mod}`, `-require=${module}@${version}`]);
    assert.equal(edit.status, 0, `go mod edit -require=${module}@${version} failed: ${edit.stdout}${edit.stderr}`);
    const args = ['build', '-mod=mod', `-modfile=${mod}`, './...'];
    this.buildCommand = `go ${args.join(' ')} (with ${module}@${version} required)`;
    this.build = go(args);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

Then(P + 'the build succeeds without a {string} diagnostic', function (diagnostic) {
  const output = `${this.build.stdout}${this.build.stderr}`;
  assert.ok(!output.includes(diagnostic), `${this.buildCommand} reported "${diagnostic}":\n${output}`);
  assert.equal(this.build.status, 0, `${this.buildCommand} exited ${this.build.status}:\n${output}`);
});

When(P + 'I read {string}', function (file) {
  this.text = fs.readFileSync(file, 'utf8');
  this.file = file;
});

Then(P + 'its {string} row names the producer module {string}', function (suite, module) {
  const rows = this.text.split('\n')
    .filter((l) => l.trim().startsWith('|'))
    .map((l) => l.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((c) => c.trim()))
    .filter((cells) => cells[0] === suite);
  assert.equal(rows.length, 1, `${this.file}: expected exactly one table row for ${suite}, found ${rows.length}`);
  assert.equal(rows[0][1], module, `${this.file}: the ${suite} row names the producer module ${rows[0][1]}, want ${module}`);
});
