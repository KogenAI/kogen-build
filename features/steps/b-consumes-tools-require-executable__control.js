// Observer of b-consumes-tools-require-executable: runs named Go tests of
// ./consumes/kogen-verification with the real go tool, from the checkout's real path, against a
// chosen kogen-verification, and judges the go test -json events. The kogen-verification is
// either the one go.mod pins (`pinned`: Tools.Require wants node and quint to be regular files and
// refuses "<tool> is not available at <path>; reinstall Kogen") or a temporary copy of that module
// whose Tools.Require is replaced, reached through a temporary go.mod (-modfile) with a replace
// directive, so no checkout file changes. Every replacement wants node to be an executable regular
// file and quint a regular file, and words its refusals the new way ("node is not available: ...");
// all but `executable` carry one defect a consumer test must catch:
//   executable  the rule and wording of kogen-verification's broken-kogen-install-blamed-on-intent;
//   pathless    the refusal does not name the tool's path;
//   toolless    the refusal does not name the tool;
//   wrongclass  the refusal's class is not environment;
//   wrongreason the refusal's reason is not tool_missing;
//   wrongexit   the refusal's exit code is not 3;
//   quintfirst  quint is judged before node.
// It runs the go binary of `go env GOROOT` directly, so a host wrapper named go that queues test
// runs adds no waiting. As a command it is the observer's own control:
//   node b-consumes-tools-require-executable__control.js <verification> <test>...
// It prints one line, the verdict on whether every named test passed with no test skipped or
// failed, and exits 0 when it accepts and 1 when it rejects.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

const VERIFICATION = 'github.com/KogenAI/kogen-verification';
const PKG = './consumes/kogen-verification';
// Tools.Require of the pinned kogen-verification's verify/env.go.
const PINNED = 'func (t Tools) Require() error {\n' +
  '\tif !isRegular(t.Node) {\n' +
  '\t\treturn sys.Fail("environment", "tool_missing", "node is not available at "+t.Node+"; reinstall Kogen", 3)\n' +
  '\t}\n' +
  '\tif !isRegular(t.Quint) {\n' +
  '\t\treturn sys.Fail("environment", "tool_missing", "quint is not available at "+t.Quint+"; reinstall Kogen", 3)\n' +
  '\t}\n' +
  '\treturn nil\n' +
  '}\n';

// require_ is the Go text of a replacement Tools.Require: node must be an executable regular
// file, quint a regular file; o changes one property of the refusals.
function require_(o = {}) {
  const cls = o.cls || 'environment';
  const reason = o.reason || 'tool_missing';
  const exit = o.exit || 3;
  const detail = (tool, field, rule) => {
    if (o.pathless) return `"${tool} is not available: ${tool} is not ${rule}; the Kogen installation is broken, reinstall Kogen"`;
    if (o.toolless) return `"a tool is not available: "+t.${field}+" is not ${rule}; the Kogen installation is broken, reinstall Kogen"`;
    return `"${tool} is not available: ${tool} is not ${rule} at "+t.${field}+"; the Kogen installation is broken, reinstall Kogen"`;
  };
  const node = '\tif st, err := os.Stat(t.Node); err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0o111 == 0 {\n' +
    `\t\treturn sys.Fail("${cls}", "${reason}", ${detail('node', 'Node', 'an executable file')}, ${exit})\n` +
    '\t}\n';
  const quint = '\tif !isRegular(t.Quint) {\n' +
    `\t\treturn sys.Fail("${cls}", "${reason}", ${detail('quint', 'Quint', 'a regular file')}, ${exit})\n` +
    '\t}\n';
  return 'func (t Tools) Require() error {\n' + (o.quintFirst ? quint + node : node + quint) + '\treturn nil\n}\n';
}

const VARIANTS = {
  executable: require_(),
  pathless: require_({ pathless: true }),
  toolless: require_({ toolless: true }),
  wrongclass: require_({ cls: 'configuration' }),
  wrongreason: require_({ reason: 'tool_unusable' }),
  wrongexit: require_({ exit: 1 }),
  quintfirst: require_({ quintFirst: true }),
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

// prepare makes the temporary module copy and go.mod of a changed kogen-verification in tmp and
// returns the GOFLAGS that use them; for `pinned` it returns plain -trimpath.
function prepare(root, go, verification, tmp) {
  if (verification === 'pinned') return '-trimpath';
  const text = VARIANTS[verification];
  if (!text) throw new Error(`unknown kogen-verification ${verification}; want pinned or one of ${Object.keys(VARIANTS).join(', ')}`);
  let r = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', VERIFICATION], { cwd: root, encoding: 'utf8' });
  if (r.status === 0 && r.stdout.trim() === '') {
    // The module is not in the module cache yet.
    spawnSync(go, ['mod', 'download', VERIFICATION], { cwd: root, encoding: 'utf8' });
    r = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', VERIFICATION], { cwd: root, encoding: 'utf8' });
  }
  if (r.status !== 0 || r.stdout.trim() === '') throw new Error(`go list -m -f {{.Dir}} ${VERIFICATION} exited ${r.status}: ${r.stdout}${r.stderr}`);
  const dir = path.join(tmp, 'kogen-verification');
  fs.cpSync(r.stdout.trim(), dir, { recursive: true });
  spawnSync('chmod', ['-R', 'u+w', dir]);
  const file = path.join(dir, 'verify', 'env.go');
  const source = fs.readFileSync(file, 'utf8');
  const n = source.split(PINNED).length - 1;
  if (n !== 1) throw new Error(`kogen-verification ${verification}: the pinned kogen-verification's verify/env.go holds Tools.Require ${n} times, want once`);
  fs.writeFileSync(file, source.replace(PINNED, text));
  const mod = path.join(tmp, 'go.mod');
  fs.writeFileSync(mod, fs.readFileSync(path.join(root, 'go.mod'), 'utf8') + `\nreplace ${VERIFICATION} => ${dir}\n`);
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(tmp, 'go.sum'));
  const flags = '-trimpath -modfile=' + mod;
  const used = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', VERIFICATION], { cwd: root, encoding: 'utf8', env: { ...process.env, GOFLAGS: flags } });
  if (used.status !== 0 || used.stdout.trim() !== dir) {
    throw new Error(`kogen-verification ${verification}: go does not use the changed copy (go list -m exited ${used.status}: ${used.stdout}${used.stderr})`);
  }
  return flags;
}

// runGo runs `go test -count=1 -json -run ^(tests)$ ./consumes/kogen-verification` in root against
// the kogen-verification and resolves to the run: its command, exit status, top-level test
// results, outputs and own go test -json rows (run and result), and the rest of its output.
function runGo(root, verification, tests) {
  const go = goBinary();
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'bctre-'));
  let flags;
  try {
    flags = prepare(root, go, verification, tmp);
  } catch (err) {
    fs.rmSync(tmp, { recursive: true, force: true });
    return Promise.reject(err);
  }
  const args = ['test', '-count=1', '-json', '-run', '^(' + tests.join('|') + ')$', PKG];
  const command = `go ${args.join(' ')} against the ${verification} kogen-verification`;
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
  const [verification, ...tests] = argv;
  if (!verification || tests.length === 0) {
    console.log(`usage: <pinned|${Object.keys(VARIANTS).join('|')}> <test>...`);
    return 2;
  }
  const run = await runGo(realRoot(), verification, tests);
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
