# Display file index paths unambiguously

Slug: unambiguous-file-index-paths
Model: not applicable — displayPath is a pure function of one path string, with no state
Acceptance:
- .kogen/intents/unambiguous-file-index-paths/acceptance.feature
- features/steps/unambiguous-file-index-paths__steps.js
- features/steps/unambiguous-file-index-paths__driver/fixture_test.go.txt

In `build/file_index.go`, `displayPath` currently escapes backslashes only for a path that also contains a control character, so a filename containing a literal backslash followed by `n` shows the same as a filename containing an actual newline followed by `n`. This Intent makes the file index display such distinct repository paths differently.

Rules:
- A path containing a backslash or a control character (below 0x20 or 0x7f) is displayed on a single line with `\\` for each backslash and, for each control character, `\n`, `\t`, `\r` or `\xNN` (two lowercase hex digits for any other control character). The escaping is applied to every backslash whether or not the path contains a control character.
- A path with no backslash and no control character is displayed unchanged (including spaces, non-ASCII characters and punctuation).
- Display is deterministic: the same path always displays the same.
- Out of scope and unchanged: filesystem path access, the index selection and omission policy, counts, ordering and required-path matching (which use raw names).

The change is in `build/file_index.go`; `build/file_index_test.go` gains coverage of path display escaping.

## Decisions
- Backslash escaping no longer depends on the presence of a control character; the existing escape forms (`\\`, `\n`, `\t`, `\r`, `\xNN`) are kept as they are, so paths with control characters keep displaying as before. (evidence: build/file_index.go; build/file_index_test.go) (source: request)
- Paths without a backslash or control character are returned unchanged, so existing readable entries do not change. (evidence: build/file_index.go) (source: request)
- Acceptance drives the real `fileIndex` through a Go fixture compiled over the unchanged tree with a go build overlay, as an earlier Intent of this repository does; no provider or filesystem access is involved. (evidence: features/steps/build-bound-initial-file-index__steps.js) (source: request)
- Backslash escaping in displayPath no longer depends on a control character being present; existing escape forms are kept; paths with neither backslash nor control character are returned unchanged. (decision: D1, round 1) (constraint: escape forms stay \\, \n, \t, \r, \xNN) (constraint: no change to selection/omission policy or raw-name matching) (evidence: build/file_index.go; build/file_index_test.go) (assumption: A scratch go test with a build overlay in this session showed the fixture red on the unchanged file_index.go and green with the one-line change; Kogen readiness is the authoritative run.) (source: request)

## Source comparison
- Addressed: literal backslash-n versus actual newline-n paths display differently (scenario 1).
- Addressed: backslashes escaped when the path has no control character (scenario 2).
- Addressed: ordinary paths unchanged and control characters keep deterministic visible escapes (scenario 3).
- Omitted by design: filesystem path access and index selection/omission policy, per the request's out-of-scope statement.

## Tests
- Literal backslash-n and newline-n paths display differently: `TestDisplayPathBackslashNDiffersFromNewline` added to build/file_index_test.go (unit)
- Backslashes are escaped in paths without control characters: `TestDisplayPathEscapesBackslashWithoutControlCharacter` added to build/file_index_test.go (unit)
- Ordinary paths unchanged and control characters keep their escapes: extend `TestFileIndexControlCharacters` and add an ordinary-path case in build/file_index_test.go (unit)
- The acceptance scenarios exercise the same three rules through `fileIndex`: acceptance.feature with the Go fixture (scenario)

## Feasibility
- none
