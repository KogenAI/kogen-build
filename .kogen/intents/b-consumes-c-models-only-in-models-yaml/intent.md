# Pin coordinator that reads roles from models.yaml

Slug: b-consumes-c-models-only-in-models-yaml
Model: not applicable — only the producer pin, test code, test fixtures and two recorded goldens change; kogen-build gains no state or rules
Acceptance:
- .kogen/intents/b-consumes-c-models-only-in-models-yaml/acceptance.feature
- features/steps/b-consumes-c-models-only-in-models-yaml__steps.js
- features/steps/b-consumes-c-models-only-in-models-yaml__control.js

Consumer row of kogen-coordinator's c-models-only-in-models-yaml: commit `d74daae36f56d4e195d6ff04e79e74eab1530bb7`, "Read model roles only from the models files", trailer `Kogen-Intent: c-models-only-in-models-yaml`. That change removes a contract kogen-build relies on, on purpose. There is no compatibility layer and none is wanted. Its rules, as its code states them:
- `project.ParseConfig`, `LoadConfig` and `ReadableConfig` refuse a root `models:` key in `.kogen/project.yaml`. The error is `environment/project_config_invalid`, exit 3. Its detail is the file's path plus one line, `line <N>: models is no longer read from .kogen/project.yaml; move the model lineup to .kogen/models.yaml`.
- `LoadConfig` takes every role only from the models files, in this order: `project.DefaultModels()`, then `.kogen/models.yaml`, then `.kogen/local/models.yaml`. Each role a file names replaces the whole role. A models file is a root mapping whose only key is `models`, with the roles under it (the format `project.ModelsTemplate()` writes).
- `RenderConfig` and `Template` write no `models:` block.

This repository follows the new contract only. After this change, every test in the module passes with that producer pinned. No test or code accepts a `models:` key in `.kogen/project.yaml`, and no fallback is added. Production code does not change. It reads the configuration only through `project.LoadConfig` and the baseline only through `project.Baseline`, and it writes no lineup.

## What holds now (HEAD ea52949, checked against the producer in a scratch copy)
- `go.mod` pins `github.com/KogenAI/kogen-coordinator v0.0.0-20261105001101-4116df40cbf1`.
- `build/lineup_layout_test.go` keeps two layouts and picks one through `lineupMoved`. `lineupMoved` is true only when `ParseConfig` refuses `models:` with reason `models_moved`. That reason belonged to an earlier, never-landed producer candidate. The real producer refuses with `project_config_invalid`, so `lineupMoved` stays false and every test takes the old branch.
  - `(*env).setRole` rewrites the role in `.kogen/project.yaml`'s `models:` block. `RenderConfig` no longer writes that block, so the fixture fails with `no build role in:` / `no review role in:`.
  - `loadLineup` calls `ParseConfig` with `models:`, which is now refused.
  - The never-taken "moved" branch writes roles at the top level of `.kogen/models.yaml`, without the `models:` root key. The producer refuses that file too: observed `environment/project_config_invalid: …/.kogen/models.yaml`.
- The Elixir fixtures keep `models:` at line 17 of their `.kogen/project.yaml`: `build/testdata/legacy/project`, `build/testdata/legacy76/project` and `build/testdata/legacy76/staged-project`. Resuming their checkpoints now exits 3 with:
  ```
  environment/project_config_invalid: <checkout>/.kogen/project.yaml
    line 17: models is no longer read from .kogen/project.yaml; move the model lineup to .kogen/models.yaml
    no newly created Build run; correct the named prerequisite before trying again
  ```
  `legacyMovedLineupRefusal` (`build/legacy_test.go`) expects the old candidate's `environment/models_moved: …` text instead, and it runs only when `lineupMoved` is true.
- `TestStateGoldenReviewApproveRecord` and `TestStateGoldenReviewChangesRecord` record a real reviewed Build in a fixture whose `.kogen/project.yaml` is `RenderConfig` output. Without `models:` that fixture's candidate tree is `70f9bc85dd20ab36f77bff2605ddb64c3a5678cf`, not `72ecb9382e879020c31bd3dea04dd9fb3ad3e451`. `review-1-moved.json` beside each golden belongs to the old candidate and is stale: it lacks `"requirements":[]`.
- `consumes/kogen-coordinator/project_fixture_test.go` `fixtureConfig` has a `models:` block (build `{provider: anthropic, model: build-model, effort: high}`, review `{executor: claude-code, model: review-model, effort: max}`). Its `.kogen/models.yaml` is `project.ModelsTemplate()`.

