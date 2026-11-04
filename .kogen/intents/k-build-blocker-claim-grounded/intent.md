# Ground blocker claims in verification failures

Slug: k-build-blocker-claim-grounded
Model: not applicable — one decision per claim (backed or not) over the latest verification pass; it adds no state, no persisted field and no new transition
Acceptance:
- .kogen/intents/k-build-blocker-claim-grounded/acceptance.feature
- features/steps/k-build-blocker-claim-grounded__steps.js

Today a builder can end a Build as `build_blocked` with any well-formed `report_blocker` call (native) or `EndBlocker` round result (external executor). Well-formed means nonempty unique current protected paths and nonblank reason and evidence strings. Nothing ties the claim to verification. After this change, a well-formed claim stops the Build only if it is backed: at least one of its claimed paths appears in the evidence of a red item of the latest verification pass of the current candidate tree. A claim that is not backed is not a stop. The builder gets the unbacked message (below), no protected edit is granted and nothing is recorded as a blocker. The Build then goes on exactly as it does after today's invalid-blocker feedback. A backed claim stops the Build exactly as today.

Order for every claim, native and external:
1. Well-formedness (`validBlocker`) is unchanged. A malformed claim still gets today's `invalid blocker: ...` message, unchanged.
2. The custody checks that today run inside `stopBlocked` (`BaselineIntact`, then `headIntact`) run next, for every well-formed claim, before the backing check. A changed protected file or a moved HEAD stops the Build as today (`baseline_changed`, `head_moved`), whether or not the claim is backed.
3. Backing check. The current candidate tree is the tree identity a verification pass records as `Tree`: `repo.Tree(b.Start, b.Excluded)`, the same call `drive` makes at a round start. The grounding pass is this Build's most recent verification pass, which is the whole-Intent pass `b.LastPass` or a later stage-gate pass, but only if its `Tree` equals the current tree. Otherwise there is no grounding pass. Kogen does not run a fresh pass to check a claim.
4. Backed: `stopBlocked` runs exactly as today. That covers:
   - the result: `outcome` `build_blocked`, and `blocker` with `classification` `BUILDER-REPORTED` and the claim's own `paths`, `reason` and `evidence`, unchanged and in reported order;
   - the `builder_blocker` event;
   - the stdout lines, where `paths: ` lists exactly the reported paths joined with `, `;
   - the facts;
   - for external executors, the `Save(Continuation{Stage: StageBuilderVerify})` before the stop;
   - the `kogen intent recover` record.
5. Not backed: no stop, no `Blocker` and no `builder_blocker` event. Native: the message is the `report_blocker` tool output, and the round continues. External: the message becomes the round's `blockerNote`, as the invalid-claim note does today. It prefixes the next feedback, after the round's normal boundary, fixers and verification.

