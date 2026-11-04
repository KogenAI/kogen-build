# Stop pinning reviewer wording in context bytes

Slug: k-build-reviewer-prompt-bytes-unpinned
Model: not applicable — only one test's comparison of the reviewer's opening bytes and its golden change; no product state or transition is added
Acceptance:
- .kogen/intents/k-build-reviewer-prompt-bytes-unpinned/acceptance.feature
- features/steps/k-build-reviewer-prompt-bytes-unpinned__steps.js
- features/steps/k-build-reviewer-prompt-bytes-unpinned__controls.js

This change fixes `TestContextExternalNativeBytes` (build/context_external_test.go). kogen-review `e5005fa` "Require evidence per requirement before approval" (Kogen-Intent: k-review-intent-requirement-ledger) turned it red. That commit adds the `Requirements:` rule (`review.RequirementsRule`, so `ReviewerInstructions()` = prompt + LimitRaises + RequirementsRule), rewords the batching paragraph of review/prompts/reviewer.md ("run your focused probes (each requirement without evidence, stated limits, ...)") and gives `submit_review` a `requirements` parameter. The external review task (the text kogen-review appends after the authority in an external reviewer's brief) changes the same way. The golden `build/testdata/context-external-golden.json` froze those bytes, wording included. With kogen-review main (`81d3bae`, module version `v0.0.0-20261103212337-81d3baef1d48`), 7 of 8 subtests fail on today's tree, all but `external/external_with_context`. For example, `native/native_with_context` fails at build/context_external_test.go:491 with `reviewer differs at byte 20572 (29971 bytes, want 26593)`, and `native/external_without_context` fails at :484 with `reviewer.stable differs at byte 17661 (21008 bytes, want 17931)`. The request quoted :481/:474/:504 at kogen-build 10f190b; the lines moved with k-build-context-bytes-unpinned.

After this change the test asserts what kogen-build contributes to the reviewer's opening request, not the reviewer's wording, which kogen-review owns. The reviewer's instructions and the `submit_review` definition are compared with what the kogen-review the test is built with exports. The external review task is only required to carry the exported `review.LimitRaises`. The golden holds no reviewer wording. Everything else in the reviewer parts is compared as today: the model, effort, cache key, tool choice, the order of the tools, the authority, the review source, the caller diagnostic and where it is placed. The test passes with the pinned kogen-review `v0.0.0-20261103124347-cfc2fb29917c`, with `e5005fa` (`v0.0.0-20261103161351-e5005fa1daf8`) and with main `81d3bae`.

## The rule
- **Native reviewer part** (`reviewer`, from `reviewBody` reduced by `kogenOwnedRequest`). In `openings`, before it is compared or recorded:
  - Its `instructions` must equal `review.ReviewerInstructions()`, and they are replaced by the string `<review.ReviewerInstructions()>`. `reviewBody` keeps finding the request by that equality.
  - The entry of `tools` named `review.SubmitReviewTool` (`submit_review`) must equal `review.ReviewerToolDefinitions(review.SubmitReviewTool)[0]`, both compared as decoded JSON values. It is replaced in place by `{"review_tool_definition":"submit_review"}`, keeping the tool order.
  - A definition that differs from the export fails the subtest with a message naming `submit_review`.
  - Nothing else in the reduced request changes. It is still encoded with sorted keys, and `withoutLineupFile` still applies.
- **External reviewer parts** (`reviewer.stable`, `reviewer.dynamic` of `native/external without context`, `native/external with context` and `external/external without context`). Their golden entries no longer hold these two parts: they hold only the builder parts, the parts count is checked against those, and `-update` records only those. Instead, each of these subtests derives its expectation from the native reviewer part of the golden entry `native/native without context` or `native/native with context`, whichever matches whether the subtest supplies the diagnostic. Under `-update` it uses this run's own `openings(t, extCombos[0], <same diagnostic or nil>)`, as the external-builder subtests already do. From that part's `input`:
  - A is the `text` of the first message's first content block (the authority).
  - S is the `text` of the first message's second content block (the review source).
  - With context, D is the second message's `content` (the diagnostic message).
  - `reviewer.dynamic` must equal S byte for byte.
  - `reviewer.stable` must begin with A + "\n" + D + "\n" + "\n" with context, or A + "\n" without. A mismatch is reported through `sameBytes` with its byte offset.
  - The rest of `reviewer.stable` is kogen-review's external review task. It must be nonempty and contain `review.LimitRaises` exactly once; its other bytes are not compared.
- **Unchanged checks.**
  - The external-builder `with context` subtests keep their checks: the builder brief equals the omitted-context brief plus the diagnostic message, and an external builder's native reviewer equals the native builder's reviewer. That comparison now uses the normalized native reviewer.
  - `external/external with context` keeps comparing only the builder parts.
  - All 8 subtests still run, and `withoutLineupFile` (k-build-context-bytes-unpinned) stays as it is.
- **The golden.** `build/testdata/context-external-golden.json` keeps its six entries.
  - Every builder part keeps today's bytes.
  - The three native `reviewer` parts are today's, with only the instructions and the `submit_review` definition replaced by the placeholders above.
  - The external reviewer parts are gone.
  - It contains none of the reviewer's wording. `go test ./build -run '^TestContextExternalNativeBytes$' -update` against the pinned kogen-review or against main writes exactly the committed bytes.
- **The doc comment.** The test's doc comment states that the reviewer's instructions, the `submit_review` definition and the external review task are kogen-review's, compared with its exports and not stored in the golden.
- **Not changed:** go.mod, go.sum, production code and every other test.

## Out of scope
- The other tests that `e5005fa` turns red: TestKReviewRetryUnchangedValidRequests, TestKRevodGatePartsUnchanged, TestStateGoldenReviewApproveRecord, TestStateGoldenReviewChangesRecord and TestTheReviewerApprovesUnchangedCodeWithoutRepairs (build/review_test.go:140 hashes the reviewer instructions). The request places them outside this cell; they are not changed here and should be run after the change.
- The coordinator's protected-file membership of `.kogen/models.yaml` (k-build-context-bytes-unpinned, landed at efdc8d1). This change builds on it.
- Production code, README, consumes/ suites and go.mod pins.

## Decisions
- The test compares the reviewer parts with what kogen-review exports at the pinned and at the tested version (`review.ReviewerInstructions()`, `review.ReviewerToolDefinitions(review.SubmitReviewTool)`, `review.LimitRaises`), plus kogen-build's own contributions (context diagnostic, brief, ordering, omitted-context equalities), instead of frozen bytes of the producer's prompt and tool definitions (source: request)
- The golden no longer contains any reviewer wording (source: request)
- A change of only the wording of kogen-review's reviewer instructions, `submit_review` definition or external review task does not fail this test (source: request)

## Source comparison
- Addressed: "`go test ./build -run '^TestContextExternalNativeBytes$'` passes against kogen-review main (`81d3bae` today) and against the pinned `cfc2fb29917c`". Acceptance scenario 1 runs the test with the committed go.mod, which pins `v0.0.0-20261103124347-cfc2fb29917c` without a replace. The outline runs it against `e5005fa` and `81d3bae`, taken from the module cache through a temporary go.mod passed with `-modfile`. Each run must report the test and exactly its 8 subtests passing.
- Addressed: "`consumer-check.sh github.com/KogenAI/kogen-review <review-main-checkout> <mirrors> 'kogen-build|<fix-sha>|.|'` no longer lists TestContextExternalNativeBytes". consumer-check.sh is not in this repository. Its predicate for this test is the same `go test` against review main, which the outline's `81d3bae` example runs.
- Addressed, narrowed to this test: "No test of kogen-build fails when only the wording of kogen-review's reviewer instructions or `submit_review` definition changes". Scenario 3 runs the test against a copy of the pinned kogen-review whose wording alone is edited, and requires all 8 subtests to pass. The edits cover reviewer.md, the LimitRaises sentence, the `submit_review` description, the `# Review task` heading and the "Finish with" sentence of the external review task. This conflicts with the request's own statement that the five other red tests are outside this cell and were not analysed. That statement wins, so those tests, including the instruction hash at build/review_test.go:140, are left for their own changes.
- Addressed: "the test asserts what kogen-build contributes ... not the reviewer's text". Scenarios 4 and 5 run copies of the pinned kogen-review whose external or native reviewer drops the caller diagnostic. The test must still fail, in exactly the subtests that carry it (`native/external_with_context`; `native/native_with_context` and `external/native_with_context`). This rules out an implementation that stops comparing the reviewer parts.
- Addressed: "The golden no longer contains any reviewer wording". Scenario 6 checks the golden part by part: builder parts by SHA-256 of today's bytes, native reviewer parts by SHA-256 of their JSON with sorted keys, which holds the placeholders. It also requires that the golden name none of the reviewer's wording, pinned or main. Scenario 7 runs `-update` in a scratch copy against the pin and against main and requires the committed bytes.
- Superseded: k-build-context-bytes-unpinned's acceptance pins the golden's SHA-256 `9b753a918827ef61e9072fea1e8af6ec132aed43b64f3a5a921e104cfe698790`. This change deliberately rewrites the golden. The request says the two Intents land one after the other and the second rebases; this is the second.
- Not used: `review.Prompt(...)`. The test's native reviewer runs without the on_demand mode, and `reviewBody` already identifies it by `review.ReviewerInstructions()`.
- Preserved literals: the test name, the golden path, the culprit `e5005fa` and its parent `640bdc6`, the pinned `cfc2fb29917c` and the main `81d3bae` with their module versions, `review.RequirementsRule`, `review.ReviewerInstructions()`, the `requirements` parameter, the 8 subtest names and the quoted failures.

## Tests
- The native reviewer's instructions and `submit_review` definition are compared with kogen-review's exports at the tested version, and the golden holds placeholders: updated TestContextExternalNativeBytes passes against the pinned kogen-review, `e5005fa`, main `81d3bae` and a wording-only edited copy (acceptance scenarios 1–3) (regression)
- An external reviewer's parts are the native reviewer's authority, diagnostic and source, followed by kogen-review's review task carrying `review.LimitRaises`: TestContextExternalNativeBytes, which still fails when the external or native reviewer drops the diagnostic (acceptance scenarios 4–5) (regression)
- The golden keeps its builder bytes, has native reviewer parts with placeholders, has no external reviewer parts and names no reviewer wording, and -update against the pin and main reproduces it: acceptance scenarios 6–7 (regression)
- go.mod keeps its kogen-review pin and the configured checks run with it: acceptance scenario 1 and the configured go-test check (existing)

## Feasibility
- I ran these in this read-only external session, in scratch copies outside the checkout. They are not Kogen probe records.
  - On the unchanged tree, `go test -modfile=<tmp go.mod replacing kogen-review with the module cache folder of v0.0.0-20261103212337-81d3baef1d48> -count=1 -run '^TestContextExternalNativeBytes$' ./build` fails 7 subtests, at the request's byte offsets (20572, 17661, 18565). With the committed go.mod, all 8 pass in about 7 s.
  - I ran `-update` against main in a scratch copy and compared the result with today's golden. Builder parts: identical. Native reviewer parts: they differ only in `instructions` and the `submit_review` tool. External `reviewer.dynamic`: identical. External `reviewer.stable`: identical up to the review task.
  - After the placeholders above, the native reviewer parts of pin and main have the same SHA-256 of sorted-key JSON (the fingerprints in scenario 6). In today's golden, every external `reviewer.stable` equals A + "\n" (+ D + "\n" + "\n") + the review task and every `reviewer.dynamic` equals S, with A, D and S taken from `native/native` as specified.
  - `go mod download -json github.com/KogenAI/kogen-review@<version>` resolves the pin, `e5005fa` and `81d3bae` from the module cache. Their go.mod files require the same coordinator, executor and verification versions as kogen-build's, so a directory replace compiles without go.sum changes.
  - The edited copies of the pinned kogen-review compile; each edit's text occurs exactly the listed number of times. On the unchanged tree, the wording-only copy fails all 7 comparing subtests (legitimate red). The drop-diagnostic copies fail exactly `native/external_with_context`, and exactly `native/native_with_context` and `external/native_with_context`. Under the rule above, those two failure sets stay the same.
  - The cucumber dry run binds all 48 steps of the 9 scenarios. The three scenarios that use edited copies ran in about 21 s in total.
- evidence: {"id":"reviewer-bytes-judges","kind":"observer","inputs":["features/steps/k-build-reviewer-prompt-bytes-unpinned__steps.js","features/steps/k-build-reviewer-prompt-bytes-unpinned__controls.js"],"observers":["features/steps/k-build-reviewer-prompt-bytes-unpinned__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-reviewer-prompt-bytes-unpinned__steps.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-reviewer-prompt-bytes-unpinned__steps.js bad","exit":1,"output":"rejected\n"}]}
- The observer's controls:
  - Run judge, known-good inputs: the real `go test -json` rows (output rows left out) of this test on the unchanged tree. One run is with the pin (exit 0, all pass); the other is against the external drop-diagnostic copy (exit 1, only `native/external_with_context` failed).
  - Run judge, refused variants: a failed, missing, extra or skipped test; no test run; a wrong exit; no package result; a passing run where a failure is required; a failing run where all must pass; a failing run where another subtest must fail.
  - Golden judge, known-good input: today's golden (read directly while its SHA-256 is `9b753a91…`, otherwise from git blob `a3db31e16ed35670739db3d538318beaecc01278`) rewritten as specified.
  - Golden judge, refused variants: today's golden; the golden keeping the instructions, the `submit_review` definition or the external reviewer parts; a changed builder byte; an extra part; a missing entry; reviewer wording outside the parts.
  - The controls qualify the judges only, not product behaviour.
