# Mask plain ms durations in the budget-off test

Slug: k-build-budget-off-sweep-ms
Model: not applicable — only one test's masking of wall-clock values changes; no product state or transition is added
Acceptance:
- .kogen/intents/k-build-budget-off-sweep-ms/acceptance.feature
- features/steps/k-build-budget-off-sweep-ms__steps.js

This change fixes `TestBuildRequestBudgetOff` (build/build_request_budget_test.go:283). kogen-verification `292a2d1` "Sweep orphans after every verification step" (Kogen-Intent: k-verify-sweep-other-steps) turned it red. From that commit on, every project command run through `runStepAs` is swept once it returns, and each sweep writes one log event `{"event":"step_sweep","name":<step>,"stopped":<int>,"ms":<int>}` (verify/env.go, verify/concurrent.go, verify/sweep.go). Its `ms` is the measured duration of the sweep. Before that commit, only acceptance was swept.

The test runs a Build twice, with the budget omitted and with budget 0. It requires the two observations (requests, log.jsonl, checkpoint.json, result.json, commit message) to be identical after `observeBuild` masks volatile values. Today the mask (`volatileKeys`, :195-201, and `volatileKey`, :203-205) covers a fixed set of keys plus any key ending in `_ms` or `_at`, but not the plain key `ms`. So the two runs differ by the sweep's own duration, and the test fails at :335 with:

```
log line 24 differs at byte 45:
omitted: ...{"at":"<volatile>","event":"step_sweep","ms":3,"name":"acceptance","stopped":0}
zero:    ..."ms":4,...
```

The line is 24, 34 or 44 depending on the run. Against verification main `3cb7305` (`v0.0.0-20261103201747-3cb7305a5690`), the unchanged test fails in every run in which the two Builds' sweep durations differ. That was 5 of 5 runs in one scratch run and 2 of 5 runs in Kogen's readiness run (fail, pass, pass, pass, fail). Requiring 5 passing runs out of 5 therefore separates the fix from the unchanged tree. With the go.mod pin `v0.0.0-20261103020618-01aa16c01767`, the test passes.

## The rule
- `volatileKey(k)` is true, so `observeBuild`'s `walk` replaces the value with `"<volatile>"` at any depth on both sides, when any of these holds:
  - `k` is in `volatileKeys` (unchanged set);
  - `k` ends in `_ms` or `_at` (unchanged);
  - `k` is exactly one of the plain wall-clock duration keys `ms`, `elapsed`, `duration` or `wall` (new). `ms` is the step_sweep duration. `wall` is the seconds duration that kogen-verification's probe record already writes (verify/probe.go `json:"wall"`). `elapsed` and `duration` are the remaining plain duration names.
- Matching is on the whole key: `ms_count`, `stopped`, `name`, `event`, `count`, `limit`, `outcome`, `pass` and `offset` stay compared byte for byte. The plain `offset` key is not volatile. In these observations it is the read_file tool's integer offset argument (kogen-loop tools/defs.go), not a clock value. Wall-clock offsets carry the `_ms` suffix (`request_offset_ms`) and are already masked.
- The mask does not name or special-case any producer event: there is no `event == "step_sweep"` branch, and no line is dropped or reordered. The test does not assert the presence, number, position or fields of `step_sweep` events.
- Everything else in `observeBuild` stays as it is today: the `cache_ids` handling, the path and run-id replacements, the hash masking and `budgetObservation`. The rest of `TestBuildRequestBudgetOff` also stays as it is: the 8-request count, the `build_request_budget` absence checks, and the `reflect.DeepEqual` comparison. Its failure report keeps the form `<part> line <n> differs at byte <k>:` followed by the `omitted: ...` and `zero: ...` lines. So two Builds that differ in a non-timing field, for example step_sweep's `stopped`, still fail the test.
- The comment above `volatileKeys` also names the plain duration keys.
- Only build/build_request_budget_test.go changes. go.mod, go.sum, production code, goldens, README and the consumes/ suites do not change.

Acceptance runs well within the 120-second limit, which the previous attempt exceeded. Measured times are about 27 s for TestBuildRequestBudgetOff with `-count=5` against main and about 6 s for each `-count=1` run (pin and counterexample), plus a few seconds of compilation each. Separate `go test` invocations of the package run one after the other, so running them in parallel saves nothing. Each go test process has a 100 s limit.

## Decisions
- The plain key `ms` and any other plain wall-clock duration key are volatile, next to the existing `_ms` and `_at` rules; the mask does not name the producer's events (source: request)
- The test keeps asserting that the budget-off Build is identical to the budget-omitted one, and does not pin the presence, number, position or fields of `step_sweep` events (source: request)
- The test still fails if the two runs differ in a non-timing field (source: request)

