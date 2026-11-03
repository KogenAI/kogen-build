# Explain edits that leave red failures unmoved

Slug: k-build-unmoved-red-feedback
Model: .kogen/intents/k-build-unmoved-red-feedback/model.qnt invariants=NoticeOnlyForEditedRepeat,Nonnegative seed=17 samples=100 steps=20
Acceptance:
- .kogen/intents/k-build-unmoved-red-feedback/acceptance.feature
- features/steps/k_unmoved_steps.js
- features/steps/k_unmoved_test.go.txt

Prepend a diagnostic to builder feedback when a red verification pass has the same stage and failing items as the preceding red pass, despite a changed candidate tree. This is information, never a new stop condition. Preserve the existing ordinary feedback verbatim and in its existing order after the prefix. Implement within Build's feedback policy (not the verification or coordinator dependencies); cover whole-Intent and scoped-stage feedback, native and external builders.

The motivating Build had twelve red scenarios with identical failure text across nine passes, while edits went to one unrelated file; it stopped as no_progress only after a tool-free answer. Reproduce that shape as well as the shorter three-round synthetic example, without requiring a real provider.

## Decisions
- Start the diagnostic on the second consecutive red pass with the same failing set. Count includes the first pass (2, then 3, etc.). Use `Failing set did not move for N consecutive red passes. Last round changed: PATHS. None of the failing items changed.` The request permits wording and threshold choice. (source: request)
- If one identical nonempty failure text is carried by a strict majority of failing items, append `Common failure text (address first): TEXT` once in the prefix. Equal-sized groups and all-different messages get no common-cause line. Do not claim this text is a proven root cause. Do not deduplicate the ordinary per-item feedback. (source: request)
- Use a blank line between prefix and the unchanged ordinary feedback; for the single-file fixture PATHS is `unrelated.txt`. Multiple changed paths are sorted repository-relative paths, separated by comma-space. (source: request)
- A first red, green, changed failing set or changed stage has no diagnostic. A tool-free identical-tree round retains the existing missing-items notice and no_progress sequence, without the new prefix. (source: request)

Compare sets by stable failing-item identity, including individual nonpassing acceptance scenarios (not merely the aggregate `acceptance` gate), plus failing check/model/stage items. Failure-message changes alone are not changes to item identity. Preserve feedback ordering independently of set comparison. Reset the run on green or a changed set/stage; a fresh/resumed conversation starts without a previous red comparison. Derive last-round changed paths from the preceding verified tree to this verified tree, including additions/deletions, not cumulative edits against the original baseline. A same-tree round must not emit an edited-tree diagnostic. Do not change verification, stopping rules, round limits, approval, review, checkpoint schema or commit behavior. Do not add configuration or provider calls. Existing checks and useful frozen goldens remain intact.

## Source comparison
- addressed: Request Problem and Need (unrelated edits, twelve identical failures across nine passes) map to the motivating regression, diagnostic semantics above, and history/shared acceptance fixtures.
- addressed: Synthetic acceptance three edits and shared failure text map to shared; one scenario passing in round two maps to partial (even though the aggregate acceptance gate remains red); differing messages map to different.
- addressed: Synthetic tool-free missing-items then no_progress is asserted by every edited-tree fixture after its scripted edit rounds; first-red and green compatibility map to compatibility and existing TestNamedFeedbackCompatibility. Ordinary feedback is compared in full after normalizing only its pass number in the repeated-red fixtures.
- addressed: Need's majority-or-all common text maps to the strict-majority decision; Tests requires focused majority, tie and boundary coverage in addition to all-same/all-different end-to-end fixtures.
- addressed: Need's no stop and no repeated/reordered ordinary feedback map to continued provider requests through every scripted edit round, the unchanged suffix comparison, and preserved existing stall tests.
- omitted: no requested behavior or supplied reproducer. Historical anecdotes remain motivation, not assertions about unavailable historical records. No external protocol change or documentation dependency is introduced.

## Tests
- Emit edited-tree identical-set diagnostics from pass two, counting consecutive red passes and naming only last-round paths: add focused comparison/rendering tests and shared/history Build regression fixtures (regression)
- Treat scenario-level membership changes and stage changes as changed sets even if aggregate gates remain red; reset after green: add focused sequence tests and partial Build fixture (unit)
- Summarize a nonempty strict-majority failure text once, with all-same, all-different, majority, half/tie and adjacent majority boundaries: add focused rendering tests; shared and different Build fixtures cover wiring (unit)
- Preserve ordinary per-item bytes and order, and first-red/green behavior: repeated-red suffix comparison and existing TestNamedFeedbackCompatibility without updating its frozen golden (regression)
- Keep native missing-items/no_progress and external stopping rules unchanged: existing noprogress_test.go and k19_no_progress_test.go plus tool-free tails of edited-tree fixtures (existing)
- Wire the notice into scoped stages and external feedback as well as native whole-Intent feedback, without extra stop conditions: add integration cases using the existing stage and fake-executor fixtures (integration)

## Feasibility
- Go's real `-json` runner was observed emitting exactly one run and pass for TestARedAnswerWithoutAToolCallAfterTheSameRedStopsWithTheCandidatePreserved (probe: exit 0, 5.0 s); TestNamedFeedbackCompatibility was also observed passing (probe: exit 0, 11.5 s). The observer uses JSON events, not remembered summary wording.
- Quint initially rejected unparenthesized assignment RHS conjunctions; the corrected model typechecked (probe: exit 0, 0.4 s).
- evidence: {"id":"unmoved-observer","kind":"observer","inputs":["features/steps/k_unmoved_steps.js","features/steps/k_unmoved_test.go.txt"],"observers":["features/steps/k_unmoved_steps.js","features/steps/k_unmoved_test.go.txt"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k_unmoved_steps.js control-good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k_unmoved_steps.js control-bad","exit":1,"output":"observer rejected\n"}]}

Controls run the real local Build fixture and exercise the same final validator with known-good prefixed captures or a deliberately corrupted third-pass count sentence. They reach suffix preservation and later-pass assertions independently of missing-feature red; they are not evidence that the feature is implemented. The overlay is a protected acceptance test fixture, not production code, and its Go test is supplied here rather than an unwritten test the builder must invent.
