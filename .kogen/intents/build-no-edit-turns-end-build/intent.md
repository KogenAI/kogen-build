# Stop re-verifying builder rounds without edits

Slug: build-no-edit-turns-end-build
Model: .kogen/intents/build-no-edit-turns-end-build/model.qnt invariants=AcceptanceOncePerTree,NoEditStopFollowsNotice,RoundsCountedApart seed=7 samples=200 steps=12
Acceptance:
- .kogen/intents/build-no-edit-turns-end-build/acceptance.feature
- features/steps/build-no-edit-turns-end-build__steps.js

Treat a builder round that leaves the candidate unchanged as its own condition, for the native builder and for external executors (such as `claude-code`) alike.

Today, a builder can take a short read-only look and then end its turn with a text-only status message ("I couldn't complete this, no files were changed, I am not calling finish"). Kogen then verifies the same tree again every round. The native rule only catches rounds with no tool call at all, so a read-only look slips past it. The external rule waits for three identical red passes and stops as `executor_round_limit`. Each round the builder receives the same failure list again, and the Build ends without having tried.

After this change, Kogen does not verify the same tree twice when the builder made no edits after a red pass. Instead:
- The builder is told its round made no edits, and is asked to edit toward the first named failure or report a protected-file blocker.
- Rounds without edits are counted separately from rounds that changed the tree.
- A second round without edits into the same red stops the Build as `no_progress`, with a stop reason that names "no edits".

Rounds that edit are unchanged: verification, ordinary feedback, the unmoved-failing-set prefix, round caps, the check ledger, review and commit. A builder-reported protected-file blocker (`report_blocker`, or an external blocker claim) still stops the Build as `build_blocked` with its paths, reason and evidence named.

Definitions (all in package `build`):
- Candidate tree: the tree identity that verification uses, `checkRepo().Tree(b.Start, b.Excluded)`. It equals a clean pass's `pass.Tree`. Changes to ignored or excluded paths are therefore not edits.
- Round without edits: a builder round after which, once the configured fixers have run once as today, the candidate tree equals the candidate tree at the start of that round. The round may end with native `finish`, a native answer without a tool call, or an external `finish`/`no_call` ending, including an invalid blocker claim. A fixer that changes the tree makes the round one with edits.
- Remembered red pass: the latest verification pass of this Build or resume invocation that is red, clean (`pass.Clean`, nothing moved) and has a nonempty tree. It may be a whole-Intent pass or a stage gate pass, and is remembered with its stage (`""` for the whole Intent, or the active stage ID). A green pass clears it. It is kept in memory only, not in the persisted State or result, so a fresh or resumed invocation starts without one.

The rule, in `roundEnd` (build/builder.go), applies after the fixers and before `stageGate`/`verifyAndReview`:
1. The round made edits: verify exactly as today.
2. The round made no edits, and there is no remembered red pass with this stage and this exact tree (for example round 1, or the first round after a resume): print the progress line `kogen: round <R> made no edits`, then verify as today.
3. The round made no edits, the remembered red pass `P` has this stage and this tree, and the builder has not yet received the no-edit notice for `P`:
   - Do not verify. No pass number is used, no `pass` event is logged, acceptance and checks are not run, and `Repairs` is not incremented.
   - Print `kogen: round <R> made no edits; not verifying the tree of pass <P> again; asked the builder to edit toward <FIRST> or report a protected-file blocker`.
   - The builder's next input is exactly the notice below. When an external invalid blocker claim set the blocker note, the notice follows that note as today (note, blank line, notice).
4. The round made no edits, `P` matches, and the notice for `P` was already given: print `kogen: round <R> made no edits`, then stop with `sys.Fail("candidate", "no_progress", <reason>, 1)` without any further verification or model request. The checkpoint is left as the current `no_progress` stop leaves it (`builder_verify`), so `kogen intent resume <id>` verifies again and continues.

`<R>` is the Build's round number (`b.Round`). `<FIRST>` is the first item of `missingAcceptanceItems(P)`, in the form `<status>: <name> (line <n>)`, for example `failed: Export item 1 (line 2)`. When P has no missing acceptance item, `<FIRST>` is `P.Failing[0]`.

The notice is these four lines, joined by `\n`, with no trailing newline:
```
Round <R> made no edits: the candidate is byte-identical to the tree verification pass <P> checked, so it was not verified again; pass <P> is still red.
Make an edit toward the first named failure: <FIRST>.
If a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.
Another round without edits stops the Build as no_progress (no edits).
```
The notice is the whole message: the pass feedback the builder already holds is not repeated. For the native builder it answers the round's `finish` call or is sent as a user message, as feedback is today.

