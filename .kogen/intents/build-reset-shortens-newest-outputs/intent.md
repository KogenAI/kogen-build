# Shorten newest Build outputs on a window reset

Slug: build-reset-shortens-newest-outputs
Model: not applicable — one pure size-fitting rule inside a single function call; there is no state that interacts across steps
Acceptance:
- .kogen/intents/build-reset-shortens-newest-outputs/acceptance.feature
- features/steps/build-reset-shortens-newest-outputs__steps.js
- features/steps/build-reset-shortens-newest-outputs__driver/shorten_test.go.txt

When the mandatory part of a Build working window (the opening message plus the reset message) fits the ceiling but the reset window with the newest exchange does not, `prepareContext` for the build owner shortens the `function_call_output` items of that newest exchange instead of refusing with `candidate/context_budget_exceeded`. Outputs are shortened one at a time, largest first and, among equal sizes, the later one first, until the window fits. A shortened output keeps its first 2048 bytes and its last 2048 bytes (each cut at a character boundary) with one line between them that states how many bytes were omitted. Calls, assistant text, call identifiers and every output that did not have to be shortened stay byte for byte. The archive written for the reset holds the full original input, as today, so custody records what the builder actually saw. If the window still does not fit after every output larger than 4096 bytes has been shortened, or if the mandatory window does not fit, the refusal is exactly today's refusal with today's text and numbers. An exchange with no output larger than 4096 bytes is never changed. Out of scope: the review owner and its deferral of pinned reads, `build/material.go`, the ceiling, the reserved output tokens, the reset message layout and all documentation.

## Decisions
- Only the build owner shortens outputs; the review owner keeps its deferral of pinned reads unchanged. (source: request)
- Shortening runs only when the mandatory window fits and the reset window with the newest exchange is refused; a reset window that already fits leaves every output byte for byte unchanged. (evidence: build/context.go) (source: request)
- Candidates are the `function_call_output` items of the newest exchange whose string output is larger than 4096 bytes; they are shortened one at a time, largest first, equal sizes from the end of the exchange backwards, stopping at the first window that fits. (source: request)
- A shortened output is its first 2048 bytes, one line stating the decimal count of omitted bytes, and its last 2048 bytes, each cut at a character boundary; the exact wording of the omission line is not fixed beyond stating the decimal count. (assumption: the wording of the omission line is free) (source: request)
- The archive file and its recorded sha256 are built from the full original input, not the shortened one. (evidence: build/context.go; build/context_test.go) (source: request)
- Refusal when the mandatory window does not fit, or when the window still does not fit after every output larger than 4096 bytes was shortened, keeps today's error text and numbers (measured and mandatory bytes, ceiling, output reserve). (evidence: build/context_test.go) (source: request)
- The acceptance fixture is a Go test file kept as a `.go.txt` helper that the step compiles into the `build` test binary through `go test -c -overlay`, because `prepareContext` is unexported; its opening message is sized with the package measurement so that the mandatory window leaves a fixed room below the ceiling. The Build still adds its own focused tests to `build/context_test.go`. (evidence: features/steps/build-bound-initial-file-index__steps.js) (source: request)
- Shorten newest-exchange outputs for the build owner only, in prepareContext, via a construct helper that archives the full original input separately from the sent input (decision: D1, round 1) (constraint: archive and sha256 hold the full original input) (constraint: refusal text unchanged) (evidence: build/context.go; build/context_test.go; features/steps/build-bound-initial-file-index__steps.js) (assumption: A scratch run in a copy of the checkout (go test -run TestShorten with the correct patch applied) passed all five focused tests; the unchanged tree failed the first three; five wrong overlays were each rejected by their named test. Full package tests were not run.) (source: request)

## Source comparison
- Shorten newest-exchange outputs largest first, later first among equals, 2048 head and 2048 tail bytes with an omitted-bytes line: addressed by scenario 1 and the Decisions; `build/context.go` only.
- Calls, assistant items and call identifiers unchanged: addressed by scenario 2.
- Archive holds the full original input with matching sha256: addressed by scenario 3.
- Mandatory window over the ceiling refuses with today's text and writes no archive: addressed by scenario 4 (passes on the unchanged tree). The refusal text that must stay as it is, in the form observed in the failing run, is `candidate/context_budget_exceeded: build: measured 541994 bytes; mandatory 509935 bytes; ceiling 524288, output reserve 131072 tokens` followed by the retained checkpoint and history sentences; only the measured and mandatory numbers vary with the window.
- Whole reset window that already fits is untouched: addressed by scenario 5 (passes on the unchanged tree).
- The refusal after every output above 4096 bytes is shortened is today's refusal: covered by the same condition as scenario 4 and the unchanged refusal code path; no separate scenario because no wrong behavior of it is distinguishable from scenario 4's text check.
- Review owner, `build/material.go`, ceiling, output reserve, reset message layout, documentation: omitted by the request.

## Tests
- A newest exchange of two 12000-byte outputs over the room keeps the earlier output and shortens only the later one to its first and last 2048 bytes with the omitted count 7904: TestShortenLaterOutputFirst in build/context_test.go (unit)
- Calls, assistant items and call identifiers of the newest exchange stay byte for byte: TestShortenKeepsCallsAndIdentifiers in build/context_test.go (unit)
- The archive and its sha256 hold the full original input: TestShortenArchiveHoldsTheFullInput in build/context_test.go (integration)
- An unfit mandatory window still refuses with today's text and writes no archive: TestShortenStillRefusesWhenTheMandatoryWindowDoesNotFit in build/context_test.go (unit)
- A reset window that already fits leaves outputs unchanged: TestShortenLeavesAFittingWindowAlone in build/context_test.go (unit)
- none: the collection scenario does not apply, since the operation shortens at most the outputs of one exchange and issues no query or external call per record.

## Feasibility
- none
