# Stop builds on repeated failure despite edits

Slug: build-stops-on-repeated-failure
Model: .kogen/intents/build-stops-on-repeated-failure/model.qnt invariants=Boundary,Reset,Consecutive seed=1 samples=100 steps=12
Acceptance:
- .kogen/intents/build-stops-on-repeated-failure/acceptance.feature
- features/steps/build-stops-on-repeated-failure__steps.js
- features/steps/build-stops-on-repeated-failure__driver/run.js
- features/steps/build-stops-on-repeated-failure__driver/fixture.go.txt

Stop native and external candidates with candidate/no_progress after four consecutive clean red verification passes with an identical failing-set fingerprint, regardless of edits or candidate tree changes. Clean means no moved files as well as Clean=true. A different fingerprint starts a new count at one; green or unclean passes clear the count. Keep externalStalled, the external three-pass identical-tree rule and the no-edit notice/stop rule unchanged. Apply the new rule to whole-Intent and stage verification.

Reuse the fingerprint semantics: stage, ordered failing items, and b.PassFeedback with pass numbering normalized, not the candidate tree. Record fingerprint, count and failing items in the no_progress event on this stop. The stop message names failing items and, when acceptance fails, failing scenario names with feature lines. Preserve the candidate and ordinary stop/checkpoint handling; do not make another builder request after the fourth identical pass.

Write failing regression tests before implementing the rule. Update existing expectations that explicitly required continuing past four unchanged failures, without weakening coverage of write rounds or of the external three-pass rule. Run the build package tests with `go test ./build`, followed by configured checks. No provider integration or authentication changes are requested.

## Decisions
- Keep the new fingerprint-only streak separate from the external identical-tree streak and invocation-local like the existing watch. (decision: D1, round 1) (evidence: build/noprogress.go; build/route.go; build/builder.go) (source: request)
- Use a no_progress event with fingerprint, count and failing fields, and include scenario names and feature lines in the candidate/no_progress diagnostic. (decision: D2, round 1) (evidence: build/noprogress.go; build/noprogress_test.go) (source: request)

## Source comparison
- addressed: request's build/noprogress.go change and both builder paths are covered by the outcome above, Native edits cannot hide stagnation and External edits cannot hide stagnation in acceptance.feature. The independent tree-sensitive externalNoProgressPasses=3 rule remains unchanged; the new limit is 4.
- addressed: request's reset requirements and existing fingerprint inputs are preserved above and modeled by Boundary, Reset and Consecutive. Implementation unit regressions must also cover green, Clean=false, nonempty Moved, changed stage, and changed feedback messages resetting the streak. A changed fingerprint starts at one rather than zero.
- addressed: all four requested regression scripts are executable in acceptance.feature. The 10 November 2026 studio-coord-model-heads-drift native gpt-6.1-terra observation (7, 6, 5, 3, 2, 2 then green at pass 8) remains historical evidence, not a claim that the local fixture reproduces the earlier unlisted pass. Real progress finishes green executes the supplied failing-set sequence and must land successfully.
- addressed: requested diagnostic and event evidence is checked by both stagnation scenarios. Requested tests-first sequencing and build-package test command are required above. Existing tests are evidence of current behavior, not authority to retain the old unlimited repeated-failure behavior.
- omitted: no requested behavior or required non-secret input is deliberately omitted. No new settings, transport behavior, persistent streak state or unrelated refactoring is included.

## Tests
- Repeated failure despite native edits stops at four: add a fake-native Build regression that checks four passes, candidate/no_progress, scenario name and feature line, and event fields (regression)
- Repeated failure despite external tree changes stops at four: add a fake-external Build regression with a fresh tree every pass and the same diagnostic/event assertions (regression)
- Shrinking failures remain progress: execute the supplied shrinking scenario sequence through a fake builder and require green completion (regression)
- Fingerprint changes reset the streak: Three identical passes followed by a different failure and then three more identical ones: execute these failures and green without stopping (regression)
- Green and unclean passes reset the streak: focused pass-policy tests for green, Clean=false and moved files, plus changed stage and feedback message (unit)
- Existing stopping behavior stays intact: retain the external third-identical-tree stop and no-edit notice/stop regressions while updating only superseded four-pass continuation expectations (existing)
- Build package stays healthy: run go test ./build and configured project checks (integration)

## Feasibility
- Local observer qualification accepted a known-good implementation and rejected one omitting scenario names. A separate shrinking-fixture probe completed green. No network dependency changed.
- Quint typecheck and 100 samples of 12 steps with Boundary and Reset and Consecutive passed. An earlier model used a reserved name and was corrected before submission.
- An attempted go test ./build exceeded the scratch probe's 300-second limit; it is not passing evidence and remains a required implementation check.
- evidence: {"id":"repeated-observer","kind":"observer","inputs":[".kogen/intents/build-stops-on-repeated-failure/controls.json"],"observers":["features/steps/build-stops-on-repeated-failure__driver/run.js","features/steps/build-stops-on-repeated-failure__driver/fixture.go.txt"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/build-stops-on-repeated-failure__driver/run.js good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/build-stops-on-repeated-failure__driver/run.js bad","exit":1,"output":"observer rejected\n"}]}
