# Verify again after a no-edit round on an

Slug: build-env-red-no-edit-host-stop
Model: not applicable — the change is one condition in the no-edit round rule; the scenarios cover each outcome
Acceptance:
- .kogen/intents/build-env-red-no-edit-host-stop/acceptance.feature
- features/steps/build-env-red-no-edit-host-stop__steps.js
- features/steps/build-env-red-no-edit-host-stop__driver/acceptance_test.go.txt

When the remembered red verification pass failed with an environment failure, a builder round that left the tree of that pass unchanged is not given the no-edit notice and does not count toward the pass's no-edit stop. In `build/noprogress.go`, a failing check of the red pass that `verify.EnvFailure` reports an environment failure for, whose kind (as `envFailureKind` in `build/envfailure.go` classifies the line) is timeout, disk, memory or files, qualifies the pass. Such a round is verified again as an ordinary round, so the existing host-or-base rule compares the two consecutive passes and stops as environment/host_or_base_failure with its existing lines and exit 3. A red pass with no such failure keeps today's behaviour exactly: the first no-edit round gets the notice and is not verified, the second stops as candidate/no_progress. The rule holds for the whole-Intent pass and for a stage gate pass alike. Out of scope: go.mod and go.sum, the kogen-verification repository and which lines count as environment failures, the host-or-base message and lines, external executor stop rules, documentation.

## Decisions
- `noEditRound` returns to verification (check true, no notice, no noticeRound set) when the remembered red pass for the same stage and tree has a failing check with an environment failure of kind timeout, disk, memory or files; the pass is verified as an ordinary round and `hostOrBaseStop` does the comparison. (evidence: build/noprogress.go; build/envfailure.go; build/builder.go) (source: request)
- A helper in `build/noprogress.go` decides this: any failing check of the red pass whose output `verify.EnvFailure` reports and whose `envFailureKind` is not other qualifies it; one such check suffices, other failing checks need not be environmental. (evidence: build/noprogress.go; build/envfailure.go) (source: request)
- Pass 1 has an ordinary failure in check vet and a disk failure `write /tmp/a: no space left on device` in check test, and round 2 makes no edits: round 2 is verified again, and a repeat of the disk kind in check test stops as host_or_base_failure naming check test. Acceptance proves the verify-again half with a remembered red pass that has exactly those two failing checks, driven through the existing `noEditRound`, and proves the stop half end to end with a disk failure in check test. Open limit, separate from this guarantee: the pinned verification stops at the first red item, so a real pass records one failing check; the two-failing-check pass is therefore constructed, not produced by a real run. (evidence: build/noprogress.go; build/envfailure.go; build/noprogress_test.go) (assumption: the pinned kogen-verification stops at the first red item, observed in a scratch run rather than a tracked file) (source: request)
- A red pass with no such failure keeps the notice on the first no-edit round and the candidate/no_progress stop on the second, with the existing lines and no extra pass. (decision: D4, round 1) (evidence: build/noprogress.go; build/noprogress_test.go) (source: request)
- The rule is in `noEditRound`, which serves the whole-Intent pass and the stage gate pass alike. (decision: D5, round 1) (evidence: build/builder.go; build/stages_test.go) (source: request)
- The Build adds focused model-free Go tests for the outcomes in `build/noprogress_test.go` and `build/env_failure_park_test.go`, using the existing scripted builder and check helpers; acceptance runs a driver copy of six tests because the Build's test names are unknown beforehand. (decision: D6, round 1) (evidence: build/noprogress_test.go; build/env_failure_park_test.go) (source: request)
- noEditRound returns to verification (no notice) when the remembered red pass for the same stage and tree has a failing check with an environment failure of kind timeout, disk, memory or files (decision: D1, round 1) (constraint: whole-Intent and stage gate alike) (constraint: ordinary red keeps today's behaviour) (evidence: build/noprogress.go; build/envfailure.go; build/builder.go) (assumption: Scratch run only; not Kogen evidence.) (source: request)
- Acceptance scenario 4 uses a single failing check (disk) because verification stops at the first red item, so two failing checks cannot co-occur end to end (decision: D2, round 1) (evidence: build/env_failure_park_test.go) (assumption: Observed in a scratch run of the pinned kogen-verification, not a tracked file.) (source: request)
- Scenario 4 is covered by the request's own setup (ordinary failure in check vet, disk failure in check test) via a constructed remembered red pass driven through noEditRound, plus an end-to-end disk stop (decision: D3, round 1) (constraint: one qualifying failing check suffices; other failing checks need not be environmental) (evidence: build/noprogress.go; build/envfailure.go; build/noprogress_test.go) (assumption: The pinned verification stops at the first red item (scratch observation), so the two-failing-check pass is constructed rather than produced by a real run.) (source: request)

## Source comparison
- Request: no notice and no no-edit count when the red pass has an environment failure, verify again so host-or-base stops with exit 3; addressed by D1, D2 and scenarios 1 and 4.
- Request: a repeat that is not environmental neither stops for the host nor ends as no_progress; addressed by scenario 2.
- Request: ordinary red keeps today's behaviour; addressed by D4 and scenario 3.
- Request: ordinary failure in check vet and disk failure in check test, round 2 verified again; addressed by D3 and scenarios 4 and 5.
- Request: stage gate pass alike; addressed by D5 and scenario 6.
- Request: go.mod and go.sum, kogen-verification, environment line list, host-or-base message and lines, external executor stop rules, documentation; omitted from the behavior, preserved by scope.

## Tests
- A timeout red pass, a no-edit round and a repeated timeout stop as host_or_base_failure with exit 3, two passes and no notice: Go test in build/noprogress_test.go (integration)
- A timeout red pass followed by an ordinary failure after a no-edit round neither stops for the host nor ends as no_progress: Go test in build/noprogress_test.go (integration)
- An ordinary red pass keeps the notice and the no_progress stop: existing tests in build/noprogress_test.go (existing)
- A disk failure repeated after a no-edit round stops naming its check: Go test in build/env_failure_park_test.go (integration)
- A red pass with an ordinary failure in check vet and a disk failure in check test is verified again after a no-edit round: Go test in build/noprogress_test.go (unit)
- A stage gate timeout repeated after a no-edit round stops as host_or_base_failure: Go test in build/env_failure_park_test.go (integration)
- none: collection scenario omitted because the change processes no collection of records

## Feasibility
- none
