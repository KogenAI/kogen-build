# Preserve newline filenames in the builder index

Slug: preserve-newline-paths-in-builder-index
Model: not applicable — the change is a pure function of the repository's path listing, with no state
Acceptance:
- .kogen/intents/preserve-newline-paths-in-builder-index/acceptance.feature
- features/steps/preserve-newline-paths-in-builder-index__steps.js
- features/steps/preserve-newline-paths-in-builder-index__driver/fixture_test.go.txt

The builder brief's file index (`build/builder.go`, `build/file_index.go`) must keep a filename containing a newline as one path. `Attempt.brief` collects paths with `git ls-files -z --cached --others --exclude-standard`, and `fileIndex` takes that NUL-delimited listing (each path followed by NUL, an empty string for no paths) and splits on NUL. A path containing control characters is displayed in an escaped form on a single line, so each path stays identifiable as one entry; an ordinary path is displayed unchanged. The escaping applies wherever the index displays a path: the verbatim listing, the listed paths and the omitted-directory lines. Path counts, required-path matching and ordering use the raw names. The existing bounded-index behavior and the 65536-byte constant are unchanged, and the bound is measured on the displayed output. Out of scope: repository path policy, other Git commands, the byte limit.

## Decisions
- The Build chooses the escape syntax for control characters; acceptance asserts only that the displayed entry is one line with no control character and keeps the path's visible start and end. (assumption: the user accepts the Build's choice of escape syntax) (source: request)
- A listing that fits the bound is shown as one path per line in git order, with no metadata, as before. (evidence: build/file_index.go) (source: request)
- Only `fileIndex` and the `ls-files` call in `Attempt.brief` change. (evidence: build/builder.go; build/file_index.go) (source: request)
- fileIndex takes the NUL-delimited git ls-files -z listing and displays every path with control characters escaped as \n, \t, \r, \xNN; other paths unchanged (decision: D1, round 1) (constraint: byte limit 65536 unchanged and measured on escaped output) (constraint: counts, ordering and required matching use raw names) (evidence: build/file_index.go; build/builder.go; build/file_index_test.go) (assumption: Escape form for backslash and other bytes is a shaper choice the user must confirm; a scratch overlay run in this session showed the three fixture tests pass on a correct implementation.) (source: request)

## Source comparison
- Addressed: NUL-delimited collection and one escaped entry per newline filename with accurate counts (scenario 1).
- Addressed: required newline paths stay present as one entry in an oversized listing (scenario 2).
- Addressed: ordinary paths keep their display and oversized listings stay within the existing bound (scenario 3).
- Omitted by design: path policy, other Git commands, the byte limit.

## Tests
- Newline filename is one escaped entry with true counts, also through the real Git listing: `TestNewlineFilenameIsOneEscapedEntryWithTrueCounts` (integration)
- Required newline path stays one entry in an oversized listing: `TestRequiredNewlinePathStaysOneEntryWhenOversized` (unit)
- Ordinary paths unchanged and the bound holds: `TestOrdinaryPathsKeepDisplayAndOversizedStaysBounded` (regression)
- Existing index behavior: `TestFileIndexVeryWideRepositoryStaysBounded` updated to a NUL listing in build/file_index_test.go (existing)

## Feasibility
- none
