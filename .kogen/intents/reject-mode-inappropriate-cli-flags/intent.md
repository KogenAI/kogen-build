# Reject CLI flags that do not belong to the mode

Slug: reject-mode-inappropriate-cli-flags
Model: not applicable — the change only adds stateless argument validation
Acceptance:
- .kogen/intents/reject-mode-inappropriate-cli-flags/acceptance.feature
- features/steps/reject-mode-inappropriate-cli-flags__steps.js

The standalone `kogen-build` program currently registers both `--slug` and `--id` for both modes and silently ignores the one that does not belong: `build` ignores `--id` and `resume` ignores `--slug`. After this change, `build` given `--id` and `resume` given `--slug` are refused as usage errors whose message names the inappropriate flag. The documented forms (`build --project <checkout> --slug <slug>` and `resume --project <checkout> --id <build-id>`) keep being accepted. The change is limited to `cmd/kogen-build/main.go` and `cmd/kogen-build/main_test.go`; Build and Resume behavior and the documented command forms are unchanged.

## Decisions
- `build` with `--id` (even with an empty value) is refused through the existing usage refusal: category `usage`, kind `invalid_arguments`, exit code 2, one JSON object on stdout. The message names `--id` as not valid for build and does not name `--slug` before the expected-forms text, which is still appended. (evidence: cmd/kogen-build/main.go; README.md) (source: request)
- `resume` with `--slug` (even with an empty value) is refused the same way; the message names `--slug` as not valid for resume and does not name `--id` before the expected-forms text. (evidence: cmd/kogen-build/main.go) (source: request)
- The inappropriate-flag refusal is checked after flag parsing and the stray-argument check, and before the missing-required-flag check, so `build --id x` without a slug names `--id`. Presence is detected by whether the flag was supplied (flag.FlagSet.Visit), not by a non-empty value.  (evidence: cmd/kogen-build/main.go) (assumption: ordering of this refusal relative to the missing-flag refusal when both apply) (source: request)
- `build --slug <slug>` and `resume --id <build-id>` (each with optional `--project`) stay accepted: they pass argument validation and reach Build or Resume. (evidence: cmd/kogen-build/main.go) (source: request)

## Source comparison
- Refuse build with id, naming id: addressed by scenario "A build with --id is refused".
- Refuse resume with slug, naming slug: addressed by scenario "A resume with --slug is refused".
- Build with slug and resume with id remain accepted: addressed by the two accepted scenarios.
- Build and Resume behavior, documented command forms: out of scope, untouched.

## Tests
- build with `--id` is refused and the message names `--id`: add a case in cmd/kogen-build/main_test.go for parseArgs, and a scenario running the compiled program (unit)
- resume with `--slug` is refused and the message names `--slug`: add a case in cmd/kogen-build/main_test.go for parseArgs (unit)
- build with `--slug` and resume with `--id` remain accepted: extend TestParseArgsAccepts in cmd/kogen-build/main_test.go, and scenarios running the compiled program (unit)
- command-line behavior end to end: scenarios compiling and running the kogen-build program (scenario)

## Feasibility
- none
