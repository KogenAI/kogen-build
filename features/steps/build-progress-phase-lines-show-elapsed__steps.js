const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const slug = 'build-progress-phase-lines-show-elapsed';
const feature = `.kogen/intents/${slug}/acceptance.feature`;
const helper = `features/steps/${slug}__driver/fixture_test.go.txt`;
const testName = 'TestPhaseElapsedAcceptance';

function observe(actual, expected) {
  assert.deepEqual(actual, expected, 'Build stage suffixes or unchanged tool lines differ');
}

function runFixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'phase-elapsed-'));
  try {
    const overlay = path.join(dir, 'overlay.json');
    const binary = path.join(dir, 'build.test');
    const result = path.join(dir, 'result.json');
    fs.writeFileSync(overlay, JSON.stringify({Replace: {
      [path.resolve('build/phase_elapsed_acceptance_test.go')]: path.resolve(helper)
    }}));
    const compile = ['test', '-c', '-overlay', overlay, '-o', binary, './build'];
    const built = spawnSync('go', compile, {encoding:'utf8', timeout:75000});
    assert.equal(built.status, 0, `${testName}: go ${compile.join(' ')}: status=${built.status} ${built.error || ''}\n${built.stdout}\n${built.stderr}`);
    const args = ['-test.run', `^${testName}$`, '-test.count=1', '-test.timeout=30s'];
    const p = spawnSync(binary, args, {encoding:'utf8', timeout:35000, env:{...process.env, PHASE_ELAPSED_RESULT:result}});
    const diagnostic = `${testName}: ${binary} ${args.join(' ')}: status=${p.status} ${p.error || ''}\n${p.stdout}\n${p.stderr}`;
    assert.equal(p.status, 0, diagnostic);
    assert.ok(fs.existsSync(result), `${diagnostic}\n${testName} was absent, skipped or did not produce its result`);
    return JSON.parse(fs.readFileSync(result, 'utf8'));
  } finally {
    fs.rmSync(dir, {recursive:true, force:true});
  }
}

if (require.main === module) {
  const mode = process.argv[2];
  const expected = JSON.parse(fs.readFileSync(feature, 'utf8').split('"""')[1]);
  const actual = JSON.parse(fs.readFileSync(`.kogen/intents/${slug}/${mode}.json`, 'utf8'));
  try {
    observe(actual, expected);
    console.log('observer accepted');
  } catch (error) {
    if (error.code !== 'ERR_ASSERTION') throw error;
    console.log('observer rejected');
    process.exitCode = 1;
  }
} else {
  const {Given, When, Then} = require('@cucumber/cucumber');
  Given(`${slug}: the focused Go fixture is available`, function() {
    assert.ok(fs.statSync(helper).isFile());
  });
  When(`${slug}: I run the model-free Build and resume with a fake clock`, {timeout:115000}, function() {
    this.phaseElapsed = runFixture();
  });
  Then(`${slug}: stage suffixes and unchanged tool lines are exactly:`, function(text) {
    observe(this.phaseElapsed, JSON.parse(text));
  });
}