The stop reason is `no edits: round <R> made no edits after the builder was told round <R0> made none; verification pass <P> is still red and was not run again; edits are left in place`. `<R0>` is the round that received the notice. The error lines are, in order:
- `candidate tree: <P.Tree>`
- `failing: <P.Failing joined by ", ">`
- `first failure: <FIRST>`
- `rounds without edits: <N> of <T>`, where `<N>` counts this invocation's rounds without edits (including verified ones) and `<T>` counts all builder rounds this invocation ran, including the stopping round.
- ``No further verification or model request was made. Fix the named failure by hand, or `kogen intent resume <run id>` to verify again and give the builder the feedback again.``

The result outcome stays `no_progress`.

Removed, because a red tree is now never verified twice in a row and both older rules can no longer trigger:
- The native tool-free rule in build/noprogress.go: `stalled`, `missingItemsNotice`, the `missing_items_notice` event, and `progressWatch`'s tool counter and its red/noticed tracking.
- The external identical-red rule: `externalStalled`, `externalNoProgressPasses` and the `executor_no_progress` event.

Update their callers in the same change: `steps.Verify` in build/route.go and `stageGate` in build/builder.go. A previous attempt broke because the merged build/route.go still called `b.stalled`. Build on current main and keep its `route.Cycle` routing. Keep `unmovedNotice` and its call sites unchanged.

No change to:
- the builder's instructions or tool definitions (prompts, `defs_golden.json` and `prompts_golden.json` stay as they are);
- the persisted State schema or the state goldens;
- result fields or configuration;
- the kogen-coordinator, kogen-verification and kogen-executor dependencies.

The acceptance step file embeds a Go test that it overlays as `build/no_edit_turns_acceptance_test.go`, so the Build must not create that path. The Build must also keep these existing test helpers with their current signatures:
- `newEnv`;
- the `env` methods `write`, `approve`, `build`, `onlyRun`, `result`, `runPath` and `useExecutor`, and the fields `prov` and `aux`;
- `attempt.shown` and `attempt.errs`;
- `writeFile`, `obj`, `documentMessage`, `caseMessages`, `sse` and `handler`;
- `newFakeExecutor`, `fakeExecutor.seen` and `withExecutor`;
- `fakeProvider.then`, `fakeProvider.calls` and `fakeProvider.requests`.

## Decisions
- A round without edits is detected from the candidate tree, not from tool calls. A read-only look followed by a text-only answer counts as a round without edits. (source: request)
- Steering strength, which the request leaves to Kogen: the builder gets one notice per red pass, naming the first failure and the two allowed next actions. The next round without edits into the same red stops the Build. An edit round in between resets this, because its pass is a new red pass. (source: request)
- The no-edit notice replaces the repeated pass feedback for that round and uses the exact wording above. Every round without edits gets a progress line on stderr. (source: request)
- The stop keeps outcome `no_progress`. Its reason starts with `no edits:` and counts rounds without edits separately from all rounds. (source: request)
- One rule covers native and external builders, and whole-Intent and stage passes. It replaces both older stall rules. (source: request)

## Source comparison
- addressed: The Problem (a text-only status after a short read-only look; the unchanged tree re-verified each pass; the same failure list again; the Build giving up after five or six passes) maps to rule 3 (no verification of an identical red tree) and rule 4 (stop named "no edits"). Acceptance scenarios "text-only" (native read_file then text) and "external-text-only" (external `no_call` with that status message) cover it.
- addressed: The Need ("say so to the builder, ask it to either make an edit toward the first named failure or report a protected-file blocker explicitly, and count such rounds apart") maps to the exact notice text, the `<FIRST>` rule, the per-round progress lines and the `rounds without edits: N of T` stop line.
- addressed: Synthetic acceptance 1 (about 25 red scenarios needing two features; text only every round) maps to the fixture's 25 scenarios (13 export, 12 import). The scenario expects acceptance to run once, 1 pass, round 3's input to equal the notice, and stderr lines for rounds 1–3.
- addressed: Synthetic acceptance 2 (a small edit in round 3, then text only) maps to scenario "late-edit": 2 acceptance runs; notices after rounds 2 and 4, where the second names `failed: Export item 2 (line 4)` after the increment; stop reason `no edits: round 5 …`; `rounds without edits: 4 of 5`; and no "round 3 made no edits" line.
- addressed: Synthetic acceptance 3 (a protected-file blocker with no edits; existing behaviour) maps to scenario "blocker": `report_blocker` after two rounds without edits stops as `build_blocked` with `paths: .kogen/intents/fixture/acceptance.feature` and the reason, never as `no_progress`.
- addressed: Synthetic acceptance 4 (edits every round, failing set not reduced, behaviour unchanged) maps to scenario "edits": 4 acceptance runs for 4 edit rounds, with the ordinary feedback and the existing `Failing set did not move…` prefixes observed byte-for-byte on the unchanged tree. No no-edit notice appears until text-only rounds follow.
- addressed: The previous attempt (the merged tree failed at build/route.go:35 with `b.stalled undefined`) maps to the instruction to build on current main and update the route.go and builder.go callers together.
- superseded: Intent k-build-unmoved-red-feedback decided that a tool-free identical-tree round keeps the missing-items notice and `no_progress` sequence. Its historical fixture (features/steps/k_unmoved_test.go.txt) is not run by this Build and is not edited. The external rule tested in build/k19_no_progress_test.go (third identical red as `executor_round_limit`) is also superseded.
- omitted: no new configuration, persisted state, prompt text or provider call. No external protocol or library fact is involved, so there is no documentation dependency.

