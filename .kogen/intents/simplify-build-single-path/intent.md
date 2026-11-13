# Remove legacy wording from Build recovery refusal

Slug: simplify-build-single-path
Model: not applicable — the change renames one refusal message and deletes no state
Acceptance:
- .kogen/intents/simplify-build-single-path/acceptance.feature
- features/steps/simplify-build-single-path__steps.js

The Build already has one path: the dormant Build variants, the old-checkpoint decoder and the config ghost normalization are gone from build/. What remains is the recovery refusal of a checkpoint whose version is not 4, which still calls it a "legacy checkpoint version". Such a checkpoint is refused with `unknown checkpoint: incompatible checkpoint version <v>` (the wording resume already uses), still with kind `recovery_unsupported`, and no decoding path exists. No option is added. Existing tests expecting the old wording (build/recover_test.go) are updated; version-3 refusal stays tested for both resume and recovery. The full go test suite remains the project's go-test check.

## Decisions
- Recovery refuses a non-4 checkpoint version with `unknown checkpoint: incompatible checkpoint version <v>`, keeping kind `recovery_unsupported`. (evidence: build/recover.go; build/resume.go; build/recover_test.go) (decision: D1, round 1) (source: request)
- The `shellAfter` seam is retained: it is an unexported test clock for the shell wait, not a production mode, option or checkpoint field. (evidence: build/hooks.go) (decision: D2, round 1) (source: request)

## Source comparison
- Dormant BuildRequestBudget, ReadLedger and brief caps, legacy.go and ghost normalization: already absent from build/ at HEAD; acceptance keeps them absent. (evidence: build/hooks.go; build/resume.go; build/state.go)
- Version-3 refusal with no decoding path: already refused by decodeCheckpoint; recovery wording is made consistent.
- Full go test suite and unchanged source custody, approval and commit behavior: existing tests.

## Tests
- Version 2 and 3 checkpoints are refused by recovery with the incompatible wording: update the cases in build/recover_test.go (unit)
- Removed names and legacy wording are absent from Go files: scenario in acceptance.feature (scenario)
- Build, resume and recovery behavior is unchanged: existing build package tests (existing)

## Feasibility
- none
