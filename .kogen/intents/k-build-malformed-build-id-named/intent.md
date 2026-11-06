# Name malformed and unknown Build ids in refusals

Slug: k-build-malformed-build-id-named
Model: not applicable — the refusal chosen depends only on the argument's shape and on whether its run directory exists; no state changes or interacting rules are added
Acceptance:
- .kogen/intents/k-build-malformed-build-id-named/acceptance.feature
- features/steps/k-build-malformed-build-id-named__steps.js

`build.Resume` (`kogen intent resume <id>`, standalone `kogen-build resume --project <checkout> --id <id>`) and `build.Recover` (`kogen intent recover <id> --output <dir>`) currently refuse an argument that is not shaped like a Build id with the bare text `unknown checkpoint`. Resume also appends the drift/fresh-approval advice. Neither says what a Build id is or how to find one. This change gives each cause its own refusal. Class, kind and exit code stay the same, so callers keep working.

A Build id has the existing format `^[a-f0-9]{8}$`: exactly 8 lowercase hexadecimal characters (evidence: build/resume.go:24 `var buildID = regexp.MustCompile(`^[a-f0-9]{8}$`)`, used by both resume, build/resume.go:132, and recover, build/recover.go:164). Checks run in this order, before any other check, and nothing is created or written for any of these refusals:

1. **Not a Build id** (the argument does not match the format). Both commands refuse with exit 1, class `candidate`, kind `resume_unsupported` (resume) or `recovery_unsupported` (recover). The refusal detail (the `sys.Error` `Detail`, which is the standalone JSON `error.message`) is exactly:
   `<Q> is not a Build id; a Build id is 8 lowercase hexadecimal characters, such as 0a1b2c3d; kogen intent list lists Build attempts`
   Here `<Q>` is the argument quoted by Go's `strconv.Quote`, for example `"build-1" is not a Build id; ...` and `"" is not a Build id; ...`. The resume drift advice (`; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build`) and the recover advice (`; inspect the run/receipt and checkout; correct the named fact or use ordinary Refine and fresh exact approval`) are not appended. Resume's existing generic handling of refusals raised before a run exists is unchanged. It still adds its detail line `no newly created Build run; correct the named prerequisite before trying again` (evidence: build/build.go:78 `noRunNotice` and build/build.go:146-149 `conclude`, which appends it to every pre-run refusal).
2. **No recorded Build** (the id is well formed but `.kogen/local/runs/<id>` does not exist in the checkout). Same class, kinds and exit 1. The detail is exactly:
   `Build <id>: no Build with this id in this checkout; kogen intent list lists Build attempts`
   Neither advice suffix is appended. For example, `0a1b2c3d` in a checkout with no `.kogen/local/runs/0a1b2c3d` gives `Build 0a1b2c3d: no Build with this id in this checkout; kogen intent list lists Build attempts`.