## Tests
- A round without edits after a red pass at the same stage is not verified again, and the builder receives exactly the no-edit notice naming the first failure: acceptance "text-only" and "external-text-only"; replace TestAToolFreeRoundIntoTheSameRedGetsOneNoticeNamingTheMissingItemsAndTheBuildLands with a no-edit notice test in noprogress_test.go (regression)
- The second consecutive round without edits into the same red stops as candidate/no_progress, with reason `no edits: …` and the listed lines, and makes no further request: acceptance "text-only"; update TestARedAnswerWithoutAToolCallAfterTheSameRedStopsWithTheCandidatePreserved and TestAnIdenticalRedGetsNoSecondNoticeAfterARoundWithToolCallsThatChangedNothing (regression)
- An edit round between rounds without edits verifies its new tree and resets the notice, later notices name the then-first failure, and rounds without edits are reported per round and counted separately: acceptance "late-edit" (integration)
- Rounds without edits on a tree no red pass has checked (the first round, or the first after a resume) are still verified: acceptance round 1 of every no-edit scenario; keep TestAStoppedNoProgressBuildResumesFromItsCheckpoint passing (integration)
- A fixer-changed tree counts as an edit and an ignored-file change does not: existing TestARoundWithoutAToolCallContinuesWhenTheFixersChangedTheCandidate and TestAChangedFileModeIsProgressForARoundWithoutAToolCall stay unchanged; update the pass and request counts in TestAnIgnoredFileChangeIsNotProgressBecauseVerificationSeesOnlyTheCandidateTree (existing)
- Stage gates follow the same rule (notice, then stop, at the active stage): update TestARedStageGateRepeatedWithoutAToolCallStopsAtTheStage (integration)
- External executors follow the same rule instead of the third-identical-red round limit: rewrite TestExternalNoProgressStopsOnThirdIdenticalRed to expect a notice and then `no_progress`; keep TestExternalNoProgressResetsWhenTheTreeChanges (integration)
- A protected-file blocker after rounds without edits still stops as build_blocked with its paths and reason: acceptance "blocker"; existing blocker_test.go (existing)
- Edit rounds that do not shrink the failing set keep their verification and feedback unchanged: acceptance "edits"; existing unmoved_test.go and TestNamedFeedbackCompatibility with its frozen golden unchanged (existing)
- The no-edit state rules (at most one acceptance run per consecutive tree, a stop only after a notice, rounds counted separately): model.qnt invariants (unit)

## Feasibility
- On the unchanged tree, the embedded fixture reached the intended paths in every mode, and only the no-edit expectations failed, which is legitimate missing-feature red (probe: exit 1 each, 1.7–3.4 s):
  - native read-then-text: acceptance ran 3 times and the Build continued until the scripted provider refused;
  - external `no_call`: the Build stopped as `executor_round_limit` after 3 identical passes;
  - "edits": the feedback starts for rounds 2–5 matched the current output exactly;
  - "blocker": the `build_blocked` lines matched.
- Go resolves overlay keys against `$PWD`. In a checkout under a symlinked temporary path, the overlay ran zero tests until the step set `PWD` to Node's `process.cwd()`. The final step does that, and it requires exactly one `run` and one `pass` event of TestNoEditTurnsAcceptance in `go test -json` (probe: exit 0, 5 s).
- Cucumber dry-run bound all 5 scenarios exactly once with `features/steps/*.js` (probe: exit 0, 0.1 s).
- The Quint model typechecked, and `run --invariant='AcceptanceOncePerTree and NoEditStopFollowsNotice and RoundsCountedApart' --seed=7 --max-samples=200 --max-steps=12` found no violation (probe: exit 0, 1 s).
- evidence: {"id":"noedit-observer","kind":"observer","inputs":["features/steps/build-no-edit-turns-end-build__steps.js"],"observers":["features/steps/build-no-edit-turns-end-build__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/build-no-edit-turns-end-build__steps.js control-good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/build-no-edit-turns-end-build__steps.js control-bad","exit":1,"output":"observer rejected\n"}]}

Both controls work the same way:
1. They run the real "text-only" Build fixture and require its plumbing to hold: the number of acceptance executions equals the number of logged passes (at least 1), and round 2 received feedback.
2. They run the same final validator on a known-good observation built from the text-only expectations, and on a known-bad observation whose stop reason lacks `no edits:`.

The bad control prints `observer rejected` only when the validator rejected it at that stop-reason assertion; any other failure exits 2. The controls do not depend on the feature being implemented, and they are not evidence that it is.
