# Stop reading the coordinator's opt-in options

Slug: k-drop-optins-build
Model: not applicable — each removed option becomes a fixed default; no state or interacting rules change
Acceptance:
- .kogen/intents/k-drop-optins-build/acceptance.feature
- features/steps/k_drop_optins_build_steps.js
- features/steps/k_drop_optins_build_test.go.txt

This is the consumer step of K-DROP-OPTINS (change_type: removal), under the simpler-is-better rule: every option becomes a constant at today's value, or is deleted. Defaults stay byte-identical unless a finding names a different winner.

K-DROP-OPTINS is kogen-coordinator candidate 1645f34ca82e432ff31f2694947a51e996d8f700, "Remove the opt-in configuration options". It removes the opt-in options from `project.Config`. kogen-build still reads them, so its landing consumer check fails to compile. The parent's order rule applies: the consumer removes a reader while the option still exists, and only then does the producer drop it.

After this change kogen-build reads none of these coordinator options:
- `models.<role>.max_rounds`: `project.Role.MaxRounds` and `project.DefaultExternalRounds`
- `shape_workers`: `project.Config.ShapeWorkers` and `project.Workers`
- `shape_audit`: `project.Config.ShapeAudit` and `project.Audit`
- `shape_decision_check`: `project.Config.ShapeDecisionCheck` and `project.DecisionCheck`
- `review_material`: `project.Config.ReviewMaterial` and `project.ReviewMaterial`

kogen-build references none of these identifiers. Each reader uses today's default value as a constant. kogen-build compiles and passes against the coordinator it pins today (options present) and against a coordinator without them.

Work only in kogen-build. Leave go.mod and go.sum unchanged: they keep pinning kogen-coordinator v0.0.0-20261102193107-c4a49a4fe84f. The candidate is a scratch replace for local runs only. Do not change kogen-coordinator, `.kogen/project.yaml`, CLI commands or flags. Raise no timeout.

What is removed:
- The `role.MaxRounds` reads and the `project.DefaultExternalRounds` fallback.
- The legacy (Elixir checkpoint) conversion of `shape_workers`.
- The `ReviewMaterial` normalization in the configuration comparison.
- The `on_demand` review material mode: everything only that mode reached, with the tests that exercised only it.

What changes for a project that still sets an option on today's coordinator: kogen-build ignores the value. A configured `max_rounds` no longer caps a role, and `review_material: {mode: on_demand}` reviews with parts. This repository's own `.kogen/project.yaml` sets build `max_rounds: 6` and review `max_rounds: 2`. Build behaviour is unchanged because the claude-code executor's own cap is 6, but this repository's external reviews now get the executor's cap (6) instead of 2.

