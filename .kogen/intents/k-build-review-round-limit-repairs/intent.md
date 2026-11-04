# Vary the review round-limit test's failing items

Slug: k-build-review-round-limit-repairs
Model: not applicable — only the scripted rounds and the fixture configuration of one existing test change; no product state or transition is added
Acceptance:
- .kogen/intents/k-build-review-round-limit-repairs/acceptance.feature
- features/steps/k-build-review-round-limit-repairs__steps.js
- features/steps/k-build-review-round-limit-repairs__controls.js

This change fixes `TestClaudeLineupReviewRoundLimit` (build/review_lineup_test.go:516). kogen-review `932c061` "Stop nonconverging reviewer repairs" (Kogen-Intent: k-review-repair-no-progress-stop) turned it red. Before that commit the reviewer's repair loop returned to the reviewer without limit. Now it stops with `candidate/review_blocked` after three consecutive final verifications that fail with the same set of stage-qualified failing items, or after twenty unsuccessful repair rounds (review/independent.go). kogen-review computes that set from the failing stages: `acceptance`, `model`, `model:<path>`, `setup:<name>`, `check:<name>`. Today every scripted review round writes `red\n` to app.txt and approves, so every final verification fails with the same set `acceptance`. With kogen-review main (`81d3bae`, module version `v0.0.0-20261103212337-81d3baef1d48`) the `default` and `zero` subtests (cap 6) fail at build/review_lineup_test.go:536 with `stop lacks "candidate/executor_round_limit:"`. The Build stopped with `candidate/review_blocked: repeated identical final failures after 3 repair rounds; current failing stage/items: acceptance`. `one` and `two` still pass.

After this change the test proves what kogen-build owns, without relying on the reviewer's repair loop being unbounded. The executor's per-role round cap ends the review role with `candidate/executor_round_limit`. The cap is the review cap, or 6 when that is 0. The work is preserved and no other role is restarted. Every scripted review round still leaves a tree that final verification finds red, so the reviewer never gets it green. Consecutive rounds now fail in different stages, so no two consecutive final verifications fail with the same set of items, and the cap is reached before any no-progress stop of kogen-review. The test asserts none of kogen-review's stop thresholds or wording.

## The change (build/review_lineup_test.go, `TestClaudeLineupReviewRoundLimit` only)
- **Fixture configuration.** The subtest's env is made with one configured check, `e := newEnv(t, func(c *project.Config) { c.Checks = []project.Check{{Name: "app", Run: "if grep -qx unchecked app.txt; then exit 1; fi"}} })`, instead of `newEnv(t)`. The check passes on `baseline\n`, on the builder's `candidate\n` and on `red\n`, and fails only on a line `unchecked`. The fixture's acceptance (accept.sh in helper_test.go, unchanged) fails only on a line `red`.
- **Scripted review rounds.** `f.review` uses its round number `n`, which starts at 1 for each review session. An odd round writes `red\n` to app.txt with `lineupWriteApp`: acceptance fails and the `app` check passes. An even round writes `unchecked\n`: acceptance passes and the `app` check fails. Each round still returns `verdictJSON("approve")`. The failing set therefore alternates between `acceptance` and `check:app`, whether or not verification runs later stages after a failing one.
- **Preserved work.** The `preserved work` assertion expects the last round's content: `red\n` when `tc.max` is odd (`one`), `unchecked\n` when it is even (`default`, `two`, `zero`).
- **Comments.** The comment above `f.review` says that the rounds alternate between a tree whose acceptance fails and a tree whose `app` check fails, so final verification never fails with the same items twice in a row and the reviewer never gets it green. It must not name kogen-review's thresholds or stop wording.
- **Unchanged.** Everything else in the test stays as it is today:
  - the four subtests `default` (nil, 6), `one` (1, 1), `two` (2, 2) and `zero` (0, 6);
  - `wantStop(t, a, "candidate/executor_round_limit")`;
  - the review round count equal to `tc.max`;
  - one review session, one build session and one build round;
  - HEAD unchanged;
  - the recorded `review_executor.rounds` equal to `tc.max` (`int(x["rounds"].(float64)) != tc.max`, build/review_lineup_test.go:545).
- **Not changed:** go.mod and go.sum (kogen-review stays pinned at `v0.0.0-20261103124347-cfc2fb29917c` without a replace), production code, helper_test.go, the `lineupExec` fake, and every other test.

## Out of scope
- `TestExternalReviewCheckpointsRecordTheGreenBase/an_external_review_keeps_its_base_through_failed_repairs` (build/review_resume_external_test.go:43, `stage = "review", want "review_verify"`) also turns red at `932c061` with the same early stop. The request places it outside this cell and it was not analysed. It is not changed here.
- The other tests that kogen-review main turns red (consumer-check.sh at its parent `111e487` reports other red tests). They are not changed here.

## Decisions
- The test proves the executor's per-role review cap (6 when 0) ends the review role with `candidate/executor_round_limit`, with the work preserved and no other role restarted (source: request)
- The scripted review rounds keep the final verification red but fail with a different set of items from one round to the next, so the cap, not kogen-review's no-progress stop, is reached first (source: request)
- The test asserts neither kogen-review's stop thresholds nor its wording (source: request)
- The recorded `review_executor.rounds` still equals the cap in every subtest (source: request)

