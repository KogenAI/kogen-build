'use strict';
// Steps of k-build-finish-whole-intent-reminder.
// The scripted Builds run the real program: `go build ./cmd/kogen-build`, then
// `kogen-build build --project <fixture> --slug fixture` on a temporary Git project whose
// Intent adds `--loud` behind the existing entry point greet.js. A scripted local provider (this
// file's HTTP server, KOGEN_PROVIDER_URL) stands in for ChatGPT and records every request with
// the number of commits on the fixture branch when it arrived. Each capture is judged by judge(),
// against the expectations the feature states. The golden steps run existing tests by exact name.
// Run with node, this file is the scripted Builds' observer control: `control-good` and
// `control-bad` run both scripted Builds on the current tree and judge known-good and known-bad
// variants of the real captures.
const {spawn, spawnSync} = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');

const P = 'k-build-finish-whole-intent-reminder: ';
const FEATURE = '.kogen/intents/k-build-finish-whole-intent-reminder/acceptance.feature';
const INSTRUCTIONS = 'build/prompts/builder.md';
const BUILDER_OPENING = 'You implement an approved Intent in this Git repository';
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
  if (r.status !== 0) throw new Error(`kfw-fixture: git ${args.join(' ')}: ${r.stderr}`);
  return r.stdout;
}

function write(root, rel, content) {
  const p = path.join(root, rel);
  fs.mkdirSync(path.dirname(p), {recursive: true});
  fs.writeFileSync(p, content);
}

