'use strict';
// Steps of k-build-measured-figures-only.
// The scripted Builds run the real program: `go build ./cmd/kogen-build`, then
// `kogen-build build --project <fixture> --slug fixture` against a temporary Git project, with
// a scripted local provider (this file's HTTP server) standing in for ChatGPT. No step names a
// Go test the Build has to write; the golden steps run existing tests by exact name.
// Run with node, this file is the scripted Build's observer control: `control-good` / `control-bad`.
const {spawn, spawnSync} = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');

const FEATURE = '.kogen/intents/k-build-measured-figures-only/acceptance.feature';
const INSTRUCTIONS = 'build/prompts/builder.md';
const BUILDER_OPENING = 'You implement an approved Intent in this Git repository';
const NOTE = 'NOTE.md';
// Go commands on this host may queue behind other suites before they start.
const GO_TIMEOUT_MS = 900000;
const RUN_TIMEOUT_MS = 300000;

function sha256(data) {
  return crypto.createHash('sha256').update(data).digest('hex');
}

// run spawns a command without blocking the event loop (the scripted provider serves from it).
function run(cmd, args, opts, timeoutMs) {
  return new Promise(resolve => {
    const child = spawn(cmd, args, {...opts, stdio: ['ignore', 'pipe', 'pipe']});
    let stdout = '', stderr = '', timedOut = false;
    child.stdout.on('data', d => { stdout += d; });
    child.stderr.on('data', d => { stderr += d; });
    const timer = setTimeout(() => { timedOut = true; child.kill('SIGKILL'); }, timeoutMs);
    child.on('error', err => { clearTimeout(timer); resolve({status: null, stdout, stderr: stderr + String(err), timedOut}); });
    child.on('close', status => { clearTimeout(timer); resolve({status, stdout, stderr, timedOut}); });
  });
}

function git(root, env, ...args) {
  const r = spawnSync('git', args, {cwd: root, env, encoding: 'utf8'});
  if (r.status !== 0) throw new Error(`kmf-fixture: git ${args.join(' ')}: ${r.stderr}`);
  return r.stdout;
}

function write(root, rel, content, mode) {
  const p = path.join(root, rel);
  fs.mkdirSync(path.dirname(p), {recursive: true});
  fs.writeFileSync(p, content, {mode: mode || 0o644});
}

function gitEnv(extra) {
  const env = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith('GIT_')) env[k] = v;
  return {...env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1', GIT_TERMINAL_PROMPT: '0', ...extra};
}

const FIXTURE_INTENT = `# Preserve the fixture

Slug: fixture
Model: not applicable — no model for this fixture

Acceptance:
- .kogen/intents/fixture/acceptance.feature
- features/steps/fixture_steps.js

The fixture app holds the candidate, and NOTE.md records the effect of the change.

## Tests
- the app holds the candidate: the acceptance scenario (existing)

## Source comparison
- addressed: app.txt holds the candidate; source request; contract Tests and acceptance.feature.

## Feasibility
- none
`;
const FIXTURE_FEATURE = 'Feature: Fixture\n  Scenario: The app holds the candidate\n    Then the fixture app holds the candidate\n';
const FIXTURE_STEPS = "const {Then} = require('@cucumber/cucumber');\nconst fs = require('node:fs');\nThen('the fixture app holds the candidate', function () {\n  if (fs.readFileSync('app.txt', 'utf8') !== 'candidate\\n') throw new Error('app.txt does not hold the candidate');\n});\n";
const FIXTURE_ACCEPTANCE = `NODE_PATH="$KOGEN_CUCUMBER" "$KOGEN_NODE" "$KOGEN_CUCUMBER/@cucumber/cucumber/bin/cucumber.js" --strict --require 'features/steps/*.js' --format "message:$KOGEN_ACCEPTANCE_REPORT" "$KOGEN_ACCEPTANCE_FEATURES"`;

