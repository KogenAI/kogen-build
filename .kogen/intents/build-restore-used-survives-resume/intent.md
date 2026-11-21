# Keep the restore-used flag through Build resume

Slug: build-restore-used-survives-resume
Model: not applicable — the change persists one boolean in the existing checkpoint state; no interacting state machine is added
Acceptance:
- .kogen/intents/build-restore-used-survives-resume/acceptance.feature
- features/steps/build-restore-used-survives-resume__steps.js
- features/steps/build-restore-used-survives-resume__driver/restore_used.go.txt

A resumed Build must remember that its attempt already used its one baseline restore, so a protected file is never restored a second time. The attempt record stores whether the restore was used as a JSON boolean under the exact key `restore_used`: written as true after the restore is used and as false otherwise. A resumed attempt reads it back, so BaselineRestored on the resumed attempt behaves as on the original: a second protected-file edit is refused with the existing refusal text and the file is not restored again. A record without the key resumes as not used, with no error. Resuming again and saving again keeps the value.

Scope: build/build.go, build/resume.go, build/records.go, the declaration of the record's state type in build/core_state.go (only to add the one field named above), and new model-free test files in build/. Out of scope: every other file, all docs, which files are protected, the approved-digest rule, the restore message text, and any Build phase other than resume.

## Decisions
- The attempt record stores `restore_used` as a JSON boolean, written true after the restore is used and false otherwise; a resumed attempt reads it back. (evidence: build/build.go; build/core_state.go; build/resume.go) (source: request)
- A record without `restore_used` resumes as not used with no error. (evidence: build/state.go) (source: request)
- The Build adds the model-free tests TestResumeKeepsRestoreUsed (integration), TestResumeUnusedRestoreStillAvailable (integration), TestResumeOldRecordWithoutRestoreField (unit) and TestRestoreUsedSurvivesTwoResumes (integration) in new files in build/; the acceptance runs its own driver tests by name, because the requested tests do not exist before the Build. (evidence: build/resume_test.go; build/helper_test.go) (source: request)
- Persist restore_used as State.RestoreUsed (json restore_used, always written), replacing the in-memory restored flag (decision: D1, round 1) (constraint: absent key resumes as not used without error) (evidence: build/build.go; build/core_state.go; build/resume.go; build/state.go) (assumption: Acceptance driver tests were not compiled or run by the shaper; state goldens must be regenerated with -update.) (source: request)
- Resume restores the used flag from the record key restore_used; a record without it resumes as not used (decision: D2, round 1) (evidence: build/state.go; build/resume.go) (source: request)
- Build adds the four named model-free tests; acceptance runs its own driver tests by name (decision: D3, round 1) (evidence: build/resume_test.go; build/helper_test.go) (source: request)

## Source comparison
- addressed: restore flag lost on resume (build/build.go restored flag, build/resume.go); source request; acceptance.feature scenarios 1 and 4.
- addressed: unused restore stays available after resume; source request; acceptance.feature scenario 2.
- addressed: record without `restore_used` resumes without error; source request; acceptance.feature scenario 3.
- omitted: every file outside the stated scope, all docs, protected-file set, approved-digest rule, restore message text, other phases; source request (out of scope).

## Tests
- A used restore survives a resume and the second edit is refused: TestResumeKeepsRestoreUsed (integration)
- An unused restore is still available after a resume: TestResumeUnusedRestoreStillAvailable (integration)
- A record without restore_used resumes with the restore available: TestResumeOldRecordWithoutRestoreField (unit)
- The used value survives two resumes: TestRestoreUsedSurvivesTwoResumes (integration)

## Feasibility
- none
