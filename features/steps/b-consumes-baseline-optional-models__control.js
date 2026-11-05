// Observer of b-consumes-baseline-optional-models: runs named Go tests of one package of this
// module with the real go tool, from the checkout's real path, against a chosen
// kogen-coordinator, and judges the go test -json events. The coordinator is either the one
// go.mod pins (`pinned`, whose project.Baseline always lists .kogen/models.yaml right after
// .kogen/project.yaml) or a temporary copy of that module whose project.Baseline is changed,
// reached through a temporary go.mod (-modfile) with a replace directive, so no checkout file
// changes:
//   absent     project.Baseline leaves .kogen/models.yaml out when the project has no such file
//              (tracked, or untracked and not ignored), as kogen-coordinator's
//              coord-baseline-optional-files-when-present does;
//   misplaced  project.Baseline lists .kogen/models.yaml first instead of right after
//              .kogen/project.yaml (a wrong baseline order, which the frozen request goldens reject).
// It runs the go binary of `go env GOROOT` directly, so a host wrapper named go that queues test
// runs adds no waiting. As a command it is the observer's own control:
//   node b-consumes-baseline-optional-models__control.js <coordinator> <package> <test>...
// It prints one line, the verdict on whether every named test passed with no test skipped or
// failed, and exits 0 when it accepts and 1 when it rejects.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

const COORDINATOR = 'github.com/KogenAI/kogen-coordinator';
// The last two lines of project.Baseline in the pinned coordinator's project/baseline.go.
const TAIL = '\tall = slices.DeleteFunc(all, func(p string) bool { return p == ModelsPath })\n' +
  '\treturn slices.Insert(all, slices.Index(all, ConfigPath)+1, ModelsPath), nil\n';
// The absent-file rule of coord-baseline-optional-files-when-present, inserted before TAIL.
const ABSENT = '\tout, err := gitOK(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ModelsPath)\n' +
  '\tif err != nil {\n' +
  '\t\treturn nil, fmt.Errorf("git ls-files failed: %w", err)\n' +
  '\t}\n' +
  '\tif !slices.Contains(strings.Split(out, "\\x00"), ModelsPath) {\n' +
  '\t\treturn all, nil\n' +
  '\t}\n';
const VARIANTS = {
  absent: ABSENT + TAIL,
  misplaced: TAIL.replace('slices.Index(all, ConfigPath)+1', '0'),
};

// goBinary is GOROOT's go, or plain go when GOROOT cannot be read.
function goBinary() {
  const r = spawnSync('go', ['env', 'GOROOT'], { encoding: 'utf8' });
  const bin = r.status === 0 ? path.join(r.stdout.trim(), 'bin', 'go') : '';
  return bin && fs.existsSync(bin) ? bin : 'go';
}

function realRoot() {
  return fs.realpathSync(process.cwd());
}

// prepare makes the temporary module copy and go.mod of a changed coordinator in tmp and returns
// the GOFLAGS that use them; for `pinned` it returns plain -trimpath.
function prepare(root, go, coordinator, tmp) {
  if (coordinator === 'pinned') return '-trimpath';
  const text = VARIANTS[coordinator];
  if (!text) throw new Error(`unknown coordinator ${coordinator}; want pinned, absent or misplaced`);
  let r = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', COORDINATOR], { cwd: root, encoding: 'utf8' });
  if (r.status === 0 && r.stdout.trim() === '') {
    // The module is not in the module cache yet.
    spawnSync(go, ['mod', 'download', COORDINATOR], { cwd: root, encoding: 'utf8' });
    r = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', COORDINATOR], { cwd: root, encoding: 'utf8' });
  }
  if (r.status !== 0 || r.stdout.trim() === '') throw new Error(`go list -m -f {{.Dir}} ${COORDINATOR} exited ${r.status}: ${r.stdout}${r.stderr}`);
  const dir = path.join(tmp, 'kogen-coordinator');
  fs.cpSync(r.stdout.trim(), dir, { recursive: true });
  spawnSync('chmod', ['-R', 'u+w', dir]);
  const file = path.join(dir, 'project', 'baseline.go');
  const source = fs.readFileSync(file, 'utf8');
  const n = source.split(TAIL).length - 1;
  if (n !== 1) throw new Error(`coordinator ${coordinator}: the pinned kogen-coordinator's project/baseline.go holds the end of project.Baseline ${n} times, want once`);
  fs.writeFileSync(file, source.replace(TAIL, text));
  const mod = path.join(tmp, 'go.mod');
  fs.writeFileSync(mod, fs.readFileSync(path.join(root, 'go.mod'), 'utf8') + `\nreplace ${COORDINATOR} => ${dir}\n`);
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(tmp, 'go.sum'));
  const flags = '-trimpath -modfile=' + mod;
  const used = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', COORDINATOR], { cwd: root, encoding: 'utf8', env: { ...process.env, GOFLAGS: flags } });
  if (used.status !== 0 || used.stdout.trim() !== dir) {
    throw new Error(`coordinator ${coordinator}: go does not use the changed copy (go list -m exited ${used.status}: ${used.stdout}${used.stderr})`);
  }
  return flags;
}

