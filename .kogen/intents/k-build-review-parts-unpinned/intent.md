# Assert review-part outcomes, not prompt layout

Slug: k-build-review-parts-unpinned
Model: not applicable — only tests, test data and README text change; no product state or transition is added or changed
Acceptance:
- .kogen/intents/k-build-review-parts-unpinned/acceptance.feature
- features/steps/k-build-review-parts-unpinned__steps.js
- features/steps/k-build-review-parts-unpinned__controls.js

kogen-review's k-review-parts-linear-input (candidate 7d005ed6, Build 7397652b) changes how a review in parts is sent. Each part is read once, and the "# Review task" part catalog and the re-sent history are gone. Eight kogen-build tests in package build pin that layout, so delivery's consumer gate refused the candidate twice (land log k-review-parts-linear-input-20261104T071731Z.log):

```
kogen-build	TestAReviewerThatNeverReadsIsStopped	review_test.go:1463: review requests = 1, want 6
kogen-build	TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation	review_test.go:1028: no catalog in # Review task
kogen-build	TestChangesAfterEveryPartWasReadStaysBlocked	review_test.go:1415: no catalog in # Review task
kogen-build	TestChangesWhilePartsAreUnreadIsSentBackToRead	review_test.go:1441: exit = 1, want 0
kogen-build	TestKReviewRetrySeparateAllowances	-
kogen-build	TestKReviewRetrySeparateAllowances/S-DECREL:_unread_rejections,_prose,_then_the_forced_verdict_in_the_same_conversation	k_reviewretry_decrel_test.go:25: exit 4 err <nil>
kogen-build	TestKReviewRetrySeparateAllowances/reverse_order_in_a_fresh_review:_prose,_then_a_rejected_verdict,_then_a_valid_one	k_reviewretry_decrel_test.go:56: no catalog in # Review task
kogen-build	TestKReviewVerdictStrandReset	k_review_verdict_strand_test.go:72: exit 1 err <nil>
kogen-build	TestKRevodGatePartsUnchanged	k_revod_gate_parts_test.go:180: parts-mode review request 1 changed: {Bytes:367060 SHA256:a4f87dad5df2c4bacc22338c1c11846f3f10b8a80751e17a72cfcd2ffcbf125d Pref
kogen-build	TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval	review_test.go:902: no catalog in # Review task
```

After this change, those eight tests (the "review-parts tests") assert the outcomes kogen-build relies on, not kogen-review's prompt text or layout. Lesson 4: a consumer never pins a producer's bytes or wording. The tests keep their names and the two named subtests of TestKReviewRetrySeparateAllowances keep theirs, so the consumer gate and the acceptance find them. Their doc comments state the outcome each one checks. Only `_test.go` files, the two `k-revod-gate` request goldens and one README sentence change. There is no production change: no non-test Go file, go.mod or go.sum changes, and go.mod keeps pinning kogen-review `v0.0.0-20261103124347-cfc2fb29917c`.

