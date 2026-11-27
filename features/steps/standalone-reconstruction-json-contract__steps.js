'use strict';
const { Given, When, Then } = require('@cucumber/cucumber');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert');

// Each mutation breaks one property of cmd/kogen-build/main.go. The anchors are the
// current source text of the command, so a missing anchor fails loudly.
const mutations = {
  'the readback exits 0 for an item with no recorded reconstruction': [
    'rec, r.ExitCode, p = build.ReadReconstruction(*project, args.item, s.Env)',
    'rec, _, p = build.ReadReconstruction(*project, args.item, s.Env)',
  ],
  'the readback creates custody state in the checkout': [
    'rec, r.ExitCode, p = build.ReadReconstruction(*project, args.item, s.Env)',
    '_ = os.MkdirAll(filepath.Join(*project, ".kogen", "local"), 0o700)\n\t\t\trec, r.ExitCode, p = build.ReadReconstruction(*project, args.item, s.Env)',
  ],
  'reconstruct reports an interrupted run as held': [
    'StopAfter: args.stopAfter})',
    'StopAfter: args.stopAfter})\n\t\t\tif rec.State == "interrupted" {\n\t\t\t\trec.State = "held"\n\t\t\t}',
  ],
  'reconstruct ignores the stop-after value': [
    'StopAfter: args.stopAfter',
    'StopAfter: ""',
  ],
  'the command accepts an invalid stop-after value': [
    'if refusal != "" {',
    'if refusal != "" && !strings.Contains(refusal, "--stop-after") {',
  ],
  'the command creates the archive before refusing an invalid stop-after value': [
    'args, refusal := parseArgs(os.Args[1:], os.Stderr)',
    'for i, a := range os.Args {\n\t\tif a == "--stop-after" && i+1 < len(os.Args) && os.Args[i+1] != "preserved" && os.Args[i+1] != "fenced" {\n\t\t\tfor j, b := range os.Args {\n\t\t\t\tif b == "--archive" && j+1 < len(os.Args) {\n\t\t\t\t\t_ = os.MkdirAll(os.Args[j+1], 0o700)\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n\targs, refusal := parseArgs(os.Args[1:], os.Stderr)',
  ],
};

let outcome;

function goTest(env) {
  const r = spawnSync('go', ['test', '-count=1', '-timeout', '45s', './cmd/kogen-build'], {
    cwd: process.cwd(),
    encoding: 'utf8',
    env,
    timeout: 52000,
  });
  return { status: r.status, output: (r.stdout || '') + (r.stderr || '') };
}

Given('standalone-reconstruction-json-contract: the kogen-build checkout', function () {
  assert.ok(fs.existsSync(path.join(process.cwd(), 'cmd', 'kogen-build', 'main.go')), 'cmd/kogen-build/main.go is missing');
});

When('standalone-reconstruction-json-contract: the command package tests run unmodified and with the {string} mutation of main.go', { timeout: 115000 }, function (name) {
  assert.ok(mutations[name], 'unknown mutation ' + name);
  const main = path.join(process.cwd(), 'cmd', 'kogen-build', 'main.go');
  const source = fs.readFileSync(main, 'utf8');
  const [anchor, replacement] = mutations[name];
  assert.ok(source.includes(anchor), 'main.go lacks the mutation anchor ' + JSON.stringify(anchor));
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'standalone-reconstruction-mutation-'));
  try {
    fs.writeFileSync(path.join(dir, 'main.go'), source.replace(anchor, replacement));
    fs.writeFileSync(path.join(dir, 'overlay.json'), JSON.stringify({ Replace: { [main]: path.join(dir, 'main.go') } }));
    const base = spawnSync('go', ['env', 'GOFLAGS'], { cwd: process.cwd(), encoding: 'utf8' }).stdout.trim();
    outcome = {
      name,
      unmodified: goTest({ ...process.env, GOFLAGS: base }),
      // The tests build the command with go build, which inherits this overlay.
      mutated: goTest({ ...process.env, GOFLAGS: (base + ' -overlay=' + path.join(dir, 'overlay.json')).trim() }),
    };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

Then('standalone-reconstruction-json-contract: the unmodified tests pass and the mutated tests fail on a test assertion', function () {
  const { name, unmodified, mutated } = outcome;
  assert.strictEqual(unmodified.status, 0, 'go test ./cmd/kogen-build failed on the unmodified tree:\n' + unmodified.output.slice(0, 3000));
  assert.match(unmodified.output, /^ok\s+github\.com\/KogenAI\/kogen-build\/cmd\/kogen-build\s/m);
  assert.notStrictEqual(mutated.status, 0, 'go test ./cmd/kogen-build passed with the mutation "' + name + '": the command-level tests do not detect it:\n' + mutated.output.slice(0, 3000));
  assert.match(mutated.output, /^--- FAIL: /m, 'go test ./cmd/kogen-build failed with the mutation "' + name + '" without a failing test:\n' + mutated.output.slice(0, 3000));
});