// runGo runs `go test -count=1 -json -run ^(tests)$ <pkg>` in root against the coordinator and
// resolves to the run: its command, exit status, top-level test results, outputs and own go test
// -json rows (run and result), and the rest of its output.
function runGo(root, coordinator, pkg, tests) {
  const go = goBinary();
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'bcbom-'));
  let flags;
  try {
    flags = prepare(root, go, coordinator, tmp);
  } catch (err) {
    fs.rmSync(tmp, { recursive: true, force: true });
    return Promise.reject(err);
  }
  const args = ['test', '-count=1', '-json', '-run', '^(' + tests.join('|') + ')$', pkg];
  const command = `go ${args.join(' ')} against the ${coordinator} kogen-coordinator`;
  return new Promise((resolve) => {
    const child = spawn(go, args, { cwd: root, env: { ...process.env, GOWORK: 'off', GOFLAGS: flags } });
    let stdout = '';
    let stderr = '';
    let done = false;
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr += d));
    child.on('error', (err) => {
      stderr += `could not run go: ${err.message}\n`;
      finish(null);
    });
    child.on('close', (status) => finish(status));
    function finish(status) {
      if (done) return;
      done = true;
      fs.rmSync(tmp, { recursive: true, force: true });
      const results = {};
      const outputs = {};
      const rows = {};
      let raw = '';
      for (const line of stdout.split('\n')) {
        if (!line.startsWith('{')) {
          raw += line ? line + '\n' : '';
          continue;
        }
        let ev;
        try {
          ev = JSON.parse(line);
        } catch {
          raw += line + '\n';
          continue;
        }
        if (!ev.Test) {
          if (ev.Action === 'output' || ev.Action === 'build-output') raw += ev.Output || '';
          continue;
        }
        const top = ev.Test.split('/')[0];
        if (ev.Test === top && ['run', 'pass', 'fail', 'skip'].includes(ev.Action)) (rows[top] = rows[top] || []).push(line);
        if (ev.Action === 'output') outputs[top] = (outputs[top] || '') + ev.Output;
        if (['pass', 'fail', 'skip'].includes(ev.Action) && ev.Test === top) results[top] = ev.Action;
        if (ev.Action === 'skip' && ev.Test !== top) results[ev.Test] = 'skip';
      }
      resolve({ command, status, results, outputs, rows, raw, stderr });
    }
  });
}

// passProblem is '' when every named test passed and no test of the run failed or was skipped,
// otherwise the reason, naming the tests and the command.
function passProblem(run, tests) {
  const missing = tests.filter((t) => !(t in run.results));
  const failed = Object.keys(run.results).filter((t) => run.results[t] === 'fail');
  const skipped = Object.keys(run.results).filter((t) => run.results[t] === 'skip');
  const problems = [];
  if (missing.length) problems.push(`did not run: ${missing.join(', ')}`);
  if (failed.length) problems.push(`failed: ${failed.join(', ')}`);
  if (skipped.length) problems.push(`skipped: ${skipped.join(', ')}`);
  if (problems.length === 0 && run.status !== 0) problems.push(`go test exited ${run.status}`);
  return problems.length ? `${problems.join('; ')} (${run.command}, exit ${run.status})` : '';
}

// clip shortens text for a failure message: at most 40 lines, each cut to 300 characters.
function clip(text) {
  const lines = text.split('\n');
  const kept = lines.slice(0, 40).map((l) => (l.length > 300 ? `${l.slice(0, 300)}... (${l.length} characters)` : l));
  if (lines.length > 40) kept.push(`... (${lines.length - 40} more lines)`);
  return kept.join('\n');
}

// ranLines is, for each named test, what go test reported for it: its own top-level go test
// -json rows as go printed them (its run row and its pass, fail or skip row), or that it did not
// run.
function ranLines(run, tests) {
  return tests.map((t) => (run.rows[t] ? run.rows[t].join('\n') : `${t}: did not run (no go test -json row for it in ${run.command})`)).join('\n');
}

// details is, first, what go test reported for each named test (ranLines), then the clipped
// output of the run's failing, skipped or missing tests, or its clipped raw output.
function details(run, tests) {
  const names = new Set([...tests.filter((t) => run.results[t] !== 'pass'), ...Object.keys(run.results).filter((t) => run.results[t] !== 'pass')]);
  const parts = [ranLines(run, tests)];
  for (const t of names) if (run.outputs[t]) parts.push(clip(run.outputs[t]));
  if (parts.length === 1) parts.push(clip(run.raw + run.stderr));
  return parts.join('\n');
}

module.exports = { VARIANTS, runGo, passProblem, details, ranLines, clip, realRoot };

async function main(argv) {
  const [coordinator, pkg, ...tests] = argv;
  if (!coordinator || !pkg || tests.length === 0) {
    console.log('usage: <pinned|absent|misplaced> <package> <test>...');
    return 2;
  }
  const run = await runGo(realRoot(), coordinator, pkg, tests);
  const p = passProblem(run, tests);
  if (p) {
    console.log(`rejected: ${tests.join(', ')}: ${p.replace(/ \(go .*$/, '')}`);
    return 1;
  }
  console.log(`passed: ${tests.join(', ')}`);
  return 0;
}

if (require.main === module) {
  main(process.argv.slice(2)).then(
    (code) => process.exit(code),
    (err) => {
      console.log(`error: ${err.message}`);
      process.exit(2);
    },
  );
}
