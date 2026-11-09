// Steps of build-consumes-executor-claude-fixture: kogen-build's context/external tests use the
// signed Executor Claude fixture (testkit/clauderuntime) instead of their own copy of the Claude
// runtime script. The Go runs go through build-consumes-executor-claude-fixture__observer.js, from
// the checkout's real path, against the pinned kogen-executor or a temporary copy of it with one
// change; no checkout file changes. The file steps read the checkout. Every failure message names
// the test, the go command and its outcome, or the files and what they hold.
'use strict';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { Given, When, Then } = require('@cucumber/cucumber');
const observer = require('./build-consumes-executor-claude-fixture__observer.js');

const P = 'build-consumes-executor-claude-fixture: ';
const LONG = { timeout: 600000 };
const EXECUTOR = 'github.com/KogenAI/kogen-executor';
// The focused context/external tests of ./build are the tests whose names contain this text (an
// unanchored go test -run pattern); the copies of kogen-executor run the same without the two
// long ones.
const FOCUSED = 'ContextExternal';
const FOCUSED_QUICK_SKIP = 'ContextExternal(ReviewMatrix|NativeBytes)';

function sha256(bytes) {
  return crypto.createHash('sha256').update(bytes).digest('hex');
}

function checkoutFile(rel) {
  return fs.readFileSync(path.join(observer.realRoot(), ...rel.split('/')), 'utf8');
}

// goOut runs the go tool in the checkout and returns its result with a description for messages.
function goOut(args) {
  const r = spawnSync('go', args, { cwd: observer.realRoot(), encoding: 'utf8', env: { ...process.env, GOWORK: 'off' } });
  return { ...r, command: `go ${args.join(' ')}`, text: `${r.stdout || ''}${r.stderr || ''}` };
}

Given(P + 'the checkout of kogen-build', function () {
  assert.match(checkoutFile('go.mod'), /^module github\.com\/KogenAI\/kogen-build$/m, 'go.mod is not the module of kogen-build');
});

function requirement(mod, module) {
  const found = [...mod.matchAll(new RegExp(`^\\s*${module.replace(/\./g, '\\.')} (\\S+)(?: // indirect)?\\s*$`, 'gm'))].map((m) => m[1]);
  return found;
}

Then(P + 'go.mod requires kogen-executor {string} and go.sum holds these lines:', function (version, table) {
  const found = requirement(checkoutFile('go.mod'), EXECUTOR);
  assert.deepEqual(found, [version], `go.mod requires ${EXECUTOR} ${found.join(', ') || 'in no version'}, want exactly ${version}`);
  const sum = checkoutFile('go.sum').split('\n');
  for (const row of table.hashes()) {
    assert.ok(sum.includes(row.line), `go.sum has no line ${row.line}`);
  }
});

Then(P + 'go.mod keeps these other requirements:', function (table) {
  const mod = checkoutFile('go.mod');
  for (const row of table.hashes()) {
    const found = requirement(mod, row.module);
    assert.deepEqual(found, [row.version], `go.mod requires ${row.module} ${found.join(', ') || 'in no version'}, want ${row.version}`);
  }
});

Then(P + 'go resolves kogen-executor to {string} and that module has the file {string}', LONG, function (version, file) {
  let r = goOut(['list', '-m', '-json', EXECUTOR]);
  assert.equal(r.status, 0, `${r.command} exited ${r.status}\n${observer.clip(r.text)}`);
  let info = JSON.parse(r.stdout);
  assert.equal(info.Version, version, `${r.command}: version ${info.Version}, want ${version}`);
  if (!info.Dir) {
    const d = goOut(['mod', 'download', EXECUTOR]);
    assert.equal(d.status, 0, `${d.command} exited ${d.status}\n${observer.clip(d.text)}`);
    r = goOut(['list', '-m', '-json', EXECUTOR]);
    info = JSON.parse(r.stdout);
  }
  assert.ok(info.Dir, `${r.command}: the module has no directory`);
  assert.ok(fs.existsSync(path.join(info.Dir, ...file.split('/'))), `${EXECUTOR} ${info.Version} (${info.Dir}) has no file ${file}`);
});

Then(P + 'the tests of {string} import {string}', function (pkg, imp) {
  const r = goOut(['list', '-f', '{{join .TestImports "\\n"}}\n{{join .XTestImports "\\n"}}', pkg]);
  assert.equal(r.status, 0, `${r.command} exited ${r.status}\n${observer.clip(r.text)}`);
  assert.ok(r.stdout.split('\n').includes(imp), `${r.command}: the tests of ${pkg} do not import ${imp}`);
});

