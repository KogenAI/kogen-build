# Reject relative recovery archive paths

Slug: reject-relative-recovery-output
Model: not applicable — the change is a stateless validation of one argument
Acceptance:
- .kogen/intents/reject-relative-recovery-output/acceptance.feature
- features/steps/reject-relative-recovery-output__steps.js
- features/steps/reject-relative-recovery-output__driver/acceptance_test.go.txt

`RecoverInput.Output` is documented as an absolute directory, but `Recover` only cleans the supplied path, so a relative output creates the recovery archive under the process working directory. After this change `Recover` refuses a relative output path before it creates any file or changes the checkout, and the refusal names the rejected path exactly as supplied. Valid absolute output paths outside the checkout recover exactly as before. The change is limited to `build/recover.go` and `build/recover_test.go`; CLI parsing, archive contents and cleanup behavior are out of scope and unchanged.

## Decisions
- A relative `RecoverInput.Output` (one for which `filepath.IsAbs` is false) is refused with the existing recovery refusal (`recoveryRefused`, category `candidate`, kind `recovery_refused`, exit 1); the message names the output path as supplied. (evidence: build/recover.go; build/recover_test.go) (source: request)
- The check runs at the start of `Recover`, before the repository is opened or the Build lock is taken, so a refusal leaves the checkout, the Build's run files and the process working directory untouched. (evidence: build/recover.go) (assumption: exact placement of the check inside Recover; only the observable outcome of nothing being created or changed is required) (source: request)
- Absolute outputs keep their current handling, including the existing refusals for an existing destination, an output inside the checkout and an unsafe parent. (evidence: build/recover.go; build/recover_test.go) (source: request)
- No new flag, setting or CLI change; the exact wording of the refusal beyond naming the path is not fixed. (evidence: build/recover.go) (source: request)

## Source comparison
- Refuse a relative output before creating files and name the path: addressed by scenario "A relative output path is refused before anything is written".
- Preserve recovery for a valid absolute path outside the checkout: addressed by scenario "An absolute output path outside the checkout still recovers".
- CLI parsing, archive contents, cleanup behavior: out of scope, untouched.

## Tests
- a relative output is refused, names the path, and creates no archive and changes nothing: add a test in build/recover_test.go that runs Recover from a temporary working directory against a blocked Build (regression)
- an absolute output outside the checkout still completes recovery: the existing recovery tests in build/recover_test.go, plus a scenario running recovery with an absolute output (existing)
- end to end through Recover: scenarios copying a driver test into the build package and running it (scenario)

## Feasibility
- none
