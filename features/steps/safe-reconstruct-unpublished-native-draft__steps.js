'use strict';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {Given, When, Then} = require('@cucumber/cucumber');
const O = require('./safe-reconstruct-unpublished-native-draft__observer.js');
const P = 'safe-reconstruct-unpublished-native-draft: ';
const root = path.resolve(__dirname, '../..');
const ID = O.ID;
let binary = null;

const sha = buf => crypto.createHash('sha256').update(buf).digest('hex');
function git(cwd, ...args) {
  const r = spawnSync('git', ['-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid', ...args], {cwd, encoding: 'utf8', env: {...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1'}});
  assert.equal(r.status, 0, 'git ' + args.join(' ') + ': ' + r.stderr);
  return r.stdout.trim();
}
function binaryPath() {
  if (binary) return binary;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'srund-bin-'));
  const out = path.join(dir, 'kogen-build');
  const b = spawnSync('go', ['build', '-o', out, './cmd/kogen-build'], {cwd: root, encoding: 'utf8', timeout: 100000});
  assert.equal(b.status, 0, 'go build ./cmd/kogen-build failed (exit ' + b.status + '): ' + b.stdout + b.stderr);
  binary = out;
  return out;
}
function exec(w, args) {
  const r = spawnSync(binaryPath(), args, {encoding: 'utf8', timeout: 60000, env: {PATH: process.env.PATH, HOME: w.home, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1'}});
  let json = null;
  try { json = JSON.parse(r.stdout); } catch (e) { /* reported by the assertions */ }
  return {status: r.status, stdout: r.stdout, stderr: r.stderr, json, args};
}
function rbOf(run) {
  assert.ok(run && run.json && run.json.reconstruction,
    'no reconstruction readback in the standalone JSON; command: kogen-build ' + (run && run.args.join(' ')) + '; exit ' + (run && run.status) + '; stdout: ' + (run && run.stdout) + '; stderr: ' + (run && run.stderr));
  return run.json.reconstruction;
}
function writeEvidence(w) { fs.writeFileSync(w.evidencePath, JSON.stringify(w.ev, null, 2) + '\n'); }
function walk(dir) {
  const out = [];
  for (const e of fs.readdirSync(dir, {withFileTypes: true})) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) out.push(...walk(p)); else if (e.isFile()) out.push(p);
  }
  return out;
}
function snapshot(w) {
  const files = {[w.evidencePath]: fs.existsSync(w.evidencePath) ? sha(fs.readFileSync(w.evidencePath)) : null};
  for (const b of w.ev.prior.builds) files[b.path] = fs.existsSync(b.path) ? sha(fs.readFileSync(b.path)) : null;
  return files;
}
function reconstructArgs(w, archive, extra) {
  return ['reconstruct', '--project', w.checkout, '--evidence', w.evidencePath, '--archive', archive, ...(extra || [])];
}
function doRun(w, archive, extra) {
  w.before = snapshot(w);
  w.last = exec(w, reconstructArgs(w, archive, extra));
  w.runs.push(w.last);
}
function digestTree(dir) {
  return Object.fromEntries(walk(dir).map(p => [path.relative(dir, p), sha(fs.readFileSync(p))]).sort());
}

Given(P + 'the standalone kogen-build binary is built', {timeout: 115000}, function () { binaryPath(); });

Given(P + 'a Problem20 checkout with retained receipts and fully proved evidence', function () {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'srund-'));
  this.dir = dir; this.home = path.join(dir, 'home'); this.checkout = path.join(dir, 'checkout');
  fs.mkdirSync(this.home); fs.mkdirSync(this.checkout);
  git(this.checkout, 'init', '-q');
  fs.writeFileSync(path.join(this.checkout, 'README.md'), 'baseline\n');
  git(this.checkout, 'add', '.');
  git(this.checkout, 'commit', '-q', '-m', 'original baseline');
  this.head = git(this.checkout, 'rev-parse', 'HEAD');
  const receipts = path.join(dir, 'receipts'); fs.mkdirSync(receipts);
  const builds = ID.builds.map(b => {
    const p = path.join(receipts, b.build + '.json');
    const body = JSON.stringify({stand_in_receipt: b.build, outcome: 'failed'}) + '\n';
    fs.writeFileSync(p, body);
    return {id: b.build, receipt_sha256: b.receipt_sha256, candidate_tree: b.candidate_tree, path: p, copy_sha256: sha(body)};
  });
  this.evidencePath = path.join(dir, 'evidence.json');
  this.ev = {
    queue_item: {id: ID.item, version: ID.version}, obligation: 'Problem20', owner: ID.owner,
    request: {document: ID.document, revision: ID.revision, sha256: ID.requestSha},
    current_request: {revision: ID.revision, sha256: ID.requestSha},
    baseline: {head: this.head},
    prior: {attempts: [...ID.attempts], builds, spend: ID.spend},
    effects: {publication: 'none', external: 'none'},
    owners: [], custody: {generation: ID.oldGeneration, revoked: true}, claims: {},
  };
  writeEvidence(this);
  this.archive = path.join(dir, 'archive-1');
  this.expectedOwner = ID.owner;
  this.runs = [];
});

