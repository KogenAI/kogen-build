# Remind the builder that finish means whole Intent

Slug: k-build-finish-whole-intent-reminder
Model: not applicable — one fixed line appended to one builder-bound feedback path and one bullet added to the builder's instructions; no new state, transition, stop rule or verification rule
Acceptance:
- .kogen/intents/k-build-finish-whole-intent-reminder/acceptance.feature
- features/steps/k-build-finish-whole-intent-reminder__steps.js

When a whole-Intent verification pass that answers the builder is red, the feedback the builder receives ends with one fixed reminder line: finish means the whole approved Intent works through its real entry points, and helper-only or partially wired work will be red again. The builder's standing instructions say the same thing once, as one new bullet. Nothing else changes: what verification runs and in which order, what counts as green, the ordinary per-item feedback, review feedback, stop rules, round limits, review, commit, checkpoints and the `kogen-build` command's flags and stdout result stay as they are.

The reminder line is exactly this (plain ASCII, one line, no line feed of its own):

```text
Reminder: finish means the whole approved Intent is implemented and every behavior it requires works through the real entry points that a user or an acceptance scenario reaches; helper-only or partially wired work will be red again.
```

Where it goes:
- It is appended after the ordinary feedback, separated by one blank line: the builder receives `<feedback today>` + `"\n\n"` + reminder. `<feedback today>` is the exact text the builder gets today, including the unmoved-failing-set notice and the invalid-blocker note that are put before it today, so the reminder is always the last line.
- It goes on every red whole-Intent pass that goes back to the builder, from the first, for native and external builders alike. That includes the whole-Intent pass after the last stage of a staged plan, and a red pass after an approval whose candidate drifted when it goes back to the builder.
- It is added in the Build's own feedback policy, at the point where a red whole-Intent pass becomes the builder's feedback: `(*Attempt).verifyAndReview` in `build/builder.go`, which wraps `route.Cycle` and is used only by builder rounds. When `route.Cycle` returns red feedback (not landed, no error), append `"\n\n"` + the reminder. Keep the reminder text in one constant in package `build`, for example next to the other fixed passages in `build/prompt_text.go`.
- It is not added to `(*Attempt).PassFeedback`, `steps.Verify` or `verify.Pass.Feedback`. `PassFeedback` is also the review's feedback (`reviewEnv.Feedback` with completion `submit_review`), and `route.Resume` folds a resumed commit's red feedback into an error that never reaches the builder.

Where it does not go:
- review feedback of any kind: the review's final-verification repair feedback and its resume feedback;
- a scoped stage gate's red feedback (`Stage <id> failed; remain at this stage.` …), because there finish completes a stage, not the whole Intent;
- the notice of a builder round without edits (no verification pass runs);
- any stop, error or progress message.

The builder's standing instructions, `build/prompts/builder.md` (served as `BuilderInstructions()`), gain exactly one bullet under `Rules:`. It goes on its own line directly before the bullet that starts `- When the implementation is complete, call finish.`, after the bullet that starts `- A figure you write into a project file`. The bullet is exactly this line (plain ASCII, ending with a line feed):

```text
- Finish means the whole approved Intent is implemented: every behavior it requires works through the real entry points that a user or an acceptance scenario reaches. A new helper or internal function with its own passing test, an entry point that still behaves the old way, or a part of the Intent not yet started is not complete, and a finish with such work is verified red again.
```

No other byte of `build/prompts/builder.md` changes. Today it is 3777 bytes with sha256 `915b0bc6b4bd3624010f1878143b6215f34fb39c0333e99785b187d8f97511d7`. After the change it is 4160 bytes with sha256 `2729f3c4e8bd5754252fd4d6a292178f844e2a037a09bd332f31e56ed557e4a2`. The tool definitions (`finish` and `report_blocker`, `build/testdata/defs_golden.json`) and `SubmissionSchema` stay unchanged.

