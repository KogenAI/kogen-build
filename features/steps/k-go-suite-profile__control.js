// Observer of k-go-suite-profile: runs named Go tests of ./build with the real go tool, from
// the checkout's real path, and judges go test -json events. A run may add, without changing
// any checkout file: a deliberately broken implementation (one source file replaced through
// -overlay), the shared acceptance script check (the Go source of k-go-suite-profile__shared.js,
// overlaid into package build), or stand-ins for the host's other toolchains first on PATH,
// each of which logs its own run. It runs the go binary of `go env GOROOT` directly, so a host
// wrapper named go that queues test runs behind other suites adds no waiting. As a command it
// is the observer's own control:
//   node k-go-suite-profile__control.js failed <broken implementation|plain> <test> <message>
//   node k-go-suite-profile__control.js parallel <test>
//   node k-go-suite-profile__control.js hermetic [--call-stand-in] <test>
//   node k-go-suite-profile__control.js fixture <test>
// It prints one line, the verdict, and exits 0 when it accepts and 1 when it rejects.
'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

const FIXTURE_SOURCE = require('./k-go-suite-profile__shared.js');
const FIXTURE_TARGET = 'build/k_go_suite_profile_acceptance_test.go';
const HOST_TOOLS = ['elixir', 'erl', 'mix', 'ruby', 'bundle', 'python3', 'npm', 'quint'];

// Each deliberately broken implementation replaces exactly one text of one file.
const BROKEN = {
  'finish-reminder-dropped': {
    file: 'build/builder.go',
    from: 'return feedback + "\\n\\n" + finishReminder, false, nil',
    to: 'return feedback, false, nil',
  },
  'unmoved-notice-dropped': {
    file: 'build/unmoved.go',
    from: 'func (b *Attempt) unmovedNotice(stage string, pass verify.Pass) string {\n',
    to: 'func (b *Attempt) unmovedNotice(stage string, pass verify.Pass) string {\n\tif true {\n\t\treturn ""\n\t}\n',
  },
  'maintained-models-dropped': {
    file: 'build/build.go',
    from: '\tin.MaintainedModels = models\n',
    to: '\tin.MaintainedModels = models[:0]\n',
  },
  'standalone-checks-dropped': {
    file: 'cmd/kogen-build/main.go',
    from: '\tr.Checks = out.Checks\n',
    to: '\tr.Checks = out.Checks[:0]\n',
  },
  'red-repair-uncounted': {
    file: 'build/route.go',
    from: '\tif !pass.Green {\n\t\tb.Repairs++\n\t\treturn pass, false, withNotice(',
    to: '\tif !pass.Green {\n\t\treturn pass, false, withNotice(',
  },
};

// goBinary is GOROOT's go, or plain go when GOROOT cannot be read.
function goBinary() {
  const r = spawnSync('go', ['env', 'GOROOT'], { encoding: 'utf8' });
  const bin = r.status === 0 ? path.join(r.stdout.trim(), 'bin', 'go') : '';
  return bin && fs.existsSync(bin) ? bin : 'go';
}

// runGo runs `go test -count=1 -json -run ^(tests)$ ./build` in root. options: broken (a key of
// BROKEN), fixture (overlay the shared acceptance script check), standIns (stand-in toolchains
// first on PATH), callStandIn (the control's known-bad case: run the python3 stand-in once
// after go test). It resolves to the run: its command, exit status, events by test, and the
// stand-in runs.
function runGo(root, tests, options = {}) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kgsp-'));
  const replace = {};
  if (options.broken) {
    const b = BROKEN[options.broken];
    if (!b) throw new Error(`unknown broken implementation ${options.broken}`);
    const text = fs.readFileSync(path.join(root, b.file), 'utf8');
    const n = text.split(b.from).length - 1;
    if (n !== 1) throw new Error(`broken implementation ${options.broken}: ${b.file} holds the text to replace ${n} times, want once`);
    const file = path.join(tmp, path.basename(b.file));
    fs.writeFileSync(file, text.replace(b.from, b.to));
    replace[path.join(root, b.file)] = file;
  }
  if (options.fixture) {
    const file = path.join(tmp, path.basename(FIXTURE_TARGET));
    fs.writeFileSync(file, FIXTURE_SOURCE);
    replace[path.join(root, FIXTURE_TARGET)] = file;
  }
  const flags = ['-trimpath'];
  if (Object.keys(replace).length > 0) {
    const overlay = path.join(tmp, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({ Replace: replace }));
    flags.push('-overlay=' + overlay);
  }
  const env = { ...process.env, GOFLAGS: flags.join(' ') };
  const log = path.join(tmp, 'stand-ins.log');
  const bin = path.join(tmp, 'bin');
  if (options.standIns) {
    fs.mkdirSync(bin);
    for (const tool of HOST_TOOLS) {
      fs.writeFileSync(path.join(bin, tool), `#!/bin/sh\nprintf '%s\\n' "${tool} $*" >> '${log}'\necho '${tool} stand-in 0.0'\n`, { mode: 0o755 });
    }
    env.PATH = bin + path.delimiter + process.env.PATH;
  }
  const args = ['test', '-count=1', '-json', '-run', '^(' + tests.join('|') + ')$', './build'];
  const go = goBinary();
  const command = `GOFLAGS='${env.GOFLAGS}' ${go} ${args.join(' ')}` + (options.standIns ? ' (stand-ins first on PATH)' : '');
  return new Promise((resolve) => {
    const child = spawn(go, args, { cwd: root, env });
    let stdout = '';
    let stderr = '';
    let done = false;
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr += d));
    child.on('error', (err) => {
      stderr += `could not run go: ${err.message}\n`;
      finish(null, null);
    });
    child.on('close', (status, signal) => finish(status, signal));
    function finish(status, signal) {
      if (done) return;
      done = true;
      if (options.callStandIn) spawnSync(path.join(bin, 'python3'), ['--version']);
      let calls = [];
      if (fs.existsSync(log)) calls = fs.readFileSync(log, 'utf8').split('\n').filter(Boolean);
      fs.rmSync(tmp, { recursive: true, force: true });
      const events = [];
      for (const line of stdout.split('\n')) {
        if (!line.startsWith('{')) continue;
        try {
          events.push(JSON.parse(line));
        } catch {
          // a line that is not an event is kept in stdout only
        }
      }
      resolve({ command, status: status === null ? signal : status, stdout, stderr, events, calls });
    }
  });
}