// fixtureProject makes the Git project the Build runs in: one commit with README.md, app.txt,
// .gitignore and .kogen/project.yaml; the Intent `fixture` written, not committed, and approved.
function fixtureProject(root, env) {
  git(root, env, 'init', '-q', '--initial-branch=main');
  for (const [k, v] of [['user.name', 'Kogen Fixture'], ['user.email', 'kogen-fixture@example.invalid'], ['commit.gpgsign', 'false']]) git(root, env, 'config', k, v);
  write(root, 'README.md', '# Fixture project\n');
  write(root, 'app.txt', 'baseline\n');
  write(root, '.gitignore', '.kogen/local/\n');
  write(root, '.kogen/project.yaml', [
    'version: 1', 'commands:', '  setup: []', '  fix: []', '  checks: []', '  acceptance:',
    "    run: '" + FIXTURE_ACCEPTANCE.replace(/'/g, "''") + "'", "    steps_dir: 'features/steps'", 'protect: []', '',
  ].join('\n'));
  git(root, env, 'add', 'README.md', 'app.txt', '.gitignore', '.kogen/project.yaml');
  git(root, env, 'commit', '-qm', 'fixture baseline');
  write(root, '.kogen/intents/fixture/intent.md', FIXTURE_INTENT);
  write(root, '.kogen/intents/fixture/acceptance.feature', FIXTURE_FEATURE);
  write(root, 'features/steps/fixture_steps.js', FIXTURE_STEPS);
  const baseline = ['.kogen/intents/fixture/intent.md', '.kogen/intents/fixture/acceptance.feature', 'features/steps/fixture_steps.js', '.kogen/project.yaml'].sort();
  const digest = sha256(baseline.map(p => p + '\0' + sha256(fs.readFileSync(path.join(root, p))) + '\n').join(''));
  const base = git(root, env, 'rev-parse', 'HEAD').trim();
  write(root, '.kogen/local/approvals/fixture.json', JSON.stringify({slug: 'fixture', sha256: digest, base, approver: 'Kogen Fixture <kogen-fixture@example.invalid>', approved_at: '2026-10-22T00:00:00Z', source_provenance: null}) + '\n');
}

function credentials(home) {
  const dir = path.join(home, '.kogen', 'credentials');
  fs.mkdirSync(dir, {recursive: true, mode: 0o700});
  fs.chmodSync(path.join(home, '.kogen'), 0o700);
  fs.writeFileSync(path.join(dir, 'chatgpt.json'), JSON.stringify({client_id: 'oaiapp_fake', access_token: 'fake-access-token', refresh_token: 'fake-refresh-token', expires_at: Math.floor(Date.now() / 1000) + 86400, scope: 'openid', email: 'fixture@example.invalid'}), {mode: 0o600});
}

// instructionsOf is a request's instructions: its instructions field, or its developer message.
function instructionsOf(req) {
  if (typeof req.instructions === 'string') return req.instructions;
  const first = (req.input || [])[0];
  const block = first && Array.isArray(first.content) ? first.content[0] : null;
  return block && typeof block.text === 'string' ? block.text : '';
}

function outputOf(req, callId) {
  for (const it of req.input || []) if (it && it.type === 'function_call_output' && it.call_id === callId) return typeof it.output === 'string' ? it.output : null;
  return null;
}

// parseShell reads the shell tool's completed result as the builder receives it:
// "exit status: N\n" followed by the command's output.
function parseShell(out) {
  const m = /^exit status: (-?[0-9]+)\n/.exec(out || '');
  return m ? {exit: Number(m[1]), output: out.slice(m[0].length)} : null;
}

// scriptedProvider answers /v1/responses in order: three builder turns (write app.txt; run the
// measuring command; write NOTE.md from the command's result as the Build returned it, and
// finish), then the reviewer's approval. Past the script it refuses with HTTP 400.
function scriptedProvider(command, invent) {
  const state = {requests: [], log: null, logProblem: '', summary: '', note: ''};
  let ids = 0;
  const call = (name, args) => { ids++; return {type: 'function_call', id: `function-${ids}`, call_id: `call-${ids}`, name, arguments: JSON.stringify(args)}; };
  const measure = call('shell', {command});
  const tick = '`';
  const turns = [
    () => [call('write_file', {path: 'app.txt', content: 'candidate\n'})],
    () => [measure],
    req => {
      state.log = parseShell(outputOf(req, measure.call_id));
      if (!state.log) {
        state.logProblem = `the measuring command's result is not a completed shell result: ${JSON.stringify(outputOf(req, measure.call_id))}`;
        return null;
      }
      if (invent) {
        state.note = `Measured effect: app.txt is 12 bytes, measured with ${tick}${command}${tick} on the final candidate.\n`;
        state.summary = 'Implemented app.txt and recorded its measured size.';
      } else if (state.log.exit === 0) {
        state.note = `Measured effect: app.txt is ${state.log.output.trim()} bytes, measured with ${tick}${command}${tick} on the final candidate.\n`;
        state.summary = `Implemented app.txt; its size was measured with ${tick}${command}${tick}.`;
      } else {
        state.note = `Expected effect (a target, not measured): app.txt should be about 10 bytes; ${tick}${command}${tick} could not run in this environment (exit status ${state.log.exit}).\n`;
        state.summary = `Implemented app.txt. The effect was not measured: ${tick}${command}${tick} could not run here.`;
      }
      return [call('write_file', {path: NOTE, content: state.note}), call('finish', {summary: state.summary})];
    },
    () => [call('submit_review', {verdict: 'approve', findings: []})],
  ];
  const server = http.createServer((req, res) => {
    let body = '';
    req.on('data', d => { body += d; });
    req.on('end', () => {
      if (req.method !== 'POST' || !/\/v1\/responses$/.test(req.url)) {
        res.writeHead(404).end();
        return;
      }
      let parsed = {};
      try { parsed = JSON.parse(body); } catch { /* recorded as empty */ }
      state.requests.push(parsed);
      const turn = turns[state.requests.length - 1];
      const items = turn ? turn(parsed) : null;
      if (!items) {
        res.writeHead(400, {'Content-Type': 'application/json'}).end(JSON.stringify({error: {type: 'invalid_request_error', code: 'invalid_request_error', message: 'scripted provider refusal'}}));
        return;
      }
      res.writeHead(200, {'Content-Type': 'text/event-stream', 'X-Request-Id': 'req-fixture'});
      items.forEach((item, i) => res.write('data: ' + JSON.stringify({type: 'response.output_item.done', output_index: i, item}) + '\n\n'));
      res.end('data: ' + JSON.stringify({type: 'response.completed', response: {usage: {input_tokens: 1, input_tokens_details: {cached_tokens: 0}, output_tokens: 1, output_tokens_details: {reasoning_tokens: 0}}}}) + '\n\n');
    });
  });
  return {server, state};
}

let binary = null;

// kogenBuild builds the program from this checkout once per process.
function kogenBuild() {
  if (!binary) {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kmf-bin-'));
    const out = path.join(dir, 'kogen-build');
    binary = run('go', ['build', '-o', out, './cmd/kogen-build'], {env: {...process.env, PWD: process.cwd()}}, GO_TIMEOUT_MS).then(r => {
      if (r.status !== 0) throw new Error(`kmf-fixture: go build -o ${out} ./cmd/kogen-build failed (exit ${r.status}${r.timedOut ? ', timed out' : ''}):\n${r.stdout}${r.stderr}`);
      return out;
    });
  }
  return binary;
}

// scriptedBuild runs `kogen-build build` on a fresh fixture project with the scripted provider
// and returns every problem it finds against want (the feature's expectations), each prefixed
// "kmf-reject: ".
async function scriptedBuild(rule, want, invent) {
  const program = await kogenBuild();
  const tmp = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'kmf-')));
  const root = path.join(tmp, 'project'), home = path.join(tmp, 'home');
  fs.mkdirSync(root);
  fs.mkdirSync(home);
  const {server, state} = scriptedProvider(want.command, invent);
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const url = `http://127.0.0.1:${server.address().port}`;
  try {
    const env = gitEnv({
      HOME: home, KOGEN_PROVIDER_URL: url, KOGEN_AUTH_URL: url,
      KOGEN_NODE: process.env.KOGEN_NODE || process.execPath,
      KOGEN_CUCUMBER: process.env.KOGEN_CUCUMBER || path.resolve('testdata/tools/cucumber/node_modules'),
    });
    fixtureProject(root, env);
    credentials(home);
    const r = await run(program, ['build', '--project', root, '--slug', 'fixture'], {cwd: root, env}, RUN_TIMEOUT_MS);
    const p = [];
    let result = null;
    try { result = JSON.parse(r.stdout.trim().split('\n').pop()); } catch { /* reported below */ }
    if (r.status !== 0 || !result || result.exit_code !== 0 || result.error !== null) {
      p.push(`kogen-build build exited ${r.status}${r.timedOut ? ' (timed out)' : ''}, want a landed Build; stdout ${JSON.stringify(r.stdout)}\nstderr:\n${r.stderr}`);
    }
    if (state.logProblem) p.push('command log: ' + state.logProblem);
    const expected = fs.readFileSync(INSTRUCTIONS, 'utf8');
    const builder = state.requests.filter(q => instructionsOf(q).startsWith(BUILDER_OPENING));
    const others = state.requests.length - builder.length;
    if (builder.length !== want.builderRequests) p.push(`builder requests ${builder.length}, want ${want.builderRequests}`);
    if (others !== want.reviewRequests) p.push(`review requests ${others}, want ${want.reviewRequests}`);
    builder.forEach((q, i) => {
      const text = instructionsOf(q);
      if (text !== expected) p.push(`instructions: builder request ${i + 1} does not carry ${INSTRUCTIONS} as it is in this checkout`);
      const n = text.split('\n').filter(l => l === rule).length;
      if (n !== 1) p.push(`instructions: builder request ${i + 1} carries the rule bullet ${n} times, want exactly once`);
    });
    const show = rel => spawnSync('git', ['show', 'HEAD:' + rel], {cwd: root, env, encoding: 'utf8'});
    const count = spawnSync('git', ['rev-list', '--count', 'HEAD'], {cwd: root, env, encoding: 'utf8'}).stdout.trim();
    if (count !== '2') p.push(`the branch has ${count} commits, want the baseline and one Build commit`);
    if (show('app.txt').stdout !== 'candidate\n') p.push(`committed app.txt ${JSON.stringify(show('app.txt').stdout)}, want "candidate\\n"`);
    const noteShown = show(NOTE);
    const committed = noteShown.status === 0 ? noteShown.stdout : '';
    if (noteShown.status !== 0) p.push(`note: ${NOTE} is not in the commit`);
    const log = state.log;
    if (log) {
      if (log.exit !== want.exit) p.push(`command log: ${JSON.stringify(want.command)} exited ${log.exit}, want ${want.exit}`);
      if (want.exit === 0 && log.output.trim() !== want.figure) p.push(`command log: ${JSON.stringify(want.command)} printed ${JSON.stringify(log.output)}, want ${JSON.stringify(want.figure)}`);
    }
    if (committed !== want.note) p.push(`note: committed ${NOTE}\n got ${JSON.stringify(committed)}\nwant ${JSON.stringify(want.note)}`);
    if (want.exit === 0) {
      const figures = committed.match(/[0-9]+/g) || [];
      if (!log || figures.length !== 1 || figures[0] !== log.output.trim()) p.push(`note: the figures ${JSON.stringify(figures)} in ${NOTE} are not exactly the measuring command's output ${JSON.stringify(log && log.output)}`);
      if (!committed.includes('`' + want.command + '`')) p.push(`note: ${NOTE} does not name the measuring command ${JSON.stringify(want.command)}`);
    } else {
      if (committed.includes('Measured effect') || !committed.includes('not measured')) p.push(`note: ${NOTE} presents the figure as a measurement: ${JSON.stringify(committed)}`);
      if (!state.summary.includes('not measured')) p.push(`finish summary ${JSON.stringify(state.summary)} does not say the effect was not measured`);
    }
    if (state.summary !== want.summary) p.push(`finish summary\n got ${JSON.stringify(state.summary)}\nwant ${JSON.stringify(want.summary)}`);
    return p.map(x => 'kmf-reject: ' + x);
  } finally {
    await new Promise(resolve => server.close(resolve));
    fs.rmSync(tmp, {recursive: true, force: true});
  }
}

