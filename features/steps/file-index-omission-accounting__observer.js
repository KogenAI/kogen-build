'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const controlsPath = '.kogen/intents/file-index-omission-accounting/controls.json';
const mutations = {
  accounting: [
    ['overlapping root total', s => replace(s, 'strconv.Itoa(n.direct)', 'strconv.Itoa(n.total)')],
    ['dropped root path', s => replace(s, 'strconv.Itoa(n.direct)', 'strconv.Itoa(n.direct-1)')],
  ],
  order: [
    ['input-order-dependent listed paths', s => s.replaceAll('return best', 'return omissionOrderFault(best, listing)') + `
func omissionOrderFault(out, listing string) string {
    paths := strings.Split(strings.TrimSuffix(listing, "\\x00"), "\\x00")
    if len(paths) < 2 || paths[0] < paths[len(paths)-1] { return out }
    before, rest, ok := strings.Cut(out, "## Listed paths\\n")
    if !ok { return out }
    lines := strings.Split(rest, "\\n")
    if len(lines) > 1 { lines[0], lines[1] = lines[1], lines[0] }
    return before + "## Listed paths\\n" + strings.Join(lines, "\\n")
}
`],
  ],
  guards: [
    ['required-path loss', s => replace(s, 'for _, p := range required {', 'for _, p := range []string{} {')],
    ['oversized result', s => s.replaceAll('return best', 'return best + strings.Repeat("\\n", fileIndexBytes)')],
  ],
};
function replace(source, oldText, newText) {
  assert.ok(source.includes(oldText), `mutation anchor missing: ${oldText}`);
  return source.replace(oldText, newText);
}
function runTests(source, tests, label) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'omission-accounting-'));
  try {
    fs.writeFileSync(path.join(dir, 'index.go'), source);
    fs.writeFileSync(path.join(dir, 'index_test.go'), tests);
    const binary = path.join(dir, 'index.test');
    const args = ['test', '-c', '-o', binary, path.join(dir, 'index.go'), path.join(dir, 'index_test.go')];
    const compiled = spawnSync('go', args, {encoding: 'utf8', timeout: 20000});
    assert.equal(compiled.status, 0, `${label}: go ${args.join(' ')} failed: ${compiled.error || ''}\n${compiled.stdout}\n${compiled.stderr}`);
    const ran = spawnSync(binary, ['-test.v', '-test.count=1', '-test.timeout=15s'], {encoding: 'utf8', timeout: 18000});
    const output = (ran.stdout || '') + (ran.stderr || '');
    assert.ok(!ran.error && !ran.signal, `${label}: ${binary} failed to finish: ${ran.error || ran.signal}\n${output}`);
    assert.match(output, /^=== RUN   TestFileIndexVeryWideRepositoryStaysBounded$/m, `${label}: existing TestFileIndexVeryWideRepositoryStaysBounded absent from ${binary}\n${output}`);
    return {status: ran.status, output, command: `${binary} -test.v -test.count=1 -test.timeout=15s`};
  } finally {
    fs.rmSync(dir, {recursive: true, force: true});
  }
}
function verify(group, tests = fs.readFileSync('build/file_index_test.go', 'utf8')) {
  assert.ok(Object.hasOwn(mutations, group), `unknown mutation group ${group}`);
  const source = fs.readFileSync('build/file_index.go', 'utf8');
  const baseline = runTests(source, tests, 'unchanged renderer');
  assert.equal(baseline.status, 0, `unchanged renderer: ${baseline.command}\n${baseline.output}`);
  assert.match(baseline.output, /^--- PASS: TestFileIndexVeryWideRepositoryStaysBounded\b/m, `TestFileIndexVeryWideRepositoryStaysBounded did not pass: ${baseline.command}\n${baseline.output}`);
  for (const [label, mutate] of mutations[group]) {
    const changed = mutate(source);
    assert.notEqual(changed, source, `${label}: mutation had no effect`);
    const result = runTests(changed, tests, label);
    assert.equal(result.status, 1, `${label}: regression tests did not reject fault: ${result.command}\n${result.output}`);
    assert.match(result.output, /^--- FAIL: Test\S+/m, `${label}: no failing regression test: ${result.command}\n${result.output}`);
    assert.doesNotMatch(result.output, /panic: test timed out/, `${label}: a timeout is not regression detection\n${result.output}`);
  }
}
module.exports = {verify};
if (require.main === module) {
  try {
    const controls = JSON.parse(fs.readFileSync(controlsPath, 'utf8'));
    if (process.argv[2] === 'good') {
      for (const group of Object.keys(mutations)) verify(group, controls.correct.files['build/file_index_test.go']);
      console.log('accepted complete regression fixture');
    } else if (process.argv[2] === 'bad') {
      const groups = ['accounting', 'order', 'guards', 'guards'];
      let rejected = 0;
      controls.wrong.forEach((wrong, i) => {
        try { verify(groups[i], wrong.files['build/file_index_test.go']); }
        catch (error) {
          if (!error.message.includes('regression tests did not reject fault')) throw error;
          rejected++;
        }
      });
      assert.equal(rejected, 4, 'observer accepted a weak test fixture');
      console.log('rejected 4 weak test fixtures');
      process.exitCode = 1;
    } else {
      throw new Error('expected good or bad control mode');
    }
  } catch (error) {
    console.error(error.stack);
    process.exitCode = 2;
  }
}
