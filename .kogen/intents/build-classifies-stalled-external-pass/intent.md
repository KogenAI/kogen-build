# Give external Build stalls the no-progress outcome

Slug: build-classifies-stalled-external-pass
Model: not applicable — the change is one stop classification of a pass-counting rule, with no state machine beyond the existing streak counters
Acceptance:
- .kogen/intents/build-classifies-stalled-external-pass/acceptance.feature
- features/steps/build-classifies-stalled-external-pass__steps.js
- features/steps/build-classifies-stalled-external-pass__driver/run.js
- features/steps/build-classifies-stalled-external-pass__driver/fixture.go.txt

An externally driven Build that meets the existing stall predicate in `externalStalled` (the third consecutive clean red pass with the same failing-set fingerprint over the same candidate tree) currently stops with the executor round-limit outcome. Make it stop with the structured `candidate/no_progress` outcome instead, so the final report says the Build stalled and does not read as a generic cap. The stop names the stage when the pass is a stage gate, the failing checks, and, when acceptance fails, the failing scenarios, as the existing four-pass `no_progress` stop does. The `executor_no_progress` event is still recorded. The candidate is left in place and the Build keeps its ordinary stop and checkpoint handling and native continuation.

The predicate itself is unchanged and shared by the staged path and the final path. The repeated-identical-failure rule (`repeatedFailure`, four passes, tree-independent) is unchanged. The number of rounds, any whole-Build wall-clock cap, the completion and protection rules, and every file other than `build/noprogress.go` and the Go tests of the build package are out of scope. Existing tests that expect the executor round-limit outcome for this stall are updated to the no_progress outcome.

## Decisions
- The stall stop uses outcome kind `no_progress` in category `candidate`, with message lines for the stage (only when a stage gate failed), `failing:` and, when acceptance has non-passed scenarios, `scenarios:`, reusing the existing `missingAcceptanceItems` format. (evidence: build/noprogress.go; build/k19_no_progress_test.go) (source: request)
- The three-pass identical-tree threshold, the four-pass repeat rule and the round count stay exactly as they are; no setting or wall-clock cap is added. (evidence: build/noprogress.go; build/builder.go; build/route.go) (source: request)
- The stall stop uses outcome kind no_progress in category candidate, with message lines for the stage (only when a stage gate failed), failing, and scenarios when acceptance has non-passed scenarios. (decision: D1, round 1) (constraint: predicate and thresholds unchanged) (evidence: build/noprogress.go; build/k19_no_progress_test.go) (assumption: Message wording and the stage line label are chosen by the shaper; acceptance asserts only the kind, the failing names and that the stage is named apart from the failing check.) (source: request)
- The three-pass identical-tree threshold, the four-pass repeat rule and the round count stay unchanged; no setting or wall-clock cap is added. (decision: D2, round 1) (constraint: no new setting) (evidence: build/noprogress.go; build/builder.go; build/route.go) (assumption: The staged external path was not run in a probe; the fixture combines stagedEnv and useExecutor and readiness will show whether that composition works.) (source: request)

## Source comparison
- addressed: request's final-path and staged-path stall classification are scenarios 1 and 2 of acceptance.feature; both paths call the shared `externalStalled` from `build/builder.go` and `build/route.go`.
- addressed: improving failing-set sequence finishing green with the existing round count, and identical typed failure under churning edits still stopping at four passes, are scenarios 3 and 4.
- omitted by request: docs, README, design files, round number, wall-clock cap, completion and protection rules.

## Tests
- A same-tree clean-red external streak on the final path ends as candidate/no_progress naming the failing scenario: update TestExternalNoProgressStopsOnThirdIdenticalRed in build/k19_no_progress_test.go to expect no_progress, still 3 rounds and one executor_no_progress event (regression)
- The same streak on the staged path ends as no_progress naming the stage and failing check: add a staged external Build test in the build package (regression)
- Improving sequence lands unimpeded and a churning identical failure still stops at four: existing TestExternalRepeatedFailureWithFreshTreesStops stays, plus acceptance scenarios 3 and 4 (existing)

## Feasibility
- evidence: {"id":"stall-observer","kind":"observer","inputs":[".kogen/intents/build-classifies-stalled-external-pass/controls.json"],"observers":["features/steps/build-classifies-stalled-external-pass__driver/run.js","features/steps/build-classifies-stalled-external-pass__driver/fixture.go.txt"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/build-classifies-stalled-external-pass__driver/run.js good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/build-classifies-stalled-external-pass__driver/run.js bad","exit":1,"output":"observer rejected\n"}]}