function gitEnv(extra) {
  const env = {};
  for (const [k, v] of Object.entries(process.env)) if (!k.startsWith('GIT_')) env[k] = v;
  return {...env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1', GIT_TERMINAL_PROMPT: '0', ...extra};
}

// The fixture project's files and the scripted builder's and reviewer's writes.
const GREET_BASE = "'use strict';\nconsole.log('hello');\n";
const GREET_WIRED = "'use strict';\nconst shout = require('./lib/shout');\nconsole.log(process.argv.includes('--loud') ? shout('hello') : 'hello');\n";
const SHOUT = "'use strict';\nmodule.exports = text => text.toUpperCase();\n";
const SHOUT_TEST = "'use strict';\nconst assert = require('node:assert');\nconst shout = require('../lib/shout');\nassert.strictEqual(shout('hello'), 'HELLO');\nconsole.log('shout: ok');\n";

const FIXTURE_INTENT = `# Shout the greeting with --loud

Slug: fixture
Model: not applicable — no model for this fixture

Acceptance:
- .kogen/intents/fixture/acceptance.feature
- features/steps/fixture_steps.js

The existing entry point greet.js prints HELLO when run with --loud, through a shout helper; without the flag it still prints hello.

## Tests
- greet.js --loud prints HELLO through the entry point: the acceptance scenario (scenario)
- the shout helper upper-cases its text: test/shout.test.js (unit)

## Source comparison
- addressed: greet.js --loud prints HELLO; source request; contract Tests and acceptance.feature.

## Feasibility
- none
`;
const FIXTURE_FEATURE = [
  'Feature: Greeting',
  '  Scenario: Greeting quietly',
  '    Then greet.js run with "" prints "hello"',
  '',
  '  Scenario: Greeting loudly shouts',
  '    Then greet.js run with "--loud" prints "HELLO"',
  '',
].join('\n');
const FIXTURE_STEPS = [
  "const {Then} = require('@cucumber/cucumber');",
  "const {spawnSync} = require('node:child_process');",
  "Then('greet.js run with {string} prints {string}', function (arg, want) {",
  "  const r = spawnSync(process.execPath, ['greet.js', ...(arg ? [arg] : [])], {encoding: 'utf8'});",
  "  if (r.stdout === want + '\\n') return;",
  "  // The message alone, without a stack naming the verification folder.",
  "  const e = new Error('greet.js printed ' + JSON.stringify(r.stdout) + ', want ' + JSON.stringify(want + '\\n'));",
  "  e.stack = e.message;",
  "  throw e;",
  '});',
  '',
].join('\n');
const FIXTURE_ACCEPTANCE = `NODE_PATH="$KOGEN_CUCUMBER" "$KOGEN_NODE" "$KOGEN_CUCUMBER/@cucumber/cucumber/bin/cucumber.js" --strict --require 'features/steps/*.js' --format "message:$KOGEN_ACCEPTANCE_REPORT" "$KOGEN_ACCEPTANCE_FEATURES"`;
const FIXTURE_CHECK = 'for f in test/*.test.js; do if [ -f "$f" ]; then "${KOGEN_NODE:-node}" "$f" || exit 1; fi; done';

// fixtureProject makes the Git project the Build runs in: one commit with README.md, greet.js,
// .gitignore and .kogen/project.yaml (check unit-tests runs every test/*.test.js); the Intent
// `fixture` written, not committed, and approved.
function fixtureProject(root, env) {
  git(root, env, 'init', '-q', '--initial-branch=main');
  for (const [k, v] of [['user.name', 'Kogen Fixture'], ['user.email', 'kogen-fixture@example.invalid'], ['commit.gpgsign', 'false']]) git(root, env, 'config', k, v);
  write(root, 'README.md', '# Fixture project\n\n`node greet.js` greets.\n');
  write(root, 'greet.js', GREET_BASE);
  write(root, '.gitignore', '.kogen/local/\n');
  write(root, '.kogen/project.yaml', [
    'version: 1', 'commands:', '  setup: []', '  fix: []', '  checks:', '    - name: unit-tests', "      run: '" + FIXTURE_CHECK + "'",
    '  acceptance:', "    run: '" + FIXTURE_ACCEPTANCE.replace(/'/g, "''") + "'", "    steps_dir: 'features/steps'", 'protect: []', '',
  ].join('\n'));
  git(root, env, 'add', 'README.md', 'greet.js', '.gitignore', '.kogen/project.yaml');
  git(root, env, 'commit', '-qm', 'fixture baseline');
  write(root, '.kogen/intents/fixture/intent.md', FIXTURE_INTENT);
  write(root, '.kogen/intents/fixture/acceptance.feature', FIXTURE_FEATURE);
  write(root, 'features/steps/fixture_steps.js', FIXTURE_STEPS);
  const baseline = ['.kogen/intents/fixture/intent.md', '.kogen/intents/fixture/acceptance.feature', 'features/steps/fixture_steps.js', '.kogen/project.yaml'].sort();
  const digest = sha256(baseline.map(p => p + '\0' + sha256(fs.readFileSync(path.join(root, p))) + '\n').join(''));
  const base = git(root, env, 'rev-parse', 'HEAD').trim();
  write(root, '.kogen/local/approvals/fixture.json', JSON.stringify({slug: 'fixture', sha256: digest, base, approver: 'Kogen Fixture <kogen-fixture@example.invalid>', approved_at: '2026-11-04T00:00:00Z', source_provenance: null}) + '\n');
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

function textOf(content) {
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) return content.map(b => (b && typeof b.text === 'string' ? b.text : '')).join('');
  return null;
}

// outputOf is the output a request carries for the tool call callId.
function outputOf(req, callId) {
  for (const it of req.input || []) if (it && it.type === 'function_call_output' && it.call_id === callId) return textOf(it.output);
  return null;
}

// lastUserText is the text of a request's last user message.
function lastUserText(req) {
  const items = (req.input || []).filter(it => it && it.role === 'user');
  return items.length ? textOf(items[items.length - 1].content) : null;
}

// SCRIPTS are the scripted provider's answers, in request order, per mode.
//   partial-then-wired: the builder adds only the unused shout helper with its passing unit test
//     and finishes; after the red pass it wires greet.js and finishes; the reviewer approves.
//   review-repair: the builder implements everything and finishes; the reviewer's repair breaks
//     greet.js and approves; after the red final verification it restores greet.js and approves.
// Each script stores what the Build sent back: `builder` (the output of the builder's first
// finish call) or `reviewer` (the reviewer's last user message after its first approval).
const SCRIPTS = {
  'partial-then-wired': call => {
    const finish = call('finish', {summary: 'Added the shout helper with its unit test.'});
    return [
      () => [call('write_file', {path: 'lib/shout.js', content: SHOUT}), call('write_file', {path: 'test/shout.test.js', content: SHOUT_TEST}), finish],
      (req, got) => {
        got.builder = outputOf(req, finish.call_id);
        return [call('write_file', {path: 'greet.js', content: GREET_WIRED}), call('finish', {summary: 'Wired --loud into greet.js through the shout helper.'})];
      },
      () => [call('submit_review', {verdict: 'approve', findings: []})],
    ];
  },
  'review-repair': call => [
    () => [call('write_file', {path: 'lib/shout.js', content: SHOUT}), call('write_file', {path: 'test/shout.test.js', content: SHOUT_TEST}), call('write_file', {path: 'greet.js', content: GREET_WIRED}), call('finish', {summary: 'Added --loud to greet.js through the shout helper.'})],
    () => [call('write_file', {path: 'greet.js', content: GREET_BASE}), call('submit_review', {verdict: 'approve', findings: []})],
    (req, got) => {
      got.reviewer = lastUserText(req);
      return [call('write_file', {path: 'greet.js', content: GREET_WIRED}), call('submit_review', {verdict: 'approve', findings: []})];
    },
  ],
};

// scriptedProvider answers /v1/responses from the mode's script and records each request with
// the fixture branch's commit count when it arrived. Past the script it refuses with HTTP 400.
function scriptedProvider(mode, root, env) {
  const state = {requests: [], got: {}};
  let ids = 0;
  const call = (name, args) => { ids++; return {type: 'function_call', id: `function-${ids}`, call_id: `call-${ids}`, name, arguments: JSON.stringify(args)}; };
  const turns = SCRIPTS[mode](call);
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
      const commits = Number(spawnSync('git', ['rev-list', '--count', 'HEAD'], {cwd: root, env, encoding: 'utf8'}).stdout.trim());
      state.requests.push({body: parsed, commits});
      const turn = turns[state.requests.length - 1];
      const items = turn ? turn(parsed, state.got) : null;
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
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kfw-bin-'));
    const out = path.join(dir, 'kogen-build');
    binary = run('go', ['build', '-o', out, './cmd/kogen-build'], {env: {...process.env, PWD: process.cwd()}}, GO_TIMEOUT_MS).then(r => {
      if (r.status !== 0) throw new Error(`kfw-fixture: go build -o ${out} ./cmd/kogen-build failed (exit ${r.status}${r.timedOut ? ', timed out' : ''}):\n${r.stdout}${r.stderr}`);
      return out;
    });
  }
  return binary;
}

// scriptedBuild runs `kogen-build build` on a fresh fixture project with the mode's script and
// returns its capture: the process outcome, every request (kind builder or reviewer, the
// builder's instructions, the commit count on arrival), what the Build sent back, and the
// commits it left.
async function scriptedBuild(mode) {
  const program = await kogenBuild();
  const tmp = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'kfw-')));
  const root = path.join(tmp, 'project'), home = path.join(tmp, 'home');
  fs.mkdirSync(root);
  fs.mkdirSync(home);
  const env = gitEnv({HOME: home, KOGEN_NODE: process.env.KOGEN_NODE || process.execPath, KOGEN_CUCUMBER: process.env.KOGEN_CUCUMBER || path.resolve('testdata/tools/cucumber/node_modules')});
  fixtureProject(root, env);
  credentials(home);
  const {server, state} = scriptedProvider(mode, root, env);
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const url = `http://127.0.0.1:${server.address().port}`;
  try {
    const r = await run(program, ['build', '--project', root, '--slug', 'fixture'], {cwd: root, env: {...env, KOGEN_PROVIDER_URL: url, KOGEN_AUTH_URL: url}}, RUN_TIMEOUT_MS);
    let result = null;
    try { result = JSON.parse(r.stdout.trim().split('\n').pop()); } catch { /* judged below */ }
    const norm = s => (typeof s === 'string' ? s.split(root).join('<root>').split(tmp).join('<tmp>') : s);
    const show = rel => {
      const x = spawnSync('git', ['show', 'HEAD:' + rel], {cwd: root, env, encoding: 'utf8'});
      return x.status === 0 ? x.stdout : null;
    };
    return {
      mode, status: r.status, timedOut: r.timedOut, stdout: r.stdout, stderr: norm(r.stderr), result,
      requests: state.requests.map(q => {
        const text = instructionsOf(q.body);
        return {kind: text.startsWith(BUILDER_OPENING) ? 'builder' : 'reviewer', instructions: text, commits: q.commits};
      }),
      builder: norm(state.got.builder), reviewer: norm(state.got.reviewer),
      commits: Number(spawnSync('git', ['rev-list', '--count', 'HEAD'], {cwd: root, env, encoding: 'utf8'}).stdout.trim()),
      committed: Object.fromEntries(['greet.js', 'lib/shout.js', 'test/shout.test.js'].map(f => [f, show(f)])),
    };
  } finally {
    await new Promise(resolve => server.close(resolve));
    fs.rmSync(tmp, {recursive: true, force: true});
  }
}

