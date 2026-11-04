# Accept the canonical kogen-loop module path

Slug: k-build-accept-canonical-loop-path
Model: not applicable — a module-path and dependency-pin change with no state or interacting rules of its own
Acceptance:
- .kogen/intents/k-build-accept-canonical-loop-path/acceptance.feature
- features/steps/k-build-accept-canonical-loop-path__steps.js

This is the expand step of an expand-then-contract move. kogen-build stops using the bare `kogen-loop` module, which go.mod today maps with `replace kogen-loop => github.com/KogenAI/kogen-loop v0.0.0-20261103193244-a5aa5f4ee775`. It then requires and imports kogen-loop only by its canonical path, `github.com/KogenAI/kogen-loop`. That lets kogen-build be assembled with kogen-verification 7ecd05a (`v0.0.0-20261104143230-7ecd05abea32`), which imports the canonical path and carries the approval-timeout fix. Removing the bare path from other repositories is a later contract step and is out of scope.

Today, with go.mod requiring kogen-verification 7ecd05a (which brings `github.com/KogenAI/kogen-loop` 25d87c2 in), `go build ./...` fails:
`build/baseline.go:121:31: cannot use b.s (variable of type *"kogen-loop/sys".Session) as *"github.com/KogenAI/kogen-loop/sys".Session value in argument to verify.RunWatched`.
The same mismatch appears at build/build.go:187/308/375/382/385, build/builder.go:274/370 and build/checkreuse.go:313. These are this checkout's line numbers (HEAD f64d646); they differ from the request's b9990a3 numbers 186/307/374 and 255/345.

Required end state:

1. go.mod is exactly the `go mod tidy` result of this requirement set. It has no `kogen-loop v0.0.0` requirement, no `replace` directive and no `// indirect` requirement:

```
module github.com/KogenAI/kogen-build

go 1.27.2

require (
	github.com/KogenAI/kogen-coordinator v0.0.0-20261104083402-f4bd4aebd8c2
	github.com/KogenAI/kogen-executor v0.0.0-20261104084012-b2ffb64382e8
	github.com/KogenAI/kogen-loop v0.0.0-20261104063532-dea3eb375dba
	github.com/KogenAI/kogen-review v0.0.0-20261104090930-ba84b86318eb
	github.com/KogenAI/kogen-verification v0.0.0-20261104143230-7ecd05abea32
)
```

   go.sum is the matching `go mod tidy` result. One way to get there: rewrite the imports (rule 2), then run `go mod edit -droprequire=kogen-loop -dropreplace=kogen-loop -require=github.com/KogenAI/kogen-review@v0.0.0-20261104090930-ba84b86318eb -require=github.com/KogenAI/kogen-verification@v0.0.0-20261104143230-7ecd05abea32 && go mod tidy`. Minimal version selection then raises kogen-coordinator, kogen-executor and kogen-loop to the versions above.
