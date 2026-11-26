# Count empty file inventories correctly

Slug: empty-file-index-accounting
Model: not applicable — the index is a stateless transformation of a path listing.
Acceptance:
- .kogen/intents/empty-file-index-accounting/acceptance.feature
- features/steps/empty-file-index-accounting__steps.js
- features/steps/empty-file-index-accounting__driver/index_test.go.txt

Represent an empty repository inventory as zero paths, with no blank listed path. Limit implementation changes to `build/file_index.go` and `build/file_index_test.go`. Replace phantom empty entries without changing index bounds, ordering, or omission summaries for nonempty listings.

## Decisions
- An empty listing reports zero total, listed, and omitted paths and contains no blank path entry. A listing containing only its trailing NUL is handled as zero paths. In the ordinary full index, keep the `# Files` heading and its separator with no path rows; in the summarized index, use the existing count labels and `## Listed paths` heading with no path rows or omitted-directory section. (evidence: build/file_index.go; build/builder.go) (source: request)
- A one-path listing still reports that path exactly once. Preserve the existing bounds, ordering, required-path handling and omission summaries for nonempty inventories. (evidence: build/file_index.go; build/file_index_test.go) (source: request)

## Source comparison
- Addressed: `ev-c533491e` identifies the phantom blank path from an empty split token. The zero-inventory Decision and `Empty listing` require no blank path row and zero counts; they reject the specified wrong implementation that retains the split result as one path.
- Addressed: acceptance 2 maps to `NUL-only listing`. Removing only the trailing token from this input leaves one empty token; the scenario compares the complete empty list bodies and the zero counts.
- Addressed: acceptance 3 maps to `One-path listing`. The minimal nonempty fixture guards against discarding short listings; both the ordinary index and the compact index with that path required must show it exactly once.
- Superseded: emitting a blank listed row or accounting for a blank path in an empty inventory. Omitted from the change, as requested: changes to bounds, order or nonempty omission summaries. No requested outcome is omitted or conflicts with the contract.

## Tests
- Zero-byte input has no path rows and all compact counts are zero: add a case to `TestFileIndexEmptyAccounting` in `build/file_index_test.go` (regression)
- NUL-only input has no path rows and all compact counts are zero: add a case to `TestFileIndexEmptyAccounting` (regression)
- One-path input retains exactly one path in both renderings: add the adjacent nonempty case to `TestFileIndexEmptyAccounting` (unit)
- Nonempty bounds, ordering, omission summaries and path display are unchanged: keep `TestFileIndexVeryWideRepositoryStaysBounded`, `TestFileIndexOmissionAccountingAndInputOrder` and `TestFileIndexControlCharacters` (existing)
- none: the renderer only transforms an already supplied string in memory and never performs per-path data-store or external-service work.

## Feasibility
- Acceptance uses the configured Cucumber command from the repository root, Node 24 and Go 1.27.2 (`.kogen/project.yaml`, `README.md`, `go.mod`). No provider or repository fixture is needed. The Go helper calls `fileIndex(listing string, required []string) string` for full rendering and `fileIndexBound(listing string, required []string, bound int) string` with the existing zero bound for compact rendering (`build/file_index.go`). The step compiles `./build` with the helper in a temporary overlay, runs just that helper, and compares both complete renderings from its temporary JSON report. A missing report or failed process is a harness failure, not a product mismatch.
- The toolchain check found Go 1.27.2 and Node 24. The existing file-index tests passed on the unchanged tree and the scratch correct overlay. The package overlay also compiled and the named acceptance helper ran on the unchanged tree. For NUL-only bytes the probe emitted a blank full-index row and the summary reported `total paths: 1`, `listed paths: 0`, `omitted paths: 1` and `./: 1 omitted paths`. The probe executed successfully; the rendered values were wrong.
- The final-byte observer accepted the three known-good renderings and rejected an otherwise correct empty summary with a blank listed row. That negative control reaches the compact comparison after the plain comparison succeeds.
- evidence: {"id":"file-index-observer","kind":"observer","inputs":[".kogen/intents/empty-file-index-accounting/controls.json"],"observers":["features/steps/empty-file-index-accounting__steps.js","features/steps/empty-file-index-accounting__driver/index_test.go.txt"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/empty-file-index-accounting__steps.js good","exit":0,"output":"Accepted all three rendered inventories\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/empty-file-index-accounting__steps.js bad","exit":1,"output":"Rejected a blank listed row despite zero counts\n"}]}