// goTest runs one existing test by exact name in its own process and says what happened to it.
function goTest(name) {
  const args = ['test', '-json', '-count=1', './build', '-run', '^' + name + '$'];
  const r = spawnSync('go', args, {encoding: 'utf8', env: {...process.env, PWD: process.cwd()}, timeout: GO_TIMEOUT_MS, maxBuffer: 64 * 1024 * 1024});
  const rows = (r.stdout || '').split('\n').filter(l => l.startsWith('{')).map(l => {
    try { return JSON.parse(l); } catch { return {}; }
  });
  const count = action => rows.filter(x => x.Test === name && x.Action === action).length;
  const output = rows.filter(x => x.Test === name && x.Action === 'output').map(x => x.Output).join('');
  let outcome;
  if (r.error) outcome = 'did not finish (' + r.error.message + ')';
  else if (count('run') === 0) outcome = 'absent: it ran zero times';
  else if (count('skip') > 0) outcome = 'skipped';
  else if (count('fail') > 0) outcome = 'failed';
  else if (r.status === 0 && count('run') === 1 && count('pass') === 1) outcome = 'passed';
  else outcome = `not a single passing run (exit ${r.status}, runs ${count('run')}, passes ${count('pass')})`;
  return {outcome, text: `existing test ${name}: ${outcome}; command: go ${args.join(' ')}\n${output || (r.stdout || '') + (r.stderr || '')}`};
}

