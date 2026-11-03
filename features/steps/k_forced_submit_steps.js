'use strict';
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
function run(control = '') {
 const root = process.cwd();
 const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'forced-submit-'));
 try {
  const overlay = path.join(dir, 'overlay.json');
  fs.writeFileSync(overlay, JSON.stringify({Replace:{[path.join(root,'build','zz_k_forced_submit_test.go')]:path.join(root,'features/steps/k_forced_submit_test.go.txt')}}));
  const args = ['test','-json','-overlay',overlay,'-count=1','-run','^TestKForcedSubmitAcceptance$','./build'];
  const r = spawnSync('go',args,{encoding:'utf8',env:{...process.env,PWD:root,KFORCED_CONTROL:control},maxBuffer:16<<20});
  const output = `${r.stdout || ''}${r.stderr || ''}`;
  const records = (r.stdout || '').trim().split('\n').filter(Boolean).map(line => JSON.parse(line));
  const runs = records.filter(x => x.Test === 'TestKForcedSubmitAcceptance' && x.Action === 'run');
  const passes = records.filter(x => x.Test === 'TestKForcedSubmitAcceptance' && x.Action === 'pass');
  if (runs.length !== 1 || passes.length !== 1 || r.status !== 0) throw new Error(`TestKForcedSubmitAcceptance: go ${args.join(' ')} exit ${r.status}; ran ${runs.length}, passed ${passes.length}; output:\n${output}${r.error || ''}`);
 } finally { fs.rmSync(dir,{recursive:true,force:true}); }
}
if (require.main !== module) {
 const { Then } = require('@cucumber/cucumber');
 Then('ignored forced verdict requests stop with actionable provider diagnostics without extra requests', {timeout:900000}, function(){run();});
} else {
 try { run(process.argv[2]); console.log('accepted'); }
 catch (_) { console.log('rejected'); process.exitCode=1; }
}