Then(P + 'no non-test package of {string} and {string} imports {string}', function (a, b, imp) {
  const r = goOut(['list', '-deps', '-f', '{{.ImportPath}}', a, b]);
  assert.equal(r.status, 0, `${r.command} exited ${r.status}\n${observer.clip(r.text)}`);
  assert.ok(!r.stdout.split('\n').includes(imp), `${r.command}: ${imp} is a dependency of the program, not of its tests only`);
});

function goFiles(root, rel, out) {
  for (const e of fs.readdirSync(path.join(root, ...rel.split('/')), { withFileTypes: true })) {
    const p = rel + '/' + e.name;
    if (e.isDirectory()) goFiles(root, p, out);
    else if (e.isFile()) out.push(p);
  }
  return out;
}

Then(P + 'no Go file under these folders writes its own Claude runtime script:', function (table) {
  const root = observer.realRoot();
  const hits = [];
  for (const row of table.hashes()) {
    for (const f of goFiles(root, row.folder, [])) {
      if (!f.endsWith('.go')) continue;
      fs.readFileSync(path.join(root, ...f.split('/')), 'utf8').split('\n').forEach((line, i) => {
        if (/loggedIn|claudecode\.RuntimePath|claudecode\.SeamEnv/.test(line)) hits.push(`${f}:${i + 1}: ${line.trim().slice(0, 120)}`);
      });
    }
  }
  assert.equal(hits.length, 0, `these lines write or serve a Claude runtime instead of using the fixture:\n${hits.join('\n')}`);
});

Then(P + 'the files contain these texts:', function (table) {
  const missing = [];
  for (const row of table.hashes()) {
    if (!checkoutFile(row.file).includes(row.text)) missing.push(`${row.file} does not contain ${JSON.stringify(row.text)}`);
  }
  assert.equal(missing.length, 0, missing.join('\n'));
});

// The digest of folders: SHA-256 over `<path>\0<sha256 of the file's bytes>\n` for every regular file
// under them (recursively, symbolic links not followed), by slash-separated path relative to the
// checkout in code-unit order, leaving out the files whose path matches the pattern.
Then(P + 'the files under these folders other than those matching {string} have the digest {string}:', function (pattern, want, table) {
  const dirs = table.hashes().map((r) => r.folder);
  assert.ok(dirs.length > 0 && dirs.every(Boolean), 'the table names no folder in its folder column');
  const root = observer.realRoot();
  const skip = new RegExp(pattern);
  const files = [];
  dirs.forEach((d) => goFiles(root, d, files));
  files.sort();
  const kept = files.filter((f) => !skip.test(f));
  const lines = kept.map((f) => `${f}\0${sha256(fs.readFileSync(path.join(root, ...f.split('/'))))}\n`).join('');
  const got = sha256(lines);
  assert.equal(got, want, `the ${kept.length} files under ${dirs.join(', ')} other than those matching ${pattern} have the digest ${got}, want ${want}: a file was added, removed or changed. Files and their SHA-256:\n${lines.replace(/\0/g, ' ')}`);
});

When(P + 'I run the focused context and external tests of the build package', LONG, async function () {
  this.bcxcRun = await observer.runGo(observer.realRoot(), '', FOCUSED, '');
});

Then(P + 'that run passed these tests and no test was skipped:', function (table) {
  const names = table.hashes().map((r) => r.test);
  assert.ok(names.length > 0 && names.every(Boolean), 'the table names no test in its test column');
  const p = observer.passProblem(this.bcxcRun, names);
  assert.ok(p === '', `${p}\n${observer.details(this.bcxcRun, names)}`);
});

Then(P + 'the focused context and external tests fail against each of these kogen-executor copies:', LONG, async function (table) {
  const rows = table.hashes();
  assert.ok(rows.length > 0 && rows.every((r) => r.copy), 'the table names no kogen-executor copy in its copy column');
  const runs = await Promise.all(rows.map((row) => observer.runGo(observer.realRoot(), row.copy, FOCUSED, FOCUSED_QUICK_SKIP)));
  const wrong = [];
  rows.forEach((row, i) => {
    const r = runs[i];
    const failed = Object.keys(r.results).filter((t) => r.results[t] === 'fail');
    const named = row.failing === 'any test' ? failed.length > 0 : r.results[row.failing] === 'fail';
    if (r.status === 0 || !named) {
      wrong.push(`no test of the focused run failed against the ${row.copy} kogen-executor (${row.defect}; want ${row.failing} to fail): ${r.command}, exit ${r.status}, failed: ${failed.join(', ') || 'none'}\n${observer.clip(r.raw + r.stderr)}`);
    }
  });
  assert.ok(wrong.length === 0, wrong.join('\n'));
});
