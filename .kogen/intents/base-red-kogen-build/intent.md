# Bound the slow kogen-build go-test check

Slug: base-red-kogen-build
Model: not applicable — the change makes existing tests faster and has no state of its own
Acceptance:
- .kogen/intents/base-red-kogen-build/acceptance.feature
- features/steps/base-red-kogen-build__steps.js

The go-test check (`GOFLAGS=-trimpath go test ./...`) fails on a clean main because the `build` package's tests do not finish inside the bound: a run with a 240 s timeout panicked with "test timed out" and many parallel tests still running. Fix the slow tests of the `build` package by replacing real waits and wall-clock thresholds with injected clocks and fake commands, without removing, skipping or weakening any test or assertion, without raising any timeout or the Build verification bound, and without any new flag, setting or environment variable.

## Decisions
- The slow check is go-test; the package that does not finish is `build`. A bounded run on this host (go test -count=1 -timeout 90s ./build) reported these tests still running: TestOneInvalidVerdictIsSentBackAndASecondEndsTheBuild (29s), TestClaudeLineupReviewExactTree (22s) and TestClaudeLineupReviewMaterialDelivery (22s); the Build must find and fix the real waits behind them and the other slow tests, for example the 20 s `time.After` in `hang` and `time.Sleep` polling in build tests. (evidence: build/helper_test.go; build/review_test.go; build/baseline_wall_test.go) (assumption: the slow tests were observed on one host only; the second-host run and the named check build-host-stop-matches-failure-kind, which matches no test or string in the repository, were not reproduced) (source: request)
- Acceptance requires `go test -count=1 -timeout 100s ./build` to pass on the tree, so the package finishes well inside the 120 s step limit with a timeout that is not raised. (assumption: 100 s is the chosen bound for acceptance; it is not the Build verification bound, which is unchanged) (source: request)
- The Build records the go test ./build time before and after on the same host in its decision. (source: request)
- No test is removed, skipped or weakened, and no timeout, flag, setting or environment variable is added. (source: request)

## Source comparison
- Request: fix or bound the slow or hanging test with injected clocks and fake commands; addressed by the Decisions above.
- Request scenario 2 (original assertion kept on an injected clock): covered by the existing tests being kept unchanged in assertion; omitted as a separate acceptance step because the assertions live in the Go tests themselves.

## Tests
- the build package's tests finish inside a 100 s go test timeout: run go test ./build with -timeout 100s (integration)
- the changed tests keep their original assertions on an injected clock or fake command: the existing Go tests of build/ (existing)

## Feasibility
- none
