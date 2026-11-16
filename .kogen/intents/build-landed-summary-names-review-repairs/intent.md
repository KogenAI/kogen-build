# Name review repairs in the landed summary

Slug: build-landed-summary-names-review-repairs
Model: not applicable — the change is one report line derived from two recorded tree identities; it has no state machine of its own.
Acceptance:
- .kogen/intents/build-landed-summary-names-review-repairs/acceptance.feature
- features/steps/build-landed-summary-names-review-repairs__steps.js
- features/steps/build-landed-summary-names-review-repairs__driver/fixture_test.go.txt

A landed Build does not say that its review rewrote code: `repairs: N` counts the builder's repairs and the reviewer's repairs together, and no line names the files the reviewer changed. This Intent makes the landed summary (the output of `commitApproved` in build/commit.go) answer "did the review write part of this commit, and which files?" for every landed Build.

When the review changed the candidate before approving it, the landed summary prints one more line, `  review repaired: ` followed by the paths that differ between the tree first delivered to the reviewer (the green pass the review started on) and the committed tree. Paths are sorted; at most 5 are named; when more than 5 differ the list ends with `, and N more` where N is the number left out (`  review repaired: app.txt, r1.txt, r2.txt, r3.txt, r4.txt, and 2 more`). The line goes immediately after the `  review: approved` line and its `  note: ` lines and immediately before the `  repairs: ` line. When the review changed nothing no such line is printed, whether the Build landed in one pass or after a builder repair. The paths come from the review's own start and end trees, never from the builder's edits, and the line is the same for the native and the external reviewer. The review's start tree is kept in the Build's recorded state (a new optional `review_start` field, omitted when empty) so a Build resumed inside its review reports the same paths; a checkpoint without it prints no line.

Unchanged: the `  repairs: ` line (counting and text), `  review: approved`, the `  note: ` lines, result.json, review-1.json, the run log, facts, exit codes, every decision, stop reports (the `  edited: ` line), configuration, the golden state files under build/testdata/state, and kogen-review.

## Decisions
- The review start tree is recorded in a new `State.ReviewStart` field (`json:"review_start,omitempty"`), set in `steps.Review` (build/route.go) from the green pass handed to the review, so every reviewer implementation and every fresh review shares it, and read in `commitApproved`; the line is derived from `repo.Changed(ReviewStart, committed tree)` sorted. A Build resumed inside its review keeps the recorded start tree through the checkpoint. (decision: D1, round 1) (evidence: build/host.go; build/core_state.go; build/commit.go; build/route.go; build/hooks_test.go) (source: request)
- The `repairs` counter (`b.Repairs`, incremented by `reviewEnv.Repaired` and after a red verification) is not changed. The native scenario is a Build whose builder repairs once after a red verification and whose reviewer then repairs app.txt with write_file and approves, so its summary shows `  repairs: 1 ` and the review repaired line; a green native reviewer repair alone does not increment the counter in the current code and that counting stays. (decision: D2, round 1) (evidence: build/review_test.go; build/host.go; build/build_test.go) (assumption: the native reviewer repair scenario carries a builder repair so that `  repairs: 1 ` holds without changing the counting) (source: request)
- A failure to compare the two trees, an empty path list or a missing start tree prints no line; the landing is never failed by this report. (decision: D3, round 1) (evidence: build/commit.go) (assumption: report-only failure handling is not specified; silence was chosen) (source: request)
- Acceptance has exactly four scenarios, one per requested case, and compiles a fixture Go test file as an overlay of build/review_repair_summary_acceptance_test.go, as the spend-report Intent does, so no test the Build must invent is run. The native reviewer scenario also covers the requested resume rule with a second step pair: a scripted reviewer interrupts a review (as TestAResumeInAReviewStageContinuesWithTheReviewer does), a scripted resumed reviewer repairs app.txt and approves, and the resumed Build's landing must name app.txt. (decision: D4, round 1) (evidence: features/steps/build-output-reports-executor-spend__steps.js; build/review_lineup_test.go; build/hooks_test.go) (source: request)

## Source comparison
- Expected 1 (line text, sorting, cap of 5, `, and N more`, placement before `  repairs: `): addressed by scenarios 1, 3 and 4.
- Expected 2 (no line when the review changed nothing, builder repair never produces it): addressed by scenario 2.
- Expected 3 (paths from review start and end trees, native and external, resumed Build keeps the start tree): addressed by the `ReviewStart` state field and scenarios 1 and 4 (scenario 4 includes the resumed review).
- Observed evidence, quoted: stderr carries `kogen: review 1: the candidate changed after its contents were delivered`; the summary of a Build with a repair prints `  repairs: 1 ` or `  repairs: 2 `; the example landing is
```
landed rails-ft-data-export-and-import 6e68c26f (Build c463315b)
  checks: acceptance 5/5, model not applicable, test passed
  checks, before the Build → after (verification pass 2):
    1. test: passed → passed
  review: approved
  repairs: 1 · tokens: 0 · time: 18m58s
```
  whose stderr says the reviewer changed `app/models/zip_file/reader.rb` and `test/models/account/import_test.rb`; other repaired Builds print `review: approved, 1 note(s)` (the form `, N note(s)`). This is the problem statement the line answers.
- Source places: `  checks: ` and the `  note: ` lines in `commitApproved`; `reviewEnv.Repaired` and `b.Repairs`; `Approved{Pass, Review}`; `steps.Review` and `delegate.Review`. Stop reports (`  edited: `) are untouched.
- Controls run only the fixture tests of this Intent (`go test ./build -run` style filtering by the exact names TestRepairSummaryExternalRepair, TestRepairSummaryBuilderRepairOnly, TestRepairSummaryManyPaths, TestRepairSummaryNativeRepair, TestRepairSummaryResumedReview) so they fit the 300 s budget.

## Tests
- A reviewer repair on the external route is named, in order, between the review and repairs lines: TestLandedSummaryNamesExternalReviewRepair in build/review_repair_summary_test.go (unit)
- A builder repair with an unchanged review prints no review repaired line: TestLandedSummaryOmitsLineForBuilderRepair in build/review_repair_summary_test.go (unit)
- More than five repaired paths name five and `, and N more`: TestLandedSummaryCapsRepairedPaths in build/review_repair_summary_test.go (unit)
- A native reviewer repair is named the same way, and a review interrupted and resumed before it approves still names the repaired paths: TestLandedSummaryNamesNativeReviewRepair in build/review_repair_summary_test.go (unit)

## Feasibility
- none
