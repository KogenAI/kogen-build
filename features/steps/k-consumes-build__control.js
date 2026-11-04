'use strict';
// Observer controls of the k-consumes-build acceptance: the consumes observer
// (k-consumes-build__observer.js) and the go test judgement (k-consumes-build__report.js).
// It defines no step and does nothing when the acceptance runner loads it.
//
//   node features/steps/k-consumes-build__control.js good|bad
//
// Both modes copy go.mod, go.sum, build/ and cmd/ of this checkout into a scratch tree whose
// consumes/<producer>/ folders hold the known-good fixtures of k-consumes-build__fixtures.js
// (refs_test.go, one test TestRefs that references every use of kogen-build's code and asserts
// nothing) and whose consumes/README.md maps every use to TestRefs. bad first adds known defects
// to the scratch tree. Each mode runs the observer on the scratch tree and go test -race
// -count=1 -shuffle=on ./consumes/... in it under the hostile environment, and prints the
// observer's output and the go test problems ("go test: ok" without any). Exit 0: no problem;
// 1: problems; 3: the control could not be prepared.
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const suites = [
  ['kogen-coordinator', 'github.com/KogenAI/kogen-coordinator'],
  ['kogen-executor', 'github.com/KogenAI/kogen-executor'],
  ['kogen-loop', 'kogen-loop'],
  ['kogen-review', 'github.com/KogenAI/kogen-review'],
  ['kogen-verification', 'github.com/KogenAI/kogen-verification'],
];

function edit(file, from, to) {
  const text = fs.readFileSync(file, 'utf8');
  if (text.split(from).length !== 2) throw new Error(`${file} does not hold ${JSON.stringify(from)} once`);
  fs.writeFileSync(file, text.replace(from, to));
}

function main(mode) {
  const report = require('./k-consumes-build__report.js');
  const fixtures = require('./k-consumes-build__fixtures.js');
  const root = path.resolve(__dirname, '..', '..');
  if (mode !== 'good' && mode !== 'bad') throw new Error('usage: k-consumes-build__control.js good|bad');
  const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'k-consumes-build-control-'));
  try {
    const tree = path.join(scratch, 'tree');
    fs.mkdirSync(tree);
    for (const f of ['go.mod', 'go.sum']) fs.copyFileSync(path.join(root, f), path.join(tree, f));
    for (const d of ['build', 'cmd']) fs.cpSync(path.join(root, d), path.join(tree, d), { recursive: true });
    const c = path.join(tree, 'consumes');
    for (const [s] of suites) {
      fs.mkdirSync(path.join(c, s), { recursive: true });
      fs.writeFileSync(path.join(c, s, 'refs_test.go'), fixtures.refsTest(s));
    }
    fs.writeFileSync(path.join(c, 'README.md'), fixtures.readme());
    if (mode === 'bad') {
      const readme = path.join(c, 'README.md');
      fs.writeFileSync(path.join(c, 'kogen-coordinator', 'private_test.go'),
        'package consumes\n\nimport "github.com/KogenAI/kogen-build/build"\n\nvar _ = build.Build\n');
      edit(readme, '| `route.Steps` | TestRefs |\n', '| `route.Steps` | refs |\n');
      edit(readme, '| `core.HashHex` | TestRefs |\n', '| `core.Unused` | TestRefs |\n');
      fs.writeFileSync(path.join(c, 'kogen-executor', 'fail_test.go'),
        'package consumes\n\nimport "testing"\n\nfunc TestKnownBad(t *testing.T) { t.Fatal("known bad") }\n');
      edit(path.join(c, 'kogen-loop', 'refs_test.go'), '\t_ = tools.ReadEvent\n', '');
      edit(readme, '| `sys.Fail` | TestRefs |\n', '| `sys.Fail` | TestRefs |\n| `sys.Fail` | TestRefs |\n');
      edit(readme, '| `review.Env` | TestRefs |\n', '| `review.Env` | TestEnv |\n');
      edit(path.join(c, 'kogen-verification', 'refs_test.go'), '\t_ = verify.Cap\n', '');
      fs.appendFileSync(path.join(c, 'kogen-verification', 'refs_test.go'), '\nvar _ = verify.Cap\n');
      fs.writeFileSync(path.join(c, 'kogen-verification', 'helper.go'), 'package consumes\n');
      fs.appendFileSync(readme, '\n### kogen-setup\n\n| Use | Tests |\n| --- | --- |\n| `setup.Run` | TestRefs |\n');
    }
    const observed = report.observe(tree, suites);
    process.stdout.write(observed.output);
    const pkgs = suites.map(([s]) => `github.com/KogenAI/kogen-build/consumes/${s}`);
    const problems = report.testProblems(report.goTest(tree, ['-race', '-count=1', '-shuffle=on', './consumes/...'], true), pkgs);
    process.stdout.write(problems.length ? problems.map((p) => `go test: ${p}\n`).join('') : 'go test: ok\n');
    return observed.status === 0 && problems.length === 0 ? 0 : 1;
  } finally {
    fs.rmSync(scratch, { recursive: true, force: true });
  }
}

if (require.main === module) {
  try {
    process.exitCode = main(process.argv[2]);
  } catch (e) {
    console.log(String(e && e.message ? e.message : e));
    process.exitCode = 3;
  }
}
