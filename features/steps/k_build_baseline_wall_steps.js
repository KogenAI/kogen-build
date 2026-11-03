'use strict';
const {spawnSync}=require('node:child_process');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const assert=require('node:assert/strict');
// The driver is a program, not a test: features/steps/k_build_baseline_wall_test.go.txt is overlaid as
// internal/kbwdriver/main.go, built, and run once per mode; it runs real Builds through build.Build.
function run(mode) {
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'k-build-baseline-wall-'));
 try {
  const root=fs.realpathSync(process.cwd());
  const overlay=path.join(dir,'overlay.json');
  const driver=path.join(dir,'kbwdriver');
  fs.writeFileSync(overlay,JSON.stringify({Replace:{[path.join(root,'internal/kbwdriver/main.go')]:path.join(root,'features/steps/k_build_baseline_wall_test.go.txt')}}));
  const args=['build','-overlay',overlay,'-o',driver,'./internal/kbwdriver'];
  const b=spawnSync('go',args,{cwd:root,encoding:'utf8',env:{...process.env,PWD:root},timeout:300000,maxBuffer:8*1024*1024});
  if(b.status!==0) throw new Error(`go ${args.join(' ')} exit ${b.status}:\n${(b.stdout||'')+(b.stderr||'')}`);
  const r=spawnSync(driver,[mode],{encoding:'utf8',timeout:300000,maxBuffer:8*1024*1024});
  const output=(r.stdout||'')+(r.stderr||'');
  assert.equal(r.status,0,`baseline wall driver ${mode} exit ${r.status}:\n${output}`);
  assert.ok(output.includes(`KBW ${mode}: PASS\n`),`baseline wall driver ${mode} did not report its pass: exit ${r.status}:\n${output}`);
 } finally {fs.rmSync(dir,{recursive:true,force:true});}
}
if(require.main===module){
 try{run(process.argv[2]);console.log('observer accepted');}
 catch(e){console.log('observer rejected');process.exitCode=1;}
}else{
 const {Then}=require('@cucumber/cucumber');
 Then('a Build whose baseline runs check {string} then failing check {string} records each check\'s start, end and wall time, with {string} at least {int} ms',{timeout:620000},function(slow,fast,again,ms){
  assert.deepEqual([slow,fast,again,ms],['slow','fast','slow',1000]);run('wall');
 });
 Then('a Build interrupted by SIGTERM at least {int} ms into baseline check {string} names the baseline and that check in its stop record',{timeout:620000},function(ms,name){
  assert.deepEqual([ms,name],[400,'slow']);run('stop');
 });
 Then('a Build interrupted by SIGTERM during setup names no baseline in its stop record',{timeout:620000},function(){
  run('setup');
 });
}
