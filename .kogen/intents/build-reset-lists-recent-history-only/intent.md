# Limit reset message history to recent archives

Slug: build-reset-lists-recent-history-only
Model: not applicable — the change shortens one message built from a list; it adds no state
Acceptance:
- .kogen/intents/build-reset-lists-recent-history-only/acceptance.feature
- features/steps/build-reset-lists-recent-history-only__steps.js
- features/steps/build-reset-lists-recent-history-only__tests/reset_history_acceptance.go.tmpl

Every Build working-window reset message lists every archived window of the Build, so the mandatory part of the request grows with each reset until the Build stops with candidate/context_budget_exceeded although it was making progress. The reset message now lists in full only the new archive and the 7 archives before it (at most 8 references, in order, oldest of them first). When more archived windows exist, the message also carries one object named `history_omitted` with `count` (the number of archived windows not listed), `first` (the path of the oldest archived window), `last` (the path of the newest archived window that is not listed) and `note` (a short sentence that the omitted windows stay in native custody and are read by path). The Build state and checkpoint keep every reference exactly as today, so history custody checks on resume are unchanged. A window with at most 8 references in all produces exactly today's message, with no `history_omitted` object. With no prior references only the new archive is listed; references are never merged, reordered or deduplicated.

Observed failure that motivates the change: a Build log with 197 lines of the form `kogen: build local reset A -> B bytes; ceiling 524288`, where the size after the reset B rose by about 184 bytes per reset (close to the roughly 210 bytes of one archive reference, a path plus a 64-character sha256), ended with `candidate/context_budget_exceeded: build: measured 541994 bytes; mandatory 509935 bytes; ceiling 524288, output reserve 131072 tokens`. The ceiling and the output reserve are unchanged by this Intent.

Scope: the construction of the reset message in `prepareContext` in build/context.go, and Go tests in build/context_test.go. Out of scope: build/material.go and the review window rules, testdata/reset-golden.json, build/state.go, observations carried by the reset, the ceiling and every other budget, and all documentation.

The Build also adds focused model-free tests in build/context_test.go for the five cases below, built like TestTheLocalResetEqualsTheElixirReference: an Attempt whose build working context already holds synthetic archive references with long paths in the run folder, and an input over the 524288-byte ceiling.

## Decisions
- The reset message lists in full the new archive and the 7 archives before it (at most 8 references, oldest of them first); `history_omitted` is written only when more than 8 references exist, with count, first, last and note. (evidence: build/context.go) (source: request)
- The window of 8 applies to the reset message only: the saved working context and the checkpoint keep every reference, so resume custody checks are unchanged. (evidence: build/context.go; build/context_test.go) (source: request)
- Where the review owner's pinned-catalog filtering (history_in_catalog) applies, it is applied inside the 8-reference window only; omitted references are counted in history_omitted. (evidence: build/context.go) (assumption: the request does not say how catalog-listed references interact with the window; only the review owner is affected) (source: request)
- Acceptance runs a Go test template copied into a scratch copy of the checkout, because the names of the Build's own tests in build/context_test.go cannot be known in advance; the existing TestTheLocalResetEqualsTheElixirReference is run unchanged beside it. (evidence: build/context_test.go) (source: request)
- The reset message lists in full the new archive and the 7 archives before it (at most 8 references, oldest of them first); history_omitted is written only when more than 8 references exist, with count (references not listed), first (path of the oldest archived window), last (path of the newest unlisted window) and a short note that omitted windows stay in native custody and are read by path. (decision: D1, round 1) (constraint: No history_omitted object for 8 or fewer references) (constraint: references are never merged, reordered or deduplicated) (evidence: build/context.go) (source: request)
- The window of 8 applies to the reset message only: resetWindow.history, the saved WorkingContext history and the checkpoint keep every reference, so resume custody checks are unchanged. (decision: D2, round 1) (constraint: Saved history is not shortened) (evidence: build/context.go; build/context_test.go) (source: request)
- Where the review owner's pinned-catalog filtering (history_in_catalog) applies, it is applied inside the 8-reference window only; omitted references are counted in history_omitted. (decision: D3, round 1) (evidence: build/context.go) (assumption: The request does not say how catalog-listed references interact with the window; only the review owner is affected and the build owner's reset is the reported problem.) (source: request)
- Acceptance runs a Go test template copied into a scratch copy of the checkout, because the Build's own tests in build/context_test.go have names that cannot be known in advance; the template builds an Attempt like the golden reset test and the existing TestTheLocalResetEqualsTheElixirReference is run unchanged beside it. (decision: D4, round 1) (evidence: build/context_test.go) (source: request)

## Source comparison
- Request scenarios 1 to 5 (8 listed with the new archive last; history_omitted count 193 with first `/1.json` and last `/193.json`; the saved history keeps all 201 references; messages for 100 and 400 prior references differ by at most 64 bytes; 7 prior references list 8 with no history_omitted and 8 prior list 8 with count 1, and the golden test passes unchanged): addressed by the scenarios of acceptance.feature (scenario 5 is split into the 7-reference case, the 8-reference case and the golden comparison).
- The count rule, no merging, reordering or deduplication, the failure evidence literals and the out-of-scope list: addressed above. Omitted, conflicting or superseded: none.

## Tests
- the reset message with 200 prior references lists the new archive (201) last, preceded by 194 to 200 in ascending order: TestResetHistoryAcceptance/s1 (unit)
- history_omitted has count 193 and first and last paths ending in /1.json and /193.json: TestResetHistoryAcceptance/s2 (unit)
- the saved working context still lists all 201 references in order: TestResetHistoryAcceptance/s3 (unit)
- messages built with 100 and 400 prior references differ in length by at most 64 bytes, so the message does not grow with the number of resets: TestResetHistoryAcceptance/s4 (unit)
- 7 prior references give 8 listed and no history_omitted: TestResetHistoryAcceptance/s5a (regression)
- 8 prior references give 8 listed and history_omitted count 1: TestResetHistoryAcceptance/s5b (unit)
- the golden reset comparison is unchanged: TestTheLocalResetEqualsTheElixirReference (existing)
- the Build adds equivalent focused tests for these cases: build/context_test.go (unit)

## Feasibility
- none
