# Report executor spend in every Build outcome

Slug: build-output-reports-executor-spend
Model: not applicable — report text only; no new workflow state or decisions
Acceptance:
- .kogen/intents/build-output-reports-executor-spend/acceptance.feature
- features/steps/build-output-reports-executor-spend__steps.js
- features/steps/build-output-reports-executor-spend__driver/fixture_test.go.txt

A Build's output must report what the Build spent. Today the landed summary prints `tokens: 0` for a Build whose builder or reviewer ran through an external executor (claude-code), which contradicts the executor lines below it; and a Build that stops (blocked, round-limited, review-blocked, interrupted) prints no repairs/tokens/time line and no executor lines, although result.json records the figures. One helper in the build package now writes the spend lines, and both the landed summary (build/commit.go) and the stop report (`details` in build/stop.go, when `report` is true) use it. The helper reuses `executorUsageLine` for the executor lines' text.

The spend lines, in this order: the repairs line `  repairs: <n> · tokens: <native total> · time: <duration>` (joined by ` · `), except that its tokens part is left out when the native total `b.Usage.Total()` is exactly "0" and at least one executor line follows (so `  repairs: 0 · time: 3m25s` when the builder round sleeps 205 s and the review round approves without sleeping); in every other case (a native Build, a native total other than "0" including "unknown", or no executor line) it stays as today; then the `  executor: ` line for a non-native builder, with the same text and condition as today; then the `  review executor: ` line when there is a review executor, with the same text as today.

In the landed summary the spend lines stay where they are. In the stop report they go immediately after the `  edited: ` line (or after the comparison lines when nothing was edited) and immediately before the `  run log: ` line, for every stop with `report` true (blocked, round-limited, review-blocked, interrupted). A stop with `report` false (the startup phase) prints nothing new. A native Build prints no executor line (no `  executor: native`) on landing or on stop.

Exact literals the behavior is specified by:
- Landed line shape today (landed AoR example): `  repairs: 1 · tokens: 0 · time: 13m07s`, followed by `  executor: claude-code (harness-reported tokens input=6607784 cached=6464136 output=44938 reasoning=unknown, estimate $3.3403)` and `  review executor: claude-code (harness-reported tokens input=445080 cached=396139 output=5682 reasoning=unknown, estimate $0.2922)`.
- A failed Build today (release 30cf6619) prints `failed rails-ft-saas-billing: build_blocked (Build 76e139e5)`, then `  not run because an earlier item failed: test`, `  still failing: acceptance`, the comparison, the edited line and the run log line, and no spend; after the change the spend lines sit between the `  edited: ` line and the `  run log: ` line.
- Round-limit stop (external builder, two rounds of usage 100 input, 40 output, 0.25 cost): lines `  edited: app.txt`, then `  repairs: <n> · time: <duration>`, then `  executor: claude-code (harness-reported tokens input=200 cached=0 output=80 reasoning=unknown, estimate $0.5000)`, then the line starting `  run log: `.
- External reviewer blocks: the review round returns a changes verdict with a blocking finding and usage 50 input, 10 cached, 5 output, 0.25 cost, so the Build stops with candidate/review_blocked, and the stop prints `  executor: claude-code (harness-reported tokens input=100 cached=0 output=40 reasoning=unknown, estimate $0.5000)` directly followed by `  review executor: claude-code (harness-reported tokens input=50 cached=10 output=5 reasoning=unknown, estimate $0.2500)`.

Report text only: result.json (`usage`, `attempt_usage`, `executor`, `review_executor`), the run log, facts, exit codes and every decision stay unchanged; no new configuration; no change outside the build package. The existing build_test.go expectation `repairs: 0 · tokens: 2 · time: \d+s` for a native Build keeps passing unchanged. Existing stop expectations that match whole adjacent lines such as `edited` followed by `run log` may be updated only to insert the new spend lines. There is deliberately no scenario for an external builder with a nonzero native total (no fixture exists for it); the omission rule is pinned by the scenarios on the landed external, native landed, round-limit and native stop cases.

The Build also adds one focused, model-free Go test file in the build package next to executor_test.go and review_lineup_test.go (run with `go test ./build -run` and its test-name pattern), covering the six scenarios of acceptance.feature with the existing fake executors; the supplied acceptance fixture is separate from it.

## Decisions
- One helper in the build package (called `putSpend` in the reference control) writes the repairs, executor and review executor lines; commit.go's landed summary and stop.go's `details` (inside `if report`, after the `edited` line and before the `run log` line) both call it (decision: D1, round 1) (evidence: build/commit.go; build/stop.go) (source: request)
- The tokens part of the repairs line is omitted only when `b.Usage.Total()` is exactly "0" and at least one executor line (builder non-native, or a review executor) follows (decision: D2, round 1) (evidence: build/commit.go; build/build_test.go) (source: request)
- Acceptance cannot rely on a test file the Build must invent, so the protected acceptance step compiles a supplied Go fixture (one test per scenario, using newFakeExecutor, newLineupExec, useLineup, withLineup, externalWritesApp, harnessUsage, verdictJSON and testing/synctest) with a temporary Go overlay and runs it; the Build still writes its own focused test file per the request; the startup-phase stop is the existing commit-msg-hook baseline_changed stop (decision: D3, round 1) (evidence: build/executor_test.go; build/review_lineup_test.go; build/progress_elapsed_test.go; build/commit_test.go; build/helper_test.go) (source: request)

## Source comparison
- Addressed — Request Expected 1 (single helper, spend line order and tokens omission rule): specified above; scenarios 1, 2, 3 and 5 pin it.
- Addressed — Request Expected 2 (landed spend lines stay in place): scenarios 1 and 2.
- Addressed — Request Expected 3 (stop report placement for every `report` true stop; nothing for startup): scenarios 3 and 4 (adjacent lines), 5, 6. Interrupted shares the same `details` call and is not separately executed.
- Addressed — Request Expected 4 (native prints no executor line): scenario 5.
- Addressed — Observed AoR evidence (the `tokens: 0` lines beside executor lines; the build_blocked report with `  still failing: acceptance` and no spend) is retained as motivation and literals above and in acceptance.feature; it is not reproduced locally.
- Replaced — the earlier narrow requests build-summary-omits-zero-native-tokens and build-failure-report-shows-executor-usage are superseded by this one change.

## Tests
- Landed external builder and reviewer omit the zero native token total, with executor lines after: add a focused Go test next to the external executor tests, run via `go test ./build -run` (unit)
- A landed native Build keeps ` · tokens: 2 · `: the existing build_test.go expectation plus the native landed case (unit)
- An external builder's round-limit stop prints edited, repairs, executor and run log lines adjacently: scenario 3 test (unit)
- An external reviewer's blocking verdict stop (candidate/review_blocked) prints the executor then review executor lines adjacently: scenario 4 test (unit)
- A native Build's stop prints a repairs line with tokens and no executor line: scenario 5 test (unit)
- A startup-phase stop prints no spend lines: scenario 6 test (unit)

## Feasibility
- none
