// Shared by the steps of facts-history-cap-regression and run as its observer: runs the Go tests
// of build/facts_test.go (or of that file with a patch applied) against build/facts.go or a copy of
// it with one defect, through `go test -overlay`; no checkout file changes. Usage:
//   node control.js <patch file or -> <unchanged|defect name>
// Exit 0 means the defect is rejected, 1 that it was not, 2 that the control itself is broken.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const A = 'list = list[max(0, len(list)-20):]';
const B = 'f.record[key] = entries[max(0, len(entries)-20):]';
const C = 'r[key] = entries[max(0, len(entries)-20):]';
const D = 'r["evidence_truncated"] = truncated || len(entries) > 20';

function keyed(cond, kind) {
  return (src) => {
    if (kind === 'oldest') {
      src = src.replace(A, `if ${cond} { list = list[:min(20, len(list))] } else { ${A} }`);
      return src.replace(B, `if ${cond} { f.record[key] = entries[:min(20, len(entries))] } else { ${B} }`);
    }
    src = src.replace(A, `if !(${cond}) { ${A} }`);
    return src.replace(B, `if ${cond} { f.record[key] = entries } else { ${B} }`);
  };
}

const DEFECTS = {
  'verification-oldest': { snippets: [A, B], apply: keyed('key == "verification"', 'oldest') },
  'verification-unbounded': { snippets: [A, B], apply: keyed('key == "verification"', 'unbounded') },
  'review-oldest': { snippets: [A, B], apply: keyed('key == "reviews"', 'oldest') },
  'review-unbounded': { snippets: [A, B], apply: keyed('key == "reviews"', 'unbounded') },
  'shared-limit': {
    snippets: [A, B],
    apply: (src) => src.replace(A, '').replace(B, `if key == "reviews" {\n entries = entries[:min(len(entries), max(0, 20-len(f.record["verification"].([]any))))]\n }\n ${B}`),
  },
  'evidence-silent': { snippets: [D], apply: (src) => src.replace(D, 'r["evidence_truncated"] = truncated') },
  'evidence-oldest': { snippets: [C], apply: (src) => src.replace(C, 'r[key] = entries[:min(20, len(entries))]') },
  'evidence-unbounded': { snippets: [C], apply: (src) => src.replace(C, 'r[key] = entries') },
};

function root() {
  return process.cwd();
}

// run runs the facts tests with an optional replacement test file and an optional defect.
function run(testFile, defect) {
  const r = root();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fhcr-'));
  try {
    const replace = {};
    const testPath = path.join(r, 'build', 'facts_test.go');
    let testSrc = fs.readFileSync(testPath, 'utf8');
    if (testFile) {
      testSrc = fs.readFileSync(testFile, 'utf8');
      replace[testPath] = testFile;
    }
    const names = [...testSrc.matchAll(/^func (Test\w+)\(/gm)].map((m) => m[1]);
    if (defect !== 'unchanged') {
      const d = DEFECTS[defect];
      if (!d) return { error: `unknown defect ${defect}` };
      const src = fs.readFileSync(path.join(r, 'build', 'facts.go'), 'utf8');
      for (const s of d.snippets) {
        if (!src.includes(s)) return { error: `build/facts.go no longer contains \`${s}\`, which the ${defect} copy replaces` };
      }
      const copy = path.join(dir, 'facts.go');
      fs.writeFileSync(copy, d.apply(src));
      replace[path.join(r, 'build', 'facts.go')] = copy;
    }
    const overlay = path.join(dir, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({ Replace: replace }));
    const args = ['test', '-overlay', overlay, '-count=1', '-v', '-run', `^(${names.join('|')})$`, './build'];
    const p = spawnSync('go', args, { cwd: r, encoding: 'utf8', timeout: 100000, maxBuffer: 1 << 26 });
    const out = (p.stdout || '') + (p.stderr || '');
    const results = {};
    for (const m of out.matchAll(/^--- (PASS|FAIL|SKIP): (\w+)/gm)) results[m[2]] = m[1];
    const command = `go ${args.join(' ')}`;
    return { names, results, status: p.status, out: out.length > 3000 ? out.slice(-3000) : out, command };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

// runPatched applies a patch to a temporary copy of build/facts_test.go and runs with it.
function runPatched(patch, defect) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'fhcr-patch-'));
  try {
    fs.mkdirSync(path.join(tmp, 'build'));
    fs.copyFileSync(path.join(root(), 'build', 'facts_test.go'), path.join(tmp, 'build', 'facts_test.go'));
    const p = spawnSync('git', ['apply', path.resolve(patch)], { cwd: tmp, encoding: 'utf8' });
    if (p.status !== 0) return { error: `git apply ${patch} failed: ${p.stderr}` };
    const moved = path.join(tmp, 'facts_test.go.txt');
    fs.renameSync(path.join(tmp, 'build', 'facts_test.go'), moved);
    return run(moved, defect);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

// unchangedProblem describes why a run of the unchanged build/facts.go is not a full pass, or returns ''.
function unchangedProblem(res) {
  if (res.error) return res.error;
  if (res.names.length === 0) return 'build/facts_test.go declares no Test function';
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
  if (!rejects(res)) return `no facts test failed against the ${defect} copy of build/facts.go: ${res.command}, exit ${res.status}, results ${JSON.stringify(res.results)}\n${res.out}`;
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
