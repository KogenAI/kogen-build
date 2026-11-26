const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const slug = 'build-fit-optional-file-inventory';
const helper = `features/steps/${slug}__driver/inventory_test.go.txt`;
const testName = 'TestFitOptionalInventoryAcceptance';

function replay(name, bytes, ceiling, files = {}) {
  assert.match(name, /^(Brand|ProductRoom|Small|Authority)$/);
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'inventory-acceptance-'));
  try {
    const mapping = {[path.resolve('build/zz_inventory_acceptance_test.go')]: path.resolve(helper)};
    for (const [destination, source] of Object.entries(files)) {
      const local = path.join(temp, path.basename(destination));
      fs.writeFileSync(local, source);
      mapping[path.resolve(destination)] = local;
    }
    const overlay = path.join(temp, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({Replace: mapping}));
    const binary = path.join(temp, 'build.test');
    const compile = ['test', '-overlay', overlay, '-c', '-o', binary, './build'];
    const built = spawnSync('go', compile, {encoding: 'utf8', timeout: 50000});
    const id = `${testName}/${name}`;
    if (built.status !== 0) {
      return {ok: false, ran: false, detail: `${id}: go ${compile.join(' ')} status=${built.status}\n${built.error || ''}\n${built.stdout}\n${built.stderr}`};
    }
    const args = ['-test.run', `^${testName}$/^${name}$`, '-test.count=1', '-test.v'];
    const p = spawnSync(binary, args, {
      cwd: path.resolve('build'), encoding: 'utf8', timeout: 50000,
      env: {...process.env, INVENTORY_FIXTURE_BYTES: String(bytes), INVENTORY_FIXTURE_CEILING: String(ceiling)},
    });
    const stdout = p.stdout || '';
    const ran = stdout.split('\n').includes(`=== RUN   ${id}`);
    const passed = stdout.split('\n').some(line => line.trimStart().startsWith(`--- PASS: ${id} (`));
    return {
      ok: p.status === 0 && ran && passed,
      ran, detail: `${id}: build.test ${args.join(' ')} (./build) status=${p.status}\n${p.error || ''}\n${stdout}\n${p.stderr}`,
    };
  } finally {
    fs.rmSync(temp, {recursive: true, force: true});
  }
}

if (require.main === module) {
  // Feasibility only: calibrate the same observer against the declared correct fixture
  // and a working provider send that silently loses a required path in the reduced index.
  const mode = process.argv[2];
  const controls = JSON.parse(fs.readFileSync(`.kogen/intents/${slug}/controls.json`, 'utf8'));
  assert.ok(mode === 'good' || mode === 'bad');
  const correct = controls.correct.files;
  const cases = mode === 'good' ? [['Brand', 532421], ['ProductRoom', 513246], ['Small', 0], ['Authority', 0]] : [['Brand', 532421]];
  const files = mode === 'good' ? correct : {...correct, ...controls.wrong.find(w => w.name === 'lose a required path').files};
  for (const [name, bytes] of cases) {
    const result = replay(name, bytes, 524288, files);
    if (mode === 'bad' && result.ran && !result.ok && result.detail.includes('reduced inventory no longer lists required path "app.txt"')) {
      console.log('Rejected provider input missing a required path');
      process.exit(1);
    }
    assert.ok(mode === 'good' && result.ok, result.detail);
  }
  console.log('Accepted all four local Build fixtures');
} else {
  const {Given, When, Then} = require('@cucumber/cucumber');
  Given(`${slug}: the local native Build fixture is available`, function () {
    assert.ok(fs.statSync(helper).isFile(), `${testName}: missing fixture for ./build: ${helper}`);
  });
  When(`${slug}: I replay {string} with {int} bytes against the {int}-byte rendered ceiling`, {timeout: 110000}, function (name, size, ceiling) {
    this.inventoryResult = replay(name, size, ceiling);
  });
  When(`${slug}: I replay {string} at the {int}-byte rendered ceiling`, {timeout: 110000}, function (name, ceiling) {
    this.inventoryResult = replay(name, 0, ceiling);
  });
  Then(`${slug}: the opening fits without losing authority or required paths and the omitted path remains discoverable`, function () {
    assert.ok(this.inventoryResult.ok, this.inventoryResult.detail);
  });
  Then(`${slug}: the reset fits before shortening the newest outputs and keeps the exact archive and authority`, function () {
    assert.ok(this.inventoryResult.ok, this.inventoryResult.detail);
  });
  Then(`${slug}: the opening keeps the existing ordered listing and exact authority`, function () {
    assert.ok(this.inventoryResult.ok, this.inventoryResult.detail);
  });
  Then(`${slug}: the ceiling and its adjacent values admit fitting authority and refuse irreducible authority with recoverable evidence`, function () {
    assert.ok(this.inventoryResult.ok, this.inventoryResult.detail);
  });
}
