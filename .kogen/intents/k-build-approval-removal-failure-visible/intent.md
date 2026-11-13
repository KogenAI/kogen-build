# Report a failed removal of the spent approval file

Slug: k-build-approval-removal-failure-visible
Model: not applicable — one error result is surfaced after the commit; no state machine changes
Acceptance:
- .kogen/intents/k-build-approval-removal-failure-visible/acceptance.feature
- features/steps/k-build-approval-removal-failure-visible__steps.js

When a Build lands its commit it removes the intent's approval file (`project.ApprovalPath`). Today the result of that removal is discarded in `build/commit.go`. After the commit is recorded, a removal that fails for any reason other than the file being already absent must not undo or fail the landing (exit status unchanged, outcome still landed) but is recorded and shown as a named note. A successful removal, and an already absent file, behave exactly as today with no note.

The note: the final report on stdout gets one extra line `  note: could not remove the approval file <path>: <error>` (placed with the other review note lines, before the `repairs:` line), where `<path>` is the approval path and `<error>` the operating system error text (for example `permission denied`). The run record (`log.jsonl`) gets one event `{"event":"approval_removal_failed","path":<path>,"error":<error text>}` written once, right after the landing is checkpointed. Resuming an already landed Build only prints its existing "already landed" line: it removes nothing, adds no event and prints no note, so the note appears once.

## Decisions
- The removal failure is a note, not an error: the landing, exit status and outcome are unchanged. (source: request)
- An absent approval file (`os.IsNotExist`) is not a failure. (source: request)
- The note format and the `approval_removal_failed` event name are fixed as described above. (assumption: the user accepts these exact wording and event names) (source: request)
- Acceptance copies a Go test, kept as data in the intent folder, into a scratch copy of the checkout's build package and runs it, because the landing needs the package's fake provider fixtures. (evidence: build/commit.go; build/helper_test.go; build/build_test.go) (source: request)

## Source comparison
- Addressed: normal removal reports no note (subtest normal).
- Addressed: already-gone approval reports no note (subtest already-gone, the approval removed by a pre-commit hook).
- Addressed: unremovable path (parent folder made read-only) still lands, one note with path and error in the report and run record (subtest unremovable).
- Addressed: the note appears once also on resume after landing (subtest unremovable resumes the landed Build).

## Tests
- Failed removal is a note in report and run record, landing unchanged, once also on resume: acceptance scenario running the Go subtests normal, already-gone and unremovable (scenario)
- Existing landing test keeps passing with no note: TestBuildLandsOneCommitWithTheIntentTrailer (existing)

## Feasibility
- none