// view is what the run shows of test: its own actions, and its and its subtests' output.
function view(run, test) {
  const actions = new Set();
  let output = '';
  for (const e of run.events) {
    if (e.Test === test && e.Action !== 'output') actions.add(e.Action);
    if ((e.Test === test || (e.Test || '').startsWith(test + '/')) && e.Action === 'output') output += e.Output;
  }
  return { actions, output };
}

function result(actions) {
  for (const [a, word] of [['fail', 'FAIL'], ['pass', 'PASS'], ['skip', 'SKIP']]) if (actions.has(a)) return word;
  return 'no result line';
}

// kgspMessage is the text of the shared acceptance script check's first "kgsp: " failure.
function kgspMessage(output) {
  const m = /: kgsp: (.*)/.exec(output);
  return m ? m[1] : '';
}

// passProblem is why the run does not show test passing (and, when asked, running in parallel,
// and with no stand-in toolchain run), or '' when it does.
function passProblem(run, test, { parallel = false, hermetic = false } = {}) {
  const v = view(run, test);
  if (!v.actions.has('pass')) {
    const why = kgspMessage(v.output);
    return `${test} did not pass (${result(v.actions)})` + (why ? `: ${why}` : '');
  }
  if (parallel && !v.actions.has('pause')) return `${test} passed but did not run in parallel`;
  if (hermetic && run.calls.length > 0) return `host toolchain reached during ${test}: ${run.calls.join('; ')}`;
  return '';
}

// failProblem is why the run does not show test failing with message (and, when asked, running
// in parallel), or '' when it does.
function failProblem(run, test, message, { parallel = false } = {}) {
  const v = view(run, test);
  if (!v.actions.has('fail')) return `${test} did not fail (${result(v.actions)})`;
  if (!v.output.includes(message)) return `${test} failed without ${JSON.stringify(message)}`;
  if (parallel && !v.actions.has('pause')) return `${test} failed but did not run in parallel`;
  return '';
}

// details is the evidence a scenario failure quotes: the test's own go test output (its
// "=== RUN" and "--- PASS"/"--- FAIL" lines included), or the end of go's output when the test
// printed nothing, then the command and its exit status.
function details(run, test) {
  const v = view(run, test);
  const out = v.output ? v.output.slice(0, 6000) : (run.stdout + run.stderr).slice(-4000);
  return `${out}\n${run.command}\nexit status ${run.status}`;
}

function realRoot() {
  return fs.realpathSync(process.cwd());
}

module.exports = { BROKEN, HOST_TOOLS, runGo, view, passProblem, failProblem, details, realRoot };

async function main(argv) {
  const [mode, ...rest] = argv;
  const root = realRoot();
  let verdict;
  if (mode === 'failed' && rest.length === 3) {
    const [broken, test, message] = rest;
    const run = await runGo(root, [test], broken === 'plain' ? {} : { broken });
    const p = failProblem(run, test, message);
    verdict = p ? `rejected: ${p}` : `failed: ${test}`;
  } else if (mode === 'parallel' && rest.length === 1) {
    const run = await runGo(root, rest);
    const p = passProblem(run, rest[0], { parallel: true });
    verdict = p ? `rejected: ${p}` : `passed in parallel: ${rest[0]}`;
  } else if (mode === 'hermetic' && (rest.length === 1 || (rest.length === 2 && rest[0] === '--call-stand-in'))) {
    const test = rest[rest.length - 1];
    const run = await runGo(root, [test], { standIns: true, callStandIn: rest.length === 2 });
    const p = passProblem(run, test, { hermetic: true });
    verdict = p ? `rejected: ${p}` : `no host toolchain: ${test}`;
  } else if (mode === 'fixture' && rest.length === 1) {
    const run = await runGo(root, rest, { fixture: true });
    const p = passProblem(run, rest[0]);
    verdict = p ? `rejected: ${p}` : `passed: ${rest[0]}`;
  } else {
    console.log('usage: failed <broken implementation|plain> <test> <message> | parallel <test> | hermetic [--call-stand-in] <test> | fixture <test>');
    process.exit(2);
  }
  console.log(verdict);
  process.exit(verdict.startsWith('rejected: ') ? 1 : 0);
}

if (require.main === module) {
  main(process.argv.slice(2)).catch((err) => {
    console.log(`error: ${err.message}`);
    process.exit(3);
  });
}
