'use strict';
// Steps of k-review-seam: the package seam between build, build/review and build/core, checked
// with the go command in this checkout or in a temporary copy of it (harness in k_review_seam/).
const { Given, When, Then, After } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { root, MODULE, copyCheckout, goRun, goFiles, packageName, replaceAll, digest, judges, relevant } = require('./k_review_seam/harness.js');

const slow = { timeout: 30 * 60 * 1000 };

After(function () {
  if (this.seamCopy) fs.rmSync(this.seamCopy, { recursive: true, force: true });
  if (this.seamOverlay) fs.rmSync(this.seamOverlay, { recursive: true, force: true });
});

function fail(problems) {
  if (problems.length > 0) throw new Error(problems.join('\n'));
}

function judged(world, judge, expected) {
  const rec = world.seamRun;
  if (!rec) throw new Error('no go command ran before this step');
  const problems = judges[judge](expected, rec);
  if (problems.length > 0) {
    throw new Error([`${rec.command} (in ${rec.dir}), exit ${rec.exit}: ${problems.join('; ')}`, 'relevant lines of its output:', ...relevant(rec.output)].join('\n'));
  }
}

function words(command) {
  const w = command.split(' ').filter(Boolean);
  if (w[0] !== 'go') throw new Error(`not a go command: ${command}`);
  return w.slice(1);
}

When('the seam lists the Go dependencies, tests included, of {string}', slow, function (pkg) {
  this.seamRun = { ...goRun(root, ['list', '-deps', '-test', '-f', '{{.ImportPath}}', pkg]), dir: root };
  this.seamListed = pkg;
});

Then('the kogen-build packages among them are exactly:', function (table) {
  judged(this, 'deps', { allowed: table.raw().map((r) => r[0]) });
});

Then('the seam\'s package {string} imports:', slow, function (pkg, table) {
  this.seamRun = { ...goRun(root, ['list', '-f', '{{join .Imports "\\n"}}', pkg]), dir: root };
  judged(this, 'imports', { imports: table.raw().map((r) => r[0]) });
});

Given('a seam copy of this checkout', function () {
  this.seamCopy = copyCheckout();
});

When('the seam runs {string} in the copy', slow, function (command) {
  if (!this.seamCopy) throw new Error('no seam copy');
  this.seamRun = { ...goRun(this.seamCopy, words(command)), dir: this.seamCopy };
});

When('the seam runs {string} in this checkout', slow, function (command) {
  this.seamRun = { ...goRun(root, words(command)), dir: root };
});

Then('the seam run passes, and a test of build\\/core or build\\/review ran', function () {
  judged(this, 'passed', { packages: [`${MODULE}/build/core`, `${MODULE}/build/review`] });
});

Given('in the seam copy a new package {string} exists and a non-test Go file of {string} imports it', function (probe, rel) {
  if (!this.seamCopy) throw new Error('no seam copy');
  const name = packageName(this.seamCopy, rel);
  const probeDir = path.join(this.seamCopy, probe);
  fs.mkdirSync(probeDir, { recursive: true });
  fs.writeFileSync(path.join(probeDir, 'seamprobe.go'), `package ${path.basename(probe)}\n\n// Probe exists only to be imported.\nconst Probe = 1\n`);
  fs.writeFileSync(path.join(this.seamCopy, rel, 'zz_seamprobe.go'), `package ${name}\n\nimport _ "${MODULE}/${probe}"\n`);
});

Then('the seam run fails in a test whose output names {string}', function (name) {
  judged(this, 'refused', { names: [name] });
});

Then('each of these texts is in a non-test Go file under {string} and in no non-test Go file of {string} or under {string}:', function (home, flat, deep, table) {
  const contents = (files) => files.map((f) => [f, fs.readFileSync(path.join(root, f), 'utf8')]);
  const homes = contents(goFiles(root, home, true));
  const others = [...contents(goFiles(root, flat, false)), ...contents(goFiles(root, deep, true))];
  const problems = [];
  for (const [text] of table.raw()) {
    if (!homes.some(([, c]) => c.includes(text))) problems.push(`no non-test Go file under ${home} contains ${JSON.stringify(text)}`);
    for (const [f] of others.filter(([, c]) => c.includes(text))) problems.push(`${f} contains ${JSON.stringify(text)}`);
  }
  fail(problems);
});

Then('these files have these SHA-256 digests:', function (table) {
  fail(table.raw().map(([rel, want]) => [rel, digest(path.join(root, rel)), want])
    .filter(([, got, want]) => got !== want).map(([rel, got, want]) => `${rel} has SHA-256 ${got}, want ${want}`));
});

Then('no file under {string} is modified, deleted or renamed relative to HEAD', function (rel) {
  const st = spawnSync('git', ['diff', '--no-renames', '--name-status', 'HEAD', '--', rel], { cwd: root, encoding: 'utf8', maxBuffer: 64 << 20 });
  if (st.status !== 0) throw new Error(`git diff in ${root} failed (exit ${st.status}): ${st.stderr}`);
  fail(st.stdout.split('\n').filter(Boolean).filter((l) => !l.startsWith('A\t')).map((l) => `${l.replace('\t', ' ')} relative to HEAD`));
});

Then('the seam run passes, and these tests report PASS:', function (table) {
  judged(this, 'tests', { tests: table.raw().map((r) => r[0]) });
});

Given('in the seam copy every {string} in the non-test Go files under {string} is replaced by {string}', function (from, rel, to) {
  if (!this.seamCopy) throw new Error('no seam copy');
  const n = replaceAll(this.seamCopy, rel, from, to);
  if (n === 0) throw new Error(`no non-test Go file under ${rel} contains ${from}`);
});

Then('the seam run fails in a test whose output names each of:', function (table) {
  judged(this, 'refused', { names: table.raw().map((r) => r[0]) });
});

When('the seam compiles {string} as a new package of this module', slow, function (fixture) {
  this.seamOverlay = fs.mkdtempSync(path.join(os.tmpdir(), 'k-review-seam-overlay-'));
  const dir = path.join(root, 'kogenreviewseamconsumer');
  if (fs.existsSync(dir)) throw new Error(`${dir} exists`);
  const overlay = path.join(this.seamOverlay, 'overlay.json');
  fs.writeFileSync(overlay, JSON.stringify({ Replace: { [path.join(dir, 'consumer.go')]: path.join(root, fixture) } }));
  this.seamRun = { ...goRun(root, ['build', `-overlay=${overlay}`, './kogenreviewseamconsumer']), dir: root };
});

Then('the seam run compiles without a diagnostic', function () {
  judged(this, 'compiled', {});
});

Then('the seam run lists every name in {string}', function (rel) {
  const names = fs.readFileSync(path.join(root, rel), 'utf8').split('\n').filter(Boolean);
  if (names.length === 0) throw new Error(`${rel} names no test`);
  judged(this, 'listed', { names });
});


Then('the seam\'s README.md has a line that starts with each of:', function (table) {
  const lines = fs.readFileSync(path.join(root, 'README.md'), 'utf8').split('\n');
  fail(table.raw().map((r) => r[0]).filter((p) => !lines.some((l) => l.startsWith(p))).map((p) => `README.md has no line that starts with ${JSON.stringify(p)}`));
});
