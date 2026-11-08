const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

function last(text) { return text.split(/[\r\n]/).filter(x => x.includes('kogen: building (round 1)')).pop() || ''; }
function observe(rule, data, expected) {
  if (rule === 'latest') {
    for (const [i, name, status, file] of [[1, expected.first, expected.success, expected.path], [2, expected.second, expected.failure, '']]) {
      const line = last(data.snapshots[i]);
      assert.ok(line.includes(name), `snapshot ${i}: latest tool ${name} absent: ${line}`);
      assert.ok(line.includes(status), `snapshot ${i}: status ${status} absent: ${line}`);
      if (file) assert.ok(line.includes(file), `snapshot ${i}: path ${file} absent: ${line}`);
    }
    assert.ok(!last(data.snapshots[2]).includes(expected.first), 'latest line retains the previous tool');
    assert.ok(!last(data.snapshots[2]).includes(expected.path), 'pathless tool retains the previous path');
  } else if (rule === 'private') {
    assert.ok(!data.output.includes(expected.secret), 'tool result content leaked');
  } else {
    assert.equal(last(data.snapshots[0]), expected.baseline);
    assert.equal(data.snapshots.length, 1);
    assert.ok(!data.output.includes(expected.secret), 'unexpected tool content');
  }
}

const fixture = String.raw`package build

import (
 "context"
 "encoding/json"
 "net/http"
 "os"
 "strings"
 "sync"
 "testing"
)

type kProgressBuffer struct { mu sync.Mutex; text strings.Builder }
func (b *kProgressBuffer) Write(p []byte) (int,error) { b.mu.Lock(); defer b.mu.Unlock(); return b.text.Write(p) }
func (b *kProgressBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.text.String() }

func TestKProgressAcceptanceFixture(t *testing.T) {
 e := newEnv(t)
 e.credentials()
 s, _, _ := e.session()
 out := &kProgressBuffer{}
 s.Stdout, s.Stderr = out, out
 snapshots := []string{}
 capture := func(next handler) handler { return func(w http.ResponseWriter, r *http.Request) { snapshots = append(snapshots, out.String()); next(w,r) } }
 if os.Getenv("K_PROGRESS_MODE") == "none" {
  e.prov.then(capture(refused()))
 } else {
  writeFile(t, e.root+"/progress-input.txt", "TOOL_RESULT_PRIVATE_7081", 0600)
  e.run("git", "add", "progress-input.txt")
  e.run("git", "commit", "-qm", "add progress input")
  e.approve()
  e.prov.then(
   capture(e.prov.calls("read_file", map[string]any{"path":"progress-input.txt"})),
   capture(e.prov.calls("shell", map[string]any{"command":"printf TOOL_RESULT_PRIVATE_7081; exit 7"})),
   capture(refused()),
  )
 }
 _, _ = Build(context.Background(), s, Input{Deps:e.deps(), Project:e.root, Slug:testSlug})
 if len(e.prov.requests()) != len(snapshots) { t.Fatal("TestKProgressAcceptanceFixture did not capture every request") }
 want := 3
 if os.Getenv("K_PROGRESS_MODE") == "none" { want = 1 }
 if len(snapshots) != want { t.Fatalf("TestKProgressAcceptanceFixture reached %d requests; want %d",len(snapshots),want) }
 raw, err := json.Marshal(map[string]any{"snapshots":snapshots,"output":out.String()})
 if err != nil { t.Fatal(err) }
 if err := os.WriteFile(os.Getenv("K_PROGRESS_RESULT"),raw,0600); err != nil { t.Fatal(err) }
}
`;

function runFixture(mode) {
 const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-progress-'));
 try {
  const source = path.join(dir, 'fixture_test.go');
  const overlay = path.join(dir, 'overlay.json');
  const result = path.join(dir, 'result.json');
  fs.writeFileSync(source, fixture);
  fs.writeFileSync(overlay, JSON.stringify({Replace:{[path.resolve('build/k_progress_acceptance_test.go')]:source}}));
  const command = ['test', '-v', '-count=1', '-overlay', overlay, './build', '-run', '^TestKProgressAcceptanceFixture$', '-timeout', '90s'];
  const p = spawnSync('go', command, {encoding:'utf8', timeout:120000, env:{...process.env,K_PROGRESS_MODE:mode,K_PROGRESS_RESULT:result}});
  assert.equal(p.status, 0, `TestKProgressAcceptanceFixture: go ${command.join(' ')}: ${p.error || ''}\n${p.stdout}\n${p.stderr}`);
  assert.ok(fs.existsSync(result), `TestKProgressAcceptanceFixture did not produce its result: go ${command.join(' ')} status=${p.status}\n${p.stdout}\n${p.stderr}`);
  return JSON.parse(fs.readFileSync(result,'utf8'));
 } finally { fs.rmSync(dir,{recursive:true,force:true}); }
}

function control(kind) {
 const expected = {first:'read_file',second:'shell',success:'ok',failure:'failed',path:'progress-input.txt',secret:'TOOL_RESULT_PRIVATE_7081',baseline:'kogen: building (round 1)'};
 const good = {snapshots:[expected.baseline, expected.baseline+' — read_file progress-input.txt ok',expected.baseline+' — shell failed'],output:'no payload'};
 const none = {snapshots:[expected.baseline],output:expected.baseline};
 if (kind === 'good') {
  observe('latest',good,expected); observe('private',good,expected); observe('none',none,expected);
  console.log('observer controls accepted'); return;
 }
 const badCases = [
  ['latest',{...good,snapshots:[expected.baseline,expected.baseline,good.snapshots[2]]}],
  ['latest',{...good,snapshots:[...good.snapshots.slice(0,2),expected.baseline+' — read_file ok']}],
  ['latest',{...good,snapshots:[expected.baseline,expected.baseline+' — read_file ok',good.snapshots[2]]}],
  ['latest',{...good,snapshots:[...good.snapshots.slice(0,2),expected.baseline+' — shell ok']}],
  ['latest',{...good,snapshots:[...good.snapshots.slice(0,2),expected.baseline+' — shell failed progress-input.txt']}],
  ['private',{...good,output:expected.secret}],
  ['none',{snapshots:[expected.baseline+' changed'],output:''}]
 ];
 for (const [rule,data] of badCases) {
  let rejected = false; try { observe(rule,data,expected); } catch { rejected = true; }
  assert.ok(rejected, `bad ${rule} control accepted`);
 }
 console.log('observer controls rejected'); process.exitCode = 1;
}
if (require.main === module) { control(process.argv[3]); }
else {
 const {When,Then} = require('@cucumber/cucumber');
 When('k-build-progress-latest-tool: I run a fake builder with {string}', function(mode) { this.kProgress = runFixture(mode); });
 Then('k-build-progress-latest-tool: progress reports {string} then {string} with statuses {string} and {string} and path {string}', function(first,second,success,failure,file) { observe('latest',this.kProgress,{first,second,success,failure,path:file}); });
 Then('k-build-progress-latest-tool: console output excludes {string}', function(secret) { observe('private',this.kProgress,{secret}); });
 Then('k-build-progress-latest-tool: progress remains {string} without tool events', function(baseline) { observe('none',this.kProgress,{baseline,secret:'TOOL_RESULT_PRIVATE_7081'}); });
}
