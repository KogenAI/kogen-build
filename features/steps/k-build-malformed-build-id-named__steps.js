'use strict';
// Acceptance of k-build-malformed-build-id-named.
// Resume runs through the standalone entry point: the step builds ./cmd/kogen-build once per run and
// runs `kogen-build resume --project <checkout> --id <id>`, whose stdout is one JSON object.
// Recover has no standalone entry point in this module: the step writes the Go harness HARNESS_SOURCE
// below to a scratch file outside the checkout, overlays it into ./cmd/kogen-build as
// cmd/kogen-build/k_build_malformed_build_id_named_acceptance_test.go, builds that test binary once per
// run (go test -c -vet=off -overlay) and runs TestKBuildMalformedBuildIDNamedRecover, which calls
// build.Recover once and prints one observation line, "KBMI <json>".
const {Given, When, Then} = require('@cucumber/cucumber');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const P = 'k-build-malformed-build-id-named: ';
const TARGET = 'cmd/kogen-build/k_build_malformed_build_id_named_acceptance_test.go';
const TEST = 'TestKBuildMalformedBuildIDNamedRecover';
const PREFIX = 'KBMI ';
const LONG = {timeout: 600000};

const HARNESS_SOURCE = `package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-build/build"
	"github.com/KogenAI/kogen-loop/sys"
)

// TestKBuildMalformedBuildIDNamedRecover runs build.Recover once with KBMI_PROJECT, KBMI_ID and
// KBMI_OUTPUT and prints one observation line, "KBMI <json>": the exit code, the structured refusal
// (category, kind, message) or null, and what the recovery printed.
func TestKBuildMalformedBuildIDNamedRecover(t *testing.T) {
	project, id, output := os.Getenv("KBMI_PROJECT"), os.Getenv("KBMI_ID"), os.Getenv("KBMI_OUTPUT")
	if project == "" || output == "" {
		t.Fatal("KBMI_PROJECT and KBMI_OUTPUT are required")
	}
	s := sys.New()
	var out strings.Builder
	s.Stdout, s.Stderr = &out, &out
	exit, err := build.Recover(context.Background(), s, build.RecoverInput{Project: project, ID: id, Output: output})
	obs := map[string]any{"exit": exit, "error": nil, "output": out.String()}
	var e *sys.Error
	switch {
	case errors.As(err, &e):
		obs["exit"] = e.Exit
		obs["error"] = map[string]any{"category": e.Class, "kind": e.Reason, "message": e.Detail}
	case err != nil:
		obs["exit"] = -1
		obs["error"] = map[string]any{"category": "go", "kind": "error", "message": err.Error()}
	}
	b, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("KBMI " + string(b))
}
`;

const owned = [];
process.on('exit', () => { for (const d of owned) fs.rmSync(d, {recursive: true, force: true}); });

function scratch(prefix) {
  const dir = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), prefix)));
  owned.push(dir);
  return dir;
}

// goEnv is this process's environment without the acceptance runner's own variables.
function goEnv(extra) {
  const out = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith('KOGEN_ACCEPTANCE_')) out[k] = v;
  return {...out, ...extra};
}

function run(cmd, args, options) {
  const r = spawnSync(cmd, args, {encoding: 'utf8', maxBuffer: 64 * 1024 * 1024, timeout: 540000, ...options});
  const shown = `${cmd} ${args.join(' ')} exited ${r.status}${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error.message : ''}:\n` +
    `${(r.stdout || '').slice(-20000)}${(r.stderr || '').slice(-20000)}`;
  return {...r, shown};
}

function checked(cmd, args, options) {
  const r = run(cmd, args, options);
  assert.equal(r.error, undefined, r.shown);
  assert.equal(r.status, 0, r.shown);
  return r.stdout.trim();
}

// once makes a value once per run; a failure is remembered so later scenarios fail fast with it.
const memo = {};
function once(key, make) {
  if (!memo[key]) {
    try { memo[key] = {value: make()}; } catch (e) { memo[key] = {error: e}; }
  }
  if (memo[key].error) throw memo[key].error;
  return memo[key].value;
}

