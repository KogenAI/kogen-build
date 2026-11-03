# Explain ignored forced review submissions

Slug: k-review-forced-no-submit
Model: not applicable — diagnostic propagation only; existing review retry and checkpoint state transitions remain unchanged
Acceptance:
- .kogen/intents/k-review-forced-no-submit/acceptance.feature
- features/steps/k_forced_submit_steps.js
- features/steps/k_forced_submit_test.go.txt

When a provider returns no tool call despite API-forced submit_review, retain the existing bounded fresh-review retry. If that ends without a verdict, surface an honest actionable provider-failure diagnostic instead of only the generic review_invalid message. Preserve the candidate and existing review-only resume eligibility, without inventing an approval or changes verdict. Keep candidate/review_invalid compatibility and existing structured fault records. No extra requests, CLI commands, flags, raised timeouts or model runs in tests.

The operator diagnostic must contain `provider ignored forced submit_review`, the actual route and model, `review 2` for the second review (actual review number otherwise), `candidate preserved` when eligible, and the existing `kogen intent resume` invocation with the actual Build ID. Do not claim preservation or resume eligibility when existing safety checks refuse it. Unrelated invalid reviews must not be mislabeled as ignored forced tool calls.

## Decisions
- Stop with explicit provider failure after the existing automatic retry; add no recovery request. Identify ignored tool, route/model and affected review, preserve the candidate and explain existing resume. (source: answer 1)
- Keep all existing safety, verdict validation and review allowances. (source: request)

## Source comparison
- Addressed: request Need and answer 1 by the operator diagnostic and unchanged bounded retry above, exercised by acceptance.feature through a real Build with a local scripted provider.
- Addressed: request Constraints by no new commands/flags, timeouts or paid model tests.
- Addressed: reported 3ca3098a and 89beb048 (kogen-verification, K-GOLDEN-UPDATE-VERIFICATION, 573d023, terra/medium), and c139c0da (kogen-coordinator, K-GOLDEN-UPDATE-COORDINATOR, 3 Nov 01:10Z), whose forced no-tool turns follow source reads: the acceptance fixture supplies prose then ignored forced turns in both reviews, observes two faults and exactly one automatic retry. Existing source/reset regressions must remain passing.
- Omitted from executable fixture: inaccessible historical Mac Studio paths `/Users/kogen-runner/work.noindex/src/k-golden-update-verification/.kogen/local/runs/3ca3098a` and `89beb048`, and laptop `~lanes/work.noindex/src/k-golden-update-coordinator/.kogen/local/runs/c139c0da`, are evidence locations, not runtime dependencies. The 1235 parts, 7.4 MB and roughly 170k-token request are historical scale, not a new size requirement. K-REVIEW-VERDICT-STRAND (33bbe2e) remains preserved, not superseded. No live reproduction is required.

## Tests
- Forced submit_review ignored in both reviews: synthetic real-Build regression asserting operator diagnostic, route/model, affected review and resumable candidate, unchanged HEAD and bounded requests (regression)
- First forced failure followed by a valid verdict still completes: retain TestKReviewRetry2LogReplay and TestKReviewVerdictStrandReset (existing)
- Existing review-only resume and candidate safety remain unchanged: retain k_review_invalid_resume_test.go and k_review_verdict_strand_test.go (existing)
- Other invalid verdict failures are not provider failures: add focused diagnostic regression for ordinary invalid feedback (regression)

## Feasibility
- evidence: {"id":"diagnostic-observer","kind":"observer","inputs":["features/steps/k_forced_submit_test.go.txt","features/steps/k_forced_submit_steps.js"],"observers":["features/steps/k_forced_submit_test.go.txt","features/steps/k_forced_submit_steps.js"],"checks":[{"name":"good","command":"NODE_PATH=\"$KOGEN_CUCUMBER\" \"$KOGEN_NODE\" features/steps/k_forced_submit_steps.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"NODE_PATH=\"$KOGEN_CUCUMBER\" \"$KOGEN_NODE\" features/steps/k_forced_submit_steps.js bad","exit":1,"output":"rejected\n"}]}