## What the review-parts tests no longer assert
These rules apply to the eight tests and to every helper they call: `reviewCatalog`, `catalogParts`, `readParts`, `readThen`, `confirmingReviewer`, `pinnedOf`, `deliverThen`, `allDelivered`, `authorityOf` and the `allowanceKit` closures. A helper that other tests also use may change, provided those tests still pass with the pinned kogen-review.
- **Request layout.** They do not read anything out of a reviewer request by its position or heading. In particular, they do not look for the pinned-part catalog in the opening's second content block (`no catalog in …`), and they do not compare the opening's first block (`authorityOf`). They make no assumption about a "# Review task" heading.
- **Request counts.** They assert no exact number of reviewer or provider requests: no `len(f.reviewRequests()) == N`, no `f.count()-len(reqs) == N`, and no request picked by an index that follows from an expected count (such as `reqs[1]`, `reqs[2]` or `reqs[4]` meaning "the request after the first verdict"). This includes the `eq(t, "review requests", …, 1)` in `revodGateEmptySourceControls` (build/k_revod_gate_test.go). An upper bound that kogen-build's own working window guarantees, such as "no request is larger than `524_288+4096` bytes", is allowed.
- **Request bytes.** They do not compare the bytes, byte counts or SHA-256 of reviewer requests. TestKRevodGatePartsUnchanged drops `revodGateDescribe`, `revodGateRequest`, `revodGateGolden` and its `-update` writing, and the test only (`revodGatePartsRequests`) keeps the run. The goldens `build/testdata/k-revod-gate/parts-requests.json` and `build/testdata/k-revod-gate/parts-requests-moved.json` are deleted. README.md's list of frozen goldens (the `-update` paragraph under "Golden files") no longer names `k-revod-gate/parts-requests.json`; the rest of that sentence stays.
- **kogen-review's sentences.** They do not match kogen-review's refusal or reminder wording. Examples are "Read all pinned source evidence before approving:", "pinned part(s) unread; read them before a verdict", "invalid verdict:", "Call submit_review.", "Not a valid verdict: the review ended without calling submit_review." and the regular expressions built on them that `allowanceKit.unread` uses.
- **What stays allowed.** Tests may rely on kogen-build's own artifacts and on what kogen-review exports:
  - The pinned part files that kogen-build's `materialPrepare` writes, `<run dir>/review-source/<review>-<candidate tree>/<n>.json`. They are numbered 1..N, mode 0600 in a 0700 folder, and each is JSON with an empty `input` and `review_source` {`review`, `part`, `parts`, `text`}. Part paths, ordered by n, come from these files or from the review working context that kogen-build saves.
  - kogen-build's native prefill messages (`prefillMarker`, `prefilledParts`) and its working-window reset handoff (`resetMarker`, `resetHandoff`, "Local working-window reset").
  - The Build's exit code and stdout, its run record (the result's `outcome`, `reviews` and `passes`, the `review-N.json` records, and run events such as `review_material`, `review_source_evidence`, `review_source_prefill`, `window_reset`, `review`, `review_retry`, `review_tool_choice` and `provider_fault`), Git state, and request fields kogen-build sends itself (`tool_choice`, `prompt_cache_key`).
  - Recognising reviewer requests by `review.ReviewerInstructions()`.
  - A test also passes when its run record has more or fewer events of a kind than today, as long as the asserted outcome holds.
- **Catalog contents.** TestLargeSource… no longer reads the catalog fields (`version`, `start_head`, `candidate_tree`, `files`, `source_bytes`, `source_sha256`) from a request. It may read them from kogen-build's own saved review state or drop them. material_test.go:532 already asserts them through `materialPrepare`, and it is unchanged.

## What each test asserts
Each test runs the real Build with the real independent reviewer against the scripted provider, as today. Each of the following must hold with the pinned kogen-review, with kogen-review main and with the 7d005ed6 stand-in (see Acceptance below).
- **TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval.**
  - The builder writes `bigFile()` to app.txt.
  - The review source is pinned as at least 20 part files under `<run>/review-source/1-<candidate tree>/`, with the modes, JSON shape and numbering above.
  - The parts' `text`s, joined in order, are the review source. It contains "# Diff of existing/deleted paths against start HEAD <short start head>" and "--- app.txt\n" + the content.
  - A scripted reviewer reads every part (paths from the part files, in batches) and then approves. The Build exits 0, review-1.json has verdict approve, and HEAD:app.txt is the content.
  - One `review_source_evidence` event names the part count.
  - No request exceeds the ceiling above.
  - A working-window reset is no longer required.
- **TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation** (the fault test). This is a scripted reviewer that approves with one part unread.
  - It reads, or receives through kogen-build's prefill, every pinned part except one, P, and then approves. In the approving response it may also read P: a read and an approval in one response deliver nothing, as today.
  - The test confirms through kogen-build's custody that P was undelivered at that approval. If kogen-build's prefill had delivered P, the fixture is broken and the test fails saying so.
  - The approval is refused. P's path appears in what Kogen returns to the reviewer for that `submit_review` call (the matching `function_call_output` in a later reviewer request), or in the recorded problems of the invalid review.
  - No review record says approve while P is unread, and the candidate is not committed while P is unread.
  - The test may end with the current review_invalid stop: exit 1, HEAD unchanged, stdout naming review_invalid, no review-1.json. Or a later approval may land after P has been delivered.
