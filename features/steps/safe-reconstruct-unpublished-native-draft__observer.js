'use strict';
// Shared fixture identities and readback observer for the Problem20 reconstruction acceptance.
const ID = {
  item: 'bb13338f-045e-4853-8cc4-90ec80dc8b4f',
  version: 18,
  document: 'queue-request-bb13338f-045e-4853-8cc4-90ec80dc8b4f',
  revision: 3,
  requestSha: 'ee87cf09b602ba3eec64112ecf8e85bbf1d539864308a2ab542ac467a9202cb4',
  builds: [
    {build: 'b97d96270b9f4801aa09a7ba00e64f84', receipt_sha256: '73668cf73f9771253344e497d8fb945c6f7f4b9b2839f19d5983ae728768a65e', candidate_tree: '1fdbec543200f5030b1f21f410cd306c2c1c0c69'},
    {build: 'c3b576c985bb4299b1b443666eed172a', receipt_sha256: 'e8f22fb502c149de55c8cd08d0d90962b57e6e33849a993a90099df87d631fc9', candidate_tree: 'ecd5a130597b8316d37a3d5a35742e7e59231c7a'},
  ],
  attempts: ['577e7c601aab41a39eb0820fc5fee5fb', '985d693a96de458189226e9a50ea91df'],
  spend: {note: 'stand-in prior spend record', units: 42},
  owner: 'stack-menu-fleet',
  oldGeneration: 'gen-1',
};
const GATES = ['admission', 'shape', 'build', 'checks', 'review'];
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);

function priorProblems(rb, out) {
  const prior = rb.prior || {};
  const receipts = (prior.receipts || []).map(r => ({build: r.build, receipt_sha256: r.receipt_sha256, candidate_tree: r.candidate_tree}));
  if (!same(receipts, ID.builds)) out.push('prior receipts, receipt hashes or candidate trees are not preserved: ' + JSON.stringify(receipts));
  if (!same(prior.attempts, ID.attempts)) out.push('prior attempts are not preserved: ' + JSON.stringify(prior.attempts));
  if (!same(prior.spend, ID.spend)) out.push('prior spend is not preserved unchanged: ' + JSON.stringify(prior.spend));
}
function commonProblems(rb, out) {
  const q = rb.queue_item || {};
  if (!same({id: q.id, version: q.version, state: q.state}, {id: ID.item, version: ID.version, state: 'held'})) out.push('queue item is not the original held item: ' + JSON.stringify(q));
  if (rb.obligation !== 'Problem20') out.push('obligation is not Problem20: ' + rb.obligation);
  const r = rb.request || {};
  if (!same({document: r.document, revision: r.revision, sha256: r.sha256}, {document: ID.document, revision: ID.revision, sha256: ID.requestSha})) out.push('original request identity is not retained: ' + JSON.stringify(r));
  priorProblems(rb, out);
  for (const k of ['landing', 'deployment', 'acceptance']) if (rb[k] !== 'not_observed') out.push(k + ' is not reported separately as not_observed: ' + JSON.stringify(rb[k]));
  if (typeof rb.owner !== 'string' || !rb.owner) out.push('owner is not named');
  if (typeof rb.next_action !== 'string' || !rb.next_action) out.push('next action is not named');
  if (!Array.isArray(rb.attempts) || !Array.isArray(rb.wakes)) out.push('attempts and wakes must be arrays');
}
function createdProblems(rb, ctx) {
  const out = [];
  if (!rb) return ['no reconstruction readback'];
  if (rb.state !== 'created') out.push('state is not created: ' + rb.state);
  commonProblems(rb, out);
  if (!same(rb.baseline && rb.baseline.head, ctx.head)) out.push('original baseline head is not retained');
  if (rb.archive !== ctx.archive) out.push('archive is not reported: ' + rb.archive);
  const attempts = rb.attempts || [], wakes = rb.wakes || [];
  if (attempts.length !== 1) out.push('expected exactly one fresh attempt, got ' + attempts.length);
  if (wakes.length !== 1) out.push('expected exactly one wake, got ' + wakes.length);
  const a = attempts[0] || {};
  const old = [ID.oldGeneration, ID.requestSha, ...ID.attempts, ...ID.builds.flatMap(b => [b.build, b.receipt_sha256, b.candidate_tree])];
  if (typeof a.id !== 'string' || !a.id || old.includes(a.id)) out.push('attempt id is not fresh: ' + a.id);
  if (typeof a.generation !== 'string' || !a.generation || old.includes(a.generation)) out.push('generation is not fresh: ' + a.generation);
  if (!same({r: a.request_revision, s: a.request_sha256, b: a.baseline_head}, {r: ID.revision, s: ID.requestSha, b: ctx.head})) out.push('attempt is not bound to request revision 3 and the original baseline: ' + JSON.stringify(a));
  if (!same(rb.generation, {old: ID.oldGeneration, new: a.generation})) out.push('generation readback does not name the fenced and the new generation: ' + JSON.stringify(rb.generation));
  const w = wakes[0] || {};
  if (typeof w.id !== 'string' || !w.id || w.attempt !== a.id) out.push('wake is not bound to the fresh attempt: ' + JSON.stringify(w));
  for (const g of GATES) if ((rb.gates || {})[g] !== 'pending') out.push('gate ' + g + ' is not pending for the fresh attempt: ' + JSON.stringify((rb.gates || {})[g]));
  return out;
}
function heldProblems(rb, ctx) {
  const out = [];
  if (!rb) return ['no reconstruction readback'];
  if (rb.state !== 'held') out.push('state is not held: ' + rb.state);
  commonProblems(rb, out);
  if (rb.owner !== ctx.owner) out.push('owner is ' + rb.owner + ', expected ' + ctx.owner);
  if (typeof rb.reason !== 'string' || !rb.reason) out.push('hold reason is not stated');
  if ((rb.attempts || []).length !== 0 || (rb.wakes || []).length !== 0) out.push('a held run must not report an attempt or wake');
  return out;
}
module.exports = {ID, GATES, createdProblems, heldProblems};

if (require.main === module) {
  const ctx = {head: '0123456789abcdef0123456789abcdef01234567', archive: '/archive'};
  const good = {
    state: 'created', queue_item: {id: ID.item, version: ID.version, state: 'held'}, obligation: 'Problem20',
    request: {document: ID.document, revision: ID.revision, sha256: ID.requestSha}, baseline: {head: ctx.head},
    prior: {receipts: ID.builds.map(b => ({...b, path: '/x'})), attempts: ID.attempts, spend: ID.spend},
    archive: ctx.archive, generation: {old: ID.oldGeneration, new: 'gen-2'},
    attempts: [{id: 'f'.repeat(32), generation: 'gen-2', request_revision: 3, request_sha256: ID.requestSha, baseline_head: ctx.head}],
    wakes: [{id: 'e'.repeat(32), attempt: 'f'.repeat(32)}],
    gates: Object.fromEntries(GATES.map(g => [g, 'pending'])), landing: 'not_observed', deployment: 'not_observed', acceptance: 'not_observed',
    owner: ID.owner, next_action: 'run the ordinary pipeline',
  };
  const bad = {...good, prior: {...good.prior, receipts: good.prior.receipts.slice(0, 1)}, landing: 'landed'};
  const problems = createdProblems(process.argv[2] === 'bad' ? bad : good, ctx);
  console.log(problems.length === 0 ? 'observer accepted' : 'observer rejected');
  process.exit(problems.length === 0 ? 0 : 1);
}
