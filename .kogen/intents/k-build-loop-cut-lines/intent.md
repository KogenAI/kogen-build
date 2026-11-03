# Stop pinning kogen-loop's exact tool-result cut

Slug: k-build-loop-cut-lines
Model: not applicable — a test-only change to two assertions of one test file; there is no state, transition or interacting rule to model
Acceptance:
- .kogen/intents/k-build-loop-cut-lines/acceptance.feature
- features/steps/k_build_loop_cut_lines_steps.js
- features/steps/k_build_loop_cut_lines_knobs_test.go.txt

kogen-loop main eed48c4 ("Align tool text cuts with complete lines", L-CUT-LINE-ALIGNED) cuts a long tool result at complete lines instead of at an exact byte count. kogen-build's `build/loop_knobs_test.go` still pins the old byte-exact cut, so the consumer check of every kogen-loop landing fails in kogen-build. Against eed48c4, the whole `go test ./...` fails only these two subtests: `loop_knobs_test.go:114: shell result: 12303 bytes delivered, want about 16384` and `loop_knobs_test.go:164: reviewer shell result: 12303 bytes delivered, want about 16384`. The knob shell result `exit status: 0\n` followed by 30000 `a` bytes is delivered as its first line, the marker and a 12259-byte tail.

After this change, the cut subtests of `TestLoopKnobsNativeBuilder` and `TestLoopKnobsReview` assert only what kogen-build relies on from a cut tool result:
1. the delivered text is at most 16384 bytes;
2. it carries kogen-loop's omission marker, matched by the existing `knobMarker` regexp `\n\[\.\.\. (\d+) bytes omitted \.\.\.\]\n`, wherever the marker falls;
3. the builder (or reviewer) still gets the result: the Build (or review) exits 0 and the cut result is among the tool outputs sent to the provider, as `knobOutput` finds it today.

They no longer assert a minimum delivered size (today `len(out) < bound-64` fails) or the marker's position (today it must start at bytes 4093-4096). That way, a later change to where or how kogen-loop cuts, within the bound and with the marker, cannot fail kogen-build. The exact cut bytes are kogen-loop's own and are pinned in kogen-loop's contract tests (L-CONTRACT-CUT-BYTES), not here.

Boundaries:
- The change is confined to `build/loop_knobs_test.go`: `knobCut`, its two call sites, the two subtest names and the file's header comment. The test functions `TestLoopKnobsNativeBuilder` and `TestLoopKnobsReview` keep their names. Each keeps its cut subtest, which still runs `knobShell` through the real native Build or review and judges the delivered result. Their other subtests (input budget, inline candidate source, no authority breakpoint) are unchanged.
- No production code changes. This includes `build/records.go`: its `cutFields`/`omittedMarker` and `TestCutFieldsOnlyForCutResults` stay as they are.
- `go.mod` and `go.sum` are unchanged, and kogen-loop stays pinned at `github.com/KogenAI/kogen-loop v0.0.0-20261102055344-c6e2360290d4`.
- No goldens change.
- No new commands or flags, and no timeout is raised.

Acceptance harness: `features/steps/k_build_loop_cut_lines_steps.js` never writes the checkout. It copies the pinned kogen-loop module (`go list -m -f '{{.Dir}}' kogen-loop`) into a temporary directory and patches the copy's `tools/tools.go` with one cut variant. It then runs `go test -modfile=<temp go.mod> -count=1 -v -run '^(TestLoopKnobsNativeBuilder|TestLoopKnobsReview)$' ./build` from the checkout, where the temporary go.mod replaces `kogen-loop` with that copy. The variants are:
- `pinned`: unchanged.
- `line-aligned`: eed48c4's `cutText`, verbatim.
- `head-only`: a head of the bound less the marker, then the marker, which ends the text.
- `over-limit`: the pinned cut at 20480 bytes.
- `unmarked`: the first 16384 bytes, with no marker.

`features/steps/k_build_loop_cut_lines_knobs_test.go.txt` is a known-good `build/loop_knobs_test.go`. The step file uses it only for its own observer controls (`node features/steps/k_build_loop_cut_lines_steps.js good|bad`, applied through `go test -overlay`), not in the scenarios.

## Decisions
- "Within the limit" means `len(delivered) <= 16384`, the bound both subtests already pass to `knobCut`. There is no lower bound on the delivered size. (source: request)
- "Ends with the cut marker when it was cut" is asserted as "the delivered result contains the omission marker" (`knobMarker`). The marker may end the text or be followed by a kept tail. See Source comparison. (source: request)
- "The builder still gets the result" stays as today: the run exits 0, and `knobOutput` finds the cut result among the delivered tool outputs (the native builder's `e.prov.toolOutputs()`, and the reviewer's `function_call_output` items). (source: request)
- The marker's position and the exact delivered byte count are not asserted. `knobCut` drops its `head` parameter, and both call sites pass only the bound `16384`. (source: request)
- Both cut subtests are renamed from "a tool result is cut at 16384 bytes with a 4096-byte head" to "a long tool result is cut within 16384 bytes and still delivered". The header comment no longer claims a 4096-byte head. The old wording describes the pinned position that is no longer asserted. (source: request)

