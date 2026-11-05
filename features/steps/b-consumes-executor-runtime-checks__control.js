// Observer of b-consumes-executor-runtime-checks: runs TestContextExternalFirstPrompt (or other
// named Go tests of ./build) with the real go tool, from the checkout's real path, against a chosen
// kogen-executor, and judges the go test -json events of the named tests and their subtests. The
// kogen-executor is either the one go.mod pins (`pinned`: the managed Claude Code runtime must be a
// regular, non-empty, executable file) or a temporary copy of that module with one change, reached
// through a temporary go.mod (-modfile) with a replace directive, so no checkout file changes:
//   checked         the runtime checks of kogen-executor's kogen-runs-unverified-claude-code-runtime:
//                   the file must also have the pinned size of the platform (skipped while
//                   KOGEN_CLAUDE_CODE_URL names a test origin, whose own manifest makes the size
//                   unknown), be owned by the current user and not be writable by group or others;
//   strictseam      as checked, but the size is checked against the pin even with a test origin;
//   doubledprompt   the first prompt carries the stable authority (and so the diagnostic) twice;
//   blankprompt     the first prompt is the role's framing only, without the brief;
//   extradiagnostic the first prompt ends with the words "Caller diagnostic data".
// It runs the go binary of `go env GOROOT` directly, so a host wrapper named go that queues test
// runs adds no waiting; with a umask it runs go through /bin/sh after `umask <mask>`. As a command
// it is the observer's own control:
//   node b-consumes-executor-runtime-checks__control.js <executor> <umask|-> <test> [<subtest>...]
// It prints one line, the verdict on whether every named test and subtest passed with no test
// skipped or failed, and exits 0 when it accepts and 1 when it rejects.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

const EXECUTOR = 'github.com/KogenAI/kogen-executor';
const PKG = './build';

// The kind checks of runtimeUsable in the pinned kogen-executor's executor/claudecode/runtime.go.
const PINNED_KIND = '\tcase info.Mode().Perm()&0o111 == 0:\n' +
  '\t\tkind = "not executable"\n' +
  '\t}\n' +
  '\tif kind != "" {\n';

// kindChecks is PINNED_KIND followed by the size, owner and mode checks of the new runtime rules.
function kindChecks(sizeWithSeam) {
  const size = sizeWithSeam ? 'info.Size() != want.size' : '!seam && info.Size() != want.size';
  return '\tcase info.Mode().Perm()&0o111 == 0:\n' +
    '\t\tkind = "not executable"\n' +
    '\t}\n' +
    '\tif kind == "" {\n' +
    '\t\t_, seam := os.LookupEnv(SeamEnv)\n' +
    '\t\twant, _ := pinFor(runtime.GOOS, runtime.GOARCH)\n' +
    '\t\t_ = seam\n' +
    '\t\tst, _ := info.Sys().(*syscall.Stat_t)\n' +
    '\t\tswitch {\n' +
    `\t\tcase ${size}:\n` +
    '\t\t\tkind = fmt.Sprintf("%d bytes, not the pinned %d bytes", info.Size(), want.size)\n' +
    '\t\tcase st == nil || int(st.Uid) != os.Getuid():\n' +
    '\t\t\tkind = "not owned by the current user"\n' +
    '\t\tcase info.Mode().Perm()&0o022 != 0:\n' +
    '\t\t\tkind = "writable by group or others"\n' +
    '\t\t}\n' +
    '\t}\n' +
    '\tif kind != "" {\n';
}

// The first prompt of the pinned kogen-executor's executor/claudecode/session.go.
const PINNED_PROMPT = '\treturn constraintsFor(role) + promptSep + brief.Stable + promptSep + brief.Dynamic\n';

const VARIANTS = {
  checked: { file: 'runtime.go', from: PINNED_KIND, to: kindChecks(false) },
  strictseam: { file: 'runtime.go', from: PINNED_KIND, to: kindChecks(true) },
  doubledprompt: { file: 'session.go', from: PINNED_PROMPT, to: '\treturn constraintsFor(role) + promptSep + brief.Stable + promptSep + brief.Stable + promptSep + brief.Dynamic\n' },
  blankprompt: { file: 'session.go', from: PINNED_PROMPT, to: '\treturn constraintsFor(role)\n' },
  extradiagnostic: { file: 'session.go', from: PINNED_PROMPT, to: '\treturn constraintsFor(role) + promptSep + brief.Stable + promptSep + brief.Dynamic + promptSep + "Caller diagnostic data"\n' },
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

// prepare makes the temporary module copy and go.mod of a changed kogen-executor in tmp and
// returns the GOFLAGS that use them; for `pinned` it returns plain -trimpath.
function prepare(root, go, executor, tmp) {
  if (executor === 'pinned') return '-trimpath';
  const v = VARIANTS[executor];
  if (!v) throw new Error(`unknown kogen-executor ${executor}; want pinned or one of ${Object.keys(VARIANTS).join(', ')}`);
  let r = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', EXECUTOR], { cwd: root, encoding: 'utf8' });
  if (r.status === 0 && r.stdout.trim() === '') {
    // The module is not in the module cache yet.
    spawnSync(go, ['mod', 'download', EXECUTOR], { cwd: root, encoding: 'utf8' });
    r = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', EXECUTOR], { cwd: root, encoding: 'utf8' });
  }
  if (r.status !== 0 || r.stdout.trim() === '') throw new Error(`go list -m -f {{.Dir}} ${EXECUTOR} exited ${r.status}: ${r.stdout}${r.stderr}`);
  const dir = path.join(tmp, 'kogen-executor');
  fs.cpSync(r.stdout.trim(), dir, { recursive: true });
  spawnSync('chmod', ['-R', 'u+w', dir]);
  const file = path.join(dir, 'executor', 'claudecode', v.file);
  const source = fs.readFileSync(file, 'utf8');
  const n = source.split(v.from).length - 1;
  if (n !== 1) throw new Error(`kogen-executor ${executor}: the pinned kogen-executor's executor/claudecode/${v.file} holds the replaced text ${n} times, want once`);
  fs.writeFileSync(file, source.replace(v.from, v.to));
  const mod = path.join(tmp, 'go.mod');
  fs.writeFileSync(mod, fs.readFileSync(path.join(root, 'go.mod'), 'utf8') + `\nreplace ${EXECUTOR} => ${dir}\n`);
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(tmp, 'go.sum'));
  const flags = '-trimpath -modfile=' + mod;
  const used = spawnSync(go, ['list', '-m', '-f', '{{.Dir}}', EXECUTOR], { cwd: root, encoding: 'utf8', env: { ...process.env, GOFLAGS: flags } });
  if (used.status !== 0 || used.stdout.trim() !== dir) {
    throw new Error(`kogen-executor ${executor}: go does not use the changed copy (go list -m exited ${used.status}: ${used.stdout}${used.stderr})`);
  }
  return flags;
}

