# Reject incomplete reuse receipt records

Slug: validate-reuse-receipt-shape
Model: not applicable — receipt validation is a pure per-line check with no state that the Build depends on
Acceptance:
- .kogen/intents/validate-reuse-receipt-shape/acceptance.feature
- features/steps/validate-reuse-receipt-shape__steps.js
- features/steps/validate-reuse-receipt-shape__tests/receipt_shape.go.tmpl

readReuseReceipts in build/reuse_shadow.go accepts any decoded JSON object with a nonempty key, so a line holding only a key and status zero is treated as a green receipt and the shadow can report unsupported reuse evidence. A receipt line becomes valid only when it is complete: every field of a receipt (key, name, status, wall_ms, source and log) is present, and key, name, source and log are nonempty; status and wall_ms may be zero. Any other line, including one that is not valid JSON, is corrupt and readReuseReceipts returns the existing corrupt-line error (`receipt file <path> is corrupt at line <n>`).

When a Build meets such a receipt file, the existing handling stays: one reuse_shadow_unavailable event with a nonempty reason is recorded, no reuse_shadow event is logged, and every baseline check still runs normally. Complete receipts written by baseline runs and by whole-Intent pass runs stay readable and keep producing reuse_shadow observations.

Scope: build/reuse_shadow.go and build/reuse_shadow_test.go. Out of scope: changing verification results or skipping checks. Existing test receipts in build/reuse_shadow_test.go that omit required fields are updated to be complete.

## Decisions
- A receipt line is complete only when all six fields key, name, status, wall_ms, source and log are present with a nonempty key, name, source and log; status and wall_ms may be zero, and any other line is corrupt with the existing corrupt-line error. (evidence: build/reuse_shadow.go; build/reuse_shadow_test.go) (assumption: the request does not list the required fields; they are taken from the reuseReceipt struct and its writer) (source: request)
- A Build that meets an incomplete receipt records one reuse_shadow_unavailable event and still runs every baseline check normally; verification results and checks are not changed. (evidence: build/baseline.go; build/reuse_shadow.go) (source: request)
- A receipt line is complete only when it is a JSON object carrying all six fields key, name, status, wall_ms, source and log, with a nonempty key, name, source and log; status and wall_ms may be zero. Any other line is corrupt. (decision: D1, round 1) (constraint: the corrupt-line error text stays 'receipt file <path> is corrupt at line <n>') (evidence: build/reuse_shadow.go; build/reuse_shadow_test.go) (assumption: The request does not list the required fields; this is derived from the reuseReceipt struct and its writer.) (source: request)
- No other behavior changes: the reuse_shadow_unavailable event is recorded once per run log, and every baseline check still runs. (decision: D2, round 1) (evidence: build/baseline.go; build/reuse_shadow.go) (source: request)

## Source comparison
- Incomplete key-and-status-zero line rejected with the corrupt-line error: addressed in scenario 1 and the Go acceptance subtest r1.
- Build meeting that receipt records one reuse_shadow_unavailable event and runs every baseline check: addressed in scenario 2 (subtest r2).
- Complete baseline and pass receipts stay readable and keep producing observations: addressed in scenario 3 (subtest r3).
- Out of scope (verification results, skipping checks): unchanged.

## Tests
- an incomplete receipt line is corrupt: TestReceiptShapeAcceptance/r1 and TestReadReuseReceiptsRejectsIncompleteRecords (unit)
- a Build meeting an incomplete receipt records one reuse_shadow_unavailable event and runs every baseline check: TestReceiptShapeAcceptance/r2 (integration)
- complete baseline and pass receipts remain readable and produce reuse_shadow observations: TestReceiptShapeAcceptance/r3 (integration)
- the receipt-file bound test uses complete receipts: TestReuseReceiptFileIsBounded (existing)

## Feasibility
- none
