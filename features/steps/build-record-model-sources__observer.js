'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const roles = ['shape', 'build', 'review'];
// Discover the source container from its role/value semantics, not an invented JSON key.
function roleMaps(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return [];
  const here = roles.some(r => Object.hasOwn(value, r)) ? [value] : [];
  return here.concat(Object.values(value).flatMap(roleMaps));
}
function validate(actual, expected) {
  assert.equal(actual.error, undefined, `facts publication failed: ${actual.error}`);
  assert.deepEqual(actual.config_after, actual.config_before, 'facts must not change configuration');
  for (const phase of ['initial', 'updated']) {
    const facts = actual[phase];
    assert.ok(facts && typeof facts === 'object', `${phase}: missing published facts`);
    const maps = roleMaps(facts);
    if (expected.missing) {
      for (const map of maps) for (const role of roles) {
        const value = map[role];
        assert.ok(value === undefined || value === null || value === '', `${phase}: invented source for ${role}: ${JSON.stringify(value)}`);
      }
    } else {
      assert.ok(maps.some(map => roles.every(r => map[r] === expected.sources[r])), `${phase}: no recorded role provenance matches ${JSON.stringify(expected.sources)}; got ${JSON.stringify(maps)}`);
    }
    assert.equal(facts.slug, 'fixture');
    assert.equal(facts.state, 'active');
    assert.equal(facts.phase, phase === 'initial' ? 'startup' : 'building');
    assert.equal(typeof facts.checkout_id, 'string');
    assert.equal(typeof facts.run_id, 'string');
    assert.equal(facts.root_run, facts.run_id);
    assert.equal(facts.resumed_from, null);
    assert.equal(facts.verification_pass, null);
    assert.equal(facts.reason, null);
    assert.equal(facts.commit, null);
    assert.deepEqual(facts.blockers, []);
    assert.ok(facts.locations.run_dir && facts.evidence_locations.log);
    assert.ok(facts.owner.pid);
    if (!expected.legacy || phase === 'initial') {
      assert.deepEqual(facts.verification, []);
      assert.deepEqual(facts.reviews, []);
    }
  }
  if (expected.missing) assert.deepEqual(roleMaps(actual.updated), roleMaps(actual.initial), 'unavailable provenance must be represented consistently');
  if (expected.legacy) {
    assert.deepEqual(actual.legacy, expected.legacy, 'old source-free facts must decode without changed values');
    assert.deepEqual(actual.updated.verification, expected.legacy.verification, 'legacy verification summaries changed');
    assert.deepEqual(actual.updated.reviews, expected.legacy.reviews, 'legacy review summaries changed');
  }
}
module.exports = {validate};

if (require.main === module) {
  const fixture = JSON.parse(fs.readFileSync(path.join('.kogen/intents/build-record-model-sources', 'observer-good.json'), 'utf8'));
  if (process.argv[2] === 'good') {
    for (const entry of fixture) validate(entry.actual, entry.expected);
    console.log('accepted');
  } else if (process.argv[2] === 'bad') {
    const mutations = [
      x => { for (const r of roles) delete roleMaps(x.initial)[0][r]; },
      x => { for (const r of roles) roleMaps(x.updated)[0][r] = 'model'; },
      x => { roleMaps(x.updated)[0].review = '.kogen/models.yaml'; },
      x => { x.error = 'metadata required'; },
      x => { x.updated.slug = 'changed'; },
      x => { x.config_after.Models = {}; }
    ];
    let rejected = true;
    for (const mutate of mutations) {
      const entry = structuredClone(fixture[0]); mutate(entry.actual);
      try { validate(entry.actual, entry.expected); rejected = false; } catch (_) {}
    }
    for (const mutate of [
      x => { x.initial.unavailable = {build: 'default', review: 'default'}; },
      x => { x.legacy.phase = 'changed'; },
      x => { x.updated.reviews = []; },
      x => { x.updated.verification = []; }
    ]) {
      const entry = structuredClone(fixture[1]); mutate(entry.actual);
      try { validate(entry.actual, entry.expected); rejected = false; } catch (_) {}
    }
    if (rejected) { console.log('rejected'); process.exitCode = 1; }
    else { console.log('not rejected'); }
  } else { throw new Error('expected good or bad'); }
}
