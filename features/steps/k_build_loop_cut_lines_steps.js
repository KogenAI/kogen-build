'use strict';
const {spawnSync}=require('node:child_process');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const assert=require('node:assert/strict');
// Runs kogen-build's loop knob tests (go test -run '^(TestLoopKnobsNativeBuilder|TestLoopKnobsReview)$' ./build)
// against a scratch copy of the pinned kogen-loop module whose tool-result cut is replaced by one variant.
// The checkout is never written: go.mod and go.sum are copied and the copy is used through -modfile.
// Run directly with mode good or bad, the step file is its own observer control: it overlays
// build/loop_knobs_test.go with features/steps/k_build_loop_cut_lines_knobs_test.go.txt (good) or with that
// file carrying the old byte-exact check (bad) and expects every variant's outcome below.
const TESTS=['TestLoopKnobsNativeBuilder','TestLoopKnobsReview'];
const CUT_TEXT='func cutText(text string, bound, headSize int) string {';
const CUT_CALL='\treturn cutText(text, maxResult, defaultResultHead)\n}';
// Line-aligned is kogen-loop eed48c4's cutText ("Align tool text cuts with complete lines"), verbatim.
const LINE_ALIGNED=`func cutText(text string, bound, headSize int) string {
	if len(text) <= bound {
		return text
	}
	marker := func(n int) string { return fmt.Sprintf("\\n[... %d bytes omitted ...]\\n", n) }
	head := validPrefix(text[:headSize])
	if i := strings.LastIndexByte(head, '\\n'); i >= 0 {
		head = head[:i+1]
	}
	tailSize := bound - headSize - len(marker(len(text)))
	tail := validSuffix(text[len(text)-tailSize:])
	if i := strings.IndexByte(tail, '\\n'); i >= 0 {
		tail = tail[i+1:]
	}
	return head + marker(len(text)-len(head)-len(tail)) + tail
}
`;
// The other variants replace only Cut, the tool-result cut, with kblclCut.
const CUTS={
 'head-only':`func kblclCut(text string, bound, headSize int) string {
	if len(text) <= bound {
		return text
	}
	marker := func(n int) string { return fmt.Sprintf("\\n[... %d bytes omitted ...]\\n", n) }
	head := validPrefix(text[:bound-len(marker(len(text)))])
	return head + marker(len(text)-len(head))
}
`,
 'over-limit':`func kblclCut(text string, bound, headSize int) string {
	return cutText(text, bound+4096, headSize)
}
`,
 'unmarked':`func kblclCut(text string, bound, headSize int) string {
	if len(text) <= bound {
		return text
	}
	return validPrefix(text[:bound])
}
`,
};
const VARIANTS=['pinned','line-aligned','head-only','over-limit','unmarked'];
function go(args,opts){
 const r=spawnSync('go',args,{encoding:'utf8',timeout:600000,maxBuffer:64*1024*1024,...opts});
 return {status:r.status,signal:r.signal,error:r.error,output:(r.stdout||'')+(r.stderr||'')};
}
function patch(file,variant){
 const src=fs.readFileSync(file,'utf8');
 let out;
 if(variant==='line-aligned'){
  const start=src.indexOf(CUT_TEXT),end=src.indexOf('\nfunc validPrefix(',start);
  assert.ok(start>=0&&end>start&&src.indexOf(CUT_TEXT,start+1)<0,`${file}: no single cutText before validPrefix to replace`);
  out=src.slice(0,start)+LINE_ALIGNED+src.slice(end);
 }else{
  assert.ok(src.split(CUT_CALL).length===2,`${file}: Cut does not end in one ${JSON.stringify(CUT_CALL)}`);
  out=src.replace(CUT_CALL,CUT_CALL.replace('cutText','kblclCut'))+'\n'+CUTS[variant];
 }
 fs.chmodSync(file,0o644);
 fs.writeFileSync(file,out);
}
// knobs runs the knob tests with the variant cut and returns the go test outcome.
function knobs(variant,overlayFile){
 assert.ok(VARIANTS.includes(variant),`unknown kogen-loop cut ${variant}`);
 const root=fs.realpathSync(process.cwd());
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'k-build-loop-cut-lines-'));
 try{
  const env={...process.env,GOWORK:'off',PWD:root};
  delete env.GOFLAGS;
  const m=go(['list','-m','-f','{{.Dir}}','kogen-loop'],{cwd:root,env});
  const loopDir=m.output.trim();
  assert.ok(m.status===0&&loopDir&&fs.existsSync(path.join(loopDir,'tools','tools.go')),`go list -m -f {{.Dir}} kogen-loop exit ${m.status}: no downloaded kogen-loop module:\n${m.output}`);
  const loop=path.join(dir,'kogen-loop');
  fs.cpSync(loopDir,loop,{recursive:true});
  const w=spawnSync('chmod',['-R','u+w',loop],{encoding:'utf8'});
  assert.equal(w.status,0,`chmod -R u+w ${loop}: ${w.stderr}`);
  if(variant!=='pinned') patch(path.join(loop,'tools','tools.go'),variant);
  fs.copyFileSync(path.join(root,'go.mod'),path.join(dir,'go.mod'));
  fs.copyFileSync(path.join(root,'go.sum'),path.join(dir,'go.sum'));
  const e=go(['mod','edit','-replace','kogen-loop='+loop,path.join(dir,'go.mod')],{cwd:root,env});
  assert.equal(e.status,0,`go mod edit -replace kogen-loop=${loop}: ${e.output}`);
  const args=['test','-modfile='+path.join(dir,'go.mod'),'-count=1','-v','-run','^('+TESTS.join('|')+')$'];
  if(overlayFile){
   const overlay=path.join(dir,'overlay.json');
   fs.writeFileSync(overlay,JSON.stringify({Replace:{[path.join(root,'build','loop_knobs_test.go')]:overlayFile}}));
   args.push('-overlay',overlay);
  }
  args.push('./build');
  const r=go(args,{cwd:root,env});
  return {...r,command:'go '+args.join(' ')};
 }finally{
  spawnSync('chmod',['-R','u+w',dir]);
  fs.rmSync(dir,{recursive:true,force:true});
 }
}
// status says, for each named test, whether go test ran it and with what result.
function status(out){
 return TESTS.map(t=>{
  const seen=r=>new RegExp(`^--- ${r}: ${t} \\(`,'m').test(out);
  return `${t} ${seen('PASS')?'ran and passed':seen('FAIL')?'ran and failed':seen('SKIP')?'was skipped':'did not run'}`;
 }).join(', ');
}
// results are go test's own lines: the named tests' results and runs first, then failures and the summary.
function results(out){
 const lines=out.split('\n');
 const top=lines.filter(l=>TESTS.some(t=>l.startsWith(`--- PASS: ${t} (`)||l.startsWith(`--- FAIL: ${t} (`)||l.startsWith(`--- SKIP: ${t} (`)));
 const runs=lines.filter(l=>TESTS.some(t=>l===`=== RUN   ${t}`));
 const rest=lines.filter(l=>!top.includes(l)&&!runs.includes(l)&&/^\s+--- (PASS|FAIL|SKIP): |^\s+\S+_test\.go:\d+: |^(ok|FAIL)\s|^FAIL$|^panic: |no tests to run|^# |\.go:\d+:\d+: /.test(l));
 return [...top,...runs,...rest.slice(0,60)].join('\n');
}
function expect(variant,outcome,overlayFile,attach){
 assert.ok(outcome==='pass'||outcome==='fail',`unknown outcome ${outcome}`);
 const r=knobs(variant,overlayFile);
 if(attach) attach(`${r.command}\nexit ${r.status}\n${r.output}`,'text/plain');
 const msg=`${results(r.output)}\n${status(r.output)}; want both to ${outcome} with the ${variant} kogen-loop cut; go test exit ${r.status}${r.signal?' signal '+r.signal:''}${r.error?' '+r.error.message:''}; command: ${r.command}`;
 for(const t of TESTS){
  if(outcome==='pass') assert.match(r.output,new RegExp(`^--- PASS: ${t} \\(`,'m'),msg);
  else assert.match(r.output,new RegExp(`^--- FAIL: ${t} \\(`,'m'),msg);
 }
 if(outcome==='pass') assert.equal(r.status,0,msg);
 else assert.ok(r.status!==0&&r.status!==null,msg);
}
const EXPECTED=[['pinned','pass'],['line-aligned','pass'],['head-only','pass'],['over-limit','fail'],['unmarked','fail']];
// BAD_KNOB_CUT is the old byte-exact size check, put into the good fixture for the bad control.
const BAD_KNOB_CUT=`func knobCut(t *testing.T, what, out string, bound int) {
	t.Helper()
	if !knobMarker.MatchString(out) {
		t.Fatalf("%s: no omission marker in %d bytes: %.200q", what, len(out), out)
	}
	if len(out) > bound || len(out) < bound-64 {
		t.Fatalf("%s: %d bytes delivered, want about %d", what, len(out), bound)
	}
}
`;
function control(mode){
 const good=path.join(fs.realpathSync(process.cwd()),'features','steps','k_build_loop_cut_lines_knobs_test.go.txt');
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'k-build-loop-cut-lines-control-'));
 try{
  let fixture=good;
  if(mode==='bad'){
   const src=fs.readFileSync(good,'utf8');
   const start=src.indexOf('func knobCut('),end=src.indexOf('\n}\n',start)+3;
   assert.ok(start>=0&&end>start);
   fixture=path.join(dir,'loop_knobs_test.go');
   fs.writeFileSync(fixture,src.slice(0,start)+BAD_KNOB_CUT+src.slice(end));
  }else assert.equal(mode,'good');
  for(const [variant,outcome] of EXPECTED) expect(variant,outcome,fixture);
 }finally{fs.rmSync(dir,{recursive:true,force:true});}
}
if(require.main===module){
 try{control(process.argv[2]);console.log('observer accepted');}
 catch(e){console.log('observer rejected');process.exitCode=1;}
}else{
 const {Then}=require('@cucumber/cucumber');
 Then('kogen-build\'s loop knob tests {word} with the {string} kogen-loop cut',{timeout:1260000},function(outcome,variant){
  expect(variant,outcome,undefined,(text,type)=>this.attach(text,type));
 });
}