- **TestChangesWhilePartsAreUnreadIsSentBackToRead.**
  - The reviewer's first answer is a changes verdict with a blocking finding while parts are unread. Every later answer reads unread parts, then approves.
  - The asserted outcome is that the changes verdict given while parts are unread never lets the candidate land with a part undelivered. Either:
    - the Build exits 0 and commits, and the approving review came after every pinned part was delivered (the last `review_material` event has `parts_read` equal to the part count, and review-1.json or the approved review's record says approve); or
    - the Build stops with exit 1, HEAD unchanged, and the run's outcome review_blocked or review_invalid.
  - kogen-review main sends the changes verdict back (first outcome). The stand-in ends the review blocked (second outcome). Either is correct.
- **TestAReviewerThatNeverReadsIsStopped.**
  - The reviewer only ever answers changes with a blocking finding and never reads.
  - The Build exits 1, HEAD is unchanged and the run's outcome is review_invalid or review_blocked.
  - It ends on its own before the 40 scripted answers run out. A run that only ends because the scripted provider refuses after the script is used up has another outcome and fails.
  - No request count is asserted.
- **TestChangesAfterEveryPartWasReadStaysBlocked.**
  - The reviewer reads every pinned part (paths from the part files), then answers changes with a blocking finding.
  - The Build exits 1, stdout names review_blocked and HEAD is unchanged.
- **TestKReviewRetrySeparateAllowances**, subtests "S-DECREL: unread rejections, prose, then the forced verdict in the same conversation" and "reverse order in a fresh review: prose, then a rejected verdict, then a valid one". Both are in build/k_reviewretry_decrel_test.go, and both keep their names and their 232 and 113 pinned parts.
  - **S-DECREL:** an approval given while parts are unread is rejected, a prose-only turn follows, and the review stays one review in one conversation (one `prompt_cache_key`). The Build lands with one review whose approval came after every one of the 232 parts was delivered (`review_material` `parts_read` 232), and app.txt is committed.
  - **Reverse order:** review 1 ends on two prose-only turns. Review 2 is a fresh review in a conversation of its own whose 113 parts are its own files under `/review-source/2-`. A forced approval in review 2, made before its parts were delivered, is rejected; review 1's delivery counts for nothing in review 2. The Build lands after review 2's approval with `reviews` 2 and `parts_read` 113 for review 2.
  - The unread counts today parsed from kogen-review's refusal sentences are dropped, or replaced by kogen-build's own custody count at that moment.
  - The script must not depend on whether a changes verdict with unread parts is sent back. For example, the first unread answer of S-DECREL becomes an approval.
  - The other subtests of TestKReviewRetrySeparateAllowances do not change.
- **TestKReviewVerdictStrandReset.** The verdict strand resumes and resets as today:
  - In review 1, a request forced to `submit_review` comes right after a real kogen-build working-window reset, within the ceiling. The reset handoff is in that request and not in the one before it.
  - The text-only answer to that request is one `provider_fault` event with fault `forced_tool_not_called`, route `chatgpt`, model `gpt-6.1-terra` and tool `submit_review`, and one `review_tool_choice` event with mode `api_forced`.
  - kogen-build's native prefill ran around the forced turn: at least two `review_source_prefill` events.
  - Review 1 ends invalid with reason `no_tool_reminder_exhausted`. One automatic `review_retry` starts review 2 in a conversation of its own that replays none of review 1's calls, and review 2 approves the same green tree.
  - The Build lands exactly one commit with HEAD:app.txt equal to `bigFile()`, `reviews` 2 and `passes` 1 (no completed check was rerun).
  - The script no longer opens review 1 with a changes verdict that relies on kogen-review's unread-parts exemption: the reset is reached through the large prose turn alone.
  - The reminder text and the `allowances` list with `unread_progress_exempt` are no longer asserted.
- **TestKRevodGatePartsUnchanged.**
  - The long parts-mode review of `revodGatePartsRequests` (pinned parts, reads, approval) lands: exit 0 and HEAD moved.
  - No reviewer request carries `required_parts`, `required_unread`, `on_demand`, `hunk-covering` or `covering changed hunks`.
  - It no longer requires that the run exercised prefill or a reset.
  - Its subtest `empty-source controls` keeps its 60 inline cells and its landed inline control without pinned source, with the exact review-request count removed.

## Acceptance
- The review-parts tests and the three required subtests pass, and no test of the run fails or is skipped, in each of these runs:
  - with the committed go.mod;
  - against kogen-review main `fee5268` (`v0.0.0-20261104082000-fee52689bd61`, the module cache, via `-modfile`);
  - against the 7d005ed6 layout stand-in: a scratch copy of that main with the `env.Prepare` result passed through `standInOpening` (the opening's source block begins with "# Review task" and carries no pinned-part catalog) and the native "changes with unread parts" send-back disabled.
- Fault test: with the stand-in, TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation passes, and fails once the copy also accepts an approval with unread parts, or once its refusal stops naming the unread parts (`%.0s`).

## Decisions
- The eight tests stop asserting kogen-review's prompt text and layout: no "catalog in # Review task", no exact review request counts, no request bytes or SHA-256 of review requests (source: request)
- They assert the outcomes kogen-build relies on: approval is refused while a pinned part is unread and that part is named; a part changed after it was read blocks approval; a verdict strand resumes and resets as today; a reviewer that never reads is stopped (source: request)
- No production change in kogen-build (source: request)
- A scripted reviewer that approves with one part unread is still refused (source: request)

## Source comparison
- **Addressed: "The tests pass against kogen-review main".** At shaping time, `git ls-remote https://github.com/KogenAI/kogen-review.git` named main `fee52689bd61` ("Reset a refused review request without duplicates", module version `v0.0.0-20261104082000-fee52689bd61`). The local mirror's main is the older `bfec92f`. Acceptance scenario 2 runs against fee5268.
  - On today's tree, against fee5268, 7 of the 8 tests pass. TestKRevodGatePartsUnchanged fails at k_revod_gate_parts_test.go:180, e.g. "parts-mode review request 1 changed: {Bytes:451913 …}, want {Bytes:448535 …}".
- **Addressed with a stand-in: "… and against kogen-review at 7d005ed6".** 7d005ed6 is not retrievable here:
  - It is absent from the mirror /Users/kogen-runner/work.noindex/mirrors/kogen-review.git and from a fresh clone of github.com/KogenAI/kogen-review.
  - `go mod download github.com/KogenAI/kogen-review@7d005ed6` reports "invalid version: unknown revision 7d005ed6".
  - Acceptance therefore runs a stand-in made from main. It reproduces the observed layout symptoms (the catalog is gone from the opening and the opening's source block begins with "# Review task") and the observed changes-verdict behaviour.
  - On today's tree the stand-in gives 9 of the 10 land-log rows with the same file:line and first message. Examples are review_test.go:1463 "review requests = 1, want 6", :1028/:902/:1415 "no catalog in # Review task", :1441 "exit = 1, want 0", k_reviewretry_decrel_test.go:56 and k_review_verdict_strand_test.go:72 "exit 1 err <nil>".
  - S-DECREL differs: k_reviewretry_decrel_test.go:25 reports "exit 1 err <nil>", not the log's "exit 4 err <nil>".
  - Passing against the stand-in is not proof of passing against 7d005ed6. Delivery's consumer gate re-runs these tests against the real candidate.
- **Addressed: "approval is refused while a pinned part is unread (and that part is named)" and "Fault test: a scripted reviewer that approves with one part unread is still refused".** TestApprovalNeeds… (scenario 4) and TestLargeSource… cover these. The two fault edits are each confirmed to make today's TestApprovalNeeds… fail on main: "exit = 0, want 1", and the refusal text lacking the part paths.
- **Addressed, mapped: "a part changed after it was read blocks approval".** The request names TestChangesAfterEveryPartWasReadStaysBlocked, which asserts that a changes verdict after every part was read stays blocked; it keeps that outcome. A pinned part whose bytes changed is refused by kogen-build's custody check, which TestReviewBudgetOmittedPartsRemainUnderCustodyValidation (build/review_budget_test.go:273) already asserts. That test pins no kogen-review layout and is unchanged.
- **Addressed: "a verdict strand resumes and resets as today"** (TestKReviewVerdictStrandReset) and **"a reviewer that never reads is stopped"** (TestAReviewerThatNeverReadsIsStopped).
- **Addressed: "No production change in kogen-build".** Scenario 6 checks that no non-test Go file, go.mod or go.sum changed. Scenario 1 checks the kogen-review pin.
- **Superseded:** `.kogen/intents/k-models-file-build/acceptance.feature:105` and `features/steps/k_golden_update_build/{good,bad}.json` name SHA-256 values of `parts-requests.json`. They belong to landed Intents and are not changed.
- **Out of scope:**
  - kogen's TestKReviewVerdictStrandPinnedResume, which lives in the kogen repository.
  - Other kogen-build tests, which are not required to pass against main or the stand-in.
  - Production code, go.mod pins and consumes/ suites.
- **Preserved literals:** the eight test names, the two subtest names, the land-log rows above, candidate 7d005ed6, Build 7397652b, the land log name, the pin `v0.0.0-20261103124347-cfc2fb29917c`, main `fee52689bd61` with its module version, and the golden paths.

## Tests
- Reviewer requests' layout, counts, bytes and kogen-review's wording are no longer asserted, and the eight tests assert kogen-build's outcomes: updated TestLargeSource…, TestApprovalNeeds…, TestChangesWhilePartsAreUnread…, TestAReviewerThatNeverReads…, TestChangesAfterEveryPartWasRead…, TestKReviewRetrySeparateAllowances (both heavy subtests), TestKReviewVerdictStrandReset and TestKRevodGatePartsUnchanged pass with the pin, with main fee5268 and with the 7d005ed6 stand-in (acceptance scenarios 1–3) (regression)
- An approval with one pinned part unread is refused and names that part: updated TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation, which fails against the stand-in once it accepts the approval or stops naming the part (acceptance scenario 4) (regression)
- The review-request byte goldens are removed and the README no longer lists them: acceptance scenario 5 checks that both golden files are gone and README.md lacks their name (regression)
- No production change; the kogen-review pin and the configured checks are unchanged: acceptance scenarios 1 and 6 and the configured go-test check (existing)

## Feasibility
- I ran these myself in this read-only external session, in scratch copies outside the checkout (/tmp). They are not Kogen probe records.
  - With the committed go.mod, `go test -json -count=1 -run '^(<the eight tests>)$' ./build` passes, all 14 tests and subtests, in about 17 s.
  - The same command against fee5268 through `-modfile` fails only TestKRevodGatePartsUnchanged, at the byte comparison.
  - A copy of fee5268 whose opening is only prefixed with "# Review task" (catalog kept) passes all of today's tests except the byte comparison, so a changed opening alone does not change these outcomes.
  - The stand-in as the feature defines it builds from the feature's own edit table and docstring, and gives the land-log rows listed under Source comparison.
  - The fault-row scenario run on today's tree fails at its first Then step with "review_test.go:1028: no catalog in # Review task" (legitimate missing-feature red).
  - Each edit's `from` text occurs exactly once in fee5268's review/independent.go, and both fault edits are effective on main.
  - The cucumber dry run binds all 68 steps of the 7 scenarios.
- evidence: {"id":"review-parts-run-judge","kind":"observer","inputs":["features/steps/k-build-review-parts-unpinned__steps.js","features/steps/k-build-review-parts-unpinned__controls.js"],"observers":["features/steps/k-build-review-parts-unpinned__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-review-parts-unpinned__steps.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-review-parts-unpinned__steps.js bad","exit":1,"output":"rejected\n"}]}
- The observer's controls:
  - **Run judge, known-good inputs:** the real `go test -json` rows (output rows left out) of two runs on today's tree. One is the eight tests with the pin (exit 0, all pass). The other is TestApprovalNeeds… against the stand-in with the approval check removed (exit 1, that test failed).
  - **Run judge, refused variants:** a failed, missing or skipped listed test; a missing required subtest; a failed or skipped extra subtest; no test run; a wrong exit; no package result; a failing run where a test must pass; a passing run where it must fail; a failing run of another test; a failing run that exited 0 or has no package result.
  - **Changed-files judge:** it accepts test, test-data, README and step changes, including a rename between two `_test.go` files. It refuses a changed or added non-test Go file, a changed go.mod or go.sum, and a rename between a test and a production file in either direction.
  - The controls qualify the judges only, not product behaviour.
