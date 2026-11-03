# Add a standalone Build command

Slug: k-build-cmd-entry
Model: not applicable — this is wiring to the existing Build state machine, not a new workflow
Acceptance:
- .kogen/intents/k-build-cmd-entry/acceptance.feature
- features/steps/k_build_cmd_entry_steps.js

Add the developer entry point cmd/kogen-build, compiled using this module with `go build -o <binary> ./cmd/kogen-build`. It must run without an assembled kogen executable or a new dependency on github.com/KogenAI/kogen. Existing dependencies, including kogen-loop, remain permitted. Nothing replaces or changes Kogen's user-facing CLI or the phase's existing approval, execution, verification, review, commit, interruption, resume or recovery policy. Do not raise any timeout.

## Decisions
- Provide `kogen-build build --project <checkout> --slug <slug>` and `kogen-build resume --project <checkout> --id <build-id>`, delegating to build.Build and build.Resume respectively. Use the existing provider credentials/configuration and tool discovery; do not require Kogen's binary. Diagnostics and existing human progress go to stderr; stdout contains exactly one JSON result. (source: request)
- Use a typed Go result with JSON fields `exit_code` (integer), `branch` (string), `tree` (string), `checks` (array), and `error` (null or an object with string `category`, `kind`, `message`). Branch is the checkout's current symbolic branch, or empty for detached HEAD; tree is the candidate Git tree identity, not a commit hash. For a pre-run refusal it is the checkout tree. Checks are the latest actual verification pass's ordered check records (preserve their existing verify JSON shape), or [] when no pass ran. Never fabricate passing checks. For post-start outcomes use the actual invocation's candidate and verification evidence, not an arbitrary latest directory or stale unrelated run. Exit code equals the phase's existing exit code. Surface structured sys.Error fields without parsing human stdout. (source: request)
- Report results on success and phase refusal/stops alike. Preserve the ordinary phase exit and effects. No evaluator-only bypass of approval or independent review, synthetic successful mode, extra timeout or provider dependency is allowed. Document usage and result schema in README.md. (source: request)

## Source comparison
- addressed: K-BUILD-CMD-ENTRY (3 Nov 2026) requests independent execution and scoring in repository kogen-build; Decisions define cmd/kogen-build and its typed candidate/check result, Tests require successful and stopped real phase wiring.
- addressed: given checkout and this repository's own go.mod; command arguments and build command above retain those boundaries. Existing build.Build/build.Resume remain the authority for phase behavior.
- addressed: no change to Kogen CLI, no timeout raised, no new dependency on kogen; outcome prose and Decisions preserve all three constraints. Existing kogen-loop module is not removed by interpreting the dependency constraint retroactively.
- omitted: no requested behavior. The filing metadata is provenance, not a new CLI surface. Recovery is unchanged and does not gain another command.

## Tests
- A standalone binary builds against this module and delegates a real pre-run Build refusal while reporting branch/tree/empty checks and the existing exit: acceptance refusal scenario (scenario)
- The binary connects a successful approved Build through verification, independent review and commit and reports its exact candidate and nonempty ordered checks: add a local scripted-provider integration test using existing build fixtures, comparing CLI JSON to actual run evidence and Git tree, without calling kogen (integration)
- Resume delegates the existing entry point and reports the existing unsupported-checkpoint refusal: acceptance resume scenario (scenario)
- Stopped post-start Builds report their actual candidate/checks rather than unrelated run evidence: add a local integration case using existing stopped-build fixtures and a decoy older run (integration)
- Existing phase behavior, timeout policy and user-facing CLI remain unchanged: retain existing phase tests and configured checks (existing)

## Feasibility
- Repository inspection found exported Build/Resume and Deps, existing provider/session wiring in build/helper_test.go, and structured checkpoint/pass evidence. A local probe confirmed cmd/kogen-build is absent (`stat .../cmd/kogen-build: directory not found`) and Git 2.43.0 is available. No external protocol changes are involved.