Red items of a pass and their evidence text. A claimed path is backed when its exact repository-relative string occurs as a substring of one red item's evidence. This also matches absolute paths in stack traces.
- Acceptance, when the pass's acceptance has `Problems`: the Intent's feature path (`project.FeaturePath(slug)`), every `Problems` entry (each non-passing scenario's entry carries its failure detail) and the acceptance command `Output`.
- The Intent model, when it failed: `project.ModelPath(slug)` and its problems. Each failed maintained model: its `Path` and its problems.
- Each setup command or check with a nonzero `Status`: its name, its command and its full recorded `Output`.
- A green pass, or one with only moved files, has no red items.

The builder's own `evidence` string never counts.

Unbacked message. `PATHS` is the claim's paths in reported order, joined with `, `. `N` is the grounding pass's number.
- No grounding pass: `blocker not backed by verification: no verification pass has run on the current tree yet, so no failure names PATHS; no protected edit is granted; make the failing items pass, or finish so verification runs on the current tree and report again with a protected path that a failure names`
- Grounding pass N: `blocker not backed by verification: no failure of verification pass N names PATHS; no protected edit is granted; make the failing items pass, or report again with a protected path that a failure names`

What stays unchanged:
- the builder prompt (`build/prompts/builder.md`);
- the tool definitions, including the `report_blocker` description;
- `SubmissionSchema`;
- the checkpoint and result schemas;
- recovery and resume;
- today's protocol error for an executor that does not declare the blocker capability.

Builds without any blocker report keep byte-identical requests and results. No golden under `build/testdata` changes, in particular `named-feedback-compat.json`, `prompts_golden.json`, `defs_golden.json`, `context-external-golden.json` and `state/fresh/checkpoint.json`. Implement in `build`: `reportBlocker`, `afterExternalRound`, `stopBlocked` or a helper beside them. Do not change the dependencies.

The protected step file `features/steps/k-build-blocker-claim-grounded__steps.js` holds the acceptance fixture as the Go source `FIXTURE_GO`. For each fixture mode, the step file writes that source to a temporary file and compiles it into package `build`'s tests with `go test -overlay`, as `build/zz_kbg_acceptance_test.go`. It then runs `TestKBGAcceptance` with `KBG_MODE` and `KBG_EXPECT`, one mode per `go test -run '^TestKBGAcceptance$'` process. The fixture calls no other test function. Each existing test that acceptance relies on runs in its own `go test -json -run '^Name$'` process.

The fixture uses these existing test helpers, so keep their names and signatures:
- `newEnv`;
- `env`, with `root`, `aux`, `prov`, `read`, `write`, `approve`, `build`, `onlyRun`, `result`, `events`, `jsonFile`, `runPath` and `session`;
- `attempt`, with `exit`, `err`, `out` and `shown`;
- `fakeProvider`, with `call`, `calls`, `then`, `requests` and `count`;
- `handler`, `sse`, `execGit`, `eventsNamed`, `documentMessage`, `caseMessages`, `obj`, `writeFile`, `physical`, `testFeature`, `testSlug` and `stepsFile`;
- the production `Recover` and `RecoverInput`.

## Decisions
- Backing rule: a claimed path is backed by a substring match in a red item's evidence. That evidence includes the Intent's feature path for a failing acceptance, the model paths for failed models, and check commands and output. One backed path is enough. The request leaves strictness to Kogen, and a real contradiction always shows in some failure (source: request)
- Kogen does not run a fresh pass to check a claim. It uses the latest pass only if that pass ran on the current tree. A builder that edited after the red pass finishes again, then reports, so a real contradiction is never lost (source: request)
- Custody checks keep precedence over the backing check, so tampering with protected bytes or HEAD stops the Build as today (source: request)
- The unbacked message texts are exactly as above. They name the paths, say that no pass has run yet when none has, say that no protected edit is granted, and point to making the failing items pass or reporting a path a failure names (source: request)
- The builder prompt and tool definitions stay unchanged, so Builds without a blocker report keep byte-identical requests (source: request)
- Acceptance runs existing tests through `go test -run`, each in its own run, and never calls test functions directly. The supplied fixture is its own test function in its own process, as the request's delivery decision after Build f0a6168f requires (source: request)

## Source comparison
- addressed: Problem/Need (a claim that no verification output supports ends the Build) → steps 3–5 and the red-item evidence rule; acceptance scenarios "first-round" and "unnamed".
- addressed: Synthetic acceptance 1 (first-round claim, no pass) → scenario "A first-round claim with no verification pass does not stop the Build". It checks the exact no-pass message, the outcome `no_progress` rather than `build_blocked`, that there is no blocker record or event, and that the next round's edit was made and verified (4 requests, 1 pass).
- addressed: Synthetic acceptance 2 (red pass, claimed protected file that no failure mentions) → scenario "unnamed". The message names `features/steps/integration_steps.js` and pass 1. The builder's own evidence string names that file and is ignored.
- addressed: Synthetic acceptance 3 (a protected step file that cannot pass, with a failure in it) → scenario "backed". It checks:
  - the same stdout lines as the existing blocker test;
  - the result, facts and event;
  - that no call runs after the stop;
  - unchanged HEAD, index, approval and protected bytes;
  - a `Recover` archive whose `blocker.json` equals the result's blocker.
- addressed: Synthetic acceptance 4 (two protected files, one named) → scenario "two-paths". The claim lists `.kogen/intents/fixture/intent.md` (not named by any failure) first and `features/steps/integration_steps.js` second. The stop is accepted, and `paths:` lists exactly those two in reported order.
- addressed: Synthetic acceptance 5 (no blocker report: byte-identical requests and results) → scenario "Builds without a blocker report keep their requests and results". It checks the golden hashes, then runs each of these alone: TestNamedFeedbackCompatibility, TestPromptsAreByteEqualToTheElixirFunctions, TestDefinitionsMatchTheElixirDefinitions, TestContextExternalNativeBytes and TestStateGoldenFreshCheckpoint.
- addressed: Need "same recover record, same checks that HEAD and protected bytes are unchanged" → the step 2 ordering and the backed scenario. Existing TestAcceptingABlockerStillRequiresTheOriginalHeadAndProtectedBytes stays valid.
- addressed: the delivery decision (no direct calls of existing test functions, which panicked on t.Parallel) → each existing test and each fixture mode runs in its own `go test -run` process.
- superseded: today every well-formed claim is accepted as a stop. The existing tests that report a claim without a backing failure (listed under Tests) are updated, not deleted.
- omitted: no external protocol, provider or library change; no documentation lookup needed.

## Tests
- A well-formed claim with no pass on the current tree does not stop the Build, and the builder gets the no-pass message: acceptance "first-round". Update TestABuilderReportedProtectedContradictionStopsBeforeLaterCallsOrGates so its stop is backed by a red pass that names the claimed file, and add an integration case for the unbacked first-round claim (integration)
- A claim whose paths no failure of the grounding pass names gets the pass-N message, and the Build continues: acceptance "unnamed" (scenario)
- Backing rule over red items: add focused unit tests of the backing decision and of message rendering. A claimed path is backed by a substring in a failing scenario's detail, by the feature path when acceptance fails, by a check's command or output, or by a model path. Green or moved-only passes, and passes on another tree, back nothing. One of several paths is enough (unit)
- A backed claim stops exactly as before (lines, result, facts, event, recover record): acceptance "backed" and "two-paths" (scenario)
- External executor claims follow the same rule: update TestExternalBlockerFromADeclaringExecutorGoesThroughTheNativePath so its valid claim is backed by an earlier red pass, and add a case where an unbacked claim does not stop and its message prefixes the next feedback (integration)
- Recovery stays unchanged: update the `blocked` helper in recover_test.go so its blocker is backed by a red pass on the same tree; the recovery tests keep their assertions (existing)
- Custody before backing, and unchanged invalid-claim feedback: existing TestAcceptingABlockerStillRequiresTheOriginalHeadAndProtectedBytes, TestMalformedEmptyAndUnprotectedBlockerReportsGrantNoWrites and TestAnInvalidExternalBlockerClaimWithoutEditsGetsTheNoteThenTheNotice (existing)
- Builds without a blocker report stay byte-identical: the golden tests run by the last acceptance scenario, with the goldens not rewritten (existing)

## Feasibility
- On the unchanged tree, the fixture source was run through `go test -overlay … ./build -run '^TestKBGAcceptance$' -count=1`, one mode per process:
  - "first-round" and "unnamed" fail only on missing-feature assertions: outcome build_blocked, a blocker recorded, no answer to report_blocker (probes: exit 1, 3–4 s);
  - "backed" and "two-paths" pass, recovery included (probes: exit 0, 3–4 s);
  - the control-good modes pass;
  - the control-bad modes fail with only `kbg-reject: blocker tool output`, because the real invalid-blocker answer stands where the expected message should be.
- The step file was then run from a scratch copy of the checkout. `node …__steps.js control-good` printed `observer accepted` (exit 0, 10 s) and `control-bad` printed `observer rejected` (exit 1, 7 s). A Cucumber `--dry-run --strict` bound every step.
- A first scratch run under `/tmp` compiled no fixture: "no tests to run", because `/tmp` is a symlink and Node and Go saw different working directories. The step file therefore passes `PWD` (the directory Node resolves against) to `go`.
- Each existing golden test passed alone through `go test -json ./build -run '^Name$' -count=1`, with exactly one `run` event and one `pass` event (probes: exit 0):
  - TestNamedFeedbackCompatibility (12 s);
  - TestPromptsAreByteEqualToTheElixirFunctions (1 s);
  - TestDefinitionsMatchTheElixirDefinitions (1 s);
  - TestContextExternalNativeBytes (7 s);
  - TestStateGoldenFreshCheckpoint (1 s).
- This host's `go` is a shim that queues heavy commands behind a global lock held by other suites. Waits of about 5 minutes were observed before a test started. The steps therefore allow 900 s per `go test` process.
- evidence: {"id":"blocker-grounding-observer","kind":"observer","inputs":["features/steps/k-build-blocker-claim-grounded__steps.js",".kogen/intents/k-build-blocker-claim-grounded/acceptance.feature"],"observers":["features/steps/k-build-blocker-claim-grounded__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-blocker-claim-grounded__steps.js control-good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-blocker-claim-grounded__steps.js control-bad","exit":1,"output":"observer rejected\n"}]}

The controls run real local Builds on the unchanged tree with the same fixture bytes and read their expectations from the feature. In them the builder claims the unprotected `app.txt`. Today that continues the Build with the invalid-blocker answer, the same continuation shape that the unbacked rule must produce. control-good substitutes the expected unbacked message for that answer. Every other assertion still judges the real Build: the outcome, the absence of a blocker record or event, the request count, the pass count, the next round's edit, the protected bytes, HEAD and approval. control-bad keeps the real answer and must be rejected for exactly that reason. These controls qualify the observer; they are not evidence that the feature is implemented.
