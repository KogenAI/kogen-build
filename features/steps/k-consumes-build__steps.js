'use strict';
// Step definitions of the Intent k-consumes-build: kogen-build's consumes/<producer>/ suites.
const { When, Then } = require('@cucumber/cucumber');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const report = require('./k-consumes-build__report.js');

const root = path.resolve(__dirname, '..', '..');

function goCommand(command, sub) {
  const args = command.split(' ');
  assert.equal(args.shift(), 'go', `${command} is not a go command`);
  assert.equal(args.shift(), sub, `${command} is not a go ${sub} command`);
  return args;
}

When('k-consumes-build: the consumes observer judges this checkout against these suites:', { timeout: 600000 }, function (table) {
  this.observed = report.observe(root, table.hashes().map((r) => [r.suite, r.module]));
});

Then('k-consumes-build: the observer finds no problem', function () {
  const r = this.observed;
  assert.ok(r.status === 0 && r.output === 'ok\n', `${r.command} (in ${root}) exited ${r.status}:\n${r.output}`);
});

When('k-consumes-build: {string} runs in this checkout', { timeout: 900000 }, function (command) {
  this.tested = report.goTest(root, goCommand(command, 'test'), false);
});

When('k-consumes-build: {string} runs in this checkout with a bogus GIT_DIR, a fresh HOME that signs Git commits, refusing HTTP proxies and no module proxy', { timeout: 900000 }, function (command) {
  this.tested = report.goTest(root, goCommand(command, 'test'), true);
});

Then('k-consumes-build: go test passed and ran tests in each of these packages:', function (table) {
  const pkgs = table.raw().map(([p]) => p);
  const problems = report.testProblems(this.tested, pkgs);
  assert.deepEqual(problems, [], `${this.tested.command}: ${problems.join('; ')}:\n${this.tested.output}`);
});

Then('k-consumes-build: {string} lists each of these packages:', function (command, table) {
  const args = goCommand(command, 'list');
  const r = report.run('go', ['list', ...args], root, report.plainEnv());
  assert.equal(r.status, 0, `${command} (in ${root}) exited ${r.status}:\n${r.output}`);
  const listed = r.output.split('\n').filter(Boolean);
  const missing = table.raw().map(([p]) => p).filter((p) => !listed.includes(p));
  assert.deepEqual(missing, [], `${command} (in ${root}) does not list ${missing.join(', ')}; it lists:\n${r.output}`);
});

// files lists the regular files under each path of the checkout, relative to it, sorted.
function files(paths) {
  const out = [];
  const walk = (rel) => {
    const abs = path.join(root, rel);
    if (!fs.existsSync(abs)) return;
    if (fs.statSync(abs).isDirectory()) {
      for (const name of fs.readdirSync(abs)) walk(path.posix.join(rel, name));
    } else {
      out.push(rel);
    }
  };
  paths.forEach(walk);
  return out.sort();
}

Then('k-consumes-build: these paths are as committed at HEAD:', function (table) {
  const paths = table.raw().map(([p]) => p);
  const env = report.plainEnv();
  const listed = report.run('git', ['ls-tree', '-r', 'HEAD', '--', ...paths], root, env);
  assert.equal(listed.status, 0, `git ls-tree -r HEAD -- ${paths.join(' ')} exited ${listed.status}:\n${listed.output}`);
  const committed = new Map(listed.output.split('\n').filter(Boolean).map((line) => {
    const [meta, file] = line.split('\t');
    return [file, meta.split(' ')[2]];
  }));
  assert.ok(committed.size > 0, `HEAD holds none of ${paths.join(', ')}`);
  const changes = [];
  for (const file of files(paths)) {
    if (!committed.has(file)) {
      changes.push(`${file} (added)`);
      continue;
    }
    const hashed = report.run('git', ['hash-object', '--no-filters', '--', file], root, env);
    assert.equal(hashed.status, 0, `git hash-object ${file} exited ${hashed.status}:\n${hashed.output}`);
    if (hashed.output.trim() !== committed.get(file)) changes.push(`${file} (changed)`);
  }
  for (const file of committed.keys()) if (!fs.existsSync(path.join(root, file))) changes.push(`${file} (deleted)`);
  assert.deepEqual(changes, [], `not as committed at HEAD: ${changes.join(', ')}`);
});

Then('k-consumes-build: the only main package outside {string} is {string}', function (outside, want) {
  const r = report.run('go', ['list', '-f', '{{.ImportPath}} {{.Name}} {{.Dir}}', './...'], root, report.plainEnv());
  assert.equal(r.status, 0, `go list -f '{{.ImportPath}} {{.Name}} {{.Dir}}' ./... (in ${root}) exited ${r.status}:\n${r.output}`);
  const skip = path.join(root, outside);
  const mains = r.output.split('\n').filter(Boolean).map((l) => l.split(' '))
    .filter(([, name, ...dir]) => name === 'main' && !(dir.join(' ') === skip || dir.join(' ').startsWith(skip + path.sep)))
    .map(([p]) => p);
  assert.deepEqual(mains, [want], `the main packages outside ${outside}/ are: ${mains.join(', ') || 'none'}`);
});
