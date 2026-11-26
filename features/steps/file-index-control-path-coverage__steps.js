'use strict';

const assert = require('node:assert/strict');
const {spawnSync} = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const slug = 'file-index-control-path-coverage';
const indexPath = 'build/file_index.go';
const testPath = 'build/file_index_test.go';
const intentDir = `.kogen/intents/${slug}`;
const existing = ['TestFileIndexOmissionAccountingAndInputOrder', 'TestFileIndexVeryWideRepositoryStaysBounded'];

function replace(source, before, after) {
  const at = source.indexOf(before);
  assert.ok(at >= 0 && source.indexOf(before, at + before.length) < 0, `expected a single renderer anchor: ${before}`);
  return source.slice(0, at) + after + source.slice(at + before.length);
}

function load(tests = fs.readFileSync(testPath, 'utf8')) {
  const source = fs.readFileSync(indexPath, 'utf8');
  const end = source.indexOf('\n// fitInventory substitutes');
  assert.ok(end > 0, `cannot isolate index rendering in ${indexPath}`);
  return {source: source.slice(0, end) + '\n', tests};
}

function run({source, tests}, label) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'control-path-'));
  try {
    const goSource = path.join(dir, 'file_index.go');
    const goTest = path.join(dir, 'file_index_test.go');
    const binary = path.join(dir, 'index.test');
    fs.writeFileSync(goSource, source);
    fs.writeFileSync(goTest, tests);
    const compile = ['test', '-c', '-trimpath', '-o', binary, goSource, goTest];
    const built = spawnSync('go', compile, {encoding: 'utf8', timeout: 10000});
    assert.equal(built.status, 0, `${label}: go ${compile.join(' ')} failed (${built.error || built.signal || built.status})\n${built.stdout}\n${built.stderr}`);
    const args = ['-test.v', '-test.count=1', '-test.timeout=3s'];
    const result = spawnSync(binary, args, {encoding: 'utf8', timeout: 4000});
    const output = result.stdout + result.stderr;
    const detail = `${label}: ${binary} ${args.join(' ')} exited ${result.error || result.signal || result.status}\n${output}`;
    assert.ok(!result.error && !result.signal && !/^panic:/m.test(output), detail);
    const runs = [...output.matchAll(/^=== RUN   (\S+)$/gm)].map(m => m[1]).filter(name => !name.includes('/'));
    const terminal = [...output.matchAll(/^--- (PASS|FAIL|SKIP): (\S+) \(/gm)];
    return {status: result.status, runs, terminal, detail};
  } finally {
    fs.rmSync(dir, {recursive: true, force: true});
  }
}

function observe(result, name, outcome) {
  assert.equal(result.runs.filter(n => n === name).length, 1, `${name} did not run exactly once: ${result.detail}`);
  assert.deepEqual(result.terminal.filter(m => m[2] === name).map(m => m[1]), [outcome], `${name} did not finish with ${outcome}: ${result.detail}`);
}

function unchanged(files) {
  // Fail for missing coverage before running any tests; passing existing tests is not the new outcome.
  const added = [...files.tests.matchAll(/^func (Test[\p{L}\p{N}_]+)\(/gmu)]
    .map(m => m[1]).filter(n => !existing.includes(n));
  assert.equal(added.length, 1, `${testPath}: expected one added Go test function for control-character path rendering, found ${added.length}`);
  const name = added[0];
  const result = run(files, 'unchanged renderer');
  assert.equal(result.status, 0, `${name}: ${result.detail}`);
  for (const n of [...existing, name]) observe(result, n, 'PASS');
  return name;
}

function faults(source, {cases, ordinary}) {
  return [
    ...cases.map(({name, raw, escaped}) => {
      const anchor = raw.charCodeAt(0) === 0x1f ? 'fmt.Fprintf(&b, `\\x%02x`, c)' : `b.WriteString(\`${escaped}\`)`;
      return [name + ' left raw', replace(source, anchor, 'b.WriteByte(c)')];
    }),
    [`non-ASCII ordinary name rewritten (e.g. ${ordinary})`, replace(source, '\t\treturn p\n', '\t\treturn strings.Map(func(r rune) rune { if r > 127 { return \'?\' }; return r }, p)\n')],
    ['fileIndex bypasses escaping', replace(source, 'full.WriteString(displayPath(p) + "\\n")', 'full.WriteString(p + "\\n")')],
  ];
}

function rejectFaults(files, name, spec) {
  for (const [label, source] of faults(files.source, spec)) {
    const result = run({...files, source}, label);
    for (const n of existing) observe(result, n, 'PASS');
    if (result.status === 0) {
      observe(result, name, 'PASS');
      assert.fail(`${name} did not detect ${label}: ${result.detail}`);
    }
    assert.equal(result.status, 1, `${name}: ${result.detail}`);
    observe(result, name, 'FAIL');
  }
}

if (require.main === module) {
  // Exercise the same baseline and mutation observer with full real Go runs.
  const {correct, wrong} = JSON.parse(fs.readFileSync(`${intentDir}/controls.json`, 'utf8'));
  const feature = fs.readFileSync(`${intentDir}/acceptance.feature`, 'utf8');
  const spec = JSON.parse(feature.split('"""')[1].replace(/^json\n/, ''));
  const mode = process.argv[2];
  assert.ok(mode === 'good' || mode === 'bad');
  const files = load((mode === 'good' ? correct : wrong.at(-1)).files[testPath]);
  const name = unchanged(files);
  try {
    rejectFaults(files, name, spec);
    assert.equal(mode, 'good', 'observer accepted the helper-only test');
    console.log('accepted the fileIndex regression test');
  } catch (error) {
    if (mode !== 'bad' || !(error instanceof assert.AssertionError) || !error.message.includes('did not detect fileIndex bypasses escaping:')) throw error;
    console.log('rejected the helper-only test at the index bypass fault');
    process.exitCode = 1;
  }
} else {
  const {Given, Then} = require('@cucumber/cucumber');
  Given(`${slug}: the documented escape cases and ordinary path`, function(doc) {
    this.pathSpec = JSON.parse(doc);
    this.pathFiles = load();
  });
  Then(`${slug}: exactly one added test runs and passes with the unchanged renderer`, {timeout: 16000}, function() {
    this.pathTest = unchanged(this.pathFiles);
  });
  Then(`${slug}: the added test rejects each raw byte, a rewritten ordinary path, and a bypass in fileIndex`, {timeout: 110000}, function() {
    rejectFaults(this.pathFiles, this.pathTest, this.pathSpec);
  });
}
