# Name the offending input in CLI usage refusals

Slug: cli-usage-refusal-names-input
Model: not applicable — the change only alters the text of a stateless argument-validation message
Acceptance:
- .kogen/intents/cli-usage-refusal-names-input/acceptance.feature
- features/steps/cli-usage-refusal-names-input__steps.js

When the standalone `kogen-build` program refuses its command line, the structured error message names the supplied mode, flag or extra positional argument that was rejected, instead of only listing the expected build and resume forms. The error keeps category `usage` and kind `invalid_arguments`; stdout stays one JSON object and the exit code stays 2. The change is limited to `cmd/kogen-build/main.go` plus a new `cmd/kogen-build/main_test.go`. Accepted flags, Build and Resume semantics, and provider or checkout behavior are unchanged.

## Decisions
- An unknown mode is refused with a message that contains the supplied mode text, followed by the existing expected-forms text; the mode is checked before flags are parsed. (evidence: cmd/kogen-build/main.go) (source: request)
- An unknown flag is refused with a message that contains the flag parser's own error text (which names the flag, as Go's flag package prints it with a single leading dash) followed by the expected-forms text. (evidence: cmd/kogen-build/main.go) (source: request)
- An extra positional argument is refused with a message that contains that argument, followed by the expected-forms text. (evidence: cmd/kogen-build/main.go) (source: request)
- A missing required slug (build) or id (resume) remains a usage refusal with category `usage`, kind `invalid_arguments`, exit code 2 and a JSON object on stdout; its message states which value is missing. (evidence: cmd/kogen-build/main.go) (source: request)
- The no-argument refusal ("expected build or resume") is unchanged because no offending input exists. (evidence: cmd/kogen-build/main.go) (source: request)
- Category `usage`, kind `invalid_arguments`, JSON stdout and exit code 2 are preserved for every usage refusal. (evidence: cmd/kogen-build/main.go; README.md) (source: request)

## Source comparison
- Offending mode in the message: addressed by scenario "An unknown mode is named in the refusal".
- Offending flag and extra positional argument in the message: addressed by the two scenarios for those inputs.
- Missing slug or id still a JSON usage refusal with exit code 2: addressed by the two missing-value scenarios.
- Build and Resume semantics, accepted flags, provider and checkout behavior: out of scope, untouched.

## Tests
- Unknown mode, unknown flag and extra positional argument are each named in the JSON error message with usage/invalid_arguments and exit code 2: scenarios compiling and running the kogen-build program (scenario)
- Missing slug for build and missing id for resume still give a JSON usage refusal with exit code 2: scenarios compiling and running the kogen-build program (scenario)
- The same refusals checked at the command boundary in the module's own tests: new cmd/kogen-build/main_test.go (integration)

## Feasibility
- none
