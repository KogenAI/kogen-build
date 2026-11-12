# Bound and type-check approval and index reads

Slug: k-build-approval-record-read-bounded
Model: not applicable — a stateless file-read check with no state transitions
Acceptance:
- .kogen/intents/k-build-approval-record-read-bounded/acceptance.feature
- features/steps/k-build-approval-record-read-bounded__steps.js
- features/steps/k-build-approval-record-read-bounded__driver/driver_test.go.txt

The Build compares the approval record and the saved index copy with the current ones at start, at resume (build/resume.go) and at review retry (build/review_retry.go, approvalIntact and indexIntact). Each file is now opened once, checked on the open handle to be a regular file, and read through a fixed limit of 64 MiB (a constant, not a setting). A shared helper does this for every such comparison site (start, resume, review retry, and the approval and index comparisons in build/recover.go). A normal-size file compares equal or different exactly as before ("the approval changed", "the staged index changed" for real differences). Otherwise the comparison is refused with an error whose text names the path and says which it is: containing "oversized" (more than 64 MiB), "not a regular file" (named pipe, directory, device) or "unreadable" (open or read failure). Such messages must not contain the word "changed". A named pipe must be refused without blocking (open non-blocking, then check the type on the handle). The generic "the approval changed" reported for a read failure is removed.

The acceptance driver is a Go test kept as driver_test.go.txt and overlaid into package build with `go test -overlay` (no checkout file changes). It builds an Attempt with Root, Slug, ApprovalBytes, GitIndex and Run.Dir and calls approvalIntact and indexIntact, so those two method names and signatures (returning error) must stay.

## Decisions
- The limit is a fixed 64 MiB constant (assumption: the user accepts 64 MiB as the limit) (source: request)
- Refusal messages contain the lowercase phrases "oversized", "not a regular file" or "unreadable" plus the path, and never "changed" (source: request)
- One shared helper serves start, resume, review retry and recover (evidence: build/review_retry.go; build/resume.go; build/recover.go) (source: request)
- The limit is a fixed 64 MiB constant (decision: D1, round 1) (assumption: The user must accept 64 MiB as the limit.) (source: request)
- Refusal messages contain the lowercase phrases oversized, not a regular file or unreadable plus the path, and never changed (decision: D2, round 1) (assumption: Exact wording is chosen here, not given by the request.) (source: request)
- One shared helper serves start, resume, review retry and recover (decision: D3, round 1) (evidence: build/review_retry.go; build/resume.go; build/recover.go) (assumption: Start-site code was not read directly.) (source: request)

## Source comparison
- Normal files compare as today: addressed by approval_same, approval_different, index_same.
- Oversized, pipe and unreadable refusals: addressed by the remaining scenarios.

## Tests
- Normal files compare unchanged: acceptance scenarios on approvalIntact and indexIntact (integration)
- Oversized, pipe and unreadable refusals with distinct messages naming the path: acceptance scenarios (integration)

## Feasibility
- none