// docString is the doc string that follows the first feature line containing marker.
function docString(marker) {
  const lines = fs.readFileSync(FEATURE, 'utf8').split('\n');
  const at = lines.findIndex(l => l.includes(marker));
  const open = lines.findIndex((l, i) => i > at && l.trim() === '"""');
  const close = lines.findIndex((l, i) => i > open && l.trim() === '"""');
  if (at < 0 || open < 0 || close < 0) throw new Error(`no doc string after ${marker} in ${FEATURE}`);
  const indent = lines[open].indexOf('"""');
  return lines.slice(open + 1, close).map(l => l.slice(indent)).join('\n');
}

// hashList reads "sha256 path" lines.
function hashList(list) {
  return list.split('\n').filter(l => l.trim() !== '').map(l => l.trim().split(/\s+/));
}

// The observer controls run the scripted Builds on the unchanged tree with an existing bullet of
// the instructions standing in for the rule, so every deeper assertion is reached: control-good
// with the scripted builder copying the command log, control-bad with it inventing a figure.
const CONTROL_MODES = ['measured', 'unavailable'];

async function control(kind) {
  const stand = fs.readFileSync(INSTRUCTIONS, 'utf8').split('\n').find(l => l.startsWith('- Do not commit, push'));
  const results = [];
  for (const mode of CONTROL_MODES) results.push(await scriptedBuild(stand, JSON.parse(docString(`the scripted Build "${mode}" `)), kind === 'bad'));
  if (kind === 'good') {
    if (results.every(p => p.length === 0)) return console.log('observer accepted');
    results.forEach((p, i) => console.error(`control-good ${CONTROL_MODES[i]}:\n${p.join('\n')}`));
    console.log('observer rejected');
    process.exitCode = 1;
    return;
  }
  if (results.every(p => p.some(x => x.startsWith('kmf-reject: note:')) && !p.some(x => x.startsWith('kmf-reject: instructions:')))) {
    console.log('observer rejected');
    process.exitCode = 1;
  } else if (results.some(p => p.length === 0)) {
    console.log('observer accepted');
  } else {
    results.forEach((p, i) => console.error(`control-bad ${CONTROL_MODES[i]}:\n${p.join('\n')}`));
    console.log('observer broken');
    process.exitCode = 2;
  }
}

