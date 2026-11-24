'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then, AfterAll} = require('@cucumber/cucumber');
const {validate} = require('./build-record-model-sources__observer');
const helper = 'features/steps/build-record-model-sources__driver/facts_test.go.txt';
let dir, binary;
function compile() {
  if (binary) return;
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'model-source-facts-'));
  const overlay = path.join(dir, 'overlay.json');
  const target = path.join(dir, 'build.test');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {
    [path.resolve('build/zz_record_model_sources_acceptance_test.go')]: path.resolve(helper)
  }}));
  const args = ['test', '-c', '-overlay', overlay, '-o', target, './build'];
  const p = spawnSync('go', args, {encoding:'utf8', timeout:100000});
  assert.equal(p.status, 0, `TestRecordModelSourcesFixture: go ${args.join(' ')}: ${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`);
  binary = target;
}
Given('build-record-model-sources: the facts publication fixture is available', function() {
  assert.ok(fs.statSync(helper).isFile());
});
When('build-record-model-sources: I publish facts with this input:', {timeout:115000}, function(text) {
  compile();
  const output = path.join(dir, 'output.json');
  fs.rmSync(output, {force:true});
  const args = ['-test.run=^TestRecordModelSourcesFixture$', '-test.count=1', '-test.timeout=10s'];
  const p = spawnSync(binary, args, {encoding:'utf8', timeout:12000, env:{...process.env, MODEL_SOURCES_INPUT:text, MODEL_SOURCES_OUTPUT:output}});
  this.sourceResult = {p, output, command: `${binary} ${args.join(' ')}`};
});
Then('build-record-model-sources: the published and readable facts satisfy:', function(text) {
  const {p, output, command} = this.sourceResult;
  const diag = `TestRecordModelSourcesFixture: ${command}: exit ${p.status}; ${p.error || ''}\n${p.stdout}\n${p.stderr}`;
  assert.equal(p.status, 0, diag);
  assert.ok(fs.existsSync(output), `${diag}\nTestRecordModelSourcesFixture absent, skipped or produced no facts`);
  validate(JSON.parse(fs.readFileSync(output, 'utf8')), JSON.parse(text));
});
AfterAll(function() { if (dir) fs.rmSync(dir, {recursive:true, force:true}); });
