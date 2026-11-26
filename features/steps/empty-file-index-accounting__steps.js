'use strict';

const assert = require('node:assert/strict');
const {spawnSync} = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const slug = 'empty-file-index-accounting';
const testName = 'TestEmptyFileIndexAcceptanceProbe';
const sourcePath = 'build/file_index.go';

function run(exe, args, options) {
  const result = spawnSync(exe, args, {encoding: 'utf8', ...options});
  assert.equal(result.status, 0, `${testName}: ${exe} ${args.join(' ')} exited ${result.error || result.signal || result.status}\n${result.stdout}\n${result.stderr}`);
}

function observe(input, source) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), `${slug}-`));
  try {
    const overlay = path.join(dir, 'overlay.json');
    const executable = path.join(dir, 'index.test');
    const output = path.join(dir, 'index.json');
    const mapping = {
      [path.resolve('build/zz_empty_file_index_acceptance_test.go')]:
        path.resolve(`features/steps/${slug}__driver/index_test.go.txt`),
    };
    if (source !== undefined) {
      const local = path.join(dir, 'file_index.go');
      fs.writeFileSync(local, source);
      mapping[path.resolve(sourcePath)] = local;
    }
    fs.writeFileSync(overlay, JSON.stringify({Replace: mapping}));
    run('go', ['test', '-overlay', overlay, '-c', '-o', executable, './build'], {timeout: 40000});
    const args = [`-test.run=^${testName}$`, '-test.count=1', '-test.timeout=20s'];
    run(executable, args, {
      cwd: path.resolve('build'), timeout: 25000,
      env: {...process.env, EMPTY_FILE_INDEX_INPUT: JSON.stringify(input), EMPTY_FILE_INDEX_OUTPUT: output},
    });
    assert.ok(fs.existsSync(output), `${testName}: ${executable} ${args.join(' ')} exited 0 but produced no index; probe did not run to completion`);
    return JSON.parse(fs.readFileSync(output, 'utf8'));
  } finally {
    fs.rmSync(dir, {recursive: true, force: true});
  }
}

function verify(input, counts, paths, source) {
  const [full, summarized] = observe(input, source);
  const heading = '# Files\n\n';
  const rows = paths.map(p => `${p}\n`).join('');
  const [total, listed, omitted] = counts;
  const notice = 'The omitted paths still exist in the repository: they remain readable and searchable with the existing file, read and search tools.';
  const summary = `${heading}total paths: ${total}\nlisted paths: ${listed}\nomitted paths: ${omitted}\n\n${notice}\n\n## Listed paths\n${rows}`;
  assert.equal(full, heading + rows, `${testName} (./build): full index for hex ${JSON.stringify(input.listing)}`);
  assert.equal(summarized, summary, `${testName} (./build): summarized index for hex ${JSON.stringify(input.listing)}`);
}

if (require.main === module) {
  const controls = JSON.parse(fs.readFileSync(`.kogen/intents/${slug}/controls.json`, 'utf8'));
  const mode = process.argv[2];
  if (mode === 'good') {
    const source = controls.correct.files[sourcePath];
    verify({listing: '', required: []}, [0, 0, 0], [], source);
    verify({listing: '00', required: []}, [0, 0, 0], [], source);
    verify({listing: '6100', required: ['a']}, [1, 1, 0], ['a'], source);
    console.log('Accepted all three rendered inventories');
  } else if (mode === 'bad') {
    const source = controls.wrong.find(w => w.name === 'Emit a blank listed row at zero counts').files[sourcePath];
    try {
      verify({listing: '', required: []}, [0, 0, 0], [], source);
    } catch (error) {
      if (!(error instanceof assert.AssertionError) || error.actual !== error.expected + '\n' ||
          !error.message.startsWith(`${testName} (./build): summarized index`)) throw error;
      console.log('Rejected a blank listed row despite zero counts');
      process.exitCode = 1;
    }
  } else {
    throw new Error('Expected good or bad control mode');
  }
} else {
  const {Given, Then} = require('@cucumber/cucumber');

  Given(`${slug}: a Git listing encoded as hex {string}`, function (listing) {
    this.indexInput = {listing, required: []};
  });
  Given(`${slug}: a Git listing encoded as hex {string} with required path {string}`, function (listing, required) {
    this.indexInput = {listing, required: [required]};
  });
  Then(`${slug}: the index reports {int} total, {int} listed, {int} omitted and lists no paths`, {timeout: 90000}, function (total, listed, omitted) {
    verify(this.indexInput, [total, listed, omitted], []);
  });
  Then(`${slug}: the index reports {int} total, {int} listed, {int} omitted and lists {string} exactly once`, {timeout: 90000}, function (total, listed, omitted, name) {
    verify(this.indexInput, [total, listed, omitted], [name]);
  });
}
