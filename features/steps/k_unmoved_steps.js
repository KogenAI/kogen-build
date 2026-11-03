'use strict';
const {spawnSync}=require('node:child_process');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const assert=require('node:assert/strict');
function run(mode) {
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'unmoved-'));
 try {
  const overlay=path.join(dir,'overlay.json');
  fs.writeFileSync(overlay,JSON.stringify({Replace:{[path.resolve('build/k_unmoved_acceptance_test.go')]:path.resolve('features/steps/k_unmoved_test.go.txt')}}));
  const name=mode==='compatibility'?'TestNamedFeedbackCompatibility':'TestKUnmovedAcceptance';
  const args=['test','-json','-overlay',overlay,'./build','-run','^'+name+'$','-count=1'];
  const r=spawnSync('go',args,{encoding:'utf8',env:{...process.env,KU_MODE:mode},timeout:240000,maxBuffer:16*1024*1024});
  const out=(r.stdout||'')+(r.stderr||'');
  const rows=(r.stdout||'').split('\n').filter(x=>x.startsWith('{')).map(x=>JSON.parse(x));
  if(r.status!==0 || rows.filter(x=>x.Test===name&&x.Action==='run').length!==1 || rows.filter(x=>x.Test===name&&x.Action==='pass').length!==1) throw new Error(`${name} absent, skipped, failed or zero runs; go ${args.join(' ')} exit ${r.status}\n${out}`);
 } finally {fs.rmSync(dir,{recursive:true,force:true});}
}
if(require.main===module) {
 try {run(process.argv[2]);console.log('observer accepted');} catch(e) {console.log('observer rejected');process.exitCode=1;}
} else {
 const {Then}=require('@cucumber/cucumber');
 Then('the unmoved red Build fixture {string} satisfies its feedback contract',{timeout:250000},function(mode,expected){
  assert.deepEqual(JSON.parse(expected),{failures:12,changedFile:'unrelated.txt',cause:'observed output was never produced',firstNoticePass:2,ordinaryFeedback:'unchanged and in order',stopAfterToolFreeRounds:2});
  run(mode);
 });
}
