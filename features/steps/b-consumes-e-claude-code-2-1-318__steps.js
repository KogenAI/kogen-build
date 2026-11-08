const { Then } = require('@cucumber/cucumber');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const {run, observe} = require('./b-consumes-e-claude-code-2-1-318__observer.js');
const prefix = 'b-consumes-e-claude-code-2-1-318: ';
Then(prefix + 'the pinned producer exports version {string}', function(expected) {
  const mod = spawnSync('go', ['list', '-m', '-json', 'github.com/KogenAI/kogen-executor'], {encoding:'utf8'});
  assert.equal(mod.status, 0, mod.stdout + mod.stderr);
  const info = JSON.parse(mod.stdout);
  assert.ok(!info.Replace, 'producer must be pinned, not replaced');
  const runtime = fs.readFileSync(path.join(info.Dir, 'executor/claudecode/runtime.go'), 'utf8');
  const declaration = runtime.match(/const\s+Version\s*=\s*"([^"]+)"/);
  assert.ok(declaration, 'pinned producer must export its runtime Version');
  assert.equal(declaration[1], expected, `go.mod pins ${info.Version}; adopt producer main whose runtime Version is ${expected}, not the old producer contract`);
});
Then(prefix + 'the fake runtime advertises only {string}', function(expected) {
  const text = fs.readFileSync('build/context_external_test.go', 'utf8');
  const cli = text.match(/--version\) echo "([^ ]+) \(Claude Code\)"/);
  const init = text.match(/"claude_code_version":"([^"]+)"/);
  assert.ok(cli, 'fake runtime --version field exists');
  assert.ok(init, 'fake runtime init version field exists');
  assert.equal(cli[1], expected, 'replace the old fake runtime --version literal');
  assert.equal(init[1], expected, 'replace the old fake runtime init version literal');
});
Then(prefix + 'the first prompt test passes for:', {timeout:180000}, function(table) {
  observe(run(), table.raw().map(row => row[0]));
});