Against the new producer, these fail: the 61 top-level `./build` tests in the acceptance scenarios, and `TestBaselineAndHash`, `TestLoadConfigReadsTheProjectConfiguration` and `TestWritePackageFacts` in `./consumes/kogen-coordinator`. All other packages pass.

## Required end state
1. Pin. Move the pin with `go get github.com/KogenAI/kogen-coordinator@main`, run against the module mirror, then `go mod tidy`. When main is the producer commit above, that resolves to `github.com/KogenAI/kogen-coordinator v0.0.0-20261106110407-d74daae36f56` (no `replace`). In `go.mod` only that require line changes. In `go.sum` the two lines of `v0.0.0-20261105001101-4116df40cbf1` are replaced by:
   ```
   github.com/KogenAI/kogen-coordinator v0.0.0-20261106110407-d74daae36f56 h1:8VmqsU9EArCYwqOTw8e9uuS54nRSlUHSUaDl8W+lYqs=
   github.com/KogenAI/kogen-coordinator v0.0.0-20261106110407-d74daae36f56/go.mod h1:MMc15p3YwlY71dMxfCdX6QlR2jXJbSxL6YA9Y7971c8=
   ```
   Nothing else changes in either file. The other four Kogen pins stay. If the mirror cannot serve that version, stop and report it as an environment failure. Do not use a `replace`, a `-modfile` or a vendored copy.
2. One lineup layout. Delete `lineupMoved` and every branch on it. Each test keeps only the behaviour of the new contract:
   - `(*env).setRole(role, entry)` takes the role's lines as today's callers pass them, indented two spaces (`  build:\n    model: m\n    effort: high\n` or `  build: {provider: anthropic, model: claude-sonnet-5-5, effort: high}\n`). It writes them under the root `models:` key of the fixture's `.kogen/models.yaml`: it starts the file as `models:\n` when the file is absent, replaces that role's entry if one is there, and keeps the other roles. It then stages the file. It never writes `.kogen/project.yaml`.
   - `loadLineup(t, models)` writes a temporary project with `.kogen/project.yaml` = `version: 1\n` and `.kogen/models.yaml` = `models:\n` + models, and returns `project.LoadConfig` of it. `Config.Models` and every assertion of its callers stay as they are.
   - Delete helpers left without callers, such as `movedRoleEntryRe` and `dedent`.
   - `lineupFileForms`, `trackedLineupFileForms`, `withoutLineupFile` and `trackLineup` stay. They describe the prompt bytes, not the layout.
3. Elixir resumes follow the new contract. The legacy refusal helper asserts, for a resume of an Elixir checkpoint whose fixture `.kogen/project.yaml` still has `models:`:
   - exit 3;
   - output containing `environment/project_config_invalid: ` followed by the path of the checkout's `.kogen/project.yaml`;
   - the line `line 17: models is no longer read from .kogen/project.yaml; move the model lineup to .kogen/models.yaml`;
   - `  no newly created Build run;`;
   - and, as today, unchanged custody, unchanged Elixir run files, and no new run or request.

   It runs unconditionally in:
   - the five `TestALegacy*` tests that fail now;
   - the `TestLegacyResumeRefusals` cases marked `config` (refused only after the configuration loads);
   - all nine `TestMain76BuildsResumeAndLand` subtests;
   - both `TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer` subtests (`review_retry_verify`, `review_verify`).

   That last test fails against the producer too, although the request's failure list omits it: it passes against coordinator `c8126f4`. Its custody-drift part, which refuses before the configuration loads, keeps its assertions. The landing assertions after the old `if lineupMoved { …; return }` become unreachable and are deleted (go vet's unreachable check would flag them), along with helpers and scenario scripts left without callers. Test names stay. Legacy cases refused before the configuration loads keep today's expectations. The Elixir fixtures, checkpoints, manifests and custody files stay byte-identical.
4. State goldens. `stateGoldenRecordPath` goes. Both tests compare with `testdata/state/review-<verdict>/review-1.json`. Regenerate those two with `go test ./build -run '^(TestStateGoldenReviewApproveRecord|TestStateGoldenReviewChangesRecord)$' -update`, then run without the flag. Each new file is the old one with both tree ids changed from `72ecb9382e879020c31bd3dea04dd9fb3ad3e451` to `70f9bc85dd20ab36f77bff2605ddb64c3a5678cf` (SHA-256 `67751b61…5b08` for approve and `49a5dcc9…a66e` for changes). Delete both `review-1-moved.json` files. Every other file under `build/testdata` stays byte-identical. That includes `named-feedback-compat.json`, `reviewretry/*.wire.json`, `context-external-golden.json`, the checkpoint goldens and the Elixir fixtures. Do not run `-update` for the whole suite.
5. Coordinator consumer suite. `fixtureConfig` has no `models:` block. `newProject` writes `.kogen/models.yaml` exactly as:
   ```
   models:
     build: {provider: anthropic, model: build-model, effort: high}
     review: {executor: claude-code, model: review-model, effort: max}
   ```
   It writes that instead of `project.ModelsTemplate()`. `TestLoadConfigReadsTheProjectConfiguration` keeps every assertion, and the roles it checks now come from that file. `TestBaselineAndHash` still lists `.kogen/models.yaml` right after `.kogen/project.yaml`. `consumes/README.md` changes only if an item this suite names changes. None is expected to.
