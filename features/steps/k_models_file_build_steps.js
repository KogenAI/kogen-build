'use strict';
// Steps of k-models-file-build: kogen-build's own Go tests run with the committed go.mod pin and
// against the K-MODELS-FILE coordinator candidate 2a4c69a6 (harness in k_models_file_build/), and
// the recorded goldens are compared by digest. A failure names each required test that did not
// pass with the command, its outcome and the lines go test printed for it.
const { When, Then } = require('@cucumber/cucumber');
const h = require('./k_models_file_build/harness.js');

function names(table) {
  return table.hashes().map((row) => row.test);
}

function topLevel(required) {
  return [...new Set(required.map((n) => n.split('/')[0]))];
}

Then('go.mod still requires github.com\\/KogenAI\\/kogen-coordinator at {word} without replacing it', function (version) {
  const mod = h.goMod();
  const req = (mod.Require || []).find((r) => r.Path === h.coordinator);
  if (!req || req.Version !== version) {
    throw new Error(`go.mod requires ${h.coordinator} ${req ? req.Version : '(nothing)'}, want ${version}`);
  }
  const replaced = (mod.Replace || []).filter((r) => r.Old.Path === h.coordinator);
  if (replaced.length > 0) {
    throw new Error(`go.mod replaces ${h.coordinator}: ${JSON.stringify(replaced)}`);
  }
});

When('these tests of package build run with the committed go.mod:', { timeout: 100 * 1000 }, function (table) {
  this.required = names(table);
  this.result = h.goTest(topLevel(this.required), null, `coordinator ${h.pin} from go.mod`);
});

When('these tests of package build run against kogen-coordinator 2a4c69a6:', { timeout: 100 * 1000 }, function (table) {
  this.required = names(table);
  this.result = h.goTest(topLevel(this.required), h.candidateModfile(),
    'coordinator 2a4c69a6 made from the pinned one and coordinator-2a4c69a6.patch');
});

Then('each of those tests passed', function () {
  h.report(this.result, this.required);
});

Then('these files have exactly these SHA-256 digests:', function (table) {
  const found = h.digestProblems(table.hashes());
  if (found.length > 0) {
    throw new Error(`file digests differ:\n${found.join('\n')}`);
  }
});