3. **Recorded Build whose checkpoint cannot be used** (the run directory exists). The checkpoint refusals keep their kinds, exit 1 and advice suffixes. Their reason gets the prefix `unknown checkpoint: ` followed by the fact that failed:
   - Resume, `checkpoint.json` unreadable: `Build <id>: unknown checkpoint: absent or legacy checkpoint; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build` (kind `resume_unsupported`).
   - Resume, every reason returned by `decodeCheckpoint` (for example `malformed required checkpoint fields`, `incompatible checkpoint version 2`, or the legacy decoder's own error text): `Build <id>: unknown checkpoint: <reason>; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build` (kind `resume_refused`).
   - Recover, `checkpoint.json` absent: `unknown checkpoint: absent checkpoint; inspect the run/receipt and checkout; correct the named fact or use ordinary Refine and fresh exact approval` (kind `recovery_unsupported`).
   - Recover, unsupported version: `unknown checkpoint: legacy checkpoint version <v>; ...` (kind `recovery_unsupported`).
   - Recover, every `decodeCheckpoint` reason: `unknown checkpoint: <reason>; inspect the run/receipt and checkout; correct the named fact or use ordinary Refine and fresh exact approval` (kind `recovery_refused`).

All other refusals, and every valid resume and recover, are unchanged. The bare `unknown checkpoint` refusal without a fact is removed.

## Decisions
- A malformed argument is refused with its own reason: it is not a Build id, what a Build id looks like (8 lowercase hexadecimal characters), and the next command (`kogen intent list` lists Build attempts). It carries no drift/fresh-approval advice. (source: request)
- Class `candidate`, kinds `resume_unsupported`/`recovery_unsupported` and exit 1 stay for malformed and unrecorded ids. (source: request)
- A well-formed id with no recorded Build says "no Build with this id in this checkout" plus `kogen intent list`. (source: request)
- The `unknown checkpoint` text is used only for a recorded Build whose checkpoint cannot be used, followed by the failed fact. (source: request)
- "What a Build id looks like" is the existing format: 8 lowercase hexadecimal characters (evidence: build/resume.go:24 `^[a-f0-9]{8}$`). Values outside it, such as `DEADBEEF`, `0a1b2c3` and `0a1b2c3d4`, are not Build ids; `0a1b2c3d` is. (source: request)
- Resume's generic pre-run detail line `no newly created Build run; correct the named prerequisite before trying again` is existing behavior that this change leaves as it is (evidence: build/build.go:78 and build/build.go:146-149). (source: request)

## Source comparison
- Request "Problem" (`build-1` refused with `unknown checkpoint`, wrong fresh-approval advice for a typo): addressed by rule 1. Scenarios run `build-1` through resume and recover and compare the exact detail.
- Request "Need" (format, "not one", `kogen intent list`; class and exit stay): rule 1. The format is the repository's existing Build id format (evidence: build/resume.go:24). The scenarios check values on each side of it: `DEADBEEF` (uppercase), `0a1b2c3` (7 characters) and `0a1b2c3d4` (9 characters) get rule 1, and `0a1b2c3d` gets rule 2.
- Request "well-formed id with no recorded Build": rule 2. "Recorded" means `.kogen/local/runs/<id>` exists, the directory every Build attempt owns (evidence: build/resume.go:135 and build/recover.go:167 read the checkpoint from `.kogen/local/runs/<id>`).
- Request "existing unknown-checkpoint text stays only for a recorded Build whose checkpoint cannot be used, and then names which fact failed" / "a recorded Build with a damaged checkpoint keeps an unknown-checkpoint refusal that names the failed fact": rule 3. In today's code, damaged-checkpoint refusals already name the fact (for example `malformed required checkpoint fields`) but do not say `unknown checkpoint`; the bare `unknown checkpoint` text comes only from malformed ids (evidence: build/resume.go:133, build/recover.go:165). Rule 3 makes the text literally true: `unknown checkpoint: <fact>`, with existing kinds and advice kept.
- Request "no run is created and no file changes": scenarios compare every checkout file (outside `.git`), HEAD, the Git index and the run list before and after, and check that recover's new output directory is not created.
- Request "Valid resume and recover behave as before": the existing integration tests `TestAnInterruptedBuildResumesAndLandsOnceInTheSameConversation` and `TestRecoveryArchivesRawFilteredBinaryModeSymlinkAndDeletedPathsWithoutIndexOrIgnoredChanges` (both pass on the unchanged tree) must still pass.
- `kogen intent resume`/`recover` and `kogen intent list` are assembled in Kogen, not this module. This module produces the refusal (`sys.Error`) that Kogen renders. Resume is exercised through this module's standalone `kogen-build resume`. `recover` has no standalone entry point here, so it is exercised through the exported `build.Recover` by a small Go test harness. The harness source is embedded in `features/steps/k-build-malformed-build-id-named__steps.js`; while acceptance runs, the step writes it to a scratch file outside the checkout and overlays it as a test of `./cmd/kogen-build`. The Build must not add a file at `cmd/kogen-build/k_build_malformed_build_id_named_acceptance_test.go`.
- The existing Intent `k-build-cmd-entry` resumes `00000000` expecting kind `resume_unsupported`. Rule 2 keeps that kind.

## Tests
- A malformed id refuses resume with the new exact detail, kind `resume_unsupported`, exit 1, no drift advice and no run or file change: acceptance scenario through `kogen-build resume`. Also update the `not an id` subtest of `TestResumeRefusals` in `build/resume_test.go` (ids `xyz`, `../../etc`, `DEADBEEF`, ``) to the new text. (scenario)
- A malformed id refuses recover with the new exact detail, kind `recovery_unsupported`, exit 1, without the recover advice, and nothing is written: acceptance scenario through `build.Recover`. Also update the `unknown id` case of `TestRecoveryRefusals` in `build/recover_test.go`; that table's shared advice assertion must not apply to it. (scenario)
- A well-formed id without `.kogen/local/runs/<id>` refuses resume and recover with `Build <id>: no Build with this id in this checkout; kogen intent list lists Build attempts`: acceptance scenarios. Also add the resume case to `TestResumeRefusals` and update the recover `absent checkpoint` case (id `00000000`) of `TestRecoveryRefusals` to the new text. (scenario)
- A recorded Build's unusable checkpoint is refused with `unknown checkpoint: <fact>`, keeping its kind and advice: acceptance scenarios for a `{broken` checkpoint and a run directory without `checkpoint.json`, through both commands. Also update the decode and `no checkpoint` cases of `TestResumeRefusals`, the `version 2` case of `TestRecoveryRefusals`, and the legacy resume tests in `build/legacy_test.go` and `build/legacy76_test.go` whose expected decode reasons now carry the prefix. (scenario)
- Valid resume and recover are unchanged: `TestAnInterruptedBuildResumesAndLandsOnceInTheSameConversation` and `TestRecoveryArchivesRawFilteredBinaryModeSymlinkAndDeletedPathsWithoutIndexOrIgnoredChanges` keep passing, and an acceptance scenario runs them. (existing)

## Feasibility
- none
