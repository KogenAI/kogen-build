# Cover control characters in the file index

Slug: file-index-control-path-coverage
Model: not applicable — test-only coverage of a stateless rendering function
Acceptance:
- .kogen/intents/file-index-control-path-coverage/acceptance.feature
- features/steps/file-index-control-path-coverage__steps.js

Add exactly one focused Go test function to `build/file_index_test.go`. Keep the existing tests and do not change `build/file_index.go` or any other implementation file. This closes a coverage gap, not a production bug: the new test must pass on the unchanged renderer. Path ordering, omission selection and file-index size policy are out of scope.

Before writing the test, read both the `displayPath` comment and its implementation in `build/file_index.go` to establish the exact escape literals. Pass a path containing newline, tab, carriage return, backslash and another non-NUL control byte, and an ordinary path with non-ASCII text, in the same NUL-delimited listing to `fileIndex(listing string, required []string) string`. Assert on the returned index text, not on `displayPath` directly. For each of the five characters, separately assert the presence of its exact escape and the absence of its raw form in its path context. Newline is also an index separator and backslash is also the escape introducer, so use surrounding path text to distinguish a raw newline within the name or a lone backslash from legitimate output. Verify that the escaped path occupies one complete line and that the ordinary path appears as an exact unchanged line.

## Decisions
- The test asserts separately for each of the five characters that its documented escape text appears in the index text and that the raw character does not. The backslash check distinguishes a lone backslash from a doubled one. (evidence: build/file_index.go) (source: request)
- A path without control characters, including a non-ASCII name, is preserved as written and is passed through the same fileIndex call. (evidence: build/file_index.go; build/builder.go) (source: request)
- Add one test function that already passes on the unchanged build/file_index.go, changing build/file_index_test.go only. (evidence: build/file_index_test.go; build/file_index.go) (source: request)

## Source comparison
- Addressed: evidence `ev-39eb9509` identifies the contract in `build/file_index.go` (`displayPath` comment and body) and the coverage gap in `build/file_index_test.go`. The scope above preserves production behavior and the existing tests.
- Addressed: request acceptance 1 is covered by the independent per-character escape and raw-form assertions and the whole-line check above. The feature supplies representative byte/escape pairs and requires the added test to reject leaving each of the five character kinds raw independently.
- Addressed: request acceptance 2 is covered by the unchanged ordinary line from the same call and the scratch fault that rewrites non-ASCII ordinary text. Request acceptance 3 is covered by requiring the returned `fileIndex` text and rejecting a scratch bypass confined to the index renderer.
- Omitted: no requested outcome; ordering, omission selection and size policy remain explicitly out of scope. Conflicting: none. Superseded: no rendering contract or existing test.

## Tests
- Exact control-path escapes and no raw within-path characters: add one focused test of the rendered line with separate positive and negative assertions for each of the five bytes (regression)
- Ordinary non-ASCII path preserved and index integration exercised: check the ordinary line in that same test's fileIndex result, so an escaping bypass in the index is observable (regression)
- Existing bounds and omission behavior: retain TestFileIndexVeryWideRepositoryStaysBounded and TestFileIndexOmissionAccountingAndInputOrder (existing)

## Feasibility
- Go 1.27.2 and Node 24 were available. Compiling the two whole files alone fails because the trailing brief-fitting method needs `Attempt`. Compiling named files from different directories also fails. The observer instead copies the actual index code through the rendering declarations, stopping before that unrelated method, and puts both Go files in the same temporary directory.
- The two existing file-index tests passed with `GOFLAGS=-trimpath go test -v -count=1 -run '^TestFileIndex' ./build` (exit 0). The correct test overlay also passed alongside those two tests in the full build package with the renderer untouched (exit 0).
- The observer compiles the focused files, runs the built binary without a test-name filter, and checks actual run and terminal results. Scratch faults change only temporary files; compile errors, crashes, skips and timeouts do not count as detection. The following commands were run against the exact proposed observer inputs: good exited 0 after all faults were detected; bad exited 1 at the bypass check after the helper-only test missed it.
- evidence: {"id":"file-index-observer","kind":"observer","inputs":["build/file_index.go",".kogen/intents/file-index-control-path-coverage/acceptance.feature",".kogen/intents/file-index-control-path-coverage/controls.json"],"observers":["features/steps/file-index-control-path-coverage__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/file-index-control-path-coverage__steps.js good","exit":0,"output":"accepted the fileIndex regression test\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/file-index-control-path-coverage__steps.js bad","exit":1,"output":"rejected the helper-only test at the index bypass fault\n"}]}