const FILES = {'greet.js': GREET_WIRED, 'lib/shout.js': SHOUT, 'test/shout.test.js': SHOUT_TEST};

// judge returns every problem of a capture against want (the feature's expectations), each
// prefixed with its area: build, sequence, instructions, feedback or review feedback.
// texts: the finish bullet the builder's instructions hold, the instructions themselves and the
// reminder line.
function judge(c, want, texts) {
  const p = [];
  const r = c.result;
  if (c.status !== 0 || !r || r.exit_code !== 0 || r.error !== null) {
    p.push(`build: kogen-build build exited ${c.status}${c.timedOut ? ' (timed out)' : ''}, want a landed Build; stdout ${JSON.stringify(c.stdout)}\nstderr:\n${c.stderr}`);
  }
  if (r && JSON.stringify((r.checks || []).map(x => [x.name, x.status])) !== JSON.stringify(want.checks)) {
    p.push(`build: the last pass's checks ${JSON.stringify((r.checks || []).map(x => [x.name, x.status]))}, want ${JSON.stringify(want.checks)}`);
  }
  if (c.commits !== 2) p.push(`build: the branch has ${c.commits} commits, want the baseline and one Build commit`);
  for (const [f, content] of Object.entries(FILES)) {
    if (c.committed[f] !== content) p.push(`build: committed ${f} is ${JSON.stringify(c.committed[f])}, want ${JSON.stringify(content)}`);
  }
  const kinds = c.requests.map(q => q.kind);
  if (JSON.stringify(kinds) !== JSON.stringify(want.requests)) p.push(`sequence: requests ${JSON.stringify(kinds)}, want ${JSON.stringify(want.requests)}`);
  const commits = c.requests.map(q => q.commits);
  if (JSON.stringify(commits) !== JSON.stringify(want.commitsAtRequest)) p.push(`sequence: commits on the branch when each request arrived ${JSON.stringify(commits)}, want ${JSON.stringify(want.commitsAtRequest)} (no commit before the final review)`);
  c.requests.forEach((q, i) => {
    if (q.kind !== 'builder') return;
    if (q.instructions !== texts.instructions) p.push(`instructions: builder request ${i + 1} does not carry ${INSTRUCTIONS} as it is in this checkout`);
    const n = q.instructions.split('\n').filter(l => l === texts.bullet).length;
    if (n !== 1) p.push(`instructions: builder request ${i + 1} carries the finish bullet ${n} times, want exactly once`);
  });
  if ('builderFeedback' in want) {
    const got = c.builder;
    const expected = want.builderFeedback + '\n\n' + texts.reminder;
    if (got !== expected) {
      const n = typeof got === 'string' ? got.split(texts.reminder).length - 1 : 0;
      p.push(`feedback: the builder's red-pass feedback (the output of its first finish call) carries the reminder line ${n} times` +
        `${typeof got === 'string' && got.startsWith(want.builderFeedback) ? '' : ' and does not start with the ordinary feedback'}` +
        `; it must be the ordinary feedback, a blank line and the reminder line\n got ${JSON.stringify(got)}\nwant ${JSON.stringify(expected)}`);
    }
  }
  if ('reviewerFeedback' in want) {
    const got = c.reviewer;
    if (got !== want.reviewerFeedback) p.push(`review feedback: the reviewer's repair feedback\n got ${JSON.stringify(got)}\nwant ${JSON.stringify(want.reviewerFeedback)}`);
    if (typeof got === 'string' && (got.includes(texts.reminder) || got.includes(texts.bullet.slice(2)))) p.push('review feedback: the reviewer\'s repair feedback carries the reminder or the finish statement');
  }
  return p;
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

function hashList(list) {
  return list.split('\n').filter(l => l.trim() !== '').map(l => l.trim().split(/\s+/));
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

// The observer controls run both scripted Builds on the current tree and judge them with the
// feature's expectations. An existing instructions bullet stands in for the finish bullet. The
// known-good variant is the real capture with the reminder line appended to the builder's
// red-pass feedback after a blank line, as the requested behaviour sends it; each known-bad
// variant must be rejected in its own area.
const STAND_IN = '- Do not commit, push, stash, reset or switch branches; Kogen commits after verification and review.';
const MODES = ['partial-then-wired', 'review-repair'];

function variants(captures, reminder) {
  const [partial, review] = captures;
  const withReminder = (c, f) => ({...c, builder: f(c.builder)});
  return {
    good: [
      ['partial-then-wired', withReminder(partial, s => s + '\n\n' + reminder)],
      ['review-repair', review],
    ],
    bad: [
      ['feedback', 'partial-then-wired', partial],
      ['feedback', 'partial-then-wired', withReminder(partial, s => reminder + '\n\n' + s)],
      ['feedback', 'partial-then-wired', withReminder(partial, s => s.split('\n').slice(0, -1).join('\n') + '\n\n' + reminder)],
      ['review feedback', 'review-repair', {...review, reviewer: review.reviewer + '\n\n' + reminder}],
      ['sequence', 'partial-then-wired', withReminder({...partial, requests: partial.requests.map((q, i) => (i === 1 ? {...q, commits: 2} : q))}, s => s + '\n\n' + reminder)],
    ],
  };
}

async function control(kind) {
  const texts = {bullet: STAND_IN, instructions: fs.readFileSync(INSTRUCTIONS, 'utf8'), reminder: docString('the reminder line is')};
  const wants = Object.fromEntries(MODES.map(m => [m, JSON.parse(docString(`the scripted Build "${m}" `))]));
  const captures = [];
  for (const m of MODES) captures.push(await scriptedBuild(m));
  const v = variants(captures, texts.reminder);
  if (kind === 'good') {
    const problems = v.good.map(([m, c]) => judge(c, wants[m], texts));
    if (problems.every(x => x.length === 0)) return console.log('observer accepted');
    problems.forEach((x, i) => console.error(`control-good ${v.good[i][0]}:\n${x.join('\n')}`));
    console.log('observer broken');
    process.exitCode = 2;
    return;
  }
  const outcomes = v.bad.map(([area, m, c]) => {
    const x = judge(c, wants[m], texts);
    return {area, x, ok: x.length > 0 && x.every(y => y.startsWith(area + ':'))};
  });
  if (outcomes.every(o => o.ok)) {
    console.log('observer rejected');
    process.exitCode = 1;
    return;
  }
  outcomes.forEach((o, i) => { if (!o.ok) console.error(`control-bad ${i + 1} (${o.area}):\n${o.x.join('\n') || 'accepted'}`); });
  console.log(outcomes.some(o => o.x.length === 0) ? 'observer accepted' : 'observer broken');
  process.exitCode = outcomes.some(o => o.x.length === 0) ? 0 : 2;
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

  Given(P + 'the finish bullet is', function (bullet) {
    if (bullet.includes('\n') || !bullet.startsWith('- ')) throw new Error('the finish bullet must be one bullet line: ' + JSON.stringify(bullet));
    this.kfwBullet = bullet;
  });

  Given(P + 'the reminder line is', function (reminder) {
    if (reminder.includes('\n') || reminder.trim() !== reminder || reminder === '') throw new Error('the reminder must be one line: ' + JSON.stringify(reminder));
    this.kfwReminder = reminder;
  });

  Then(P + 'build\\/prompts\\/builder.md is the instructions of sha256 {string} with the finish bullet inserted before the bullet starting {string}', function (base, anchor) {
    const text = fs.readFileSync(INSTRUCTIONS, 'utf8');
    const lines = text.split('\n');
    const at = lines.map((l, i) => (l === this.kfwBullet ? i : -1)).filter(i => i >= 0);
    const problems = [];
    if (at.length !== 1) problems.push(`the finish bullet is ${at.length} lines of ${INSTRUCTIONS}, want exactly 1`);
    const n = text.split(this.kfwBullet.slice(2)).length - 1;
    if (n !== 1) problems.push(`the finish statement occurs ${n} times in ${INSTRUCTIONS}, want exactly 1`);
    if (text.includes(this.kfwReminder)) problems.push(`${INSTRUCTIONS} holds the feedback reminder line; the instructions state the finish statement only as the bullet`);
    if (at.length === 1) {
      if (!(lines[at[0] + 1] || '').startsWith(anchor)) problems.push(`the line after the finish bullet is ${JSON.stringify(lines[at[0] + 1])}, want the bullet starting ${JSON.stringify(anchor)}`);
      const got = sha256(lines.filter((_, i) => i !== at[0]).join('\n'));
      if (got !== base) problems.push(`without the finish bullet ${INSTRUCTIONS} has sha256 ${got}, want ${base}: other instruction text changed`);
    }
    if (problems.length) throw new Error(problems.join('\n'));
  });

  Then(P + 'build\\/prompts\\/builder.md has {int} bytes and sha256 {string}', function (bytes, want) {
    const b = fs.readFileSync(INSTRUCTIONS);
    if (b.length !== bytes || sha256(b) !== want) throw new Error(`${INSTRUCTIONS}: ${b.length} bytes sha256 ${sha256(b)}, want ${bytes} bytes sha256 ${want}`);
  });

  Then(P + 'build\\/testdata\\/prompts_golden.json records the bytes and sha256 of build\\/prompts\\/builder.md', function () {
    const golden = JSON.parse(fs.readFileSync('build/testdata/prompts_golden.json', 'utf8'));
    const b = fs.readFileSync(INSTRUCTIONS);
    const want = {builder: {bytes: b.length, sha256: sha256(b)}};
    if (JSON.stringify(golden) !== JSON.stringify(want)) throw new Error(`build/testdata/prompts_golden.json holds ${JSON.stringify(golden)}, want ${JSON.stringify(want)}`);
  });

  Then(P + 'with the finish bullet and the reminder line taken out, these golden files keep their bytes', function (list) {
    // Both texts are plain ASCII without characters JSON escapes. In a golden the bullet is
    // followed by its line break, and the reminder follows the ordinary feedback after a blank
    // line; each is kept raw, escaped once (a JSON string) or escaped twice (JSON text kept in a
    // JSON string).
    const removals = [
      ['finish bullet', this.kfwBullet.slice(2), [this.kfwBullet + '\\\\n', this.kfwBullet + '\\n', this.kfwBullet + '\n']],
      ['reminder line', this.kfwReminder, ['\\\\n\\\\n' + this.kfwReminder, '\\n\\n' + this.kfwReminder, '\n\n' + this.kfwReminder]],
    ];
    const problems = [];
    for (const [want, file] of hashList(list)) {
      let text;
      try {
        text = fs.readFileSync(file, 'utf8');
      } catch (e) {
        problems.push(`${file}: unreadable (${e.message})`);
        continue;
      }
      const held = [];
      for (const [name, plain, forms] of removals) {
        let n = 0;
        for (const form of forms) {
          n += text.split(form).length - 1;
          text = text.split(form).join('');
        }
        if (n > 0) held.push(name);
        if (text.includes(plain)) problems.push(`${file}: records the ${name} other than in its place`);
      }
      const wanted = file.endsWith('named-feedback-compat.json') ? ['finish bullet', 'reminder line'] : ['finish bullet'];
      for (const name of wanted) if (!held.includes(name)) problems.push(`${file}: does not record the ${name}`);
      const got = sha256(text);
      if (got !== want) problems.push(`${file}: without the added lines sha256 ${got}, want ${want}: it changed other than by the finish bullet and the reminder line`);
    }
    if (problems.length) throw new Error(problems.join('\n'));
  });

  Then(P + 'these golden files keep their bytes', function (list) {
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

  Then(P + 'the existing test {string} passes in its own go test run', {timeout: GO_TIMEOUT_MS + 60000}, function (name) {
    const r = goTest(name);
    if (r.outcome !== 'passed') throw new Error(r.text);
  });

  Then(P + 'the scripted Build {string} meets these expectations', {timeout: GO_TIMEOUT_MS + RUN_TIMEOUT_MS + 60000}, async function (mode, expect) {
    if (!SCRIPTS[mode]) throw new Error(`no scripted Build ${JSON.stringify(mode)}`);
    const c = await scriptedBuild(mode);
    const p = judge(c, JSON.parse(expect), {bullet: this.kfwBullet, instructions: fs.readFileSync(INSTRUCTIONS, 'utf8'), reminder: this.kfwReminder});
    if (p.length) throw new Error(`kogen-build build of the fixture project, scripted Build "${mode}":\n${p.join('\n')}`);
  });
}
