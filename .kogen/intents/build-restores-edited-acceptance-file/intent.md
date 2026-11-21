# Restore an acceptance file the Build edited

Slug: build-restores-edited-acceptance-file
Model: not applicable — the change is a guard that compares bytes after a shell call, with one restore per attempt and no further state worth modelling
Acceptance:
- .kogen/intents/build-restores-edited-acceptance-file/acceptance.feature
- features/steps/build-restores-edited-acceptance-file__steps.js
- features/steps/build-restores-edited-acceptance-file__driver/fixture_test.go.txt

A Build that edits a protected acceptance file in a shell call today ends the whole run as a baseline change refusal and everything it did is discarded, although restoring the approved bytes would leave the acceptance exactly as approved. This Intent makes the baseline guard in `build/build.go` forgive the first such edit of an attempt. The first time in an attempt that the guard finds, after a shell call, a changed approved acceptance file or an added file in the approved baseline, it restores every changed file to the approved bytes, removes every added file, and returns to the Build a plain message naming the restored and removed paths and saying that acceptance files cannot be edited. The Build continues, and after the restore the baseline equals the approved one byte for byte, so the checks, the review and the commit see the approved acceptance. The second time in the same attempt it fails exactly as today, with the same `candidate/baseline_changed` refusal text (`the Build changed acceptance files: <paths>; nothing was committed; ...`) and nothing is committed. Scope: `build/build.go` (the guard and its call after a shell call) and the tests in `build/blocker_test.go` and `build/commit_test.go`. Refine, Shape, the shape package, the executor, docs, README and every other package are out of scope. Existing tests that expect a refusal from a first shell edit are updated to the new behavior; the refusal stays for the second edit and for every other call site of the guard (start, fixer, pass, review, commit, hooks). Nothing else is replaced or removed, and there is no setting. The acceptance fixture tests are named `TestAcceptance...`; the Build's own tests must not use that prefix.

## Decisions
- The first shell call of an attempt that changes or adds an acceptance file is restored and the Build continues; the second such shell call in the same attempt fails with the existing refusal. The count is per attempt and is the Build's own state, not a setting. (source: request)
- A changed file is restored to its approved bytes, an added file in the baseline set (a new file in the Intent's folder or matching `protect`) is removed, and a changed file the Build deleted is restored. The model-facing message is plain text that names every restored and removed path and states that acceptance files cannot be edited. (assumption: the user accepts that the exact wording is the Build's, with the paths and the phrase `acceptance files cannot be edited` required) (source: request)
- The restore applies to the guard's call after a shell call only. The guard's other calls (start, hooks, fixers, passes, review, commit) keep failing on the first difference as today. (evidence: build/build.go; build/executor_native.go; build/commit.go) (source: request)
- The second refusal keeps the existing text and the existing error class and reason (`candidate/baseline_changed`, exit 1) and commits nothing. (evidence: build/build.go; build/commit_test.go) (source: request)

## Source comparison
- Addressed: first shell edit of a changed file is restored to approved bytes, the Build continues and is shown a message naming the path (scenario 1).
- Addressed: an added file in the Intent folder is removed and named, the attempt continues (scenario 2).
- Addressed: a second edit in the same attempt fails with `the Build changed acceptance files` naming the path, and nothing is committed (scenario 3).
- Omitted by design: Refine, Shape, executor, docs and README, which the request puts out of scope.

## Tests
- The first shell edit of an approved acceptance file is restored and the Build is told: a focused Go test in `build/blocker_test.go` using the scripted provider, asserting exit 0, the bytes and the shown message (integration)
- An added file in the Intent folder is removed and named: a focused Go test in `build/blocker_test.go` (integration)
- A second shell edit in the same attempt is refused with the existing text and nothing is committed: a focused Go test in `build/commit_test.go`, and the existing refusal tests updated where they edit once by shell (integration)
- Existing hook, fixer and commit-time refusals are unchanged: the existing tests `TestAHookThatChangesTheApprovedBaselineStopsTheBuild` and the baseline_changed tests (existing)

## Feasibility
- none
