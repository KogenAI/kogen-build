'use strict';
const {Then}=require('@cucumber/cucumber');
const {spawnSync}=require('node:child_process');
const fs=require('node:fs');const os=require('node:os');const path=require('node:path');const assert=require('node:assert/strict');
const {observe}=require('./k_spec_build_observer');
Then('synthetic Builds maintain and verify models in {string} mode',{timeout:200000},function(mode){
 assert.ok(['modeled','empty'].includes(mode));
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'ks-build-'));
 try{
  const overlay=path.join(dir,'overlay.json'),receipt=path.join(dir,'receipt.json');
  fs.writeFileSync(overlay,JSON.stringify({Replace:{[path.resolve('build/k_spec_build_acceptance_test.go')]:path.resolve('features/steps/k_spec_build_driver.go.txt')}}));
  const args=['test','-overlay',overlay,'./build','-run','^TestKSpecBuildReceipt$','-count=1','-v'];
  const r=spawnSync('go',args,{encoding:'utf8',env:{...process.env,KS_MODE:mode,KS_RECEIPT:receipt},timeout:180000,maxBuffer:8*1024*1024});
  const out=(r.stdout||'')+(r.stderr||'');
  assert.equal(r.status,0,`TestKSpecBuildReceipt: go ${args.join(' ')} exit ${r.status}: ${out}`);
  assert.ok(out.includes('--- PASS: TestKSpecBuildReceipt ('),`TestKSpecBuildReceipt absent/skipped/zero runs: go ${args.join(' ')} exit ${r.status}: ${out}`);
  observe(JSON.parse(fs.readFileSync(receipt,'utf8')),mode);
 }finally{fs.rmSync(dir,{recursive:true,force:true})}
});
