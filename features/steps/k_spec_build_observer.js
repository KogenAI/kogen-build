'use strict';
const assert=require('node:assert/strict');
function observe(r,mode){
 assert.equal(r.firstExit,0,`first Build failed: ${r.firstError}`);
 assert.equal(r.firstClean,true,'first landed tree must be clean');
 assert.equal(r.historyIntact,true,'historical model changed');
 assert.equal(r.secondExit,0,`next Build failed: ${r.secondError}`);
 assert.equal(r.historyAfterSecond,true,'historical Intent changed');
 const trace=s=>s.trim()?s.trim().split('\n').map(JSON.parse):[];
 const second=trace(r.secondTrace);
 if(mode==='empty'){
  assert.deepEqual(r.maintainedPaths,[],'model-free Build created model files');
  assert.deepEqual(second,[],'model-free Build invoked Quint');return;
 }
 assert.equal(r.maintainedPaths.length,1,'landing must commit one maintained model outside historical Intents');
 const p=r.maintainedPaths[0];
 assert.deepEqual(r.maintainedContents,[model],'maintained bytes differ');
 const runs=second.filter(x=>x.args[0]==='run'&&x.args[1]===p);
 assert.ok(second.some(x=>x.args[0]==='typecheck'&&x.args[1]===p),'next Build never typechecked maintained path');
 assert.ok(runs.length>0,'next Build never ran maintained invariant');
 for(const x of runs){assert.deepEqual(x.args,['run',p,'--backend','typescript','--invariant','Safe','--seed','7','--max-samples','3','--max-steps','4']);assert.equal(x.text,model);}
 assert.notEqual(r.thirdExit,0,'violating candidate landed');
 assert.equal(r.thirdNoCommit,true,'violating Build made a commit');
 assert.ok(trace(r.thirdTrace).some(x=>x.args[0]==='run'&&x.args[1]===p&&x.text.includes('val Safe = false')),'violation did not reach maintained verification');
}
const model="module fixture { var x: int\n action init = x' = 0\n action step = x' = x\n val Safe = x == 0\n}\n";
function control(bad){
 const p='.kogen/models/fixture/model.qnt';
 const row={args:['run',p,'--backend','typescript','--invariant','Safe','--seed','7','--max-samples','3','--max-steps','4'],text:model};
 const r={firstExit:0,firstClean:true,historyIntact:true,secondExit:0,historyAfterSecond:true,maintainedPaths:[p],maintainedContents:[model],secondTrace:JSON.stringify({args:['typecheck',p],text:model})+'\n'+JSON.stringify(row),thirdExit:1,thirdNoCommit:true,thirdOutput:'invariant violated: '+p,thirdTrace:JSON.stringify({...row,text:model.replace('val Safe = x == 0','val Safe = false')})};
 // Mutate each contractual field independently: no early missing-feature red masks later predicates.
 observe(r,'modeled');observe({...r,maintainedPaths:[],maintainedContents:[],secondTrace:''},'empty');
 const mutations=[{firstExit:1},{firstClean:false},{historyIntact:false},{secondExit:1},{historyAfterSecond:false},{maintainedPaths:[]},{maintainedContents:['changed']},{secondTrace:JSON.stringify(row)},{secondTrace:JSON.stringify({args:['typecheck',p],text:model})},{secondTrace:JSON.stringify({args:['typecheck',p],text:model})+'\n'+JSON.stringify({...row,args:row.args.map(a=>a==='7'?'8':a)})},{thirdExit:0},{thirdNoCommit:false},{thirdTrace:''}];
 for(const m of mutations){let rejected=false;try{observe({...r,...m},'modeled')}catch{rejected=true}assert.ok(rejected,'observer accepted mutation '+JSON.stringify(m));}
 if(bad)observe({...r,thirdNoCommit:false},'modeled');
}
module.exports={observe};
if(require.main===module){try{control(process.argv[2]==='bad');console.log('accepted')}catch{console.log('rejected');process.exitCode=1}}