## Source comparison
- Addressed: "`go test ./build -run '^TestClaudeLineupReviewRoundLimit$'` passes (`default`, `one`, `two`, `zero`) against kogen-review main (`81d3bae` today) and against the pinned `cfc2fb29917c`". Scenario 1 runs the test with the committed go.mod, which pins `v0.0.0-20261103124347-cfc2fb29917c` without a replace. Scenario 2 runs it against `v0.0.0-20261103212337-81d3baef1d48` from the module cache, through a temporary go.mod passed with `-modfile`. Each run must report the test and exactly the four subtests passing.
- Addressed: "`consumer-check.sh github.com/KogenAI/kogen-review <review-main-checkout> <mirrors> 'kogen-build|<fix-sha>|.|'` no longer lists TestClaudeLineupReviewRoundLimit". consumer-check.sh is not in this repository. For this test its predicate is the same `go test` against review main, which scenario 2 runs.
- Addressed: "The recorded `review_executor.rounds` still equals the cap in every subtest (build/review_lineup_test.go:545)". Scenario 5 requires the test body to keep that assertion. Scenarios 1–2 require all four subtests to pass with it.
- Addressed: "must do so without relying on the reviewer repair loop being unbounded", "must not assert the producer's stop thresholds or wording". Scenario 3 runs the test against a copy of kogen-review main whose no-progress stop fires after two identical final failures in a row instead of three, and requires all four subtests to pass. Scenario 5 requires that the test body name none of `review_blocked`, `repeated identical`, `hard cap`, `FailureStreak` or `failure_streak`.
- Addressed: "the executor's per-role round cap ... ends the review role with `candidate/executor_round_limit`". Scenario 4 runs the test against a copy of kogen-review main whose stop counts any three failed final verifications in a row, whatever fails. The test must then still fail in exactly `default` and `zero`. This rules out a test that no longer reaches several red repair rounds, or that accepts any stop.
- Decided here: the extra failing stage is a configured fixture check named `app`, an existing `newEnv` option (as in build/blocker_test.go). Rounds that are green but unreviewed would also break the streak, but the request's comment and Need keep every round red.
- Not run: `932c061` itself (`v0.0.0-20261103124347` < it ≤ main) is not in the module cache. Main contains it, and scenarios 2–4 use main.
- Preserved literals: the test name, the subtest names and caps, `candidate/executor_round_limit`, `review_executor.rounds`, the culprit `932c061`, its parent `111e487`, the pin `cfc2fb29917c` and main `81d3bae` with their module versions, and the quoted failure.

## Tests
- The review cap ends the external review role with `candidate/executor_round_limit` while kogen-review stops repeated identical final failures: updated TestClaudeLineupReviewRoundLimit passes in all four subtests against the pinned kogen-review, against main `81d3bae` and against a main copy that stops after two identical failures in a row (acceptance scenarios 1–3) (regression)
- The test still reaches several red repair rounds and asserts the cap's stop, the session counts, HEAD, the preserved work and the recorded rounds, without naming kogen-review's stop: it fails in `default` and `zero` against an identity-blind main copy, and its body keeps those assertions (acceptance scenarios 4–5) (regression)
- go.mod keeps its kogen-review pin and the configured checks run with it: acceptance scenario 1 and the configured go-test check (existing)

## Feasibility
- I ran these in this read-only external session, in temporary folders outside the checkout. They are not Kogen probe records.
  - `go test -modfile=<tmp go.mod replacing kogen-review with the module cache folder of v0.0.0-20261103212337-81d3baef1d48> -count=1 -run '^TestClaudeLineupReviewRoundLimit$' ./build` on the unchanged tree fails `default` and `zero` at review_lineup_test.go:536 with the request's `candidate/review_blocked: repeated identical final failures after 3 repair rounds; current failing stage/items: acceptance` (about 6 s). With the committed go.mod the four subtests pass (exit 0).
  - kogen-review main's review/independent.go contains `slices.Equal(ids, r.sess.FailureIdentities)` once and `FailureStreak >= 3` once. The copy with the first replaced by `len(ids) > 0` compiles. On the unchanged tree it fails exactly `default` and `zero` (exit 1). Under the change it still does, since every round's final verification stays red.
  - `failureIdentities` (review/independent.go) yields `acceptance` for a failing acceptance stage and `check:<name>` for a failing check.
- evidence: {"id":"round-limit-judges","kind":"observer","inputs":["features/steps/k-build-review-round-limit-repairs__steps.js","features/steps/k-build-review-round-limit-repairs__controls.js"],"observers":["features/steps/k-build-review-round-limit-repairs__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-review-round-limit-repairs__steps.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-review-round-limit-repairs__steps.js bad","exit":1,"output":"rejected\n"}]}
- The observer's controls:
  - Run judge, known-good inputs: the real `go test -json` rows (output rows left out) of this test on the unchanged tree. One run is with the pin (exit 0, all pass). The other is against the identity-blind main copy (exit 1, `default` and `zero` failed).
  - Run judge, refused variants: a failed, missing, extra or skipped test; no test run; a wrong exit; no package result; a passing run where failures are required; a failing run where all must pass; a failing run where other subtests must fail.
  - Body judge, known-good input: today's test file (read directly while its SHA-256 is `fc9ea4eb…`, otherwise from git blob `b3b1aea32e2d6102ab2308517dbe93f32a6a8c1d`).
  - Body judge, refused variants: the stop changed to `candidate/review_blocked`, the recorded-rounds assertion removed, a comment naming `repeated identical`, and the function missing.
  - The controls qualify the judges only, not product behaviour.