// runGo runs `go test -count=1 -json -run ^(tests)$ ./build` in root against the kogen-executor,
// under the umask when one is given, and resolves to the run: its command, exit status, the results
// of every test and subtest, outputs per top-level test, the named tests' own go test -json rows
// (run and result), and the rest of its output.
function runGo(root, executor, umask, tests) {
  const go = goBinary();
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'bcert-'));
  let flags;
  try {
    flags = prepare(root, go, executor, tmp);
  } catch (err) {
    fs.rmSync(tmp, { recursive: true, force: true });
    return Promise.reject(err);
  }
  if (umask && !/^[0-7]{3,4}$/.test(umask)) return Promise.reject(new Error(`umask ${umask} is not octal`));
  const args = ['test', '-count=1', '-json', '-run', '^(' + tests.join('|') + ')$', PKG];
  const command = `go ${args.join(' ')} against the ${executor} kogen-executor` + (umask ? ` with umask ${umask}` : '');
  const [file, argv] = umask ? ['/bin/sh', ['-c', `umask ${umask} && exec "$0" "$@"`, go, ...args]] : [go, args];
  return new Promise((resolve) => {
    const child = spawn(file, argv, { cwd: root, env: { ...process.env, GOWORK: 'off', GOFLAGS: flags } });
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
        if (['run', 'pass', 'fail', 'skip'].includes(ev.Action)) (rows[ev.Test] = rows[ev.Test] || []).push(line);
        if (ev.Action === 'output') outputs[top] = (outputs[top] || '') + ev.Output;
        if (['pass', 'fail', 'skip'].includes(ev.Action)) results[ev.Test] = ev.Action;
      }
      resolve({ command, status, results, outputs, rows, raw, stderr });
    }
  });
}

// passProblem is '' when every named test and subtest passed and no test of the run failed or was
// skipped, otherwise the reason, naming the tests and the command.
function passProblem(run, names) {
  const sorted = (xs) => xs.slice().sort();
  const missing = names.filter((t) => !(t in run.results));
  const failed = sorted(Object.keys(run.results).filter((t) => run.results[t] === 'fail'));
  const skipped = sorted(Object.keys(run.results).filter((t) => run.results[t] === 'skip'));
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

// ranLines is, for each named test or subtest, what go test reported for it: its own go test
// -json rows as go printed them (its run row and its pass, fail or skip row), or that it did not
// run.
function ranLines(run, names) {
  return names.map((t) => (run.rows[t] ? run.rows[t].join('\n') : `${t}: did not run (no go test -json row for it in ${run.command})`)).join('\n');
}

// details is, first, what go test reported for each named test (ranLines), then the clipped
// output of the top-level tests that did not pass, or the run's clipped raw output.
function details(run, names) {
  const tops = new Set(names.map((t) => t.split('/')[0]).filter((t) => run.results[t] !== 'pass'));
  const parts = [ranLines(run, names)];
  for (const t of tops) if (run.outputs[t]) parts.push(clip(run.outputs[t]));
  if (parts.length === 1) parts.push(clip(run.raw + run.stderr));
  return parts.join('\n');
}

module.exports = { VARIANTS, runGo, passProblem, details, ranLines, clip, realRoot };

async function main(argv) {
  const [executor, umask, ...names] = argv;
  if (!executor || !umask || names.length === 0) {
    console.log(`usage: <pinned|${Object.keys(VARIANTS).join('|')}> <umask|-> <test> [<subtest>...]`);
    return 2;
  }
  const tests = [...new Set(names.map((t) => t.split('/')[0]))];
  const run = await runGo(realRoot(), executor, umask === '-' ? '' : umask, tests);
  const p = passProblem(run, names);
  if (p) {
    console.log(`rejected: ${names.join(', ')}: ${p.replace(/ \(go .*$/, '')}`);
    return 1;
  }
  console.log(`passed: ${names.join(', ')}`);
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
