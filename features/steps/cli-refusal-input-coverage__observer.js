'use strict';

const assert = require('node:assert/strict');

function readResult(run, status) {
  assert.equal(run.status, status, `unexpected process exit: ${run.stdout}${run.stderr}`);
  const result = JSON.parse(run.stdout);
  assert.equal(result.exit_code, status, 'JSON exit_code must agree with process status');
  assert.equal(typeof result.branch, 'string');
  assert.equal(typeof result.tree, 'string');
  assert.deepEqual(result.checks, []);
  assert.equal(typeof result.error.message, 'string');
  return result;
}

function observeUsage(run, { modes, binding }) {
  const result = readResult(run, 2);
  assert.equal(result.error.category, 'usage');
  assert.equal(result.error.kind, 'invalid_arguments');
  assert.equal(Object.hasOwn(result, 'continuation'), false, 'invalid input must not return continuation evidence');
  const message = result.error.message;
  if (modes) {
    const tokens = message.match(/[a-zA-Z0-9_-]+/g) || [];
    for (const mode of modes) assert.ok(tokens.includes(mode), `refusal omits supported mode ${mode}: ${message}`);
  }
  if (binding) {
    assert.ok(message.includes('--binding'), `refusal omits --binding: ${message}`);
    assert.ok(message.includes(binding), `refusal omits supplied binding path ${binding}: ${message}`);
  }
}

function observeNativeHold(run, id) {
  const result = readResult(run, 1);
  assert.equal(result.error.kind, 'not_a_git_repo');
  assert.equal(result.continuation?.build_id, id, 'valid preparation arguments must reach native readback');
}

module.exports = { observeUsage, observeNativeHold };