if (require.main === module) {
  const kind = {'control-good': 'good', 'control-bad': 'bad'}[process.argv[2]];
  if (!kind) {
    console.log('usage: node ' + path.basename(__filename) + ' control-good|control-bad');
    process.exitCode = 2;
  } else {
    control(kind).catch(err => {
      console.error(err.stack || String(err));
      console.log('observer broken');
      process.exitCode = 2;
    });
  }
} else {
  const {Given, Then} = require('@cucumber/cucumber');

  Given('k-build-measured-figures-only: the rule bullet is', function (rule) {
    if (rule.includes('\n') || !rule.startsWith('- ')) throw new Error('the rule must be one bullet line: ' + JSON.stringify(rule));
    this.kmfRule = rule;
  });

  Then('k-build-measured-figures-only: build\\/prompts\\/builder.md is the instructions of sha256 {string} with the rule bullet inserted before the bullet starting {string}', function (base, anchor) {
    const text = fs.readFileSync(INSTRUCTIONS, 'utf8');
    const lines = text.split('\n');
    const at = lines.map((l, i) => (l === this.kmfRule ? i : -1)).filter(i => i >= 0);
    const problems = [];
    if (at.length !== 1) problems.push(`the rule bullet is ${at.length} lines of ${INSTRUCTIONS}, want exactly 1`);
    const n = text.split(this.kmfRule.slice(2)).length - 1;
    if (n !== 1) problems.push(`the rule text occurs ${n} times in ${INSTRUCTIONS}, want exactly 1`);
    if (at.length === 1) {
      if (!(lines[at[0] + 1] || '').startsWith(anchor)) problems.push(`the line after the rule bullet is ${JSON.stringify(lines[at[0] + 1])}, want the bullet starting ${JSON.stringify(anchor)}`);
      const got = sha256(lines.filter((_, i) => i !== at[0]).join('\n'));
      if (got !== base) problems.push(`without the rule bullet ${INSTRUCTIONS} has sha256 ${got}, want ${base}: other instruction text changed`);
    }
    if (problems.length) throw new Error(problems.join('\n'));
  });

  Then('k-build-measured-figures-only: build\\/prompts\\/builder.md has {int} bytes and sha256 {string}', function (bytes, want) {
    const b = fs.readFileSync(INSTRUCTIONS);
    if (b.length !== bytes || sha256(b) !== want) throw new Error(`${INSTRUCTIONS}: ${b.length} bytes sha256 ${sha256(b)}, want ${bytes} bytes sha256 ${want}`);
  });

  Then('k-build-measured-figures-only: build\\/testdata\\/prompts_golden.json records the bytes and sha256 of build\\/prompts\\/builder.md', function () {
    const golden = JSON.parse(fs.readFileSync('build/testdata/prompts_golden.json', 'utf8'));
    const b = fs.readFileSync(INSTRUCTIONS);
    const want = {builder: {bytes: b.length, sha256: sha256(b)}};
    if (JSON.stringify(golden) !== JSON.stringify(want)) throw new Error(`build/testdata/prompts_golden.json holds ${JSON.stringify(golden)}, want ${JSON.stringify(want)}`);
  });

  Then('k-build-measured-figures-only: with the rule bullet taken out, these golden files that record the instructions keep their bytes', function (list) {
    // The bullet is plain ASCII without characters JSON escapes, so in a golden it is the
    // bullet followed by its line break, escaped once (a JSON string) or twice (JSON text kept
    // in a JSON string).
    const forms = [this.kmfRule + '\\\\n', this.kmfRule + '\\n', this.kmfRule + '\n'];
    const problems = [];
    for (const [want, file] of hashList(list)) {
      let text;
      try {
        text = fs.readFileSync(file, 'utf8');
      } catch (e) {
        problems.push(`${file}: unreadable (${e.message})`);
        continue;
      }
      let held = 0;
      for (const form of forms) {
        held += text.split(form).length - 1;
        text = text.split(form).join('');
      }
      if (held === 0) problems.push(`${file}: does not record the rule bullet`);
      if (text.includes(this.kmfRule.slice(2))) problems.push(`${file}: records the rule text other than as the bullet line`);
      const got = sha256(text);
      if (got !== want) problems.push(`${file}: without the rule bullet sha256 ${got}, want ${want}: it changed other than by the rule bullet`);
    }
    if (problems.length) throw new Error(problems.join('\n'));
  });

  Then('k-build-measured-figures-only: these golden files keep their bytes', function (list) {
    const changed = [];
    for (const [want, file] of hashList(list)) {
      let got;
      try {
        got = sha256(fs.readFileSync(file));
      } catch (e) {
        got = 'unreadable (' + e.message + ')';
      }
      if (got !== want) changed.push(`${file}: sha256 ${got}, want ${want}`);
    }
    if (changed.length) throw new Error('golden files changed:\n' + changed.join('\n'));
  });

  Then('k-build-measured-figures-only: the existing test {string} passes in its own go test run', {timeout: GO_TIMEOUT_MS + 60000}, function (name) {
    const r = goTest(name);
    if (r.outcome !== 'passed') throw new Error(r.text);
  });

  Then('k-build-measured-figures-only: the scripted Build {string} meets these expectations', {timeout: GO_TIMEOUT_MS + RUN_TIMEOUT_MS + 60000}, async function (mode, expect) {
    const p = await scriptedBuild(this.kmfRule, JSON.parse(expect), false);
    if (p.length) throw new Error(`kogen-build build of the fixture project, scripted Build "${mode}":\n${p.join('\n')}`);
  });
}
