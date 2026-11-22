'use strict';
const fs = require('node:fs'), os = require('node:os'), path = require('node:path'), {spawnSync} = require('node:child_process');
const name = 'TestExternalStallRoundLimitAcceptance';
// Runs the fixture test inside the build package through a go overlay, so no repository file is written.
function run(want, implementation) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stallround-'));
  try {
    const target = path.resolve('build/external_stall_round_limit_acceptance_test.go');
    if (fs.existsSync(target)) throw Error('acceptance overlay target exists');
    const fixture = path.join(dir, 'fixture_test.go');
    fs.copyFileSync(path.join(__dirname, 'fixture.go.txt'), fixture);
    const Replace = {[target]: fixture};
    for (const [p, body] of Object.entries(implementation || {})) {
      const file = path.join(dir, 'overlay-' + Object.keys(Replace).length + path.extname(p));
      fs.writeFileSync(file, body);
      Replace[path.resolve(p)] = file;
    }
    const overlay = path.join(dir, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({Replace}));
    const args = ['test', '-json', '-overlay', overlay, './build', '-run', '^' + name + '$', '-count=1'];
    const r = spawnSync('go', args, {encoding: 'utf8', env: {...process.env, PWD: process.cwd(), STALLROUND_WANT: JSON.stringify(want)}, timeout: 110000, maxBuffer: 32 * 1024 * 1024});
    const rows = (r.stdout || '').split('\n').filter(x => x.startsWith('{')).map(x => { try { return JSON.parse(x); } catch { return {}; } });
    const runs = rows.filter(x => x.Test === name && x.Action === 'run').length;
    const passes = rows.filter(x => x.Test === name && x.Action === 'pass').length;
    return {ok: r.status === 0 && runs === 1 && passes === 1, text: `${name}: go ${args.join(' ')} exit ${r.status}; ran ${runs}, passed ${passes}; absent or skipped if ran zero\n${r.stdout || ''}${r.stderr || ''}${r.error || ''}`};
  } finally { fs.rmSync(dir, {recursive: true, force: true}); }
}
module.exports = {run};
