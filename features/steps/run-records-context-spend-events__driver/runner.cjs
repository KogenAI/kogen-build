'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');
const root = process.cwd();
const slug = 'run-records-context-spend-events';
const helper = path.join(root, 'features/steps/'+slug+'__driver/records_test.go');
const tests = {custody:'TestKogenRunRecordsCustody', identity:'TestKogenRunRecordsIdentity', attribution:'TestKogenRunRecordsAttribution'};
function run(which, files = {}, expected) {
 if (!expected) {
  const feature = fs.readFileSync('.kogen/intents/'+slug+'/acceptance.feature','utf8');
  const examples = [...feature.matchAll(/"""json\s*([\s\S]*?)\s*"""/g)].map(m=>JSON.parse(m[1]));
  expected = examples[Object.keys(tests).indexOf(which)];
 }
 const name = tests[which]; assert.ok(name, 'unknown focused test '+which);
 const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'rr-overlay-'));
 try {
  const replace = {[path.join(root,'build/zz_kogen_run_records_acceptance_test.go')]:helper};
  let i = 0;
  for(const [relative, content] of Object.entries(files)) {
   const file = path.join(tmp, String(i++)+'.go'); fs.writeFileSync(file,content);
   replace[path.join(root,relative)] = file;
  }
  const overlay = path.join(tmp,'overlay.json'); fs.writeFileSync(overlay,JSON.stringify({Replace:replace}));
  const args = ['test','-json','-count=1','-tags=kogen_run_records_acceptance','-overlay='+overlay,'./build','-run=^'+name+'$'];
  const result = cp.spawnSync('go',args,{cwd:root,env:{...process.env,RR_EXPECT:JSON.stringify(expected)},encoding:'utf8',timeout:100000,maxBuffer:8*1024*1024});
  const command = 'go '+args.join(' ');
  const records = (result.stdout || '').split('\n').filter(Boolean).flatMap(line=>{try{return [JSON.parse(line)]}catch{return []}});
  const ran = records.filter(r=>r.Test===name && r.Action==='run').length;
  const passed = records.filter(r=>r.Test===name && r.Action==='pass').length;
  const failed = records.some(r=>r.Test===name && r.Action==='fail');
  return {ok:result.status===0 && ran===1 && passed===1, rejected:result.status===1 && ran===1 && failed, diagnostic:`${name}: ${command}: status=${result.status} ran=${ran} passed=${passed}\n${result.stdout || ''}${result.stderr || ''}${result.error || ''}`};
 } finally { fs.rmSync(tmp,{recursive:true,force:true}); }
}
module.exports = {run};
if (require.main === module) {
 const controls = JSON.parse(fs.readFileSync('.kogen/intents/'+slug+'/controls.json','utf8'));
 const mode = process.argv[2];
 if(mode==='good') {
  for(const which of Object.keys(tests)) {const result = run(which,controls.correct.files); assert.ok(result.ok,result.diagnostic)}
  console.log('correct guard passed');
 } else if(mode==='bad') {
  for(const wrong of controls.wrong) {
   const which = wrong.scenario==='Request and tool custody'?'custody':wrong.scenario==='Response linkage'?'identity':'attribution';
   const result = run(which,{...controls.correct.files,...wrong.files});
   assert.ok(result.rejected,wrong.name+': expected a reached test assertion rejection\n'+result.diagnostic);
  }
  console.log('wrong witnesses rejected');
  process.exitCode=1;
 } else {throw new Error('expected good or bad')}
}
