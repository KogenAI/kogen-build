# Accept forced unread-part review reminders

Slug: b-consumes-review-reminder-forces-read-file
Model: not applicable — this changes test recognition, not the review state machine
Acceptance:
- .kogen/intents/b-consumes-review-reminder-forces-read-file/acceptance.feature
- features/steps/b-consumes-review-reminder-forces-read-file__steps.js
- features/steps/b-consumes-review-reminder-forces-read-file__observer.js

Update TestKReviewVerdictStrandReset in build/k_review_verdict_strand_test.go to recognize the forced reminder whether tool_choice names submit_review or read_file while pinned parts remain unread. Apply every existing assertion about the forced request to the selected reminder and its original body/index. Keep all other assertions unchanged: the reset immediately before that request, input budget, provider-fault and exhaustion text, and the fresh review approving and landing. No non-test implementation changes, dependency updates or unrelated cleanup are permitted.

This is the consumer side of kogen-review review-reminder-reads-unread-parts (Build c2e401c0, 3c28d26a): after a no-tool turn the producer may force read_file naming unread pinned parts, and forces submit_review only after they have all been read. The supplied consumer failure was TestKReviewVerdictStrandReset at line 72, forced requests = 0, want 1 (10 Nov 18:33Z). Support both the old and new producer contract without changing the producer or requiring its candidate to be installed.

Search other tests for tool_choice comparisons against forced submit_review; change only tests broken in the same way by unread pinned parts. Do not broaden assertions for conversations with no unread pinned parts. Run the affected tests and package tests; retain the configured full checks. The pinned dependency currently exercises the old contract, so the acceptance additionally calls the actual forcedBodies selector with transmitted bodies from both contracts. The strand fixture itself establishes the unread-part context; recognizing read_file here is not a new global rule about arbitrary read_file requests.

## Decisions
- Keep the existing strand fixture and assertions and change only reminder recognition; the fixture already uses a multipart candidate. (evidence: build/k_review_verdict_strand_test.go) (source: request)
- Run the existing real Build regression as well as exercising its callable selector with both wire choices; no production abstraction is needed. (evidence: build/k_review_verdict_strand_test.go; .kogen/project.yaml) (source: request)

## Source comparison
- addressed: request Need first bullet and supplied failure; outcome paragraphs and Recognize both reminder contracts acceptance replace submit-only forcedBodies recognition while preserving the complete TestKReviewVerdictStrandReset function.
- addressed: request Need second bullet; scope explicitly requires searching all forced submit_review comparisons and only adjusting any other tests with the same unread-part failure. The existing comparisons in build/k_reviewretry2_test.go and build/k_reviewretry_test.go are evidence to inspect, not authority to weaken other assertions.
- addressed: request Need third bullet and Run the package tests; test-only implementation scope, real Build regression acceptance and configured package checks remain required. No dependency or non-test changes are authorized.
- addressed: request producer Build identifiers, date and exact diagnostic are preserved above as historical report, not a claim that this checkout contains that producer candidate.
- omitted: no requested behavior. No UI, provider transport, golden changes or review policy changes are included.

## Tests
- Recognize either forced reminder in the unread-part strand fixture: exercise forcedBodies with auto, forced submit_review and forced read_file, preserving corresponding indexes and bodies (regression)
- Preserve every existing strand assertion and real Build outcome: compare the existing regression function unchanged and execute TestKReviewVerdictStrandReset through go test ./build (integration)
- Preserve package compatibility: run go test ./build -count=1 and the configured Go package checks without updating goldens (existing)

## Feasibility
- The focused unchanged TestKReviewVerdictStrandReset passed with the pinned old producer. The attempted full build package run hit the scratch 300-second limit; that attempt does not establish a package pass. The builder must still run package tests; no timeout increase or test removal is authorized.
- evidence: {"id":"reminder-observer","kind":"observer","inputs":[".kogen/intents/b-consumes-review-reminder-forces-read-file/controls.json"],"observers":["features/steps/b-consumes-review-reminder-forces-read-file__observer.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/b-consumes-review-reminder-forces-read-file__observer.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/b-consumes-review-reminder-forces-read-file__observer.js bad","exit":1,"output":"rejected\n"}]}