## Source comparison
- Addressed: "`go test -count=5 ./build -run '^TestBuildRequestBudgetOff$'` passes against kogen-verification main (`3cb7305` today)". Scenario 1 runs `go test -json -count=5 -run '^TestBuildRequestBudgetOff$' ./build` with a temporary go.mod (passed with `-modfile`) that replaces kogen-verification with the module cache folder of `v0.0.0-20261103201747-3cb7305a5690`. All 5 runs must pass.
- Addressed, with a lower count: "... and against the pinned `01aa16c01767`". Scenario 2 asserts that go.mod still requires `github.com/KogenAI/kogen-verification v0.0.0-20261103020618-01aa16c01767` without a replace. It then runs the test with `-count=1` to stay within the 120 s acceptance limit. The configured go-test check also runs the test with the pin.
- Addressed: "`consumer-check.sh github.com/KogenAI/kogen-verification <verification-main-checkout> <mirrors> 'kogen-build|<fix-sha>|.|'` no longer lists TestBuildRequestBudgetOff". consumer-check.sh is not in this repository. Its check for this test is the same `go test` run against the verification main checkout, which scenario 1 performs.
- Addressed: "The test still fails if the two runs differ in a non-timing field". Scenario 3 uses a scratch copy of verification `3cb7305` in which each step_sweep event's `stopped` is the real count plus a process-wide sequence number. In verify/env.go and verify/concurrent.go, `"stopped": stopped,` becomes `"stopped": stopped + kBudgetOffSweepSeq(),`. The test must fail in its one run with a reported differing line that shows `"stopped":`. This also refuses a fix that drops or skips step_sweep lines, masks every number, or masks `stopped`.
- Addressed: "a key `ms` (and any other plain duration or offset key) is volatile ... without naming the producer's events" (see the rule above). "Plain offset key" is read as a wall-clock offset. The only plain `offset` key in these observations is a tool argument, and masking it would hide request differences. Wall-clock offsets stay covered by the `_ms` suffix.
- Conflicting measurement, kept truthful: the request reports the failure as deterministic, with 4 of 4 runs failing. Kogen's readiness run of the unchanged tree against `3cb7305` failed 2 of 5 runs (the other 3 had equal sweep durations). The acceptance requirement (5 of 5 pass) is unchanged.
- Addressed, the automatic retry's cause ("acceptance timed out: command exceeds the 120-second limit"). Acceptance has three scenarios with measured run times of about 27 s, 6 s and 6 s plus compilation. Only scenario 1 runs `-count=5`.
- Preserved literals: test name, file and lines, the step_sweep event shape, commits `292a2d1`, `3cb7305`, `57542e9` and `01aa16c01767`, and module versions `v0.0.0-20261103201747-3cb7305a5690` and `v0.0.0-20261103020618-01aa16c01767`.
- Omitted: the bisect itself and changes to kogen-verification. They live outside this repository.

## Tests
- A plain `ms` duration (and `elapsed`, `duration`, `wall`) is volatile, so the budget-off and budget-omitted Builds compare equal against verification main: update `volatileKey`/`volatileKeys` so TestBuildRequestBudgetOff passes 5 times in a row against `3cb7305` (acceptance scenario 1) (regression)
- The key rule: add a focused test in build/build_request_budget_test.go asserting that `volatileKey` is true for `ms`, `elapsed`, `duration`, `wall`, `wall_ms`, `request_offset_ms`, `at` and `ended_at`, and false for `stopped`, `name`, `event`, `offset`, `count`, `ms_count` and `outcome` (unit)
- A non-timing difference between the two Builds still fails the test: acceptance scenario 3 with the counting-`stopped` producer (regression)
- With the go.mod pin the test still passes: TestBuildRequestBudgetOff with the committed go.mod (acceptance scenario 2, configured go-test check) (existing)

## Feasibility
- These checks ran in this read-only external session, in scratch copies outside the checkout. They are not Kogen probe records.
  - Unchanged tree against verification main `v0.0.0-20261103201747-3cb7305a5690`, using a temporary `-modfile` with a directory replace to the module cache folder: `go test -json -count=5 -run '^TestBuildRequestBudgetOff$' ./build` failed 5 of 5 runs (`log line 24 differs at byte 46`) in 26 s. Kogen's readiness run of the same scenario failed 2 of 5 runs.
  - With the committed go.mod, the same command passed 5 of 5 runs in 26 s, exit 0.
  - With a cold GOCACHE, compiling and running `-count=1` took 11 s.
  - Two `go test` invocations of package build started together ran one after the other (54 s total).
  - A scratch prototype that only adds `"ms": true` to `volatileKeys` passed `-count=5` against main (26 s).
  - Against the counting-`stopped` copy of main, the same prototype failed with `log line 24 differs at byte 88:`, showing `"ms":"<volatile>","name":"acceptance","stopped":1}` vs `"stopped":4}`. On the unchanged tree, the counting copy also fails; the first difference is at `ms`, on a line that also shows `stopped`.
  - Kogen's readiness run of the unchanged tree: scenarios 2 and 3 passed and scenario 1 failed as above, within the time limit.
- evidence: {"id":"budget-off-run-judge","kind":"observer","inputs":["features/steps/k-build-budget-off-sweep-ms__steps.js"],"observers":["features/steps/k-build-budget-off-sweep-ms__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-budget-off-sweep-ms__steps.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-budget-off-sweep-ms__steps.js bad","exit":1,"output":"rejected\n"}]}
- The observer's known-good inputs are real `go test -json` stdout:
  - the `-count=5` passing run with the committed go.mod (44 rows, exit 0);
  - the `-count=1` failing run against the counting-`stopped` copy of `3cb7305` on the unchanged tree (15 rows, exit 1).
- The bad inputs are variants of those runs:
  - for the pass judge: one failed run, four runs, a skipped run, a nonzero exit, no package result, no test run;
  - for the comparison-failure judge: the passing run, a failure without a differing line, a difference not showing `stopped`, a build without test rows, a zero exit.
- The controls qualify the run judges only, not product behaviour.
