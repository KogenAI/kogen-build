# Require every Kogen component at its current main

Slug: k-build-loop-module-path
Model: not applicable — a dependency-pin change with no state or interacting rules of its own
Acceptance:
- .kogen/intents/k-build-loop-module-path/acceptance.feature
- features/steps/k-build-loop-module-path__steps.js

kogen-build requires kogen-loop only by its real module path, `github.com/KogenAI/kogen-loop`. Every Kogen component it pins moves to its current main, so no dependency in the build list uses the bare `kogen-loop` path. kogen-build's own behaviour does not change.

What already holds at HEAD b0e41ef: commit 612cb24 ("Accept the canonical kogen-loop module path") already dropped `replace kogen-loop => …` and the `kogen-loop v0.0.0` requirement. It also rewrote every Go import to `github.com/KogenAI/kogen-loop/...`. Today `grep -rn "\"kogen-loop/" --include=*.go .` exits 1 with no output. That state must stay as it is. What is still missing is the move of every pinned component to its current main, followed by `go mod tidy`.

Current mains, observed with `git ls-remote https://github.com/KogenAI/<component>.git refs/heads/main` on 2026-11-05 and resolved through proxy.golang.org:

| Component | main SHA | pseudo-version |
| --- | --- | --- |
| kogen-loop | `7e80986bcd043c27cafd1ce9e4e69aa9bf69e4fa` | `v0.0.0-20261104223449-7e80986bcd04` |
| kogen-coordinator | `4116df40cbf151b130db01fa9a15082711208528` | `v0.0.0-20261105001101-4116df40cbf1` |
| kogen-executor | `8a4fecd1ce0a8e9a2bfcb9e251086870980abc01` | `v0.0.0-20261104160936-8a4fecd1ce0a` |
| kogen-review | `7f6ef7d1c0a31eb90a3b7cbfa09e80fa92b91170` | `v0.0.0-20261104231132-7f6ef7d1c0a3` |
| kogen-verification | `74798e7aa09ebe72f12510f152db75dd618b023b` | `v0.0.0-20261104204404-74798e7aa09e` |

Each of these mains requires `github.com/KogenAI/kogen-loop` and none requires the bare `kogen-loop`. This includes kogen-review 7f6ef7d, so the request's "report review if still bare" fallback is not needed. The only bare `kogen-loop/tools` import in any of their sources is kogen-loop's `features/steps/read_file_binary_notice_driver.go`. That file is a `//go:build ignore` fixture and no package of the build list contains it.

Required end state:

1. Run exactly this, then `go mod tidy`:

   ```sh
   go get github.com/KogenAI/kogen-loop@7e80986bcd043c27cafd1ce9e4e69aa9bf69e4fa github.com/KogenAI/kogen-coordinator@4116df40cbf151b130db01fa9a15082711208528 github.com/KogenAI/kogen-executor@8a4fecd1ce0a8e9a2bfcb9e251086870980abc01 github.com/KogenAI/kogen-review@7f6ef7d1c0a31eb90a3b7cbfa09e80fa92b91170 github.com/KogenAI/kogen-verification@74798e7aa09ebe72f12510f152db75dd618b023b
   go mod tidy
   ```

   go.mod is then exactly:

   ```
   module github.com/KogenAI/kogen-build

   go 1.27.2

   require (
   	github.com/KogenAI/kogen-coordinator v0.0.0-20261105001101-4116df40cbf1
   	github.com/KogenAI/kogen-executor v0.0.0-20261104160936-8a4fecd1ce0a
   	github.com/KogenAI/kogen-loop v0.0.0-20261104223449-7e80986bcd04
   	github.com/KogenAI/kogen-review v0.0.0-20261104231132-7f6ef7d1c0a3
   	github.com/KogenAI/kogen-verification v0.0.0-20261104204404-74798e7aa09e
   )
   ```

   go.mod has no `replace` directive, no `kogen-loop` requirement and no `// indirect` requirement. go.sum is the matching tidy result, so `go mod tidy -diff` reports nothing. There is no go.work. `go list -m all` lists exactly the main module and these five versions.
2. No Go file imports the bare path: `grep -rn "\"kogen-loop/" --include=*.go .` finds nothing. `go list -deps -test ./...` lists no package `kogen-loop` or `kogen-loop/...`, and it does list `github.com/KogenAI/kogen-loop/sys`. Identifiers, strings and comments that only name kogen-loop stay as they are, for example `nativeVersion = "kogen-loop " + api.Library`.
3. kogen-build's own non-test Go code does not change: no logic, prompt, tool, record or output change. The configured checks pass: `test -z "$(gofmt -l .)"`, `go build ./...`, `go vet ./...` and `GOFLAGS=-trimpath go test ./...`.
4. The new producer mains carry their own behaviour changes, and some existing tests see them. The ones observed are listed under Source comparison. Most come from kogen-review f4b7f54, "Deliver review source without repetition". Update a test, golden or fixture only where its failure comes from a producer's changed behaviour at the new pins, and update it to what that producer now does. Keep every other assertion and the purpose each test states. Do not skip, delete or loosen tests to get a pass.
   - The consumes suites state what the producers now provide.
   - A fixture that was sized to make the review deliver its source in pinned parts, or to force a working-window reset, must still produce those parts or that reset with the new, shorter source. Resize the fixture; do not drop the assertion.
   - Regenerate goldens with `go test ./... -update` and review the diff. Frozen goldens such as `context-external-golden.json` and `reviewretry/*.wire.json` change only in producer-derived bytes.
   - If a failure comes from kogen-build's own code rather than a producer change, it is outside this Intent: report it as a blocker. Do not change product code to fix it.

