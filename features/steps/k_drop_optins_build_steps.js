'use strict';
// Steps of k-drop-optins-build. kogen-build must build, vet and pass its tests against the
// coordinator without the opt-in options K-DROP-OPTINS removes. That coordinator is synthesized
// from the pinned one in a temporary folder: every removed exported identifier becomes unexported
// (no other module can name it, and JSON omits it) and every removed project.yaml key becomes
// unknown (a file that sets it is refused). The behaviour cases run the overlay test
// k_drop_optins_build_test.go.txt against the pinned coordinator.
//   node features/steps/k_drop_optins_build_steps.js good|bad   observer controls of the synthesis
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert/strict');

const COORDINATOR = 'github.com/KogenAI/kogen-coordinator';
const REMOVED_IDENTIFIERS = ['MaxRounds', 'DefaultExternalRounds', 'ShapeWorkers', 'Workers', 'ShapeAudit', 'Audit', 'ShapeDecisionCheck', 'DecisionCheck', 'ReviewMaterial'];
const REMOVED_KEYS = ['max_rounds', 'shape_workers', 'shape_audit', 'shape_decision_check', 'review_material'];
const LIMIT = 40 * 60 * 1000;

function run(cmd, args, opts = {}) {
  const r = spawnSync(cmd, args, { encoding: 'utf8', timeout: LIMIT, maxBuffer: 64 * 1024 * 1024, ...opts });
  const status = r.error ? `error ${r.error.message}` : r.signal ? `signal ${r.signal}` : `exit ${r.status}`;
  return { ok: r.status === 0 && !r.error, status, stdout: r.stdout || '', out: (r.stdout || '') + (r.stderr || '') };
}

// pinnedCoordinator is the module folder of the coordinator root's go.mod pins.
function pinnedCoordinator(root) {
  const r = run('go', ['list', '-m', '-json', COORDINATOR], { cwd: root });
  if (!r.ok) throw new Error(`go list -m -json ${COORDINATOR}: ${r.status}\n${r.out}`);
  const dir = JSON.parse(r.stdout).Dir;
  if (!dir || !fs.existsSync(path.join(dir, 'project', 'config.go'))) throw new Error(`no pinned coordinator source: ${r.stdout}`);
  return dir;
}

// synthesize writes the pinned coordinator without the opt-in options to dir.
function synthesize(root, dir) {
  const ident = new RegExp(`\\b(${REMOVED_IDENTIFIERS.join('|')})\\b`, 'g');
  const key = new RegExp(`"(${REMOVED_KEYS.join('|')})"`, 'g');
  let changed = 0;
  const copy = (from, to) => {
    fs.mkdirSync(to, { recursive: true });
    for (const e of fs.readdirSync(from, { withFileTypes: true })) {
      const a = path.join(from, e.name);
      const b = path.join(to, e.name);
      if (e.isDirectory()) {
        copy(a, b);
      } else if (e.name.endsWith('.go') && !e.name.endsWith('_test.go')) {
        const text = fs.readFileSync(a, 'utf8');
        const next = text.replace(ident, 'removed$1').replace(key, '"removed:$1"');
        if (next !== text) changed++;
        fs.writeFileSync(b, next);
      } else if (e.isFile()) {
        fs.writeFileSync(b, fs.readFileSync(a));
      }
    }
  };
  copy(pinnedCoordinator(root), dir);
  if (changed === 0) throw new Error('the pinned coordinator has none of the removed options');
}

