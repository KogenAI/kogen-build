# Match host-or-base stop on failure kind

Slug: build-host-stop-matches-failure-kind
Model: not applicable — the change is one comparison rule over a per-pass memory; the scenarios cover each outcome
Acceptance:
- .kogen/intents/build-host-stop-matches-failure-kind/acceptance.feature
- features/steps/build-host-stop-matches-failure-kind__steps.js
- features/steps/build-host-stop-matches-failure-kind__driver/acceptance_test.go.txt

The host-or-base stop in `build/envfailure.go` currently compares the first environment-failure line of a check between two consecutive builder passes as exact text, so a check that fails with disk exhaustion naming a different temporary file in each pass (`write /tmp/a: no space left on device`, then `write /tmp/b: no space left on device`) is not recognised as the same failure and the Build keeps asking the builder for rounds. The environment memory instead records, for each check name, the kind of its environment failure (timeout, disk, memory, file limit) with its first line. A check stops the Build when it fails with an environment failure of the same kind as that check's failure in the previous builder pass of the same Build attempt, whether or not the first lines match exactly. A small local function in `build/envfailure.go` classifies the line returned by `verify.EnvFailure` into its kind by the same phrases that function recognises. The stop keeps its error class and name, its event name, its lines and its exit code; the failure text in the message and event is the current pass's first line. A check whose previous pass failed with a different kind, or did not fail with an environment failure, does not stop the Build, and the memory is still replaced by every builder pass. Out of scope: the verify package, which lines count as environment failures, other Build stop rules, documentation and the Build report layout.

## Decisions
- The kind is computed in `build/envfailure.go` by a local function over the line `verify.EnvFailure` returned: a line starting `panic: test timed out after ` is timeout; one containing `no space left on device` is disk; one containing `cannot allocate memory`, `out of memory` or `Out of memory: Killed process` is memory; one containing `too many open files` is the file limit. The verify package is not changed. (evidence: build/envfailure.go) (assumption: the phrase list was read from verify.EnvFailure in the pinned kogen-verification module version, not from a tracked file; the Build must keep the local function's phrases identical to that function's phrases) (source: request)
- The memory stays keyed by check name and now holds the kind and the first line per check; the stop requires the previous pass to have an environment failure for the same check name of the same kind. (evidence: build/envfailure.go) (source: request)
- A check stops the Build when it failed with the same kind in the previous builder pass, whether or not the first line matches exactly. (source: request)
- The stop keeps its error class and name, event name, lines and exit code; the failure text in the message, lines and event is the current pass's first line. (evidence: build/envfailure.go; build/env_failure_park_test.go) (source: request)
- A check whose previous pass failed with a different kind, or did not fail with an environment failure, does not stop the Build; every builder pass still replaces the memory. (source: request)
- The Build adds focused model-free Go tests for the five outcomes in `build/env_failure_park_test.go` next to the existing host-or-base tests, using its existing `newEnv`-based fixtures. (evidence: build/env_failure_park_test.go) (source: request)
- Acceptance runs five Go tests that its driver copies into `build/` for the run and removes afterwards, because the Build's own test names are not known beforehand; the driver tests use the existing test fixtures of the `build` package. (assumption: the acceptance driver tests are a proof artifact separate from the tests the Build adds to build/env_failure_park_test.go) (source: request)
- The kind is computed in build/envfailure.go by a local function over the line verify.EnvFailure returned, using the same phrases (timeout panic prefix, no space left on device, cannot allocate memory / out of memory / Out of memory: Killed process, too many open files); verify package unchanged. (decision: D1, round 1) (evidence: build/envfailure.go) (assumption: The verify phrase list was read in the module cache copy at the pinned version, not in a tracked file.) (source: request)
- The memory stays keyed by check name and holds kind plus first line; a stop requires the previous pass to have an environment failure of the same kind for the same check. (decision: D2, round 1) (evidence: build/envfailure.go) (source: request)
- A check stops the Build when it failed with the same kind in the previous builder pass, whether or not the first line matches exactly. (decision: D3, round 1) (evidence: build/envfailure.go) (source: request)
- The stop keeps its error class and name, event name, lines and exit code; the failure text is the current pass's first line. (decision: D4, round 1) (evidence: build/envfailure.go; build/env_failure_park_test.go) (source: request)
- A check whose previous pass failed with a different kind or not with an environment failure does not stop the Build; every builder pass replaces the memory. (decision: D5, round 1) (evidence: build/envfailure.go) (source: request)
- The Build adds focused model-free Go tests in build/env_failure_park_test.go next to the existing host-or-base tests. (decision: D6, round 1) (evidence: build/env_failure_park_test.go) (source: request)
- Acceptance copies a driver test file into build/ for the run and removes it afterwards, since the Build's own test names are unknown beforehand. (decision: D7, round 1) (evidence: build/env_failure_park_test.go) (assumption: The acceptance driver tests are a proof artifact separate from the tests the Build adds.) (source: request)
- The kind is computed in build/envfailure.go by a local function over the line verify.EnvFailure returned, using the same phrases (timeout panic prefix, no space left on device, cannot allocate memory / out of memory / Out of memory: Killed process, too many open files); verify package unchanged. (decision: D8, round 1) (evidence: build/envfailure.go) (assumption: The phrase list was read from verify.EnvFailure in the pinned module cache copy, not a tracked file; the Build must keep the phrases identical.) (source: request)

## Source comparison
- Request: record the kind alongside the first line and stop on the same kind; addressed by Decisions D1 to D5 and scenarios 1 and 5.
- Request: different kind, ordinary previous failure and different check name do not stop; addressed by scenarios 2, 3 and 4.
- Request: identical lines still stop with the existing message and event fields; addressed by scenario 5.
- Request: no change to the verify package, kogen-verification, which lines count as environment failures, other stop rules, documentation or report layout; omitted from the behavior, preserved by scope.

## Tests
- A check failing with the same kind of environment failure but a different first line in two consecutive passes stops the Build quoting the second line: a Go test in build/env_failure_park_test.go (integration)
- A different kind in the previous pass does not stop the Build: a Go test in build/env_failure_park_test.go (integration)
- An ordinary failure in the previous pass does not stop the Build: a Go test in build/env_failure_park_test.go (integration)
- The memory is per check name: a Go test in build/env_failure_park_test.go (integration)
- Identical lines still stop with the existing message and event: a Go test in build/env_failure_park_test.go (integration)
- The existing host-or-base tests keep passing unchanged: the existing tests in build/env_failure_park_test.go (existing)
- The five outcomes through the Build's real entry point: acceptance.feature scenarios (scenario)

## Feasibility
- none
