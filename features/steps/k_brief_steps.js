'use strict';
const {spawnSync}=require('node:child_process');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const assert=require('node:assert/strict');
function run(mode) {
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'k-brief-'));
 try {
  const overlay=path.join(dir,'overlay.json');
  fs.writeFileSync(overlay,JSON.stringify({Replace:{[path.resolve('build/k_brief_acceptance_test.go')]:path.resolve('features/steps/k_brief_test.go.txt')}}));
  const args=['test','-overlay',overlay,'./build','-run','^TestKBriefAcceptance$','-count=1','-v'];
  const r=spawnSync('go',args,{encoding:'utf8',env:{...process.env,KB_MODE:mode},timeout:180000,maxBuffer:8*1024*1024});
  const output=(r.stdout||'')+(r.stderr||'');
  if(r.status!==0) throw new Error(`TestKBriefAcceptance: go ${args.join(' ')} exit ${r.status}:\n${output}`);
  assert.ok(output.includes('--- PASS: TestKBriefAcceptance ('),`TestKBriefAcceptance absent/skipped/zero runs: go ${args.join(' ')} exit ${r.status}: ${output}`);
 } finally {fs.rmSync(dir,{recursive:true,force:true});}
}
if(require.main===module){
 try{run(process.argv[2]);console.log('observer accepted');}
 catch(e){console.log('observer rejected');process.exitCode=1;}
}else{
 const {Then}=require('@cucumber/cucumber');
 Then('the builder brief with caps {int} and {int} preserves authority and reports {int} omitted entries and {int} omitted bytes',{timeout:200000},function(files,bytes,entries,omitted){
  assert.deepEqual([files,bytes,entries,omitted],[200,8192,300,12288]);run('capped');
 });
 Then('the uncapped builder opening message matches its existing golden',{timeout:200000},function(){
  const r=spawnSync('go',['test','./build','-run','^TestOpeningMessageEqualsTheElixirReference$','-count=1','-v'],{encoding:'utf8',timeout:180000,maxBuffer:8*1024*1024});
  const out=(r.stdout||'')+(r.stderr||'');
  assert.equal(r.status,0,`TestOpeningMessageEqualsTheElixirReference: go test ./build -run ^TestOpeningMessageEqualsTheElixirReference$ exit ${r.status}: ${out}`);
  assert.ok(out.includes('--- PASS: TestOpeningMessageEqualsTheElixirReference ('),`TestOpeningMessageEqualsTheElixirReference absent/skipped/zero runs: go test ./build exit ${r.status}: ${out}`);
 });
}
