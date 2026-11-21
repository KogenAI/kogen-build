'use strict';
const fs = require('node:fs'), os = require('node:os'), path = require('node:path'), {spawnSync} = require('node:child_process');
const name = 'TestStalledExternalAcceptance';
function run(want, implementation) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stalled-'));
  try {
    const target = path.resolve('build/stalled_external_acceptance_test.go');
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
    const r = spawnSync('go', args, {encoding: 'utf8', env: {...process.env, PWD: process.cwd(), STALLED_WANT: JSON.stringify(want)}, timeout: 180000, maxBuffer: 32 * 1024 * 1024});
    const rows = (r.stdout || '').split('\n').filter(x => x.startsWith('{')).map(x => { try { return JSON.parse(x); } catch { return {}; } });
    const runs = rows.filter(x => x.Test === name && x.Action === 'run').length;
    const passes = rows.filter(x => x.Test === name && x.Action === 'pass').length;
    return {ok: r.status === 0 && runs === 1 && passes === 1, text: `${name}: go ${args.join(' ')} exit ${r.status}; ran ${runs}, passed ${passes}; absent or skipped if ran zero\n${r.stdout || ''}${r.stderr || ''}${r.error || ''}`};
  } finally { fs.rmSync(dir, {recursive: true, force: true}); }
}
// A control file is its whole content or {patch}; a patch applies to base (a string).
function materialize(p, spec, base) {
  if (typeof spec === 'string') return spec;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'patch-'));
  try {
    fs.mkdirSync(path.join(dir, path.dirname(p)), {recursive: true});
    fs.writeFileSync(path.join(dir, p), base);
    fs.writeFileSync(path.join(dir, 'x.patch'), spec.patch);
    const r = spawnSync('git', ['apply', 'x.patch'], {cwd: dir, encoding: 'utf8'});
    if (r.status !== 0) throw Error('patch for ' + p + ' did not apply: ' + r.stderr);
    return fs.readFileSync(path.join(dir, p), 'utf8');
  } finally { fs.rmSync(dir, {recursive: true, force: true}); }
}
module.exports = {run};
if (require.main === module) {
  const controls = JSON.parse(fs.readFileSync('.kogen/intents/build-classifies-stalled-external-pass/controls.json', 'utf8'));
  const impl = {};
  for (const [p, f] of Object.entries(controls.correct.files)) impl[p] = materialize(p, f, fs.readFileSync(p, 'utf8'));
  if (process.argv[2] === 'bad') {
    const w = controls.wrong.find(x => x.name === 'drop failing detail');
    for (const [p, f] of Object.entries(w.files)) impl[p] = materialize(p, f, impl[p]);
  }
  const r = run({Staged: false, Same: true, Sequence: [1, 1, 1, 1, 1], Rounds: 3, Passes: 3, Outcome: 'no_progress', Exit: 1, Message: ['candidate/no_progress', 'acceptance', 'Failure 1 (line 2)'], Lacks: ['executor_round_limit'], Stage: '', Failing: '', StallEvents: 1, RepeatEvents: 0}, impl);
  console.log(r.ok ? 'observer accepted' : 'observer rejected');
  if (!r.ok && process.env.STALLED_DEBUG) console.error(r.text);
  process.exit(r.ok ? 0 : 1);
}
