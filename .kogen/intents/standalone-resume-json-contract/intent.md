# Cover standalone resume JSON refusal

Slug: standalone-resume-json-contract
Model: not applicable — the change adds a test and has no state of its own
Acceptance:
- .kogen/intents/standalone-resume-json-contract/acceptance.feature
- features/steps/standalone-resume-json-contract__steps.js

Add one model-free integration test to `build/standalone_test.go` that builds the standalone executable (`go build -o <tmp>/kogen-build ../cmd/kogen-build`, as the existing test does) and runs `resume --project <temporary Git checkout> --id 0a1b2c3d`, a validly shaped ID (8 lowercase hexadecimal characters) with no recorded Build. It asserts: the process exit status is 1; stdout decodes as exactly one JSON object (a second value is a failure) whose `exit_code` is 1 and whose `error` is an object with category `candidate`, kind `resume_unsupported` and message `Build 0a1b2c3d: no Build with this id in this checkout; kogen intent list lists Build attempts`; no run exists under `.kogen/local/runs` afterwards; and `git status --porcelain` of the checkout is empty. Production behavior, resume checkpoint semantics and provider calls are out of scope and unchanged; only `build/standalone_test.go` is edited.

## Decisions
- Add one test that runs the built executable with resume and an unrecorded valid ID and asserts exit 1, one JSON object, structured error, no run and a clean checkout. (evidence: cmd/kogen-build/main.go; build/resume.go; build/standalone_test.go) (assumption: a scratch run of the built executable printed exit 1 and category candidate, kind resume_unsupported; the owner confirms these are the intended contract) (source: request)
- The test goes through the built executable, not a direct call of the build package's Resume. (source: request)
- The refusal exit status, category, kind and message are those the executable emits today for an unrecorded valid ID. (evidence: cmd/kogen-build/main.go; build/resume.go) (assumption: the owner confirms these are the intended contract) (source: request)
- Stdout is exactly one JSON object; nothing else follows it. (source: request)
- The refused resume creates no run and leaves the checkout unchanged. (source: request)
- The test is named `TestStandalone*Resume*` (its name contains `Resume`) or otherwise runs the executable's `"resume"` command in its body. Acceptance finds it at run time as the top-level test(s) of build/standalone_test.go whose name contains `Resume` (any case) or whose body contains the literal `"resume"`, and does not compare with Git history: Kogen's verification checkout commits the Build's tree as HEAD, so a diff against HEAD would find no added test there. It checks the test by running the file's tests on mutated scratch copies of cmd/kogen-build/main.go (a second JSON value, a created run, a null error): a resume test must fail on each. (assumption: this is the Shaper's way to verify the test; the existing two tests do not mention resume, so on the unchanged tree none is found) (source: request)

## Source comparison
- Request: standalone resume refusal case, one JSON object, exit code and structured error identifying the missing Build, no run created, checkout unchanged; addressed by the new test in build/standalone_test.go and acceptance.feature. Out of scope: production behavior, checkpoint semantics, provider calls.
- Correction (builder-reported fixture contradiction, independently confirmed from the step source): the step found the added test by diffing against `git show HEAD:build/standalone_test.go`, which is empty in verification because HEAD already holds the Build's tree; superseded by identifying the test by its Resume name or `"resume"` body. The test contract, exit status, error fields, mutations and scope are unchanged.

## Tests
- a resume of an unrecorded Build through the standalone executable is refused with exit 1 and one structured JSON object, creating no run: new test in build/standalone_test.go (integration)
- the new test rejects a second JSON value, a created run and a missing structured error: acceptance mutations of cmd/kogen-build/main.go in a scratch copy (scenario)

## Feasibility
- none
