'use strict';
// Acceptance of k-review-reopen-after-external-repair. The step writes the Go harness of
// k-review-reopen-after-external-repair__harness.js to a temporary file, overlays it into ./build as
// build/k_review_reopen_acceptance_test.go, builds the build package's test binary once
// (go test -c -vet=off -overlay) and runs the harness test TestKReviewReopenAcceptance once per mode
// (KRR_MODE, with KRR_SIMULATE=0). The harness makes a Build whose independent review ended with a
// blocking verdict (or reads a version 3 checkpoint of one), changes the checkout outside the Build,
// resumes the Build against the package's scripted provider and prints one observation line,
// "KRR <json>". This file compares that observation, as a JSON value (object keys in any order,
// arrays in order), with the one the feature expects.
//
// Run directly as `node features/steps/k-review-reopen-after-external-repair__steps.js --control good|bad`,
// it qualifies its own comparison:
// - good: every mode of the feature is run with KRR_SIMULATE=1, where the harness also rewrites the
//   checkpoint's recorded candidate to the repaired tree before the resume, so the reopening the
//   review already does is observed through the same harness code; every observation must be accepted.
// - bad: observations recorded on the unchanged tree and observations of plausible wrong
//   implementations derived from the expectations; every one must be rejected.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {HARNESS} = require('./k-review-reopen-after-external-repair__harness.js');

const TEST = 'TestKReviewReopenAcceptance';
const TARGET = 'build/k_review_reopen_acceptance_test.go';
const PREFIX = 'KRR ';
const FEATURE = '.kogen/intents/k-review-reopen-after-external-repair/acceptance.feature';

const owned = [];
process.on('exit', () => { for (const d of owned) fs.rmSync(d, {recursive: true, force: true}); });

function env(extra) {
  const out = {};
  for (const [k, v] of Object.entries(process.env)) {
    if (!k.startsWith('KOGEN_ACCEPTANCE_') && !k.startsWith('KRR_')) out[k] = v;
  }
  return {...out, ...extra};
}

let built, buildFailure;
function binary() {
  if (built) return built;
  if (buildFailure) throw buildFailure; // one build per run: later scenarios fail fast with its failure
  const root = fs.realpathSync(process.cwd());
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'krr-'));
  owned.push(dir);
  const harness = path.join(dir, 'harness_test.go');
  fs.writeFileSync(harness, HARNESS);
  const overlay = path.join(dir, 'overlay.json');
  fs.writeFileSync(overlay, JSON.stringify({Replace: {[path.join(root, TARGET)]: harness}}));
  const bin = path.join(dir, 'build.test');
  const args = ['test', '-c', '-vet=off', '-o', bin, '-overlay', overlay, './build'];
  const r = spawnSync('go', args, {cwd: root, env: env({PWD: root}), encoding: 'utf8', timeout: 900000, maxBuffer: 16 * 1024 * 1024});
  try {
    assert.equal(r.status, 0, `building the harness of ${TEST}: go ${args.join(' ')} exited ${r.status}${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error.message : ''}:\n${r.stdout}${r.stderr}`);
  } catch (e) {
    buildFailure = e;
    throw e;
  }
  built = bin;
  return bin;
}

// observe runs the harness in one mode: the run must pass the harness test exactly once and print
// exactly one observation line of that mode.
function observe(mode, simulate) {
  const bin = binary();
  const root = fs.realpathSync(process.cwd());
  const args = ['-test.run', '^' + TEST + '$', '-test.count=1', '-test.v', '-test.timeout=300s'];
  const flag = simulate ? '1' : '0';
  const r = spawnSync(bin, args, {cwd: path.join(root, 'build'), env: env({KRR_MODE: mode, KRR_SIMULATE: flag}), encoding: 'utf8', timeout: 330000, maxBuffer: 64 * 1024 * 1024});
  const stdout = r.stdout || '', stderr = r.stderr || '';
  const shown = `${TEST} (KRR_MODE=${mode} KRR_SIMULATE=${flag} build.test ${args.join(' ')}, built with go test -c -vet=off -overlay ./build) exited ${r.status}` +
    `${r.signal ? ' (' + r.signal + ')' : ''}${r.error ? ' ' + r.error.message : ''}:\n${stdout.slice(-20000)}${stderr.slice(-20000)}`;
  assert.equal(r.error, undefined, shown);
  assert.equal(r.status, 0, shown);
  assert.equal(stdout.split('\n').filter(l => l.startsWith('--- PASS: ' + TEST + ' ')).length, 1, `${TEST} did not pass exactly once: ${shown}`);
  const lines = stdout.split('\n').filter(l => l.startsWith(PREFIX));
  assert.equal(lines.length, 1, `${TEST} printed ${lines.length} observation lines, want 1: ${shown}`);
  const got = JSON.parse(lines[0].slice(PREFIX.length));
  assert.equal(got.mode, mode, `${TEST} observed mode ${got.mode}, want ${mode}: ${shown}`);
  return got;
}

