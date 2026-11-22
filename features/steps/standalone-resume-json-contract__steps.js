'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const FILE = 'build/standalone_test.go';

// Top-level tests of the file with their bodies.
function testsOf(src) {
  return src
    .split(/^(?=func )/m)
    .map((chunk) => {
      const m = /^func (Test\w+)\(/.exec(chunk);
      return m ? { name: m[1], body: chunk } : null;
    })
    .filter(Boolean);
}

// The resume test the Build added to build/standalone_test.go. The verification checkout's HEAD may
// already contain the Build's own tests, so the test is identified by what it does, not by a diff:
// a top-level test whose name contains "Resume" or whose body runs the executable's "resume" command.
function addedTests() {
  const all = testsOf(fs.readFileSync(path.join(process.cwd(), FILE), 'utf8'));
  const added = all.filter((t) => /resume/i.test(t.name) || t.body.includes('"resume"')).map((t) => t.name);
  return { all: all.map((t) => t.name), added };
}

function goTest(cwd, names) {
  const r = spawnSync('go', ['test', '-count=1', '-json', '-run', '^(' + names.join('|') + ')$', './build'], { cwd, encoding: 'utf8', timeout: 100000, maxBuffer: 1 << 26 });
  const events = (r.stdout || '').split('\n').filter((l) => l.startsWith('{')).map((l) => JSON.parse(l));
  const top = (a) => events.filter((e) => e.Test && !e.Test.includes('/') && e.Action === a).map((e) => e.Test);
  return { status: r.status, passed: top('pass'), failed: top('fail'), tail: ((r.stdout || '') + (r.stderr || '')).slice(-1500) };
}

function mutant(tests, find, replace) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'resume-mutant-'));
  fs.cpSync(process.cwd(), dir, { recursive: true, filter: (s) => !s.includes('/.git/') && !s.endsWith('/.git') && !s.includes('node_modules') });
  const f = path.join(dir, 'cmd/kogen-build/main.go');
  const src = fs.readFileSync(f, 'utf8');
  assert.ok(src.includes(find), 'standalone-resume-json-contract: mutation point not found in cmd/kogen-build/main.go: ' + find);
  fs.writeFileSync(f, src.replace(find, replace));
  return goTest(dir, tests.all);
}

function rejects(label, tests, m) {
  assert.ok(m.failed.some((t) => tests.added.includes(t)), `standalone-resume-json-contract: none of the resume tests of ${FILE} (${JSON.stringify(tests.added)}) failed when ${label}; failed tests: ${JSON.stringify(m.failed)}; output: ${m.tail}`);
}

Given('standalone-resume-json-contract: the repository checkout', function () {
  assert.ok(fs.existsSync(path.join(process.cwd(), FILE)));
});

When('standalone-resume-json-contract: the standalone tests of the build package run', { timeout: 115000 }, function () {
  this.tests = addedTests();
  this.base = this.tests.all.length ? goTest(process.cwd(), this.tests.all) : { status: 1, passed: [], failed: [], tail: 'no tests in ' + FILE };
});

Then('standalone-resume-json-contract: a test added to the standalone test file passes', function () {
  assert.ok(this.tests.added.length > 0, 'no resume test found in ' + FILE + ' (a top-level test whose name contains "Resume" or whose body runs "resume"); tests: ' + JSON.stringify(this.tests.all));
  assert.strictEqual(this.base.status, 0, 'go test of ' + FILE + ' failed: ' + this.base.tail);
  assert.ok(this.tests.added.every((t) => this.base.passed.includes(t)), 'resume tests did not all pass: ' + JSON.stringify(this.tests.added) + ' passed ' + JSON.stringify(this.base.passed));
});

Then('standalone-resume-json-contract: the added test fails when stdout holds a second JSON value', { timeout: 115000 }, function () {
  rejects('stdout held a second JSON value', this.tests, mutant(this.tests, 'defer func() { _ = json.NewEncoder(os.Stdout).Encode(r) }()', 'defer func() { _ = json.NewEncoder(os.Stdout).Encode(r); _ = json.NewEncoder(os.Stdout).Encode(r) }()'));
});

Then('standalone-resume-json-contract: the added test fails when the refusal creates a run', { timeout: 115000 }, function () {
  rejects('the refusal created a run', this.tests, mutant(this.tests, 'out := build.Outcome{}', 'out := build.Outcome{}\n\t_ = os.MkdirAll(*project+"/.kogen/local/runs/deadbeef", 0o755)'));
});

Then('standalone-resume-json-contract: the added test fails when the structured error is missing', { timeout: 115000 }, function () {
  rejects('the structured error was missing', this.tests, mutant(this.tests, 'r.Error = &problem{out.Error.Class, out.Error.Reason, out.Error.Detail}', 'r.Error = nil'));
});