function repoRoot() { return fs.realpathSync(process.cwd()); }

function cli() {
  return once('cli', () => {
    const bin = path.join(scratch('kbmi-cli-'), 'kogen-build');
    checked('go', ['build', '-o', bin, './cmd/kogen-build'], {cwd: repoRoot(), env: goEnv()});
    return bin;
  });
}

function harness() {
  return once('harness', () => {
    const root = repoRoot();
    const dir = scratch('kbmi-harness-');
    const source = path.join(dir, 'harness_test.go');
    fs.writeFileSync(source, HARNESS_SOURCE);
    const overlay = path.join(dir, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({Replace: {[path.join(root, TARGET)]: source}}));
    const bin = path.join(dir, 'kogen-build.test');
    checked('go', ['test', '-c', '-vet=off', '-o', bin, '-overlay', overlay, './cmd/kogen-build'], {cwd: root, env: goEnv({PWD: root})});
    return bin;
  });
}

// checkout is a fresh Git repository with one commit, in a physical (symlink-free) temporary path.
function checkout(world) {
  const tmp = scratch('kbmi-checkout-');
  const repo = path.join(tmp, 'checkout');
  fs.mkdirSync(repo);
  fs.mkdirSync(path.join(tmp, 'home'));
  const env = goEnv({HOME: path.join(tmp, 'home'), GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1', GIT_TERMINAL_PROMPT: '0'});
  const opts = {cwd: repo, env};
  checked('git', ['init', '-q', '--initial-branch=main'], opts);
  checked('git', ['config', 'user.name', 'Malformed Id Fixture'], opts);
  checked('git', ['config', 'user.email', 'fixture@example.invalid'], opts);
  fs.writeFileSync(path.join(repo, 'app.txt'), 'before\n');
  checked('git', ['add', 'app.txt'], opts);
  checked('git', ['-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture'], opts);
  world.kbmi = {tmp, repo, env, output: path.join(tmp, 'archive')};
  return world.kbmi;
}

function recorded(world, id) {
  const k = checkout(world);
  const dir = path.join(k.repo, '.kogen/local/runs', id);
  fs.mkdirSync(dir, {recursive: true});
  return dir;
}

function digest(data) { return crypto.createHash('sha256').update(data).digest('hex'); }

// state is every file of the checkout outside .git (mode and SHA-256, or a symbolic link's target),
// the run directories, HEAD and the bytes of the Git index.
function state(k) {
  const files = {};
  const walk = rel => {
    for (const e of fs.readdirSync(path.join(k.repo, rel), {withFileTypes: true})) {
      const p = rel ? rel + '/' + e.name : e.name;
      if (p === '.git') continue;
      const full = path.join(k.repo, p);
      if (e.isDirectory()) walk(p);
      else if (e.isSymbolicLink()) files[p] = 'symlink ' + fs.readlinkSync(full);
      else files[p] = `${(fs.lstatSync(full).mode & 0o777).toString(8)} ${digest(fs.readFileSync(full))}`;
    }
  };
  walk('');
  const runs = path.join(k.repo, '.kogen/local/runs');
  return {
    files,
    runs: fs.existsSync(runs) ? fs.readdirSync(runs).sort() : [],
    head: checked('git', ['rev-parse', 'HEAD'], {cwd: k.repo, env: k.env}),
    index: digest(fs.readFileSync(path.join(k.repo, '.git/index'))),
  };
}

function exists(p) {
  try { fs.lstatSync(p); return true; } catch (e) { if (e.code === 'ENOENT') return false; throw e; }
}

Given(P + 'a committed checkout', LONG, function () {
  checkout(this);
});

Given(P + 'a committed checkout with a recorded Build {string} whose checkpoint.json holds {string}', LONG, function (id, text) {
  fs.writeFileSync(path.join(recorded(this, id), 'checkpoint.json'), text);
});

Given(P + 'a committed checkout with a recorded Build {string} that has no checkpoint.json', LONG, function (id) {
  fs.writeFileSync(path.join(recorded(this, id), 'log.jsonl'), '');
});

When(P + 'I run the standalone resume with id {string}', LONG, function (id) {
  const k = this.kbmi;
  const bin = cli();
  k.before = state(k);
  const args = ['resume', '--project', k.repo, '--id', id];
  const r = run(bin, args, {cwd: k.tmp, env: k.env});
  assert.equal(r.error, undefined, r.shown);
  let doc;
  try { doc = JSON.parse(r.stdout); } catch (e) { assert.fail(`kogen-build stdout is not one JSON object: ${r.shown}`); }
  k.result = {exit: r.status, reported: doc.exit_code, error: doc.error, shown: r.shown};
});

When(P + 'I run recover with id {string} into a new output directory', LONG, function (id) {
  const k = this.kbmi;
  const bin = harness();
  assert.equal(exists(k.output), false, `the output directory ${k.output} already exists`);
  k.before = state(k);
  const args = ['-test.run', '^' + TEST + '$', '-test.count=1', '-test.v'];
  const r = run(bin, args, {cwd: k.tmp, env: {...k.env, KBMI_PROJECT: k.repo, KBMI_ID: id, KBMI_OUTPUT: k.output}});
  const shown = `${TEST} (harness overlaid as ${TARGET}, built with go test -c -vet=off -overlay ./cmd/kogen-build): ${r.shown}`;
  assert.equal(r.error, undefined, shown);
  assert.equal(r.status, 0, shown);
  const lines = r.stdout.split('\n');
  assert.equal(lines.filter(l => l.startsWith('--- PASS: ' + TEST + ' ')).length, 1, `${TEST} did not pass exactly once: ${shown}`);
  const observed = lines.filter(l => l.startsWith(PREFIX));
  assert.equal(observed.length, 1, `${TEST} printed ${observed.length} observation lines, want 1: ${shown}`);
  const obs = JSON.parse(observed[0].slice(PREFIX.length));
  k.result = {exit: obs.exit, reported: obs.exit, error: obs.error, shown};
});

Then(P + 'no Build run was created and no file in the checkout changed', function () {
  const k = this.kbmi;
  assert.deepEqual(state(k), k.before, `the checkout changed:\n${k.result.shown}`);
});

Then(P + 'the new output directory was not created', function () {
  const k = this.kbmi;
  assert.equal(exists(k.output), false, `the output directory ${k.output} was created:\n${k.result.shown}`);
});

Then(P + 'the command exits {int} with category {string}, kind {string} and message:', function (exit, category, kind, message) {
  const r = this.kbmi.result;
  assert.deepEqual({exit: r.exit, exit_code: r.reported, error: r.error}, {exit, exit_code: exit, error: {category, kind, message}}, r.shown);
});

Then(P + 'the existing Go tests {string} and {string} pass in package {string}', LONG, function (a, b, pkg) {
  const args = ['test', '-count=1', '-v', '-run', `^(${a}|${b})$`, pkg];
  const command = 'go ' + args.join(' ');
  const r = run('go', args, {cwd: repoRoot(), env: goEnv()});
  assert.equal(r.error, undefined, `${command} for ${a} and ${b}: ${r.shown}`);
  const lines = (r.stdout || '').split('\n');
  for (const name of [a, b]) {
    const passed = lines.filter(l => l.startsWith(`--- PASS: ${name} (`)).length;
    const failed = lines.some(l => l.startsWith(`--- FAIL: ${name} (`));
    const skipped = lines.some(l => l.startsWith(`--- SKIP: ${name} (`));
    const ran = lines.some(l => l.trim() === `=== RUN   ${name}`);
    const outcome = failed ? 'failed' : skipped ? 'was skipped' : !ran ? 'was absent: it never ran' : passed === 1 ? 'passed' : `passed ${passed} times`;
    assert.equal(passed, 1, `test ${name} did not pass exactly once (it ${outcome}); ${command} in ${repoRoot()}: ${r.shown}`);
  }
  assert.equal(r.status, 0, `${command} for ${a} and ${b}: ${r.shown}`);
});