Recorded goldens:
- `build/testdata/prompts_golden.json` records the new length and sha256 of `build/prompts/builder.md`.
- `build/testdata/named-feedback-compat.json` changes by exactly two things. Its `request` (the first request of a passing scripted Build) holds the bullet. Its `feedback` (the ordinary assertion feedback of a red builder pass) ends with the blank line and the reminder.
- `build/testdata/context-external-golden.json` changes only by the bullet.
- These goldens are frozen; this Intent is the deliberate reason they change. Regenerate them with `go test ./build -count=1 -update -run '^(TestPromptsAreByteEqualToTheElixirFunctions|TestNamedFeedbackCompatibility|TestContextExternalNativeBytes)$'`, then confirm the diff is only those lines.
- Every other golden keeps its bytes, including `defs_golden.json`, `state/fresh/checkpoint.json`, `reviewretry/approve.wire.json`, `reviewretry/changes.wire.json`, `context-golden.json` and `reset-golden.json`.
- Existing Go tests that compare a builder's whole-Intent red feedback byte for byte are updated only by the appended reminder. Tests of review feedback and of stage feedback keep their expectations.

## Decisions
- The reminder goes on every builder-bound red whole-Intent pass, from the first one, not only from the second. The request leaves this to Kogen, and a fixed rule is simpler to verify (source: request)
- The reminder's wording and the instruction bullet's wording are fixed above. Because they are distinct texts, "the finish statement exactly once" and "no other instruction text changes" can be checked byte for byte (source: request)
- The reminder is the last line, after one blank line. The ordinary feedback before it, and the notices put before that today, keep their exact bytes and order (source: request)
- The reminder goes only to the builder. Review feedback and scoped stage-gate feedback never carry it, and no stop, check or green decision depends on it (source: request)

## Source comparison
- addressed: Need (one fixed short reminder on builder-bound red feedback, only there) and Synthetic acceptance 1 (unused helper with its own passing unit test, then finish: red, no review or commit, the feedback holds the reminder and the ordinary failing items). Scenario "partial-then-wired" runs the real program. The step file builds `./cmd/kogen-build` from this checkout and runs `kogen-build build --project <fixture> --slug fixture` on a temporary Git project. That project has a baseline `greet.js` that prints `hello`, a check `unit-tests` that runs every `test/*.test.js`, and an approved fixture Intent whose Cucumber acceptance requires `node greet.js --loud` to print `HELLO`. A scripted local provider (`KOGEN_PROVIDER_URL`) answers:
  1. builder: write `lib/shout.js` and `test/shout.test.js`, then finish;
  2. builder: wire `greet.js`, then finish;
  3. reviewer: approve.

  The step checks:
  - the requests come in the order builder, builder, reviewer, and the branch still has only the baseline commit when each one arrives (no review and no commit after the red pass);
  - the output of the first finish call is exactly today's ordinary feedback (pinned in the feature), a blank line and the reminder;
  - every builder request carries the checkout's `build/prompts/builder.md` with the bullet exactly once.
- addressed: Synthetic acceptance 2 (the scripted builder then wires the entry point and finishes: green, review and commit as today). The same scenario requires that the program exits 0 with `error` null, that the latest pass's checks are `unit-tests` with status 0, that the Build lands exactly one commit, and that the committed `greet.js`, `lib/shout.js` and `test/shout.test.js` hold the scripted contents.
- addressed: Synthetic acceptance 3 (a red pass at a review stage gets no reminder). Scenario "review-repair": the builder implements everything and finishes. The reviewer's repair writes back the baseline `greet.js` and approves, and final verification is red. The reviewer's next request must carry exactly today's repair feedback, pinned in the feature, which has no reminder. The reviewer then restores `greet.js` and approves, and the Build lands one commit with only one builder request.
- addressed: Synthetic acceptance 4 (the ordinary per-item feedback text, verification order and green decisions are byte-identical apart from the reminder). Both scripted Builds pin today's exact feedback text. That text carries today's order: acceptance first, then `not run because an earlier item failed: unit-tests`. With the bullet and the reminder taken out, `named-feedback-compat.json` and `context-external-golden.json` must equal today's sha256, and the other frozen goldens keep their bytes. The existing tests TestNamedFeedbackCompatibility, TestContextExternalNativeBytes, TestDefinitionsMatchTheElixirDefinitions, TestStateGoldenFreshCheckpoint, TestKReviewRetryUnchangedValidRequests and TestOpeningMessageEqualsTheElixirReference each pass in their own `go test -run '^Name$'` run.
- addressed: Synthetic acceptance 5 (the instructions state the finish statement exactly once, and no other instruction text changes). Scenario 3 checks four things: the bullet occurs exactly once, directly before the finish bullet; taking it out gives today's sha256 `915b0bc6…`; the new file is 4160 bytes with sha256 `2729f3c4…`; and `prompts_golden.json` records those values. TestPromptsAreByteEqualToTheElixirFunctions passes in its own run.
- addressed: delivery decision (the good case of every observer runs green on the current tree before approval). The observer's controls run the real program on the unchanged tree and judge real captures. Their good case was observed accepting (exit 0) and their bad case rejecting (exit 1); see Feasibility.
- omitted: the `finish` tool's description and external executors' own instruction text are not changed ("no other instruction text changes"). Whether a real model obeys the reminder is not provable by scripted Builds and is not claimed. No external protocol or library facts are involved, so no documentation lookup was needed.

