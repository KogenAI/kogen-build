'use strict';
// The k-nodefect-scope acceptance harness. runBuild runs one real Build through the build
// package's own test environment (review_test.go.txt, added to package build with `go test
// -overlay` from a temporary folder, so no file of the checkout changes) and returns its record.
// The expectations judge that record. Run directly, it is the observer control:
//   node features/steps/k_nodefect_scope/harness.js <on_demand|parts> <blocked|invalid> <message>
// prints one line and exits 0 when the expectation holds, 1 when it does not, 2 when the
// harness itself fails.
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..', '..');
const testName = 'TestKNodefectScopeAcceptance';
const invalidFeedback = 'a blocking finding that asserts no product defect (such as inspection not completed) is not a verdict on the candidate';

function runBuild(mode, message) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'k-nodefect-scope-'));
  try {
    const overlay = path.join(dir, 'overlay.json');
    const out = path.join(dir, 'record.json');
    fs.writeFileSync(overlay, JSON.stringify({ Replace: {
      [path.join(root, 'build', 'zz_k_nodefect_scope_acceptance_test.go')]: path.join(__dirname, 'review_test.go.txt'),
    } }));
    const args = ['test', '-overlay', overlay, '-count=1', '-v', '-run', '^' + testName + '$', './build'];
    const run = spawnSync('go', args, {
      cwd: root, encoding: 'utf8', maxBuffer: 64 << 20,
      // Go names its working folder by PWD; it must be the folder the overlay path is under.
      env: { ...process.env, PWD: root, KNODEFECT_SCOPE_MODE: mode, KNODEFECT_SCOPE_MESSAGE: message, KNODEFECT_SCOPE_OUT: out },
    });
    const output = `${run.stdout || ''}${run.stderr || ''}${run.error ? String(run.error) : ''}`;
    const ran = run.status === 0 && output.includes(`--- PASS: ${testName} `) && fs.existsSync(out);
    if (!ran) {
      throw new Error(`the harness test ${testName} did not run and pass (go ${args.join(' ')} in ${root}, exit ${run.status}):\n${output}`);
    }
    return JSON.parse(fs.readFileSync(out, 'utf8'));
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

// blocked: the finding is accepted as the review's one changes verdict and the Build stops
// review_blocked with the branch unchanged, without no-defect feedback.
function blockedProblems(r) {
  const problems = [];
  if (r.outcome !== 'review_blocked') problems.push(`outcome ${r.outcome}, want review_blocked`);
  if (r.review_verdicts.join(',') !== 'changes') problems.push(`review verdicts ${r.review_verdicts.join(',')}, want changes`);
  if (!r.head_unchanged) problems.push('the branch HEAD moved');
  if (r.feedback.includes(invalidFeedback)) problems.push('the reviewer was told the finding asserts no product defect');
  return problems;
}

// invalid: every verdict is refused as a blocker that asserts no product defect; the Build
// stops review_invalid, never review_blocked, with the branch unchanged.
function invalidProblems(r) {
  const problems = [];
  if (r.outcome !== 'review_invalid') problems.push(`outcome ${r.outcome}, want review_invalid`);
  if (r.review_verdicts.length === 0 || r.review_verdicts.some((v) => v !== 'invalid')) {
    problems.push(`review verdicts ${r.review_verdicts.join(',')}, want only invalid`);
  }
  if (r.invalid_problems.length === 0 || r.invalid_problems.some((p) => !p.startsWith(invalidFeedback))) {
    problems.push(`invalid problems ${JSON.stringify(r.invalid_problems)}, want each to be the no-defect refusal`);
  }
  if (!r.head_unchanged) problems.push('the branch HEAD moved');
  return problems;
}

const expectations = { blocked: blockedProblems, invalid: invalidProblems };

function describe(r) {
  return `outcome ${r.outcome}; review verdicts ${r.review_verdicts.join(',')}`;
}

module.exports = { runBuild, expectations, describe };

if (require.main === module) {
  const [mode, expectation, message] = process.argv.slice(2);
  if (!['on_demand', 'parts'].includes(mode) || !expectations[expectation] || !message) {
    process.stdout.write('usage: harness.js <on_demand|parts> <blocked|invalid> <message>\n');
    process.exit(2);
  }
  let record;
  try {
    record = runBuild(mode, message);
  } catch (err) {
    process.stdout.write(`${err.message}\n`);
    process.exit(2);
  }
  const problems = expectations[expectation](record);
  if (problems.length > 0) {
    process.stdout.write(`FAILED: ${mode} ${expectation}: ${problems.join('; ')}\n`);
    process.exit(1);
  }
  process.stdout.write(`ok: ${mode} ${expectation}: ${describe(record)}\n`);
}