const MUTATIONS = {
  'effects unknown': w => { w.ev.effects.publication = 'unknown'; },
  'effects missing': w => { delete w.ev.effects; },
  'publication observed': w => { w.ev.effects.publication = 'published'; },
  'external effect uncertain': w => { w.ev.effects.external = 'unknown'; },
  'active owner': w => { w.ev.owners = [{name: 'native-build-worker-7', kind: 'process'}]; w.expectedOwner = 'native-build-worker-7'; },
  'owner proof missing': w => { delete w.ev.owners; },
  'custody not revoked': w => { w.ev.custody.revoked = false; },
  'custody proof missing': w => { delete w.ev.custody; },
  'request revision changed': w => { w.ev.current_request.revision = 4; },
  'request hash changed': w => { w.ev.current_request.sha256 = '0'.repeat(64); },
  'baseline moved': w => { git(w.checkout, 'commit', '-q', '--allow-empty', '-m', 'moved baseline'); },
  'receipt file missing': w => { fs.rmSync(w.ev.prior.builds[0].path); },
  'receipt copy changed': w => { fs.appendFileSync(w.ev.prior.builds[1].path, 'tampered\n'); },
  'claimed landing': w => { w.ev.claims.landed = true; },
  'claimed deployment': w => { w.ev.claims.deployed = true; },
  'claimed acceptance': w => { w.ev.claims.accepted = true; },
  'reused build result': w => { w.ev.claims.reused_gates = ['build']; },
  'reused review result': w => { w.ev.claims.reused_gates = ['review']; },
  'reused check result': w => { w.ev.claims.reused_gates = ['checks']; },
  'other queue item': w => { w.ev.queue_item.id = '11111111-2222-3333-4444-555555555555'; },
  'other obligation': w => { w.ev.obligation = 'Problem21'; },
};
Given(P + 'the evidence has this defect: {string}', function (name) {
  assert.ok(MUTATIONS[name], 'unknown fixture defect ' + name);
  MUTATIONS[name](this);
  writeEvidence(this);
});
Given(P + 'the evidence lists one more prior attempt', function () {
  this.ev.prior.attempts.push('f'.repeat(32));
  writeEvidence(this);
});

When(P + 'I run the reconstruction', {timeout: 70000}, function () { doRun(this, this.archive); });
When(P + 'I run the reconstruction stopping after {string}', {timeout: 70000}, function (phase) { doRun(this, this.archive, ['--stop-after', phase]); });
When(P + 'I run the reconstruction again with a different archive destination', {timeout: 70000}, function () {
  this.archive2 = path.join(this.dir, 'archive-2');
  doRun(this, this.archive2);
});
When(P + 'I read back the reconstruction of the queue item', {timeout: 70000}, function () {
  this.before = snapshot(this);
  this.readback = exec(this, ['reconstruction', '--project', this.checkout, '--item', ID.item]);
});
When(P + 'I record the archive contents', function () {
  assert.ok(fs.existsSync(this.archive), 'archive missing after interrupted run: ' + this.archive);
  this.preservedArchive = digestTree(this.archive);
});
When(P + 'I remember the reported attempt and wake', function () { this.first = rbOf(this.last); });

