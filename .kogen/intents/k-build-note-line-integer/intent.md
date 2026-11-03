# Preserve integer review note lines

Slug: k-build-note-line-integer
Model: not applicable — numeric decoding and display are stateless transformations; the Build lifecycle is unchanged
Acceptance:
- .kogen/intents/k-build-note-line-integer/acceptance.feature
- features/steps/k-build-note-line-integer__steps.js

Fix K-BUILD-NOTE-LINE-INTEGER on the consumer side. A JSON review note with line 1 must retain integer semantics and display as `1`, producing `note: app.txt:1: looks fine`, never `note: app.txt:1.0: looks fine`. Preserve integer semantics wherever kogen-build decodes review notes or other JSON values it subsequently displays. Use number-preserving decoding or typed integer fields, not a change to executor float display or a global truncation of floating-point values. Keep the current executor pin. Verify compatibility both with that pin and executor main commit 391e0b6 (or later); acceptance fixes the comparison revision to v0.0.0-20261103202226-391e0b6ecd4f for reproducibility.

The reported hooks_test.go:57 failure is reproduced with that revision. Inspection also found that the current `approve` test fixture constructs `float64(1)` directly rather than decoding JSON. Correct that fixture to represent a JSON integer through the consumer's real decoding path, rather than requiring Display to treat an actual float as an integer. Do not weaken its existing output assertion. Audit the relevant consumer decode paths; existing UseNumber paths in state.go and core_names.go already preserve numbers. No unrelated decoder refactoring, golden rewriting, settings, executor changes or workflow changes are requested.

## Decisions
- Render an integer note line as `app.txt:1`, matching Elixir; preserve absent-line output `README.md:: second`. (source: request)
- Support both the currently pinned executor and main at 391e0b6 or later without upgrading the production pin merely to hide the failure. (source: request)

## Source comparison
- addressed: Request Problem and Acceptance hooks regression at hooks_test.go:57, including the supplied `note: app.txt:1.0: looks fine` counterexample and `note: app.txt:1: looks fine` expectation, are covered by the feature's two executor variants and the unchanged existing hook output assertion.
- addressed: Request Need's consumer-side integer preservation and unit test for JSON line 1 displaying `1` are required in the outcome and Tests below; do not solve only the fixture while leaving lossy consumer decoding in place.
- conflicting: The request attributes the reproduced hook's float solely to JSON decoding; build/hooks_test.go:34 actually constructs float64(1). The contract explicitly corrects the fixture to decoded JSON integer semantics as well as fixing affected consumer decoding, without changing float display semantics.
- addressed: The request's historical report of main 391e0b6 on 3 Nov 20:24Z, eu2 and laptop remains historical evidence, not a new environment claim. Local comparison reproduced the same output with 391e0b6; the pinned test passed.
- omitted: No requested product behavior is intentionally omitted. Ancillary historical controls in supplied testing policy are not scope to resume unrelated work.

## Tests
- JSON note line 1 retains integer semantics and core.Display returns exactly `1`: add a focused unit regression through the actual consumer decoder, with the JSON input containing `{"severity":"note","path":"app.txt","line":1,"message":"looks fine"}` (unit)
- A green review approval is committed and prints `app.txt:1` rather than `app.txt:1.0`, while a nil line stays empty: retain TestTheReviewHookRunsAfterAGreenPassAndItsApprovalIsCommitted and make its note fixture use real integer-preserving JSON decoding; run against the pinned executor and v0.0.0-20261103202226-391e0b6ecd4f (regression)

## Feasibility
- Local probes observed the named hook passing with the current pin and failing with 391e0b6, showing `note: app.txt:1.0: looks fine`; the replacement uses a temporary modfile, not production go.mod edits.
- Documentation search returned unrelated results; authoritative encoding/json documentation was then retrieved at https://pkg.go.dev/encoding/json (status retrieved, untrusted). It states UseNumber decodes interface numbers as json.Number rather than float64. An initial heading lookup returned not_found; no claim of exhaustive research is made.
- evidence: {"id":"runner-observer","kind":"observer","inputs":["features/steps/k-build-note-line-integer__steps.js"],"observers":["features/steps/k-build-note-line-integer__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-note-line-integer__steps.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-note-line-integer__steps.js bad","exit":1,"output":"rejected\n"}]}
