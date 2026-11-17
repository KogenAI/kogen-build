# Bound the initial Build file index

Slug: build-bound-initial-file-index
Model: not applicable — the change is a pure function of the repository's path list and the brief's authority text, with no state
Acceptance:
- .kogen/intents/build-bound-initial-file-index/acceptance.feature
- features/steps/build-bound-initial-file-index__steps.js
- features/steps/build-bound-initial-file-index__driver/fixture_test.go.txt
- features/steps/build-bound-initial-file-index__driver/determinism_test.go.txt

The builder brief (`Attempt.brief` in `build/builder.go`) currently puts the whole `git ls-files --cached --others --exclude-standard` output into its Dynamic part under `# Files`. In the observed case (25445 paths, 1884154 of 1935817 prompt bytes) the provider rejected the first prompt as `prompt_too_long` before any builder tool ran. This Intent replaces that unbounded listing with a deterministic bounded index and a fitting guard, in native Build core code only (`build/builder.go` and what it needs in package `build`).

Rules:
- The file index is bounded by one fixed constant, 65536 bytes, counted from `# Files` to the end of Dynamic's index section. It is a constant, not a setting.
- A repository whose full listing fits that bound keeps today's normal form: Dynamic is the baseline text followed by `# Files`, a blank line and the listing verbatim, with no omission metadata.
- A larger listing becomes a bounded index of at most 65536 bytes starting at `# Files`, with these parts: the metadata lines `total paths: <N>`, `listed paths: <M>`, `omitted paths: <K>` (N is the true count of tracked plus untracked non-ignored paths, M+K=N); a sentence containing `omitted paths still exist` and stating that they remain readable and searchable with the existing file, read and search tools; a `## Listed paths` section with one path per line; and a `## Omitted directories` section with one line per directory `<dir>/: <n> omitted paths`, directories disjoint, each n the true number of that directory's paths not listed, summing to K. Selection is deterministic: the same repository content, in any folder, and repeated calls give byte-identical indexes.
- Paths that are protected (the Build's baseline paths) or listed under the Intent's `Acceptance:` are always listed individually when they exist.
- The Stable authority (Intent, acceptance feature, model, README, testing rules, commands, protected paths) is never clipped. If the Stable part alone cannot fit `provider.InputBudgetBytes` (524288), `brief` returns the existing typed failure `*sys.Error` with class `candidate`, reason `context_budget_exceeded`, exit 1, before any provider call.
- No enlargement of model context, executor sanity bounds, quotas, timeouts or spending caps; no new engine. File access tools are unchanged.

## Decisions
- The request asks for a bounded index without a size; the bound is one fixed 65536-byte constant, which is also the threshold below which a listing is shown verbatim. (assumption: the user accepts 65536 bytes as the index bound and verbatim threshold) (source: request)
- The guard lives in `Attempt.brief`, which both the native opening message and the external executor's first prompt consume. (evidence: build/builder.go; build/executor_native_test.go) (source: request)
- Metadata wording is pinned only for the literals named above; the rest of the wording is free. (assumption: the user accepts these literals as the index's contract) (source: request)

## Source comparison
- Addressed: bounded deterministic index with directory/count/omission metadata, truthful counts, retained required paths, normal small repositories, unchanged access to omitted paths (acceptance scenarios 1-5).
- Addressed: determinism (scenario 4 builds the brief repeatedly and in a second identical repository).
- Addressed: irreducible authority fails with the typed context-budget error before any provider call (scenario 6; the fixture uses no provider at all).
- Addressed: synthetic 25000-path mixed tracked/untracked long-path repository and a small control; no model calls; no exact vendor token counts asserted.
- Omitted by design: h23 data, Build f1dfc28f state, retries, model/quota changes; they need no edit in this module.

## Tests
- Large repository index and complete opening/prompt composition stay bounded: fixture test `TestBoundedIndexLargeRepoStaysBounded` (integration)
- Counts, directories and required paths are truthful: `TestBoundedIndexMetadataIsTruthfulAndRequiredPathsListed` (integration)
- Omitted but real path stays readable and searchable: `TestBoundedIndexOmittedPathStaysAccessible` (integration)
- Selection is deterministic: `TestBoundedIndexSelectionIsDeterministic` (integration)
- Small repository form unchanged: `TestBoundedIndexSmallRepoIsUnchanged` (regression)
- Irreducible authority fails typed before a provider: `TestBoundedIndexIrreducibleAuthorityFailsBeforeAnyProvider` (integration)
- Existing brief goldens and Build suite pass unchanged: `TestNativeOpeningInputEqualsTheOpeningMessage` and the rest (existing)

## Feasibility
- none