## Tests
- A red whole-Intent pass that goes back to the builder ends with a blank line and the reminder, after unchanged ordinary feedback and notices, for native and external builders: add an integration test in package `build` using the existing scripted-provider and fake-executor fixtures (as in `compatCapture`) that compares the builder's received feedback with the ordinary feedback + `"\n\n"` + reminder; acceptance scenario "partial-then-wired" through the `kogen-build build` CLI (scenario)
- After the builder wires the entry point the pass is green and review and commit happen as today: acceptance scenario "partial-then-wired" (scenario)
- Review repair feedback and scoped stage-gate feedback carry no reminder: acceptance scenario "review-repair" through the CLI; add a no-reminder assertion to an existing stage-gate red fixture in package `build`; TestKReviewRetryUnchangedValidRequests stays unchanged (integration)
- Ordinary per-item feedback, verification order and green decisions are unchanged apart from the reminder: regenerate `named-feedback-compat.json`, checked by TestNamedFeedbackCompatibility; acceptance scenario 4 checks that only the two lines were added (regression)
- The bullet is in `build/prompts/builder.md` exactly once, directly before the finish bullet, and no other instruction text changes: regenerate `prompts_golden.json` and `context-external-golden.json`, checked by TestPromptsAreByteEqualToTheElixirFunctions and TestContextExternalNativeBytes; acceptance scenario 3; TestDefinitionsMatchTheElixirDefinitions stays unchanged (existing)

## Feasibility
- In a scratch copy of the unchanged checkout (outside the repository), the step file built `./cmd/kogen-build` and ran both scripted Builds on fresh fixture projects with the scripted provider. The runs took about 8 s together, and both landed one commit with exit 0. The real ordinary feedback was observed and pinned in the feature:
  - "partial-then-wired": the requests were builder, builder, reviewer, with 1 commit on the branch at each, and the latest pass's checks were `[["unit-tests",0]]`;
  - "review-repair": the requests were builder, reviewer, reviewer, and the reviewer got `Final verification of your repairs failed. …` followed by today's `Verification pass 2 is not green. … then call submit_review again.` feedback.
- The fixture's own step throws an error whose stack is only its message. That keeps the scratch verification folder's path out of the feedback; with a default stack the feedback held that varying path.
- The seven existing Go tests the feature names passed on the unchanged tree in one `go test -count=1 ./build -run` run (exit 0, 9.8 s). A Cucumber `--dry-run --strict` of the feature with the step file bound all 22 steps (4 scenarios), with none undefined.
- evidence: {"id":"whole-intent-reminder-observer","kind":"observer","inputs":["features/steps/k-build-finish-whole-intent-reminder__steps.js",".kogen/intents/k-build-finish-whole-intent-reminder/acceptance.feature"],"observers":["features/steps/k-build-finish-whole-intent-reminder__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-finish-whole-intent-reminder__steps.js control-good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-finish-whole-intent-reminder__steps.js control-bad","exit":1,"output":"observer rejected\n"}]}

The controls run both scripted Builds with the real program on the current tree. They judge the real captures with the same `judge` the scenarios use and the expectations read from the feature. An existing instructions bullet (`- Do not commit, push…`) stands in for the finish bullet. This lets the judge reach every assertion that the missing feature masks today.
- control-good: the real captures, with the reminder appended after a blank line to the builder's red-pass feedback as the requested behaviour sends it. Both modes are accepted.
- control-bad: five variants, each of which must be rejected only in its own area:
  - today's unchanged builder feedback;
  - the reminder put before the ordinary feedback;
  - the reminder appended, but with a line of the ordinary feedback dropped;
  - the reminder added to the reviewer's repair feedback;
  - a commit already present when the builder's second request arrived.

In scratch, control-good printed `observer accepted` (exit 0) and control-bad printed `observer rejected` (exit 1) with these exact step bytes. The controls qualify the observer; they are not evidence that the feature is implemented.