function compare(mode, got, want) {
  assert.deepStrictEqual(got, want, `mode ${mode}: observed\n${JSON.stringify(got, null, 2)}\nwant\n${JSON.stringify(want, null, 2)}`);
}

// expectations are the feature's expected observations by mode.
function expectations() {
  const text = fs.readFileSync(FEATURE, 'utf8');
  const re = /the reopen harness in mode "([^"]+)" observes:\s*\n\s*"""\n([\s\S]*?)\n\s*"""/g;
  const out = {};
  let m;
  while ((m = re.exec(text)) !== null) out[m[1]] = JSON.parse(m[2]);
  return out;
}

const NEXT = '; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build';
const DRIFT = 'Build ID: candidate contents/modes or unrelated paths changed: app.txt' + NEXT;

// refusedBefore is an observation of a resume refused before any run or request.
function refusedBefore(o, refusal) {
  for (const k of ['same_reviewer', 'finding_returned', 'repair_delivered']) delete o[k];
  Object.assign(o, {exit: 1, stop: 'candidate/resume_refused', refusal, new_runs: 0, requests: 0, at_decision: null, outcome: null,
    child_stage: null, child_verdict: null, commits: 0, committed_app: null, approval_kept: true});
  if ('verified' in o) o.verified = ['baseline', 'candidate'];
}

// landedWithoutAsking is an observation of a commit made with no reopened reviewer request.
function landedWithoutAsking(o, app) {
  for (const k of ['same_reviewer', 'finding_returned', 'repair_delivered']) delete o[k];
  Object.assign(o, {exit: 0, stop: '', requests: 0, at_decision: null, outcome: 'landed', child_stage: 'landed', child_verdict: null,
    commits: 1, committed_app: app, approval_kept: false});
}

// BAD are the known-bad observations: the unchanged tree's refusals of an external repair, and
// plausible wrong implementations, each a change of the mode's expected observation.
const BAD = [
  {mode: 'current-fixed', why: 'unchanged tree: the repaired candidate is refused as drift', bad: o => refusedBefore(o, DRIFT)},
  {mode: 'current-unfixed', why: 'unchanged tree: the repaired candidate is refused as drift', bad: o => refusedBefore(o, DRIFT)},
  {mode: 'current-stale-approval', why: 'unchanged tree: the repaired candidate is refused as drift', bad: o => refusedBefore(o, DRIFT)},
  {mode: 'previous-fixed', why: 'unchanged tree: the repaired version 3 candidate is refused as drift', bad: o => refusedBefore(o, DRIFT)},
  {mode: 'previous-unfixed', why: 'unchanged tree: the repaired version 3 candidate is refused as drift', bad: o => refusedBefore(o, DRIFT)},
  {mode: 'previous-fixed', why: 'version 3 refused as incompatible', bad: o => refusedBefore(o, 'Build ID: incompatible checkpoint version 3' + NEXT)},
  {mode: 'current-fixed', why: 'reopened without a fresh pass of the repaired tree', bad: o => {
    o.at_decision = {pass_green: true, pass_tree_is_repaired: false, verified: ['baseline', 'candidate']};
    o.verified = ['baseline', 'candidate'];
  }},
  {mode: 'previous-fixed', why: 'version 3 reopened without a fresh pass of the repaired tree', bad: o => { o.at_decision.pass_tree_is_repaired = false; }},
  {mode: 'current-fixed', why: 'the saved candidate is restored over the repair before the review', bad: o => {
    Object.assign(o, {app_txt: 'candidate\n', committed_app: 'candidate', repair_delivered: false, verified: ['baseline', 'candidate']});
    o.at_decision.verified = ['baseline', 'candidate'];
  }},
  {mode: 'current-fixed', why: 'approved and committed without asking the reviewer', bad: o => landedWithoutAsking(o, 'repaired')},
  {mode: 'current-fixed', why: 'a fresh review instead of the same reviewer and its finding', bad: o => { o.same_reviewer = false; o.finding_returned = false; }},
  {mode: 'previous-unfixed', why: 'a fresh review instead of the same version 3 reviewer', bad: o => { o.same_reviewer = false; }},
  {mode: 'current-unfixed', why: 'the maintained finding is committed anyway', bad: o => {
    Object.assign(o, {exit: 0, stop: '', outcome: 'landed', child_stage: 'landed', child_verdict: null, commits: 1, committed_app: 'still wrong', approval_kept: false});
  }},
  {mode: 'previous-fixed', why: 'the saved terminal changes verdict is replayed before verification', bad: o => {
    for (const k of ['same_reviewer', 'finding_returned', 'repair_delivered']) delete o[k];
    Object.assign(o, {exit: 1, stop: 'candidate/review_blocked', requests: 0, at_decision: null, outcome: 'review_blocked',
      child_stage: 'review_verify', child_verdict: 'changes', commits: 0, committed_app: null, approval_kept: true});
  }},
  {mode: 'current-stale-approval', why: 'the stale approval is committed without a fresh pass or request', bad: o => {
    landedWithoutAsking(o, 'repaired');
    o.verified = ['baseline', 'candidate'];
  }},
  {mode: 'current-stale-approval', why: 'the stale approval clears the finding at the reopened request', bad: o => {
    Object.assign(o, {exit: 0, stop: '', outcome: 'landed', child_stage: 'landed', child_verdict: null, commits: 1, committed_app: 'repaired', approval_kept: false});
  }},
  {mode: 'current-custody', why: 'a protected-file change is accepted as part of the repair', bad: o => {
    Object.assign(o.protected, {refusal: '', stop: 'candidate/review_blocked', new_runs: 1, requests: 1});
  }},
  {mode: 'current-custody', why: 'a staged index change is accepted as part of the repair', bad: o => {
    Object.assign(o.index, {refusal: '', stop: 'candidate/review_blocked', new_runs: 1, requests: 1});
  }},
  {mode: 'previous-custody', why: 'a version 3 protected-file change is accepted as part of the repair', bad: o => {
    Object.assign(o, {refusal: '', stop: 'candidate/review_blocked', new_runs: 1, requests: 1});
  }},
  {mode: 'unsupported-version', why: 'an unreadable checkpoint version is resumed', bad: o => {
    Object.assign(o, {refusal: '', stop: 'candidate/review_blocked', new_runs: 1, requests: 1});
  }},
  {mode: 'unsupported-version', why: 'the refusal does not name the version', bad: o => { o.refusal = 'Build ID: malformed required checkpoint fields' + NEXT; }},
];

function control(kind) {
  const want = expectations();
  let accepted = 0, rejected = 0;
  const judge = (mode, observation, why) => {
    try {
      compare(mode, observation, want[mode]);
      accepted++;
      if (kind === 'bad') console.error(`known-bad observation accepted: ${mode}: ${why}`);
    } catch (e) {
      if (!(e instanceof assert.AssertionError)) throw e;
      rejected++;
      if (kind === 'good') console.error(`known-good observation rejected: ${mode}: ${e.message}`);
    }
  };
  if (kind === 'good') {
    const modes = Object.keys(want);
    assert.equal(modes.length, 8, `control good: the feature has ${modes.length} expected modes, want 8`);
    for (const mode of modes) {
      let got;
      try {
        got = observe(mode, true);
      } catch (e) {
        if (!(e instanceof assert.AssertionError)) throw e;
        rejected++;
        console.error(`known-good run failed: ${mode}: ${e.message}`);
        continue;
      }
      judge(mode, got, 'simulated');
    }
    return rejected === 0;
  }
  for (const b of BAD) {
    assert.ok(want[b.mode], `control bad: the feature has no expectation for mode ${b.mode}`);
    const o = JSON.parse(JSON.stringify(want[b.mode]));
    b.bad(o);
    judge(b.mode, o, b.why);
  }
  return accepted > 0;
}

if (require.main === module) {
  const kind = process.argv[2] === '--control' ? process.argv[3] : '';
  if (kind !== 'good' && kind !== 'bad') {
    console.log('usage: node features/steps/k-review-reopen-after-external-repair__steps.js --control good|bad');
    process.exit(2);
  }
  let ok;
  try {
    ok = control(kind);
  } catch (e) {
    console.log('control error: ' + e.message);
    process.exit(2);
  }
  console.log(ok ? 'observer accepted' : 'observer rejected');
  process.exitCode = ok ? 0 : 1;
} else {
  const {Then} = require('@cucumber/cucumber');
  Then('k-review-reopen-after-external-repair: the reopen harness in mode {string} observes:', {timeout: 1800000}, function (mode, doc) {
    compare(mode, observe(mode, false), JSON.parse(doc));
  });
}
