# Fix review strand test forced-tool expectation

Slug: build-review-verdict-strand-forced-read
Model: not applicable — the change only updates a Go test and its fixtures; there is no state to model
Acceptance:
- .kogen/intents/build-review-verdict-strand-forced-read/acceptance.feature
- features/steps/build-review-verdict-strand-forced-read__steps.js

TestKReviewVerdictStrandReset (build/k_review_verdict_strand_test.go) fails with `fault tool = "read_file", want "submit_review"`. The kogen-review contract forces `read_file` while pinned review parts remain unread and forces `submit_review` only once all are read; the test assumes an unconditional forced `submit_review`. Update only the affected tests and fixtures so the forced reminder is checked against the actual pinned unread-part state: forced `read_file` is valid while unread parts remain, and `submit_review` is required once none remain. No non-test production code changes.

The test keeps asserting that the strand resets before the forced request, that input stays within its budget, the provider-fault and exhaustion messages, and that a fresh review approves and lands. The forced-tool assertion (tool_choice of the forced request, the provider_fault `tool` and the review_tool_choice event) becomes conditional on the unread pinned parts, and a test case or fixture covers both states (some unread, none unread).

## Decisions
- The unread state is read from the observed window-reset handoff of the forced request (its `unread_parts` count), not hard-coded. (evidence: build/k_review_verdict_strand_test.go) (source: request)
- Only `_test.go` files and test fixtures change; production code stays untouched. (evidence: build/k_review_verdict_strand_test.go) (source: request)
- The proof is the real `go test` of TestKReviewVerdictStrandReset plus a diff scope check; no network or model calls are involved because the test uses the scripted provider. (evidence: build/k_review_verdict_strand_test.go) (source: request)

## Source comparison
- Request item 1 (test passes offline): scenario runs the test with go test -json and requires it ran and passed.
- Request item 2 (retained assertions): kept in the Go test; the scenario requires the test to pass unskipped.
- Request item 3 (conditional forced tool, both states covered): scenario requires the test file to cover both read_file and submit_review forcing branches.
- Request item 4 (full suite, test-only diff): the configured go-test check runs the full suite; scenario checks changed files.

## Tests
- Forced tool is read_file while pinned parts are unread and submit_review once none are: update TestKReviewVerdictStrandReset and add a case/fixture covering both states (unit)
- Strand reset, budget, fault and exhaustion messages and landing stay asserted: TestKReviewVerdictStrandReset (existing)

## Feasibility
- none
