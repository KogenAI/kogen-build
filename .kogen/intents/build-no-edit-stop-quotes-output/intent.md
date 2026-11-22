# Quote the failing check's output in no-edit stops

Slug: build-no-edit-stop-quotes-output
Model: not applicable — the change is a pure text rule over one remembered red pass; the scenarios cover each outcome
Acceptance:
- .kogen/intents/build-no-edit-stop-quotes-output/acceptance.feature
- features/steps/build-no-edit-stop-quotes-output__steps.js
- features/steps/build-no-edit-stop-quotes-output__driver/acceptance_test.go.txt

The no-edit notice and the candidate/no_progress stop of a Build name their first failure only by check name, so a reader learns which check was red but not why. In `build/noprogress.go`, when the first failure of the remembered red pass is a failing check (the name equals the name of a check of that pass with a nonzero status), the no-edit notice gains one sentence and the candidate/no_progress stop gains one line, each quoting that check's first nonblank output line, trimmed and cut to at most 200 characters. The notice sentence is `First line of its output: ` followed by the quoted line, placed right after the sentence that names the first failure. The stop line is `first failure output: ` followed by the quoted line, placed right after the line starting `first failure: `. When the first failure is an acceptance scenario (a line such as `failed: Add overflows (line 12)`), or the check has no nonblank output, nothing is added and both texts are byte-identical to today. Every other existing line, the exit code and the event are unchanged. Scope: build/noprogress.go and build/noprogress_test.go in kogen-build. Out of scope: go.mod and go.sum, the host-or-base stop, the external executor stop rules, the verification report layout, documentation.

## Decisions
- The quoted line is the first line of the check's output that is nonblank after trimming surrounding whitespace, trimmed, then cut to at most 200 characters (characters, never bytes, so a multibyte character is never split). (evidence: build/noprogress.go) (source: request)
- The quote is taken from the check of the pass whose name equals the first failure and whose status is nonzero; the first failure is a failing check only when the pass has no missing acceptance item, as `firstFailure` already decides. Otherwise, or when that check has no nonblank output, nothing is added. (evidence: build/noprogress.go) (source: request)
- The notice sentence `First line of its output: <quote>` comes right after the sentence that names the first failure; the request does not fix whether a line break or a space separates the two sentences, and acceptance accepts either. (evidence: build/noprogress.go; build/noprogress_test.go) (assumption: the separator between the two sentences is left to the builder) (source: request)
- The stop line is `first failure output: <quote>`, inserted right after the line starting `first failure: `; all other lines keep their order and text, the exit code and the no_progress event are unchanged. (evidence: build/noprogress.go; build/noprogress_test.go) (source: request)
- No setting is added: the 200 character cut is a constant. (evidence: build/noprogress.go) (source: request)
- The Build adds focused model-free Go tests to build/noprogress_test.go next to the existing no-edit tests, using its existing newEnv-based fixtures. Acceptance runs five driver tests that it copies into build/ for the run and removes afterwards, because the Build's own test names are not known beforehand. (evidence: build/noprogress_test.go; build/helper_test.go) (assumption: the acceptance driver tests are a proof artifact separate from the tests the Build adds to build/noprogress_test.go) (source: request)
- The quoted line is the first line of the check output nonblank after trimming, trimmed, cut to at most 200 characters (runes). (decision: D1, round 1) (evidence: build/noprogress.go) (assumption: none) (source: request)
- The quote is taken only when the first failure is a failing check (nonzero status, name equal to the first failure) and the pass has no missing acceptance item; else nothing is added. (decision: D2, round 1) (evidence: build/noprogress.go) (assumption: none) (source: request)
- The notice sentence is a separate line right after the 'Make an edit toward the first named failure' line, without trailing period. (decision: D3, round 1) (evidence: build/noprogress.go; build/noprogress_test.go) (assumption: Request does not fix the separator between sentences; a separate line is chosen.) (source: request)
- The stop line 'first failure output: <quote>' is inserted right after 'first failure: ' and the other lines keep order and text. (decision: D4, round 1) (evidence: build/noprogress.go; build/noprogress_test.go) (assumption: none) (source: request)
- No setting is added; the 200 character cut is a constant. (decision: D5, round 1) (evidence: build/noprogress.go) (assumption: none) (source: request)
- Acceptance runs five driver Go tests copied into build/ for the run and removed afterwards; the Build adds its own tests to build/noprogress_test.go. (decision: D6, round 1) (evidence: build/noprogress_test.go; build/helper_test.go) (assumption: The driver tests are a proof artifact separate from the Build's tests.) (source: request)

## Source comparison
- Request: a red pass whose first failing check `test` printed `--- FAIL: TestX` as its first nonblank line, then a no-edit round: the notice contains `First line of its output: --- FAIL: TestX` right after the sentence naming test; addressed by D1 to D3 and scenario 1.
- Request: the next no-edit round stops as candidate/no_progress with the lines containing `first failure output: --- FAIL: TestX` immediately after `first failure: test`, the other lines being the existing ones in the existing order; addressed by D4 and scenario 2.
- Request: a first output line of 300 characters is cut to 200 characters in both places; addressed by D1 and scenario 3.
- Request: no change when the first failure is an acceptance scenario such as `failed: Add overflows (line 12)`, or the check has no nonblank output; addressed by D2 and scenarios 4 and 5, which pass on the unchanged tree. The acceptance driver uses the build test fixture's own scenario line (`failed: First (line 2)`) for the same rule.
- Request: every other line, the exit code and the event stay unchanged; addressed by D4 and the exact-text assertions of scenarios 1 to 5.
- Request: out of scope go.mod/go.sum, host-or-base stop, external executor stop rules, report layout, documentation; omitted from the behavior.

## Tests
- The notice quotes the first nonblank trimmed output line of the first failing check, not another check's and not the last line: a Go test in build/noprogress_test.go (integration)
- The stop gains `first failure output: ` right after `first failure: ` and keeps its other lines in order: a Go test in build/noprogress_test.go (integration)
- A 300 character multibyte first line is cut to 200 characters in both places and a 200 character line is kept whole: a Go test in build/noprogress_test.go (integration)
- An acceptance scenario as first failure leaves the notice and the stop unchanged: a Go test in build/noprogress_test.go (regression)
- A failing check with empty or blank-only output leaves the notice and the stop unchanged: a Go test in build/noprogress_test.go (regression)
- The existing no-edit tests keep passing unchanged: the existing tests in build/noprogress_test.go (existing)
- The five outcomes through the Build's real entry point: acceptance.feature scenarios (scenario)

## Feasibility
- none