## Decisions
- Build role served by an external executor: its round cap is the executor's own `Capabilities.MaxRounds`, exactly as today when the role sets no `max_rounds`; the build `executor.Task.MaxRounds` is that cap for an external build and 0 otherwise. Settled during shaping. (source: request)
- Review role served by an external executor: its round cap is the executor's review-role `MaxRounds`, or 6 when that is 0; the 6 becomes a kogen-build constant because `project.DefaultExternalRounds` is removed. Settled during shaping. (source: request)
- The configuration comparison of resume and recover, `sameConfig(a, b *project.Config) bool` in build/state.go, keeps its name and signature. Two configurations are the same when they are equal in every field other than the removed options: every field except the four removed top-level ones (`ShapeWorkers`, `ShapeAudit`, `ShapeDecisionCheck`, `ReviewMaterial`) and every `Models` role's `MaxRounds`. Both nil are the same and exactly one nil is changed, as today. This subsumes today's rule that an empty review_material mode equals `parts`. Differences in checks, models (other than max_rounds), protect, setup, fix or acceptance still refuse resume/recover as today. Settled during shaping: kogen-build cannot name the removed fields and no longer acts on them. (source: request)
- The legacy checkpoint conversion (`legacyConv.config` in build/legacy.go) no longer reads a legacy record's `shape_workers` and sets no workers. Settled during shaping. (source: request)
- Review material is always the parts mode, and the `on_demand` mode is deleted under the parent's "or is deleted" rule. Delete the `onDemand` reader and every branch it selected, keeping the parts branch byte-identical. Delete the on_demand reviewer prompt variant, so the native reviewer's instructions are `agent.Prompt(agent.Reviewer)` verbatim, as in parts mode today. Delete the on_demand catalog fields (mode, required_parts, covers, diff_stat/hunks), the required-parts events and facts, the required-hunk-part read gate, and the no-defect blocking-finding filter (which applied only on_demand). Delete every declaration only those branches used and every comment naming the mode. Settled during shaping. (source: request)
- Keep what the parts mode uses, such as `materialManifestAt`, which build/review_parts.go calls. The `review_material` event keeps its exact parts-mode shape, including `"mode": "parts"`. No non-test Go file under build/ contains `on_demand`, `onDemand` or `OnDemand`. Settled during shaping. (source: request)
- Tests stop setting or naming the removed options and write no removed project.yaml key; each changed test uses executor capabilities where it needs a round cap. Tests that exercised only removed behaviour are deleted. Every default-behaviour test passes unchanged, including the goldens. Settled during shaping. (source: request)
- The acceptance overlay relies on these existing test helpers keeping their names and signatures. Environment: `newEnv`; the `env` methods `useExecutor`, `useLineup`, `build`, `reviewed`, `write`, `read`, `run`, `approve`, `head`, `onlyRun`, `result` and `events`; the `Input` fields `External` and `Reviewer`. Fake build executor: `newFakeExecutor`, the `fakeExecutor` fields `desc` and `tasks`, its `seen()` method, `withExecutor`, `externalWritesApp`, `harnessUsage` and `wantStop`. Fake lineup: `newLineupExec`, the `lineupExec` field `review`, its methods `rounds`, `Describe` and `Start`, `lineupWriteApp`, `verdictJSON` and `lineupConfig`. Fake provider: `e.prov` and its methods `then`, `calls` and `requests`. Review request helpers: `readThen`, `bigFile`, `requestText`, `requestInstructions`, `reviewCatalog` and `eventsNamed`. Settled during shaping. (source: request)
- Use only owned child processes and temporary folders. Never stop host processes by name or pattern, and generate no synthetic load. Synthetic tests only. (source: request)

## Source comparison
- Addressed, Problem: the complete non-test list a build against a coordinator without the options reports is build/executor_external.go:47, 83, 88 and 104; build/legacy.go:700-706; build/material.go:1211; and build/state.go:350-351. The test files that name or configure the options make up "and more" (see Tests).
- Addressed, Need: the removed set is the five opt-in options of the pinned coordinator's project/config.go. They match the K-DROP-OPTINS rows K13/K14 (shape checks), K15 (review material), K16 (shape workers) and K17 (max rounds).
- The candidate 1645f34c could not be retrieved from this host: it is absent from the local mirror and module cache, and fetching it from github.com/KogenAI/kogen-coordinator answered "not our ref". Acceptance therefore synthesizes the coordinator without those options from the pinned one: the removed identifiers become unexported and the removed project.yaml keys become unknown.
- Other audit rows (shape retry, build budget, shell timeout, node env) are not fields of the coordinator's `project.Config` and are not part of this consumer step.
- Addressed, Need: "compile and pass against both". The configured format, compile, vet and go-test checks cover the pinned coordinator. Acceptance runs go build, go vet and go test -count=1 of ./... against the synthetic coordinator, through a temporary go.mod copy with a replace.
- Addressed, Need: "today's default value as a constant", with expected values in acceptance. max_rounds 1 with build cap 3 runs 3 build rounds; with review cap 6 it runs 6 review rounds; with review cap 0 it runs 6. Configurations differing only in a removed option compare the same, while checks, model or protect changes compare changed. A project that configures on_demand is reviewed with parts.
- Addressed, Constraints: no go.mod/go.sum change, and the reference candidate stays a local-only replace. No new CLI command or flag, no raised timeout, synthetic tests only.
- Superseded by the parent removal: a configured max_rounds, shape_workers, review_material, shape_audit or shape_decision_check no longer has any effect in kogen-build, including on resume/recover equality. The on_demand mode, with its no-defect filter (k-nodefect-scope) and its on_demand review-cost behaviour, is removed. Parts-mode behaviour, including k-review-cost-wide, is unchanged.

