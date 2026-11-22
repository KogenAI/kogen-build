'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert');

const DRIVER = path.join('features', 'steps', 'build-cut-fields-follow-the-marker__driver', 'acceptance_test.go.txt');
const COPY = path.join('build', 'zz_build_cut_fields_follow_the_marker_acceptance_test.go');

let ran;
let comment;

Given('build-cut-fields-follow-the-marker: the kogen-build checkout', function () {
  assert.ok(fs.existsSync(path.join(process.cwd(), 'build', 'records.go')), 'build/records.go is missing');
  assert.ok(fs.existsSync(path.join(process.cwd(), DRIVER)), DRIVER + ' is missing');
});

When('build-cut-fields-follow-the-marker: the build test {string} runs', { timeout: 115000 }, function (name) {
  const copy = path.join(process.cwd(), COPY);
  fs.copyFileSync(path.join(process.cwd(), DRIVER), copy);
  try {
    const r = spawnSync('go', ['test', '-count=1', '-timeout', '100s', '-run', '^' + name + '$', '-v', './build'], {
      cwd: process.cwd(),
      encoding: 'utf8',
      env: { ...process.env, GOFLAGS: '-trimpath' },
      timeout: 112000,
    });
    ran = { name, status: r.status, out: (r.stdout || '') + (r.stderr || '') };
  } finally {
    fs.rmSync(copy, { force: true });
  }
});

When('build-cut-fields-follow-the-marker: the comment above cutFields is read', function () {
  const lines = fs.readFileSync(path.join(process.cwd(), 'build', 'records.go'), 'utf8').split('\n');
  const at = lines.findIndex((l) => l.startsWith('func cutFields('));
  assert.ok(at > 0, 'func cutFields is missing from build/records.go');
  const got = [];
  for (let i = at - 1; i >= 0 && lines[i].startsWith('//'); i--) got.unshift(lines[i]);
  comment = got.join('\n');
});

function passed(what) {
  assert.ok(ran, 'the test did not run');
  const msg = what + ': go test -run ^' + ran.name + '$ ./build exit ' + ran.status + ':\n' + ran.out.slice(0, 4000);
  assert.ok(ran.out.includes('--- PASS: ' + ran.name + ' '), 'test ' + ran.name + ' did not report PASS; ' + msg);
  assert.strictEqual(ran.status, 0, msg);
}

Then('build-cut-fields-follow-the-marker: the 9000 byte result with the marker at byte 15 and 31000 omitted bytes is recorded with bytes_before_cut 9000 minus the marker length plus 31000 and bytes_delivered 9000', function () { passed('marker at byte 15'); });
Then('build-cut-fields-follow-the-marker: the result with the marker at byte 4050 and the cut of 40960 bytes are recorded as before', function () { passed('marker in the former window'); });
Then('build-cut-fields-follow-the-marker: the unmarked result, the 16385 byte result and the result with a marker number that is not whole carry neither field', function () { passed('no cut fields'); });
Then('build-cut-fields-follow-the-marker: the comment is not empty and does not mention 4096', function () {
  assert.ok(comment && comment.length > 0, 'no comment above func cutFields in build/records.go');
  assert.ok(!comment.includes('4096'), 'the comment above cutFields still mentions 4096:\n' + comment);
});