## Decisions
- kogen-build requires `github.com/KogenAI/kogen-loop` at kogen-loop's current main through `go get`, with a pseudo-version require plus go.sum, no replace and no go.work (source: request)
- kogen-coordinator, kogen-executor, kogen-review and kogen-verification move to their current mains in the same change, followed by `go mod tidy` (source: request)
- "Current main" is each component's main as observed at shaping, 2026-11-05, pinned by the SHAs above (source: request)
- No behaviour change in kogen-build itself (source: request)

## Source comparison
- Addressed (already true at HEAD and kept): "Drop the `replace kogen-loop => ...` line and the `kogen-loop v0.0.0` requirement" and "Update every import from `kogen-loop/...`". Both were done by 612cb24. Scenarios 1 and 2 assert them: no replace, no `kogen-loop` requirement, the request's exact grep, and the package listing.
- Addressed: "Require `github.com/KogenAI/kogen-loop` at current kogen-loop main with `go get …@<kogen-loop main sha>`". Main is 7e80986, pinned at `v0.0.0-20261104223449-7e80986bcd04` (rule 1, scenario 1).
- Addressed: Order, "move each of them to its current main with `go get github.com/KogenAI/<component>@<its main sha>` … Then `go mod tidy`". The components are coordinator 4116df4, executor 8a4fecd, review 7f6ef7d and verification 74798e7. kogen-build itself is the module being changed, not a dependency. Scenario 1 checks the exact requirements. Scenario 2 checks `go mod tidy -diff` and the exact `go list -m all` build list, so no module of the build list is the bare `kogen-loop`.
- Addressed: Retry, "Use the newest coordinator, executor and review mains; if review is still bare when you build, report that one dependency only". Review main 7f6ef7d's go.mod requires only canonical modules, and none of the five mains imports the bare path in a buildable file. The fallback therefore does not apply. Mains that move after shaping are out of scope for this Intent.
- Addressed: Acceptance, "`go build ./... && go vet ./... && go test ./...` pass". These are the configured checks compile, vet and go-test (rule 3). In a scratch copy with rule 1 applied, `go build ./...` and `go vet ./...` passed.
- Inherited producer behaviour, observed in that scratch copy with `GOFLAGS=-trimpath go test` without `-race`. Every top-level test of package `build` ran, the last 42 of them in a second run. Each test listed below passes on unchanged HEAD:
  - consumes/kogen-review: TestMaterialSetIsMadeByTheReviewFromItsInput (`MaterialSet.Source lacks "--- app.txt\ncandidate\n"`; `Files` entries now carry `"origin"` and `"rename"`) and TestExternalReviewApprovesInItsOwnSession (`brief dynamic lacks "--- app.txt\ncandidate\n"`).
  - build: TestTheReviewerReadsTheFullSourceAndNewFilesAppearOnce, TestTheReviewerApprovesUnchangedCodeWithoutRepairs (reviewer prompt hash now `345eb028…`), TestKReviewVerdictStrandReset, TestPartsSourceOmitsChangedFileTexts, TestReviewInPartsOmitsAChangedFilesFullText, TestAReviewWhosePartsAndScreenshotsCannotShareAWindowConvergesOnItsDeliveredParts, TestTheMaterialEqualsTheElixirReference, TestTheAuthorityReadmeIsTheStartHeadsAndACheckoutEditIsOnlyCandidateData (subtest a_checkout_edit_is_in_the_source_only), TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation, TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval (`0 pinned parts`), TestKReviewRetryUnchangedValidRequests (wire request bytes), TestContextExternalNativeBytes (`reviewer.dynamic differs at byte 301`) and TestKReviewRetrySeparateAllowances (`pinned parts = 119, want 232`).
  - All other packages passed: consumes/kogen-coordinator, kogen-executor, kogen-loop and kogen-verification, and contract. Rule 4 decides how each failure is handled.
- Omitted: earlier Intents' acceptance helpers and records that name the bare path or older pins, for example `.kogen/intents/k-build-accept-canonical-loop-path/` and `features/steps/k-consumes-build__*.js`. They are historical acceptance of finished Intents and stay unchanged. kogen-loop's ignored driver fixture lives outside this repository.

## Tests
- go.mod requires every Kogen component at its current main, has no replace and no `kogen-loop` requirement: acceptance scenario reading `go mod edit -json` (integration)
- No Go file, package or build-list module uses the bare kogen-loop path, and go.mod and go.sum are tidy: acceptance scenario running the request's grep, `go list -deps -test ./...`, `go mod tidy -diff` and `go list -m all` (integration)
- kogen-build's own behaviour is unchanged against the new producer mains: the configured checks (gofmt, `go build ./...`, `go vet ./...`, `GOFLAGS=-trimpath go test ./...`), with consumes suites, goldens and fixtures updated only for producer behaviour at the new pins (existing)

## Feasibility
- none