2. Every Go import of `kogen-loop/<pkg>` in the module becomes `github.com/KogenAI/kogen-loop/<pkg>`. That covers `api`, `loop`, `provider`, `sys` and `tools` in `build`, `cmd/kogen-build`, `contract` and every `consumes/` suite, test files included. `gofmt -w .` then fixes import order. After the change, no Go file in the checkout imports the bare path. `go list -deps -test ./...` lists no package `kogen-loop` or `kogen-loop/...` and does list `github.com/KogenAI/kogen-loop/sys`. Identifiers and strings that only name kogen-loop stay as they are, for example `nativeVersion = "kogen-loop " + api.Library` and comments.
3. `consumes/README.md`'s table row for `` `consumes/kogen-loop/` `` names its producer module as exactly `` `github.com/KogenAI/kogen-loop` ``. This replaces `` `kogen-loop` (go.mod replaces it with `github.com/KogenAI/kogen-loop`) ``. Its Pins cell is unchanged. The inventory stays one row per use. The uses do not change, because only the import path moves.
4. kogen-build's own behaviour does not change. Its non-test Go code changes only its import paths and their gofmt order: no logic, prompt, tool, record or output change. The configured checks pass: `test -z "$(gofmt -l .)"`, `go build ./...`, `go vet ./...` and `GOFLAGS=-trimpath go test ./...`.
5. The canonical-path producer releases carry their own behaviour changes, and some existing tests see them (listed under Source comparison). Update a test, golden or fixture only where its failure comes from a producer's changed behaviour at the new pins. Update it to what that producer now does. Keep every other assertion, and keep the purpose each test states. Do not skip, delete or loosen tests to get a pass.
   - The consumes suites state what the producers now provide.
   - Regenerate goldens with `go test ./... -update` and review the diff. That includes frozen goldens such as `named-feedback-compat.json` and `context-external-golden.json`, which this Intent deliberately re-records, but only for the producer-derived bytes.
   - The legacy checkpoint tests keep proving that legacy (Elixir-format) checkpoints resume. Where they are refused only because the coordinator's protected baseline now also lists `.kogen/models.yaml`, adapt their fixture setup to the new producer baseline. Do not drop the resume assertions.
   - If a failure turns out to come from kogen-build's own code rather than a producer change, that is outside this Intent: report it as a blocker. Do not change product code to fix it.

## Decisions
- Expand step only: kogen-build moves to `github.com/KogenAI/kogen-loop`. Removing the bare path elsewhere is a later contract step (source: request)
- kogen-build requires kogen-verification 7ecd05a, `v0.0.0-20261104143230-7ecd05abea32`, so it can be assembled with the approval-timeout fix (source: request)
- Nothing else in kogen-build's own behaviour changes (source: request)
- No import of the bare `kogen-loop` path remains in kogen-build (source: request)

## Source comparison
- Addressed: "Reproduce first: … `go build ./...` in an assembly fails as quoted". Reproduced on HEAD f64d646 in a scratch copy: requiring kogen-verification `v0.0.0-20261104143230-7ecd05abea32` gives exactly the quoted `cannot use b.s (variable of type *"kogen-loop/sys".Session) as *"github.com/KogenAI/kogen-loop/sys".Session` errors, exit 1. Scenario "kogen-build compiles with kogen-verification 7ecd05a" runs that same build (a temporary copy of go.mod with that requirement, `go build -mod=mod -modfile=…`). It fails now and must pass after the change.
- Addressed: "`go build ./...` and `go test ./...` pass with github.com/KogenAI/kogen-loop required". Covered by rule 1 and its scenario, and by the configured checks compile, vet and go-test.
- Addressed: "no import of the bare kogen-loop path remains". Covered by rule 2 and the scenario "No package or Go file uses the bare kogen-loop path" (package listing including tests, plus a scan of every `.go` file).
- Conflicting and superseded, settled here: the request's reproduction pins `github.com/KogenAI/kogen-loop` at 25d87c2, but the end state requires `v0.0.0-20261104063532-dea3eb375dba` (dea3eb3, newer). Reason: kogen-build passes kogen-loop values (`*sys.Session`, provider clients, tools) to kogen-review, kogen-executor and kogen-coordinator, so all of them must also use the canonical path. A scratch build with only kogen-loop and kogen-verification moved failed: kogen-executor 09eabaf, kogen-review cfc2fb2 and kogen-coordinator ffca707 import the bare path.
  - Published go.mod files, checked through proxy.golang.org: the earliest canonical-path kogen-review found is ba84b86 (`v0.0.0-20261104090930-ba84b86318eb`). It requires kogen-loop dea3eb3, kogen-coordinator f4bd4ae, kogen-executor b2ffb64 and kogen-verification 7805cfe. The kogen-review versions inspected up to bfec92f still use the bare path.
  - The request's after-state names no kogen-loop version, only the canonical module, so the minimal lineup above satisfies it. It compiled, and `go vet ./...` passed, in a scratch copy with the imports rewritten.