## Tests
- External build round cap is the executor's own cap whatever max_rounds says, and the build task carries it: update TestExternalTaskAndRunRecords (executor cap 3 instead of `max_rounds: 3`; build role JSON without `MaxRounds`). Update the other executor_test.go and k19_no_progress_test.go cases that set max_rounds to set the fake's `Capabilities.MaxRounds`. Keep TestExternalRoundLimitDefaultsToTheExecutorsOwn. Acceptance overlay TestKDropOptinsBuildRounds row (build, 1, 3, 3) runs a real Build (integration)
- External review round cap is the executor's review cap, or the constant 6 when that is 0: update TestClaudeLineupReviewRoundLimit to cases driven by the executor's review capability (6, 1, 2) plus a 0 → 6 case. Update review_resume_external_test.go to bound reviews with the executor's review capability instead of `max_rounds: 2`. Acceptance overlay rows (review, 1, 6, 6) and (review, 1, 0, 6) (integration)
- Resume/recover configuration comparison ignores exactly the removed options: add a sameConfig unit test with a same case per removed option and for an empty review_material mode, and changed cases for checks, a model and protect. Acceptance overlay TestKDropOptinsBuildSameConfig covers the 9 listed fields (unit)
- Legacy checkpoint conversion no longer reads shape_workers: delete the ShapeWorkers assertions in legacy_test.go and keep the legacy conversion/resume tests otherwise passing unchanged (existing)
- Review material is always parts and on_demand is deleted: delete TestOnDemandReviewReadsEveryHunkPartAndRecordsTheMode, the on_demand cases and mode loops of k_revod_gate_test.go (including the no-defect filter cases), and the on_demand half of krevodb_test.go. Update helpers such as `reviewRequests` that name the on_demand prompt, and keep k_revod_gate_parts_test.go and the parts tests unchanged. Acceptance overlay TestKDropOptinsBuildPartsReview runs a real Build of a project that still sets on_demand, plus a source scan of build/*.go (integration)
- kogen-build compiles and passes against a coordinator without the options: acceptance synthesizes it from the pinned module and runs go build, go vet and go test -count=1 ./... with a temporary modfile replace (integration)
- Behaviour with today's defaults is byte-identical against the pinned coordinator: the unchanged goldens and the configured format, compile, vet and go-test checks pass without weakening (existing)

## Feasibility
- A synthetic coordinator rebuilt the unchanged tree with exactly the compile errors the request names, and no others in non-test code. It is the pinned module copied to a temporary folder, with the nine removed identifiers made unexported and the five removed keys made unknown. The other dependencies (kogen-executor, kogen-verification, kogen-loop) reference none of these identifiers.
- On the unchanged tree, the overlay tests compile and fail at the intended assertions: 1 build round and task MaxRounds 1; 1 review round in both review rows; sameConfig false for each removed option; and the on_demand catalog, prompt and events. Every assertion is reported independently. The preserved rows (empty mode, checks, model, protect) pass.
- In a scratch copy with an unsubmitted minimal implementation, every overlay case passed through Cucumber (13 of 13), and go build against the synthetic coordinator succeeded.
- Not run before the Build: the whole suite against the synthetic coordinator, because the unchanged tree does not compile there. Past kogen-build go-test runs took about 300 s, and the acceptance step allows 40 minutes per go command.
- Observer controls qualify the synthesis. The known-good fixture module (reads only kept fields and parses a configuration without removed keys) passes go build, vet and test against it. Each of the 15 known-bad fixtures (one removed identifier or project.yaml key each) is rejected there. The same fixtures all pass against the unmodified pinned coordinator.
- evidence: {"id":"synthetic-coordinator","kind":"observer","inputs":["features/steps/k_drop_optins_build_steps.js"],"observers":["features/steps/k_drop_optins_build_steps.js"],"checks":[{"name":"good","command":"node features/steps/k_drop_optins_build_steps.js good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"node features/steps/k_drop_optins_build_steps.js bad","exit":1,"output":"observer rejected\n"}]}
