// Shared by the steps of host-memory-failure-regression and run as its observer: runs the Go tests
// that build/env_failure_park_test.go declares beyond the ones it already had (or that file with a
// patch applied) against build/envfailure.go or a copy of it with one defect, through
// `go test -overlay`; no checkout file changes. Usage:
//   node control.js <patch file or -> <unchanged|defect name>
// Exit 0 means the defect is rejected, 1 that it was not, 2 that the control itself is broken.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

// The Test functions build/env_failure_park_test.go declared before this Intent.
const EXISTING = [
  'TestRepeatedHostTimeoutStopsTheNativeBuildAfterTheSecondPass',
  'TestHostTimeoutsThatAreNotConsecutiveDoNotStopTheBuild',
  'TestRepeatedHostTimeoutInTwoGatesOfAStageStopsTheBuild',
  'TestHostStopSameKindDifferentLinesStopsQuotingTheSecond',
  'TestHostStopDifferentKindInThePreviousPassDoesNotStop',
  'TestHostStopOrdinaryFailureInThePreviousPassDoesNotStop',
  'TestHostStopMemoryIsPerCheckName',
  'TestHostStopIdenticalLinesStillStop',
  'TestNoEditRoundAfterATimeoutRedStopsAsHostOrBaseOnTheRepeat',
  'TestNoEditRoundAfterAStageGateTimeoutStopsAsHostOrBaseOnTheRepeat',
];

const ALLOCATE = 'strings.Contains(line, "cannot allocate memory"),';
const OOM = 'strings.Contains(line, "out of memory"),';
const KILLED = 'strings.Contains(line, "Out of memory: Killed process"):';
const SAME = 'prev.kind == kind';
const QUOTE = 'stop, failure = &pass.Checks[i], text';

const DEFECTS = {
  'memory-allocate-unrecognised': { snippets: [ALLOCATE], apply: (src) => src.replace(ALLOCATE, 'false,') },
  'memory-out-of-memory-unrecognised': { snippets: [OOM], apply: (src) => src.replace(OOM, 'false,') },
  'memory-killed-process-unrecognised': { snippets: [KILLED], apply: (src) => src.replace(KILLED, 'false:') },
  'full-lines-compared': { snippets: [SAME], apply: (src) => src.replace(SAME, 'prev.line == text') },
  'first-line-quoted': { snippets: [QUOTE], apply: (src) => src.replace(QUOTE, 'stop, failure = &pass.Checks[i], prev.line') },
  'any-environment-failure-stops': { snippets: [SAME], apply: (src) => src.replace(SAME, 'prev.kind != ""') },
};

function root() {
  return fs.realpathSync(process.cwd());
}

// run runs the new Go tests of the test file (an optional replacement) with an optional defect.
function run(testFile, defect) {
  const r = root();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hmfr-'));
  try {
    const replace = {};
    const testPath = path.join(r, 'build', 'env_failure_park_test.go');
    let testSrc = fs.readFileSync(testPath, 'utf8');
    if (testFile) {
      testSrc = fs.readFileSync(testFile, 'utf8');
      replace[testPath] = testFile;
    }
    const names = [...testSrc.matchAll(/^func (Test\w+)\(/gm)].map((m) => m[1]).filter((n) => !EXISTING.includes(n));
    if (names.length === 0) return { error: 'build/env_failure_park_test.go declares no Test function besides its ten existing ones', names };
    if (defect !== 'unchanged') {
      const d = DEFECTS[defect];
      if (!d) return { error: `unknown defect ${defect}`, names };
      const src = fs.readFileSync(path.join(r, 'build', 'envfailure.go'), 'utf8');
      for (const s of d.snippets) {
        if (!src.includes(s)) return { error: `build/envfailure.go no longer contains \`${s}\`, which the ${defect} copy replaces`, names };
      }
      const copy = path.join(dir, 'envfailure.go');
      fs.writeFileSync(copy, d.apply(src));
      replace[path.join(r, 'build', 'envfailure.go')] = copy;
    }
    const overlay = path.join(dir, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({ Replace: replace }));
    const args = ['test', '-overlay', overlay, '-count=1', '-v', '-run', `^(${names.join('|')})$`, './build'];
    const p = spawnSync('go', args, { cwd: r, env: { ...process.env, PWD: r }, encoding: 'utf8', timeout: 100000, maxBuffer: 1 << 26 });
    const out = (p.stdout || '') + (p.stderr || '');
    const results = {};
    for (const m of out.matchAll(/^--- (PASS|FAIL|SKIP): (\w+)/gm)) results[m[2]] = m[1];
    const command = `go ${args.join(' ')}`;
    return { names, results, status: p.status, out: out.length > 3000 ? out.slice(-3000) : out, command };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

// runPatched applies a patch to a temporary copy of the test file and runs with it.
function runPatched(patch, defect) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hmfr-patch-'));
  try {
    fs.mkdirSync(path.join(tmp, 'build'));
    fs.copyFileSync(path.join(root(), 'build', 'env_failure_park_test.go'), path.join(tmp, 'build', 'env_failure_park_test.go'));
    const p = spawnSync('git', ['apply', path.resolve(patch)], { cwd: tmp, encoding: 'utf8' });
    if (p.status !== 0) return { error: `git apply ${patch} failed: ${p.stderr}` };
    const moved = path.join(tmp, 'env_failure_park_test.go.txt');
    fs.renameSync(path.join(tmp, 'build', 'env_failure_park_test.go'), moved);
    return run(moved, defect);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

// unchangedProblem describes why a run of the unchanged build/envfailure.go is not a full pass, or returns ''.
function unchangedProblem(res) {
  if (res.error) return res.error;
  const bad = res.names.filter((n) => res.results[n] !== 'PASS');
  if (res.status !== 0 || bad.length > 0) {
    return `${res.command} exit ${res.status}; not passed: ${bad.map((n) => `${n}: ${res.results[n] || 'did not run'}`).join(', ')}\n${res.out}`;
  }
  return '';
}

function rejects(res) {
  return !res.error && res.status !== 0 && Object.values(res.results).includes('FAIL');
}

// rejectedProblem describes why a run against a defective copy did not reject it, or returns ''.
function rejectedProblem(res, defect) {
  if (res.error) return res.error;
  if (!rejects(res)) return `no new test of build/env_failure_park_test.go (${res.names.join(', ')}) failed against the ${defect} copy of build/envfailure.go: ${res.command}, exit ${res.status}, results ${JSON.stringify(res.results)}\n${res.out}`;
  return '';
}

module.exports = { DEFECTS, run, unchangedProblem, rejectedProblem };

if (require.main === module) {
  const [patch, defect] = process.argv.slice(2);
  const res = patch === '-' ? run(null, defect) : runPatched(patch, defect);
  if (res.error) {
    process.stdout.write(`error: ${res.error}\n`);
    process.exit(2);
  }
  const ok = rejectedProblem(res, defect) === '';
  process.stdout.write(ok ? `rejected: ${defect}\n` : `not rejected: ${defect}\n`);
  process.exit(ok ? 0 : 1);
}