- Superseded and removed: `require kogen-loop v0.0.0` and `replace kogen-loop => github.com/KogenAI/kogen-loop v0.0.0-20261103193244-a5aa5f4ee775` in go.mod, and the README row wording that described that replace.
- Inherited producer behaviour. These failures were observed in a scratch copy with the lineup above. The run was not exhaustive: the `build` package hit a 4m40s probe timeout while long Claude-lineup and review-retry tests were still running, so later tests were not observed.
  - consumes/kogen-coordinator: TestBaselineAndHash, TestLoadConfigReadsTheProjectConfiguration and TestWritePackageFacts fail with `environment/project_config_invalid` on their fixture project.yaml.
  - consumes/kogen-executor: TestCoreDisplay (`core.Display(3) = "3.0", want "3"`) and TestExecutorRoundResultFields (Usage JSON now has `"cache_write":null`).
  - consumes/kogen-loop: TestToolsCutBoundsTextWithAMarkerCountingTheOmission (`marker at []`).
  - build, state goldens: TestStateGolden*Checkpoint and TestStateGoldenReview*Record. Persisted config now has `"ProtectDiagnostics":null`; review records now have `"requirements":[]`.
  - build, other goldens: TestNamedFeedbackCompatibility (the protected-baseline list in the first request now includes `.kogen/models.yaml`) and TestTheReviewerApprovesUnchangedCodeWithoutRepairs (reviewer prompt hash changed).
  - build, legacy checkpoints: TestALegacyBuilderCheckpointResumesTheSameConversationAndLandsOnce, TestALegacyBuilderVerifyCheckpointRunsTheFixersAndThePassThenLands, TestALegacyReviewCheckpointContinuesTheSameReviewerConversation, TestALegacyReviewVerifyCheckpointJudgesTheSavedVerdictWithoutAskingAgain and TestALegacyRetryCheckpointSendsTheBlockerAndTheFreshResultsToTheSameReviewer. All are refused with `baseline/configuration changed`, because the coordinator's `project.Baseline` now lists `.kogen/models.yaml`. That listing is already in kogen-coordinator f15509472852, which kogen-verification 7ecd05a itself requires.
  - build: TestBuildRequestBudgetOff failed once (`log line 45 differs at byte 2434`) and passed on a focused rerun. Check it for a new volatile field before deciding.
  - Rule 5 decides how each of these is handled. Existing checkpoints written before these producer versions are subject to the producers' new baseline; kogen-build adds no compatibility logic for that.
- Omitted: earlier Intents' acceptance helpers that map `kogen-loop=kogen-loop` (for example features/steps/k-consumes-build__*.js). They are historical acceptance of finished Intents, not part of this module's build or suite, and stay unchanged. Also omitted: kogen-loop, kogen-verification and the kogen assembly, which live outside this repository.

## Tests
- go.mod requires kogen-loop only as github.com/KogenAI/kogen-loop, with the stated canonical-path lineup and no replace: acceptance scenario reading `go mod edit -json` (integration)
- No package or Go file of kogen-build, tests included, uses the bare kogen-loop path: acceptance scenario over `go list -deps -test ./...` and every `.go` file (integration)
- kogen-build compiles when kogen-verification 7ecd05a is required (the reported assembly failure): acceptance scenario building every package with a temporary go.mod requiring it (regression)
- consumes/README.md names the canonical producer module for the kogen-loop suite: acceptance scenario reading its table row (integration)
- kogen-build's own behaviour is unchanged and its suite passes against the canonical-path producers: the configured checks (gofmt, `go build ./...`, `go vet ./...`, `GOFLAGS=-trimpath go test ./...`), with consumes suites, goldens and fixtures updated only for producer behaviour at the new pins (existing)

## Feasibility
- none
