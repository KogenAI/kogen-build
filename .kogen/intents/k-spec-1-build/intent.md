# Maintain landed models in later Builds

Slug: k-spec-1-build
Model: .kogen/intents/k-spec-1-build/model.qnt invariants=HistoryKept,Enforced seed=7 samples=50 steps=10
Acceptance:
- .kogen/intents/k-spec-1-build/acceptance.feature
- features/steps/k_spec_build_steps.js
- features/steps/k_spec_build_observer.js
- features/steps/k_spec_build_driver.go.txt

K-SPEC-1-BUILD is the feature split of K-SPEC-1 (main ruling 3 Nov), consumer of K-SPEC-1-VERIFY, for repository kogen-build. Currently a landed Intent's model remains only in its frozen Intent and later Builds never check it. Landing must also keep a maintained current copy with its Path, Invariants, Seed, Samples and Steps. Every later Build supplies all maintained models to ordinary verification; an invariant violation prevents green and landing. Historical Intents remain untouched.

Use a project-owned `.kogen/models/<intent-slug>/` directory for each independent landed model, with its model bytes and a JSON record of the existing project.ModelSpec fields pointing at the maintained copy, not the historical file. Preserve all recorded parameter strings and invariant order. Discover these records in lexical slug order and supply their ModelSpecs through verify.Input.MaintainedModels. Do not scan old Intent directories as a substitute for promotion. Publish the maintained copy and record in the same verified/reviewed commit as the Intent; prepare them before the candidate's final verification and independent review, not as unreviewed post-verification edits. Do not create maintained directories or output fields for model-free projects. Preserve no-model behavior byte for byte, including existing result/progress/log/golden outputs; do not rewrite existing maintained records merely because another Intent lands. Existing current-Intent model checks remain intact. Whole verification on normal Build and resumed verification must use the maintained input; stage gates retain verification's existing scoped behavior. Existing maintained copies are current project specifications, not frozen historical Intent files.

Pin github.com/KogenAI/kogen-verification to K-SPEC-1-VERIFY's landed commit `01aa16c017677785957359f814b55eded65bfce4` (Go pseudo-version `v0.0.0-20261103020618-01aa16c01767`). Local synthetic runs may use a scratch replace to that exact tree; do not ship a machine-local replace. No CLI change, no new project.yaml key, no timeout raised. Synthetic tests only; no live provider calls or paid services.

## Decisions
- Each landed Intent's model remains independently maintained and enforced; replacing or retiring an earlier model is outside this feature. Later models do not supersede older ones even when they describe the same behavior. (source: answer 1)
- Keep historical Intents unchanged, verify all maintained models on later Builds and preserve model-free projects byte-identically. (source: request)
- Use synthetic local project and tool fixtures only, with no CLI/configuration additions or timeout increase. (source: request)

## Source comparison
- Addressed: request Problem and first Need map to maintained copy and parameter record, historical immutability and atomic reviewed landing above; modeled acceptance observes a real Build commit, copy bytes and the next Build's exact Quint argv.
- Addressed: request second Need and Acceptance map to maintained input on every whole verification pass and modeled acceptance's later passing Build and failed invariant Build with no commit. The synthetic violation changes the maintained model, not the historical original; the driver commits that synthetic baseline before starting the failed Build to satisfy production-clean startup.
- Addressed: request third Need and “No models: unchanged” map to no new storage/output and empty acceptance plus existing golden regression coverage; no-model byte compatibility is broader than the synthetic smoke check alone.
- Addressed: request Constraints map to the exact VERIFY dependency pin, scratch replace rule, no CLI change, no project.yaml key, no raised timeout and synthetic-only evidence above.
- Addressed: answer 1 maps to independent slug-owned copies, append-only addition of models, and integration coverage required below for two successive model-bearing Intents; there is no retirement/replacement facility.
- Addressed: request identifiers K-SPEC-1-BUILD, K-SPEC-1, K-SPEC-1-VERIFY, main ruling 3 Nov, parent path and change_type feature are retained in this complete contract. No requested scope is intentionally omitted, conflicting or superseded.

## Tests
- Landing a model commits maintained bytes and exact parameters without changing historical Intent files: synthetic Build acceptance and focused landing/storage integration tests (integration)
- Later whole verification passes enforce maintained invariants and stop landing on failure: synthetic successive-Build acceptance with recorded typecheck/run argv and violated maintained model, plus resume wiring coverage (integration)
- Every landed model remains independently enforced when another model-bearing Intent lands: add synthetic successive-Intent integration coverage checking both model records, both original parameter sets and both executions (integration)
- Model-free projects remain byte-identical: empty acceptance and existing Build output, result, context and checkpoint goldens unchanged, with a focused no-storage assertion (regression)
- Stage gates remain scoped and current Intent model checks still execute: retain existing stage/model verification coverage and extend Build wiring coverage only where needed (existing)

## Feasibility
- Repository inspection found verifyInput in build/build.go is the whole-pass input seam; commitApproved only commits the already reviewed tree, so promotion must precede that boundary. Required production edits need not touch any protected path.
- Local authoritative producer source and Git history identify K-SPEC-1-VERIFY commit 01aa16c017677785957359f814b55eded65bfce4 and Input.MaintainedModels []project.ModelSpec. Cached pinned verification currently lacks that field.
- Existing TestBuildLandsOneCommitWithTheIntentTrailer was observed passing via go test ./build -run '^TestBuildLandsOneCommitWithTheIntentTrailer$' -count=1 -v (exit 0); this grounded Go runner output and the reused synthetic Build fixture. Quint model typecheck and TypeScript run with HistoryKept and Enforced, seed 7, 50 samples and 10 steps passed in a scratch probe (exit 0).
- A scratch attempt to read retained proposal files failed because proposals are not in probe copies; it establishes no observer result. The final-byte controls below are the declared observer obligation and exercise independent field mutations, including predicates masked by missing promotion.
- evidence: {"id":"maintained-build-observer","kind":"observer","inputs":["features/steps/k_spec_build_observer.js"],"observers":["features/steps/k_spec_build_observer.js"],"checks":[{"name":"good","command":"node features/steps/k_spec_build_observer.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"node features/steps/k_spec_build_observer.js bad","exit":1,"output":"rejected\n"}]}
