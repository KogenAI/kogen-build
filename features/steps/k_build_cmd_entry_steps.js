'use strict';
const {Then}=require('@cucumber/cucumber');
const {spawnSync}=require('node:child_process');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const assert=require('node:assert/strict');
function run(cmd,args,options={}) {
 const r=spawnSync(cmd,args,{encoding:'utf8',...options});
 if(r.error) throw r.error;
 return r;
}
function checked(cmd,args,options={}) {
 const r=run(cmd,args,options);
 assert.equal(r.status,0,`${cmd} ${args.join(' ')}: ${r.stdout}${r.stderr}`);
 return r.stdout.trim();
}
function exercise(mode,id,exit,kind) {
 const tmp=fs.mkdtempSync(path.join(os.tmpdir(),'k-build-entry-'));
 try {
  const bin=path.join(tmp,'kogen-build');
  checked('go',['build','-o',bin,'./cmd/kogen-build']);
  const repo=path.join(tmp,'checkout');
  fs.cpSync(path.resolve('build/testdata/legacy/project'),repo,{recursive:true});
  const env={...process.env,HOME:path.join(tmp,'home'),GIT_CONFIG_GLOBAL:'/dev/null',GIT_CONFIG_NOSYSTEM:'1',GIT_TERMINAL_PROMPT:'0'};
  const opts={cwd:repo,env};
  checked('git',['init','-q','--initial-branch=main'],opts);
  checked('git',['config','user.name','Standalone Fixture'],opts);
  checked('git',['config','user.email','standalone@example.invalid'],opts);
  checked('git',['add','.'],opts);
  checked('git',['-c','commit.gpgsign=false','commit','-qm','fixture'],opts);
  const head=checked('git',['rev-parse','HEAD'],opts);
  const tree=checked('git',['rev-parse','HEAD^{tree}'],opts);
  const args=[mode,'--project',repo,...(mode==='build'?['--slug','fixture']:['--id',id])];
  const result=run(bin,args,{cwd:tmp,env});
  assert.equal(result.status,exit,`${bin} ${args.join(' ')}: ${result.stdout}${result.stderr}`);
  const doc=JSON.parse(result.stdout);
  assert.deepEqual({exit_code:doc.exit_code,branch:doc.branch,tree:doc.tree,checks:doc.checks,kind:doc.error?.kind},
   {exit_code:exit,branch:'main',tree,checks:[],kind});
  assert.equal(typeof doc.error.category,'string');
  assert.equal(typeof doc.error.message,'string');
  assert.equal(checked('git',['rev-parse','HEAD'],opts),head);
 } finally {fs.rmSync(tmp,{recursive:true,force:true});}
}
Then('the standalone Build command refuses an unapproved fixture with exit {int} and kind {string}',{timeout:120000},function(exit,kind){exercise('build',null,exit,kind);});
Then('the standalone Resume command refuses checkpoint {string} with exit {int} and kind {string}',{timeout:120000},function(id,exit,kind){exercise('resume',id,exit,kind);});