Then(P + 'the run exits 0 and reports a created reconstruction bound to the original request and baseline', function () {
  assert.equal(this.last.status, 0, 'exit ' + this.last.status + '; stdout ' + this.last.stdout + '; stderr ' + this.last.stderr);
  const problems = O.createdProblems(rbOf(this.last), {head: this.head, archive: this.archive});
  assert.deepEqual(problems, [], problems.join('\n'));
});
Then(P + 'the archive holds byte-identical copies of the retained receipts and the evidence file', function () {
  assert.ok(fs.existsSync(this.archive) && fs.statSync(this.archive).isDirectory(), 'archive directory missing: ' + this.archive);
  const have = new Set(walk(this.archive).map(p => sha(fs.readFileSync(p))));
  for (const [p, digest] of Object.entries(this.before)) assert.ok(have.has(digest), 'archive lacks a byte-identical copy of ' + p);
});
Then(P + 'the retained receipts and the evidence file are byte-identical to before the run', function () {
  assert.deepEqual(snapshot(this), this.before);
});
Then(P + 'the run exits 1 and reports a held reconstruction that names its owner and a next action', function () {
  assert.equal(this.last.status, 1, 'exit ' + this.last.status + '; stdout ' + this.last.stdout + '; stderr ' + this.last.stderr);
  assert.equal(this.last.json && this.last.json.exit_code, 1);
  const problems = O.heldProblems(rbOf(this.last), {owner: this.expectedOwner});
  assert.deepEqual(problems, [], problems.join('\n'));
});
Then(P + 'the run exits 1 and reports a held reconstruction without attempt or wake', function () {
  assert.equal(this.last.status, 1, 'exit ' + this.last.status + '; stdout ' + this.last.stdout + '; stderr ' + this.last.stderr);
  assert.equal(this.last.json && this.last.json.exit_code, 1);
  const r = rbOf(this.last);
  assert.equal(r.state, 'held');
  assert.ok(typeof r.owner === 'string' && r.owner, 'owner must be named');
  assert.ok(typeof r.next_action === 'string' && r.next_action, 'next action must be named');
  assert.deepEqual([r.attempts, r.wakes], [[], []]);
});
Then(P + 'no archive was created', function () {
  assert.ok(!fs.existsSync(this.archive), 'archive must not exist for a held run: ' + this.archive);
});
Then(P + 'a readback of the queue item shows no attempt, no wake and no reconstruction', {timeout: 70000}, function () {
  const rb = exec(this, ['reconstruction', '--project', this.checkout, '--item', ID.item]);
  assert.equal(rb.status, 1, 'readback of an unreconstructed item must exit 1; stdout ' + rb.stdout + '; stderr ' + rb.stderr);
  const r = rbOf(rb);
  assert.equal(r.state, 'absent');
  assert.deepEqual([r.attempts, r.wakes], [[], []]);
});
Then(P + 'the run exits 1 and reports an interrupted reconstruction without attempt or wake', function () {
  assert.equal(this.last.status, 1, 'exit ' + this.last.status + '; stdout ' + this.last.stdout + '; stderr ' + this.last.stderr);
  const r = rbOf(this.last);
  assert.equal(r.state, 'interrupted');
  assert.deepEqual([r.attempts, r.wakes], [[], []]);
});
Then(P + 'a readback of the queue item reports the interrupted reconstruction without attempt or wake', {timeout: 70000}, function () {
  const rb = exec(this, ['reconstruction', '--project', this.checkout, '--item', ID.item]);
  const r = rbOf(rb);
  assert.equal(r.state, 'interrupted');
  assert.deepEqual([r.attempts, r.wakes], [[], []]);
});
Then(P + 'the archive was already preserved and is unchanged after the resumed run', function () {
  assert.ok(this.preservedArchive, 'no archive digest was recorded before the resume');
  assert.deepEqual(this.preservedArchive, digestTree(this.archive));
});
Then(P + 'the run reports the same attempt and wake as before, with exactly one of each', function () {
  assert.equal(this.last.status, 0, 'exit ' + this.last.status + '; stdout ' + this.last.stdout);
  const r = rbOf(this.last);
  assert.equal(r.attempts.length, 1); assert.equal(r.wakes.length, 1);
  assert.deepEqual(r.attempts, this.first.attempts);
  assert.deepEqual(r.wakes, this.first.wakes);
  assert.equal(r.state, 'created');
});
Then(P + 'no second archive was created and the first archive is reported', function () {
  assert.ok(!fs.existsSync(this.archive2), 'a replay must not create another archive: ' + this.archive2);
  assert.equal(rbOf(this.last).archive, this.archive);
});
Then(P + 'the run exits 1 and reports a held duplicate reconciliation', function () {
  assert.equal(this.last.status, 1, 'exit ' + this.last.status + '; stdout ' + this.last.stdout + '; stderr ' + this.last.stderr);
  const r = rbOf(this.last);
  assert.equal(r.state, 'held');
  assert.ok(r.owner && r.next_action && r.reason, 'held duplicate must name owner, next action and reason');
  assert.ok(!fs.existsSync(this.archive2), 'a duplicate must not create another archive');
});
Then(P + 'a readback of the queue item still reports the first created attempt and wake and nothing more', {timeout: 70000}, function () {
  const rb = exec(this, ['reconstruction', '--project', this.checkout, '--item', ID.item]);
  assert.equal(rb.status, 0, 'readback exit ' + rb.status + '; stdout ' + rb.stdout);
  const r = rbOf(rb);
  assert.equal(r.state, 'created');
  assert.deepEqual(r.attempts, this.first.attempts);
  assert.deepEqual(r.wakes, this.first.wakes);
});
Then(P + 'the readback exits 0 and equals the report of the creating run', function () {
  assert.equal(this.readback.status, 0, 'readback exit ' + this.readback.status + '; stdout ' + this.readback.stdout + '; stderr ' + this.readback.stderr);
  const problems = O.createdProblems(rbOf(this.readback), {head: this.head, archive: this.archive});
  assert.deepEqual(problems, [], problems.join('\n'));
  assert.deepEqual(rbOf(this.readback), this.first);
});
Then(P + 'the readback reports the three outcomes landing, deployment and acceptance separately as not observed while the queue item stays held', function () {
  const r = rbOf(this.readback);
  assert.deepEqual([r.landing, r.deployment, r.acceptance], ['not_observed', 'not_observed', 'not_observed']);
  assert.equal(r.queue_item.state, 'held');
  for (const g of O.GATES) assert.equal(r.gates[g], 'pending', 'gate ' + g);
});
