# Test file-index omission accounting

Slug: file-index-omission-accounting
Model: not applicable — this is regression coverage of a pure rendering function, with no state transitions
Acceptance:
- .kogen/intents/file-index-omission-accounting/acceptance.feature
- features/steps/file-index-omission-accounting__steps.js
- features/steps/file-index-omission-accounting__observer.js

Add direct regression tests in `build/file_index_test.go` for the large-repository file index's omitted-directory accounting and input-order independence. Keep the useful existing very-wide-repository test. Do not change file-index selection or rendering behavior, production files, or other project tests. No existing behavior is replaced or removed; the gap being removed is missing regression coverage.

Exercise `fileIndex(listing string, required []string) string` directly with a large NUL-delimited path listing that triggers the summarized index, including omitted root files, omitted files directly in a directory, and omitted descendants of that directory. Derive the omitted path set from the fixture paths minus the individually listed paths. Parse the rendered omission entries and check their counts against that set, including disjoint coverage, not just the reported total. Ensure the fixture really omits root and nested paths, so the accounting assertions cannot pass vacuously. Compare complete output strings for two orderings of the identical listing with the same required paths. In that regression fixture, also check required paths as individual listed lines and the `fileIndexBytes` bound.

Acceptance compiles the real `build/file_index.go` and candidate `build/file_index_test.go` as a focused Go test binary, runs the tests on the unmodified renderer, and checks that they fail under isolated scratch renderer faults. Compilation errors, missing tests, process errors and timeouts do not count as detecting a regression. These scratch mutations are verification only, not implementation edits. There is no new datastore or external-service operation, performance contract, dependency, or configuration option.

## Decisions
- The summary counts each omitted path exactly once across disjoint directory entries. (decision: D1, round 1) (constraint: Add regression coverage only in build/file_index_test.go; do not change file-index selection or rendering behavior.) (evidence: build/file_index.go; build/file_index_test.go) (source: request)
- Reordering the same path listing produces byte-identical output. (decision: D2, round 1) (constraint: Exercise the large-repository summarized index; preserve the existing small-listing rendering behavior.) (evidence: build/file_index.go) (source: request)
- The regression fixture still verifies that required paths are listed individually and the rendered index stays within fileIndexBytes. (decision: D3, round 1) (constraint: Keep the useful existing very-wide-repository test and include these assertions in the accounting and reordering regression fixture.) (evidence: build/file_index_test.go; build/file_index.go) (source: request)

## Source comparison
- Addressed: Request evidence `ev-d704aa36` identifies `build/file_index.go`'s deterministic, bounded, disjoint-directory contract and the coverage gap in `build/file_index_test.go`. The scope paragraph and all three scenarios add direct regression coverage in that test file without altering production selection or rendering.
- Addressed: Request acceptance 1 requires omitted paths at the root and nested directories, counted exactly once across disjoint entries. The fixture requirements above and the accounting scenario require an independently derived omitted set; overlapping totals and dropped paths are exercised as separate scratch faults by the observer.
- Addressed: Request acceptance 2 requires byte-identical output after reordering the same path listing. The complete-output comparison above and the order scenario distinguish an input-order-dependent renderer without changing the existing small-listing behavior.
- Addressed: Request acceptance 3 requires individually listed required paths and the `fileIndexBytes` bound in the regression fixture. The fixture requirements above and the guards scenario cover required-path loss and oversized output; the existing wide-repository coverage is preserved, not replaced by summary-only assertions.
- Omitted: none of the requested outcomes. Conflicting: none. Superseded: no selection or rendering behavior, settings, or tests.

## Tests
- Omitted-directory accounting: add a focused direct fileIndex regression whose independently derived omitted set is partitioned exactly once by the rendered directory entries and whose entry counts and sum agree with that set (regression)
- Input-order independence: compare the complete rendered index for two different orderings of the same large fixture with unchanged required paths (regression)
- Required paths and byte bound: assert individual required-path lines and len(output) <= fileIndexBytes in the new regression fixture while preserving TestFileIndexVeryWideRepositoryStaysBounded (regression)

## Feasibility
- evidence: {"id":"regression-observer","kind":"observer","inputs":[".kogen/intents/file-index-omission-accounting/controls.json","build/file_index.go","build/file_index_test.go"],"observers":["features/steps/file-index-omission-accounting__observer.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/file-index-omission-accounting__observer.js good","exit":0,"output":"accepted complete regression fixture\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/file-index-omission-accounting__observer.js bad","exit":1,"output":"rejected 4 weak test fixtures\n"}]}