6. Comments describe the one layout: `build/lineup_layout_test.go`, the header and record-path comments of `build/state_golden_test.go`, the legacy refusal helper and the `config` field of `TestLegacyResumeRefusals`, `project_fixture_test.go`, and `TestLoadConfigReadsTheProjectConfiguration` (the roles come from `.kogen/models.yaml`). They drop any mention of the `models:` block in `.kogen/project.yaml` or of `models_moved`.
7. Unchanged:
   - production code (`build/*.go` other than tests, and `cmd/`);
   - `contract/`;
   - `legacyAdaptBaseline`, which follows the coordinator's own baseline rule, a different contract;
   - every other test and assertion;
   - `README.md` and `docs/`;
   - this repository's own `.kogen/project.yaml`, which is protected and read only by the host's installed Kogen, not by any test.

   No test is skipped and no timeout is raised. The configured checks (`format`, `compile`, `vet`, `go-test`) pass with the new pin.

## Decisions
- Pin github.com/KogenAI/kogen-coordinator v0.0.0-20261106110407-d74daae36f56 (commit d74daae, Kogen-Intent c-models-only-in-models-yaml), moved with go get against the mirror and go mod tidy; no replace, -modfile or vendored copy (decision: D1, round 1) (constraint: go.mod changes only the coordinator require line; go.sum only swaps the coordinator's two lines) (constraint: an unavailable version is an honest environment stop) (evidence: go.mod; go.sum) (assumption: Decided under external scratch observations that are not Kogen probe records: at shaping time the mirror's main was still c8126f4, and the version string and go.sum hashes come from a scratch go get that served d74daae as main. The Build depends on the mirror's main being d74daae when it runs go get.) (source: request)
- Delete the dual-layout switch lineupMoved; setRole and loadLineup write roles under the root models: key of .kogen/models.yaml only (decision: D2, round 1) (constraint: no test writes models: into .kogen/project.yaml) (constraint: no Go file holds lineupMoved, models_moved or review-1-moved) (evidence: build/lineup_layout_test.go; build/helper_test.go) (assumption: The producer's refusal reason and models-file format were read from the producer's landing commit and observed in an external scratch run, not in a Kogen probe of this shaping.) (source: request)
- Elixir legacy resumes assert the producer's project_config_invalid refusal (line 17 models: message, exit 3, nothing changed); the Elixir fixtures stay byte-identical; unreachable landing assertions are deleted (decision: D3, round 1) (evidence: build/legacy_test.go; build/legacy76_test.go; build/testdata/legacy/project/.kogen/project.yaml) (source: request)
- Only the two review-record goldens change (tree 72ecb938 -> 70f9bc85); review-1-moved.json files are deleted; every other build/testdata file stays byte-identical (decision: D4, round 1) (evidence: build/state_golden_test.go; build/testdata/state/review-approve/review-1.json; build/testdata/state/review-changes/review-1.json) (assumption: The new tree id and golden SHA-256 values come from an external scratch run against d74daae, not from a Kogen probe.) (source: request)
- Nothing else changes; every other test stays green (source: request)

## Source comparison
- Addressed: "move this repository's go.mod pin of the producer to that main (go get against the mirror) in the same change". Rule 1; scenario 1 checks the exact `go.mod`/`go.sum` bytes, and scenarios 2–5 check that the run used `v0.0.0-20261106110407-d74daae36f56` without a replacement. When this was shaped, the mirror's main was still `c8126f4`, and d74daae was the producer's landing commit that produced the reported failures. In a scratch copy that served d74daae as main, `go get github.com/KogenAI/kogen-coordinator@main` resolved to exactly that version and `go.sum` line.
- Addressed: every reported failure. These are `TestBaselineAndHash`, `TestLoadConfigReadsTheProjectConfiguration` and `TestWritePackageFacts` (`project_config_invalid: project.yaml`); the External*, ClaudeLineup*, ContextExternal*, ReviewInParts*, KReviewRetry*, LoopKnobs*, NoProgress, UsageLimit, Unmoved, Anthropic and WholeIntent tests (`no build role in` / `no review role in` / `project_config_invalid: f`); the `TestALegacy*`, `TestLegacyResumeRefusals` and `TestMain76BuildsResumeAndLand` cases (`exit 3 … project_config_invalid`); and the two `TestStateGoldenReview*Record` tests. Rules 2–5; scenarios 2–5 run every one of them by name.
- Addressed, beyond the list: `TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer` (2 subtests, `legacy76_test.go:568: exit 3 … project_config_invalid`) also fails against d74daae. Rule 3; scenario 4.
- Addressed: "Test fixtures describe the new contract only. No test or code accepts the old behaviour, and no fallback is added". Rules 2–5; scenario 6 checks that no Go file holds `lineupMoved`, `models_moved` or `review-1-moved`. Scenario 5 checks that the moved goldens are gone. Any fixture that still put `models:` in a project configuration would fail to load against the pinned producer.
- Addressed: "Change this repository's code, tests, fixtures and docs". No production code or documentation states the removed rule. `README.md`, `docs/` and `consumes/README.md` need no change, and only test comments change (rule 6).
- Addressed: "Nothing else changes. Keep every other test green". Rule 7; scenario 5's digest of `build/testdata`; the configured checks.
- Out of scope: `legacyAdaptBaseline` belongs to the baseline rule (`.kogen/models.yaml` listed only when present). That rule is already on the producer's main and is not part of this contract, so it stays as it is. This repository's own protected `.kogen/project.yaml` is also out of scope.

## Tests
- The producer's new main is pinned and nothing else in the pins changes: acceptance scenario 1 (`go.mod`/`go.sum` bytes) and the version check in scenarios 2–5 (regression)
- The coordinator consumer suite reads roles from `.kogen/models.yaml`: update `project_fixture_test.go` (`fixtureConfig`, `newProject`); run `TestBaselineAndHash`, `TestLoadConfigReadsTheProjectConfiguration`, `TestLoadConfigDefaults` and `TestWritePackageFacts` in scenario 2 (regression)
- Roles a Build test sets come from `.kogen/models.yaml`: update `setRole` and `loadLineup` and delete `lineupMoved`; scenario 3 runs the 51 affected tests (regression)
- Resuming an Elixir Build whose project has `models:` in `.kogen/project.yaml` is refused with `environment/project_config_invalid` and changes nothing: update the legacy refusal helper and the TestALegacy*, TestLegacyResumeRefusals, TestMain76BuildsResumeAndLand and TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer tests; scenario 4 (integration)
- Review records are recorded from a project configuration without `models:`: regenerate the two `review-1.json` goldens and delete the `-moved` ones; scenario 5 (regression)
- No dual layout is left: scenario 6 (regression)
- The rest of the suite stays green: the configured go-test check (existing)

## Feasibility
- The checks below ran in a read-only external session, in scratch copies outside the checkout (`git archive HEAD`), with a scratch module cache. The producer commit was served as `main` of a scratch bare repository through a temporary `GIT_CONFIG_GLOBAL` `insteadOf`. They are not Kogen probe records.
  - `go get github.com/KogenAI/kogen-coordinator@main` resolved to `v0.0.0-20261106110407-d74daae36f56`. After `go mod tidy`, `go.mod` and `go.sum` have the SHA-256 values in scenario 1.
  - `go build ./...` and `go vet ./...` pass. `go test -count=1 ./...` fails:
    - in `./build`, the 61 top-level tests listed in scenarios 3–5 (about 210 s);
    - in `./consumes/kogen-coordinator`, the three tests above.
    All other packages pass. Against the previous main `c8126f4`, TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer, TestMain76BuildsResumeAndLand and TestLegacyResumeRefusals pass.
  - Scratch exploration only, not the product implementation: a forced moved layout with roles written under `models:` in `.kogen/models.yaml` leaves only the legacy refusal wording and the two review-record goldens failing in `./build`. The got bytes of those goldens equal today's with the tree id changed, which gives the SHA-256 values in scenario 5.
  - On the previous main, the 51 tests of scenario 3 pass in about 57 s.
  - On the unchanged tree, a dry run binds all 39 steps. Scenarios 1 and 6 fail on the old `go.mod` digest and on the `lineupMoved` hits.
- evidence: {"id":"go-test-verdict","kind":"observer","inputs":[],"observers":["features/steps/b-consumes-c-models-only-in-models-yaml__control.js"],"checks":[{"name":"good","command":"\"${KOGEN_NODE:-node}\" features/steps/b-consumes-c-models-only-in-models-yaml__control.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"${KOGEN_NODE:-node}\" features/steps/b-consumes-c-models-only-in-models-yaml__control.js bad","exit":1,"output":"rejected\n"}]}
