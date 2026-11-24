'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const root = path.resolve(__dirname, '../../..');
function run(mode, spec) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ncwa-'));
  const target = path.join(root, 'build/zz_ncwa_acceptance_test.go');
  const binary = path.join(dir, 'fixture');
  const command = 'go test -c -o <temporary-binary> ./build; <temporary-binary> -test.run ^TestNCWA$ -test.v';
  try {
    if (fs.existsSync(target)) throw new Error('refusing to overwrite ' + target);
    fs.copyFileSync(path.join(__dirname, 'fixture_test.go.txt'), target);
    const build = spawnSync('go', ['test', '-c', '-o', binary, './build'], {cwd:root, encoding:'utf8', timeout:60000, env:{...process.env, GOFLAGS:'-trimpath'}});
    if (build.status !== 0) return {ok:false, command, output:(build.stdout||'')+(build.stderr||''), status:build.status};
    const result = spawnSync(binary, ['-test.run', '^TestNCWA$', '-test.v', '-test.timeout', '45s'], {cwd:path.join(root,'build'), encoding:'utf8', timeout:48000, env:{...process.env, NCWA_CASE:mode, NCWA_SPEC:JSON.stringify(spec)}});
    const output = (result.stdout||'')+(result.stderr||'');
    return {ok:result.status===0 && output.split(/\r?\n/).includes('NCWA_PASS'), command, output, status:result.status};
  } finally {
    fs.rmSync(target, {force:true}); fs.rmSync(dir, {recursive:true,force:true});
  }
}
module.exports = {run};
if (require.main === module) {
  const result = run(process.argv[2], {});
  if (process.argv[2] === 'control-good' || process.argv[2] === 'control-bad') {
    // Stable control output; never hide compiler/startup errors as an observed rejection.
    if (!result.output.includes('held preserves every checkout and evidence byte') && !result.ok) {
      process.stderr.write(result.output); process.exit(2);
    }
    console.log(result.ok ? 'observer accepted' : 'observer rejected');
    process.exit(result.ok ? 0 : 1);
  }
  process.stdout.write(result.output); process.exit(result.ok ? 0 : 1);
}
