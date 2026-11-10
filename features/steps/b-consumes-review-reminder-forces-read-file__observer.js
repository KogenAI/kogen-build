const fs = require('fs');
const os = require('os');
const path = require('path');
const cp = require('child_process');
const target = 'build/k_review_verdict_strand_test.go';
const head = '0fb2834072bec35ffb5620795f3698df07bc66e7';
function observe(source) {
  const original = JSON.parse(fs.readFileSync('.kogen/intents/b-consumes-review-reminder-forces-read-file/controls.json','utf8')).wrong[0].files[target];
  const test = s => s.slice(s.indexOf('func TestKReviewVerdictStrandReset('), s.indexOf('// TestKReviewVerdictStrandSafety:'));
  if (test(source) !== test(original)) throw Error('Existing reset, budget, fault, exhaustion and landing assertions must remain unchanged');
  const fn = source.match(/func forcedBodies\([\s\S]*?\n\}/);
  if (!fn) throw Error('Missing callable forcedBodies');
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'reminder-contract-'));
  try {
    fs.writeFileSync(path.join(dir,'go.mod'), 'module reminderacceptance\n\ngo 1.27.2\n');
    fs.writeFileSync(path.join(dir,'reminder_test.go'), `package build
import("encoding/json";"testing")
type fakeProvider struct { bodies [][]byte }
func(f *fakeProvider) reviewBodies() [][]byte{return f.bodies}
func jsonOf(v any)string{b,_:=json.Marshal(v);return string(b)}
var forcedSubmit=map[string]any{"type":"function","name":"submit_review"}
${fn[0]}
func TestReminderAcceptance(t *testing.T){
 f:= &fakeProvider{bodies:[][]byte{[]byte("{\\"tool_choice\\":\\"auto\\"}"),[]byte("{\\"tool_choice\\":{\\"type\\":\\"function\\",\\"name\\":\\"submit_review\\"}}"),[]byte("{\\"tool_choice\\":{\\"type\\":\\"function\\",\\"name\\":\\"read_file\\"}}")}}
 indexes,bodies:=forcedBodies(t,f)
 if jsonOf(indexes)!="[1,2]"{t.Fatalf("forced reminders: %v, want [1 2]",indexes)}
 if len(bodies)!=2 || string(bodies[0])!=string(f.bodies[1]) || string(bodies[1])!=string(f.bodies[2]){t.Fatal("forced reminder bodies must retain their original indexes and bytes")}
}
`);
    const r = cp.spawnSync('go',['test','-count=1','-v','.'],{cwd:dir,encoding:'utf8'});
    if(r.status!==0 || !r.stdout.includes('--- PASS: TestReminderAcceptance')) throw Error(`Reminder callable contract failed\n${r.stdout}${r.stderr}`);
  } finally {fs.rmSync(dir,{recursive:true,force:true});}
}
module.exports = {observe};
if(require.main===module){
  const controls=JSON.parse(fs.readFileSync('.kogen/intents/b-consumes-review-reminder-forces-read-file/controls.json','utf8'));
  const source=process.argv[2]==='good'?controls.correct.files[target]:controls.wrong[0].files[target];
  try {observe(source); console.log('accepted');} catch(e){console.log('rejected');process.exitCode=1;}
}