## Source comparison
- Addressed: "assert only what kogen-build relies on from a cut tool result: within the limit, the cut marker, the builder still gets the result". This is rules 1-3 of the prose and Decisions 1-3. Acceptance scenario 1 runs the knob tests with the pinned cut and with eed48c4's line-aligned cut. Scenario 2 runs them with a head-only cut (a cut kogen-loop could adopt later), which must pass, and with an over-limit cut and an unmarked cut, which must both fail.
- Addressed: "do not pin the exact bytes or the exact cut position kogen-loop produces". The size floor and the marker-position check are removed (Decision 4). The reproduced failure `loop_knobs_test.go:114: shell result: 12303 bytes delivered, want ...` (and its reviewer twin at :164) is the red of scenario 1. On the unchanged tree, the head-only cut fails with `marker after 16353 bytes, want the head of 4096` (scenario 2).
- Conflicting literal, settled: "ends with the cut marker when it was cut". Neither the pinned cut nor eed48c4's cut ends the knob shell result with the marker, because a kept tail follows it. Requiring the marker at the end would fail kogen-build on today's pinned kogen-loop, and would re-pin a cut position, which the request forbids. The Intent requires the marker's presence in the exact form `build/records.go` parses, and the head-only variant shows that a marker at the end also passes (Decision 2).
- Out of scope (request): the exact cut bytes belong to kogen-loop's contract tests (L-CONTRACT-CUT-BYTES).
- Kept (request: "No change to kogen-build's own behaviour"): all production code, `go.mod`/`go.sum` and the kogen-loop pin. Noted, not changed: `build/records.go` `cutFields` records `bytes_before_cut`/`bytes_delivered` only when the marker starts within bytes 4000-4096. With eed48c4's line-aligned cut, the knob shell result's marker starts at byte 15, so such a result would carry no cut fields. That is production behaviour and outside this test fix. `TestCutFieldsOnlyForCutResults` uses a line-free input and passes against eed48c4.
- Kept (constraint): no new commands or flags, and no timeout raised. The acceptance step uses only existing `go` subcommands and flags (`list -m`, `mod edit -replace`, `test -modfile -overlay -count -v -run`).

## Tests
- The native builder's cut tool result is delivered within 16384 bytes, carries the omission marker and reaches the builder of a Build that exits 0. No exact size or marker position is asserted: update the cut subtest of `TestLoopKnobsNativeBuilder` in `build/loop_knobs_test.go`. It reproduces the reported failure against eed48c4's cut; acceptance scenario 1, the `line-aligned` variant (regression)
- The reviewer's cut tool result follows the same rule: update the cut subtest of `TestLoopKnobsReview` in `build/loop_knobs_test.go`; acceptance scenario 1 (regression)
- The updated subtests still reject a cut over 16384 bytes and a cut without the marker, and accept a different in-bound marked cut: acceptance scenario 2, the `over-limit`, `unmarked` and `head-only` variants (regression)
- The knob tests still pass with the pinned kogen-loop, and the other knob subtests and `TestCutFieldsOnlyForCutResults` are unchanged: acceptance scenario 1, the `pinned` variant, plus the configured checks (format, compile, vet, go-test) (existing)

## Feasibility
- Reproduced in a scratch copy, with go.mod's `kogen-loop` replaced by a clone of kogen-loop main eed48c4: `go test -count=1 ./...` failed only `TestLoopKnobsNativeBuilder` and `TestLoopKnobsReview` (`12303 bytes delivered, want about 16384` at :114 and :164); every other package and test passed (348 s).
- Acceptance on the unchanged tree (scratch copy, real Cucumber): the `pinned` step passes. `over-limit` fails both tests (`20479 bytes delivered, want about 16384`), and `unmarked` fails both (`no cut tool result among 1`), so those steps pass. `line-aligned` and `head-only` fail (`12303 bytes delivered`; `marker after 16353 bytes, want the head of 4096`): that is legitimate missing-feature red. With the fixture's `build/loop_knobs_test.go` in place, both scenarios pass (about 61 s).
- evidence: {"id":"loop-cut-observer","kind":"observer","inputs":["features/steps/k_build_loop_cut_lines_steps.js","features/steps/k_build_loop_cut_lines_knobs_test.go.txt"],"observers":["features/steps/k_build_loop_cut_lines_steps.js"],"checks":[{"name":"good","command":"node features/steps/k_build_loop_cut_lines_steps.js good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"node features/steps/k_build_loop_cut_lines_steps.js bad","exit":1,"output":"observer rejected\n"}]}
