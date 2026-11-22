# Report external Build stalls as the executor round

Slug: build-external-stall-round-limit
Model: not applicable — the change is one stop classification of an existing pass-counting rule, with no state machine beyond the existing streak counters
Acceptance:
- .kogen/intents/build-external-stall-round-limit/acceptance.feature
- features/steps/build-external-stall-round-limit__steps.js
- features/steps/build-external-stall-round-limit__driver/run.js
- features/steps/build-external-stall-round-limit__driver/fixture.go.txt

An external Build that meets the existing stall predicate in `externalStalled` (three consecutive identical clean red passes with the same failing-set fingerprint over the same candidate tree) is stopped by the executor round limit: it records the `executor_no_progress` event (rounds, fingerprint, failing items) and then ends with outcome `candidate/executor_round_limit`, leaving its edits in place. The report lines stay: the stage when a stage gate failed, the failing checks, and the acceptance scenarios that did not pass, so the runner can still tell a stall from a spent round budget by the `executor_no_progress` event and the lines. This replaces the current `candidate/no_progress` outcome for the external stall stop.

The `candidate/no_progress` outcome stays exactly as it is for the native repeat rule (`repeatedFailure`, four passes). The stall predicate, the number of rounds, wall-clock caps, completion and protection rules are unchanged; no flag, setting or environment variable is added. Only `build/noprogress.go` and the Go tests of the build package change; the existing test that asserts no_progress for the external stall is updated to the new outcome.

## Decisions
- `externalStalled` keeps its predicate and its `executor_no_progress` event, and returns the executor round-limit stop (`executor.RoundLimit`) with the stage line (only when a stage gate failed), the failing line and the scenarios line appended. (decision: D1, round 1) (constraint: predicate unchanged; no new setting) (evidence: build/noprogress.go; build/builder.go; build/route.go) (source: request)
- The native repeat rule keeps outcome no_progress and its four-pass message and bound; an ordinary external round-bound stop with changing failures records no executor_no_progress event. (decision: D2, round 1) (constraint: four-pass bound unchanged) (evidence: build/noprogress.go; build/builder.go) (source: request)

## Source comparison
- addressed: final-path and staged-path stall classification are scenarios 1 and 2 of acceptance.feature; both reach the shared `externalStalled` from `build/builder.go` and `build/route.go`.
- addressed: round bound with changing failures (scenario 3), native four-pass rule (scenario 4) and a decreasing failing set then green (scenario 5).
- superseded: the earlier Intent build-classifies-stalled-external-pass asked for no_progress for this stall; this request replaces it.
- omitted by request: spec, docs, round number, wall-clock caps, completion and protection rules, cli/go.mod and cli/go.sum.

## Tests
- A same-tree clean-red external streak on the final path ends as candidate/executor_round_limit with one executor_no_progress event and the failing check named: update TestExternalNoProgressStopsOnThirdIdenticalRed in build/k19_no_progress_test.go (regression)
- The same streak on the staged path ends as executor_round_limit naming the stage and failing check: update TestExternalNoProgressOnTheStagedPathNamesTheStage in build/k19_no_progress_test.go (regression)
- A round-bound stop with changing failures records no executor_no_progress event: existing TestExternalNoProgressResetsWhenTheTreeChanges and TestExternalRoundLimitKeepsTheCandidate, plus acceptance scenario 3 (existing)
- Native four identical passes still stop as no_progress, and a decreasing sequence then green finishes: existing TestRepeatedStageFailureDespiteEditsStops, plus acceptance scenarios 4 and 5 (existing)

## Feasibility
- none