// goChecks runs go build, go vet and go test -count=1 of every package of the module at dir
// against the coordinator at coordinator (a replace in a temporary copy of its go.mod), stopping
// at the first failure. It returns the last command run: the go test run when all three pass.
function goChecks(dir, coordinator) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kdo-mod-'));
  try {
    const modfile = path.join(tmp, 'go.mod');
    fs.copyFileSync(path.join(dir, 'go.mod'), modfile);
    fs.copyFileSync(path.join(dir, 'go.sum'), path.join(tmp, 'go.sum'));
    const edit = run('go', ['mod', 'edit', `-replace=${COORDINATOR}=${coordinator}`, modfile], { cwd: dir });
    if (!edit.ok) throw new Error(`go mod edit: ${edit.status}\n${edit.out}`);
    const used = run('go', ['list', '-modfile=' + modfile, '-m', '-json', COORDINATOR], { cwd: dir });
    if (!used.ok || (JSON.parse(used.stdout).Replace || {}).Path !== coordinator) {
      throw new Error(`the synthetic coordinator is not the one in use: ${used.status}\n${used.out}`);
    }
    let last;
    for (const [verb, ...rest] of [['build', './...'], ['vet', './...'], ['test', '-count=1', './...']]) {
      const args = [verb, '-modfile=' + modfile, ...rest];
      const r = run('go', args, { cwd: dir });
      last = { ok: r.ok, command: `go ${args.join(' ')}`, status: r.status, out: r.out };
      if (!r.ok) break;
    }
    return last;
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

// fixture writes a small module that uses the coordinator as kogen-build's go.mod pins it.
function fixture(root, dir, source, test) {
  fs.mkdirSync(dir, { recursive: true });
  const mod = fs.readFileSync(path.join(root, 'go.mod'), 'utf8').replace(/^module .*$/m, 'module kdofixture');
  fs.writeFileSync(path.join(dir, 'go.mod'), mod);
  fs.copyFileSync(path.join(root, 'go.sum'), path.join(dir, 'go.sum'));
  fs.writeFileSync(path.join(dir, 'fixture.go'), `package fixture\n\nimport "${COORDINATOR}/project"\n\n${source}\n`);
  fs.writeFileSync(path.join(dir, 'fixture_test.go'), `package fixture\n\nimport (\n\t"testing"\n\n\t"${COORDINATOR}/project"\n)\n\n${test}\n`);
}

const parses = (yaml) => `func TestParses(t *testing.T) {\n\tif _, err := project.ParseConfig(${JSON.stringify(yaml)}, "f"); err != nil {\n\t\tt.Fatal(err)\n\t}\n}`;
const kept = 'func Kept(c *project.Config) (int, string, string) {\n\tr := c.Models["build"]\n\treturn len(c.Checks), r.Executor, r.Model\n}';
const goodFixture = {
  source: kept,
  test: 'func TestKept(t *testing.T) {\n\tc, err := project.ParseConfig("version: 1\\nmodels:\\n  build: {executor: claude-code, model: b, effort: high}\\n", "f")\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif n, e, m := Kept(c); n != 0 || e != "claude-code" || m != "b" {\n\t\tt.Fatal(n, e, m)\n\t}\n}',
};
const badFixtures = {
  'Role.MaxRounds': 'var _ = project.Role{}.MaxRounds',
  DefaultExternalRounds: 'var _ = project.DefaultExternalRounds',
  'Config.ShapeWorkers': 'var _ = project.Config{}.ShapeWorkers',
  Workers: 'var _ project.Workers',
  'Config.ShapeAudit': 'var _ = project.Config{}.ShapeAudit',
  Audit: 'var _ project.Audit',
  'Config.ShapeDecisionCheck': 'var _ = project.Config{}.ShapeDecisionCheck',
  DecisionCheck: 'var _ project.DecisionCheck',
  'Config.ReviewMaterial': 'var _ = project.Config{}.ReviewMaterial',
  ReviewMaterial: 'var _ project.ReviewMaterial',
  'models.build.max_rounds': parses('version: 1\nmodels:\n  build: {executor: claude-code, model: b, effort: high, max_rounds: 2}\n'),
  shape_workers: parses('version: 1\nshape_workers:\n  enabled: false\n'),
  shape_audit: parses('version: 1\nshape_audit:\n  enabled: false\n'),
  shape_decision_check: parses('version: 1\nshape_decision_check:\n  enabled: false\n'),
  review_material: parses('version: 1\nreview_material:\n  mode: parts\n'),
};

// control is the observer control: the known-good fixture passes against the synthetic
// coordinator, and each known-bad fixture (one removed identifier or key each) fails.
function control(mode) {
  const root = process.cwd();
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kdo-control-'));
  try {
    const coordinator = path.join(tmp, 'coordinator');
    synthesize(root, coordinator);
    if (mode === 'good') {
      fixture(root, path.join(tmp, 'good'), goodFixture.source, goodFixture.test);
      const r = goChecks(path.join(tmp, 'good'), coordinator);
      return r.ok ? 'observer accepted' : `observer rejected ${r.command}: ${r.status}\n${r.out}`;
    }
    for (const [name, body] of Object.entries(badFixtures)) {
      const dir = path.join(tmp, 'bad-' + name.replace(/\W/g, '-'));
      const isTest = body.startsWith('func Test');
      fixture(root, dir, isTest ? kept : body, isTest ? body : 'func TestUses(t *testing.T) {\n\t_ = project.Config{}\n}');
      if (goChecks(dir, coordinator).ok) return `observer accepted ${name}`;
    }
    return 'observer rejected';
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

// overlayTest runs one test of the overlay against the pinned coordinator.
function overlayTest(name, env) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kdo-overlay-'));
  try {
    const overlay = path.join(tmp, 'overlay.json');
    fs.writeFileSync(overlay, JSON.stringify({ Replace: { [path.resolve('build/k_drop_optins_build_acceptance_test.go')]: path.resolve('features/steps/k_drop_optins_build_test.go.txt') } }));
    const args = ['test', '-overlay', overlay, './build', '-run', `^${name}$`, '-count=1', '-v'];
    // Go matches overlay paths against its working directory as $PWD names it.
    const r = run('go', args, { cwd: process.cwd(), env: { ...process.env, ...env, PWD: process.cwd() } });
    const what = `go ${args.join(' ')} (${Object.entries(env).map(([k, v]) => `${k}=${v}`).join(' ')}): ${r.status}`;
    // The first line states whether the test ran and its own verdict and assertions, then the full output.
    const lines = r.out.split('\n');
    const verdict = lines.find((l) => /^--- (PASS|FAIL|SKIP): /.test(l) && l.includes(` ${name} (`));
    const ran = verdict || (lines.some((l) => l === `=== RUN   ${name}`) ? `=== RUN   ${name} (no verdict)` : `${name} did not run (absent, skipped or run zero times)`);
    const assertions = lines.filter((l) => /^\s+k_drop_optins_build_acceptance_test\.go:\d+: /.test(l)).map((l) => l.trim());
    if (!r.ok || !verdict || !verdict.startsWith(`--- PASS: ${name} (`)) {
      throw new Error(`${ran}; ${assertions.length ? assertions.join('; ') : 'no assertion output'}; ${what}\n${r.out}`);
    }
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

if (require.main === module) {
  let line;
  try {
    line = control(process.argv[2]);
  } catch (e) {
    line = `observer error ${e.message}`;
  }
  console.log(line);
  process.exitCode = line === 'observer accepted' ? 0 : 1;
} else {
  const { Given, Then } = require('@cucumber/cucumber');

  Given('the coordinator kogen-build pins, synthesized without max_rounds, shape_workers, shape_audit, shape_decision_check and review_material', { timeout: 10 * 60 * 1000 }, function () {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kdo-synthetic-'));
    try {
      synthesize(process.cwd(), path.join(tmp, 'coordinator'));
    } catch (e) {
      fs.rmSync(tmp, { recursive: true, force: true });
      throw e;
    }
    this.kdoSynthetic = tmp;
  });

  Then('go build, go vet and go test of every kogen-build package pass against it', { timeout: LIMIT + 5 * 60 * 1000 }, function () {
    try {
      const r = goChecks(process.cwd(), path.join(this.kdoSynthetic, 'coordinator'));
      if (!r.ok) throw new Error(`against the coordinator without the opt-in options, ${r.command}: ${r.status}\n${r.out}`);
      assert.ok(/^ok\s+github\.com\/KogenAI\/kogen-build\/build\s/m.test(r.out), `${r.command}: ${r.status}, but it did not report the build package ok:\n${r.out}`);
    } finally {
      fs.rmSync(this.kdoSynthetic, { recursive: true, force: true });
    }
  });

  Then('a Build whose claude-code {word} role sets max_rounds {int} under an executor cap of {int} rounds for that role stops executor_round_limit after {int} {word} rounds', { timeout: 20 * 60 * 1000 }, function (role, set, cap, want, again) {
    assert.equal(again, role, `the role is named twice: ${role} and ${again}`);
    overlayTest('TestKDropOptinsBuildRounds', { KDO_ROLE: role, KDO_SET: String(set), KDO_CAP: String(cap), KDO_WANT: String(want) });
  });

  Then('resume compares a saved configuration that differs only in {string} as the {word} configuration', { timeout: 20 * 60 * 1000 }, function (field, same) {
    overlayTest('TestKDropOptinsBuildSameConfig', { KDO_FIELD: field, KDO_SAME: same });
  });

  Then('a Build of a project whose project.yaml sets review_material mode on_demand lands after a review with parts material and the plain reviewer prompt', { timeout: 20 * 60 * 1000 }, function () {
    overlayTest('TestKDropOptinsBuildPartsReview', {});
  });

  Then('no non-test Go file under build names the on_demand review material mode', function () {
    const named = fs.readdirSync('build').filter((f) => f.endsWith('.go') && !f.endsWith('_test.go'))
      .filter((f) => /on_demand|onDemand|OnDemand/.test(fs.readFileSync(path.join('build', f), 'utf8')));
    assert.deepEqual(named, [], `build files still naming the on_demand mode: ${named.join(', ')}`);
  });
}
