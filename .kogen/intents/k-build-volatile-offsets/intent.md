# Require kogen-loop main and mask volatile fields

Slug: k-build-volatile-offsets
Model: not applicable — a dependency pin and test-only changes to a golden comparison and one test fixture; no product state or transitions change
Acceptance:
- .kogen/intents/k-build-volatile-offsets/acceptance.feature
- features/steps/k_build_volatile_offsets_steps.js
- features/steps/k_build_volatile_offsets_test.go.txt
- features/steps/k_build_volatile_offsets_controls/good.txt
- features/steps/k_build_volatile_offsets_controls/bad-unchanged.txt
- features/steps/k_build_volatile_offsets_controls/bad-main-unmasked.txt
- features/steps/k_build_volatile_offsets_controls/bad-overmask.txt

K-BUILD-VOLATILE-OFFSETS. kogen-build moves its kogen-loop requirement from c6e2360 (2 Nov) to current kogen-loop main a5aa5f4, and its two tests that break against that main pass again: TestBuildRequestBudgetOff (build/build_request_budget_test.go:326) and TestAnArchivedWindowThatChangedIsRefused (build/context_test.go:118, "no archived window"). The proof is both tests passing 3 times in a row against kogen-loop main. No product behaviour changes and no timeout is raised: only `go.mod`, `go.sum` and `_test.go` files change, and no non-test Go file is edited.

What happens today (measured in a scratch copy of HEAD ca40c39 with only the kogen-loop replace moved to a5aa5f4; both tests pass with the current pin):
- kogen-loop main's L-ATTEMPT-GAP-LOAD adds two keys to every provider attempt record, so to the `provider` object of every `provider_attempt` event in a run's `log.jsonl`: `request_offset_ms` (integer milliseconds from the request start to the attempt start, observed 0 on one Build and 1 on the next) and `load_average` (the host's 1/5/15-minute load, three numbers rounded to two decimals, JSON `null` where unavailable; observed `[22.31,16.71,10.5]` against `[22.76,16.89,10.6]` in the two Builds of one test). `observeBuild` (build/build_request_budget_test.go) masks only the keys named in `volatileKeys`, so TestBuildRequestBudgetOff's comparison of its two Builds fails with "log line 3 differs at byte 1058" (or line 9, whichever attempt first differs).
- kogen-loop main's L-CUT-LINE-ALIGNED cuts tool output at complete lines. The 20000-byte single-line output of `head -c 20000 /dev/zero | tr '\0' x` now reaches the model as 12302 bytes (`exit status: 0\n` head, `[... 7743 bytes omitted ...]` marker, byte tail) instead of 16383. The builder's working window therefore first exceeds the 524288-byte input budget after 40 shell results (the request before the reset measured 512690 bytes) instead of after 31. TestAnArchivedWindowThatChangedIsRefused queues only 36 shell calls before its hanging request, so no reset (no archive) happens before the interrupt and `history` is empty. Archiving itself (path, 0600 mode, hash, custody refusal on resume) is unchanged in kogen-loop main and in kogen-build.
- Every other test of `go build ./... && go vet ./... && go test ./...` passes against kogen-loop main.

After this change:
1. go.mod's replace is exactly `replace kogen-loop => github.com/KogenAI/kogen-loop v0.0.0-20261103193244-a5aa5f4ee775` (the pseudo-version of a5aa5f4ee775f8279a442ff02bb52c10355e9f4f, kogen-loop's main at shaping time), and go.sum is the `go mod tidy` result: it carries `github.com/KogenAI/kogen-loop v0.0.0-20261103193244-a5aa5f4ee775 h1:0NcEoKZu/6sYsUyUUKGRm1vtRU9rHD9TDbJHavp8Go8=` and its `/go.mod h1:xuHxDT/F3nJeem9k/bSAtkJBPpQVnHpM4T3Sm/rCzRg=` line and no line for c6e2360290d4 (tidy also drops the unused kogen-review 11d83000d2f2 lines). The `require kogen-loop v0.0.0` line and every other requirement stay.
2. `observeBuild`'s mask (the `walk` over every decoded line of requests, log.jsonl, checkpoint.json and result.json) replaces with `"<volatile>"`, at any depth, the value of every key that:
   - is in `volatileKeys` (unchanged: the existing timing, clock and random identity keys), or
   - is `load_average` (added to `volatileKeys`: machine-dependent host load), or
   - ends in `_ms` (every millisecond offset or duration, `request_offset_ms` and any later `*_offset_ms` or `*_ms` included; this also masks the fixed `budget_ms`, whose text `budget` stays compared), or
   - ends in `_at` (every timestamp key besides the already named `at`, `timestamp`, `t0` and `started_at`, for example `ended_at`).
   Both sides of a comparison go through the same mask. Everything else (other keys, `cache_ids` handling, the path and run-id replacements and the hash masking) stays as today and is compared byte for byte; in particular `provider_request_id`, `pass`, `outcome`, `stream_bytes` and any new non-timing key still make two runs differ.
3. TestAnArchivedWindowThatChangedIsRefused queues 44 of the same shell calls (was 36) before its hanging request, so the window is reset and archived before the hang with margin over the 40 needed against kogen-loop main; the command, the interrupt, the tampering and every assertion (history nonempty, exit 1, the `candidate/resume_refused: working-context history custody missing, changed or unsafe: <path>; inspect checkpoint and archive before fresh approval` output, no further request) stay. Its comment says why the count leaves margin over the reset point.
4. A focused unit test in build/build_request_budget_test.go shows the mask catches what it should, through `observeBuild` of a recorded Build: rewriting `request_offset_ms`, `load_average` or another `*_ms` value of the run's provider attempt record leaves the observation equal; rewriting a non-timing value (for example `provider_request_id` or `pass`) makes it differ.

Existing test helpers the acceptance harness calls keep their names, signatures and meaning: `newEnv(t)`, `(*env).prov` with `then` and `landScript(f)`, `(*env).build()` (with `.exit` and `.shown()`), `(*env).onlyRun()`, `(*env).runPath(id, name)`, `observeBuild(t, e, id) budgetObservation` and the `budgetObservation` fields `requests`, `log`, `checkpoint`, `result` and `outcome`.

## Decisions
- kogen-build's kogen-loop requirement moves to current kogen-loop main, pinned to a5aa5f4 (v0.0.0-20261103193244-a5aa5f4ee775) (source: request)
- Every timing value is volatile in the golden comparison: `request_offset_ms` and any other `*_offset_ms`, `*_ms` duration or timestamp field is normalised to `<volatile>` on both sides; everything else is compared byte for byte (source: request)
- Machine-dependent fields such as `load_average` are masked the same way (source: request)
- The archived-window test adapts to how kogen-loop main archives windows rather than changing product behaviour (source: request)
- No product behaviour change; no timeout raised (source: request)
- Both tests must pass 3 times in a row against kogen-loop main (source: request)

## Source comparison
- Addressed: Rescoped need "move kogen-build's kogen-loop requirement to current kogen-loop main" by rule 1 and scenario "kogen-build requires kogen-loop main a5aa5f4"; the literal pin is the pseudo-version observed for main a5aa5f4ee775 (`git ls-remote` HEAD on 3 Nov).
- Addressed: Rescoped "masking machine-dependent fields such as load_average the way the other volatile fields are masked" and the original Need's `request_offset_ms`/`*_offset_ms`/`*_ms`/timestamp masking by rule 2, the attempt-record scenario and the comparison table rows `request_offset_ms`, `load_average`, `load_average null`, `every _ms number`, `timestamp`, `event at`, `new timing keys`.
- Addressed: "Show the normalisation catches what it should: a run with a different offset passes, and a change to any non-timing field still fails" by rule 4 and the table rows `provider_request_id`, `pass`, `outcome`, `stream_bytes`, `new non-timing key` (each `differs: log`).
- Addressed: Rescoped "adapting the archived-window expectation to how kogen-loop main archives windows" by rule 3; the measured cause is the line-aligned shell output cut (12302 instead of 16383 bytes per result), not timing or host load.
- Addressed: "Proof: both tests pass 3 times in a row against kogen-loop main" by scenario "Both broken tests pass three times in a row against kogen-loop main" (three separate uncached `go test -count=1` runs, after asserting the pin).
- Addressed: "No product behaviour change; no timeout raised" by the boundary that only go.mod, go.sum and `_test.go` files change.
- Superseded: the "Problem" and "Also (3 Nov 18:3xZ)" timing-flake framing (request_offset_ms 0 vs 1 as a flake; the archived window as possible asynchronous archive or short wait), as the request's Rescoped section states; its masking need is kept within rule 2. The archive is written synchronously before the request (build/context.go prepareContext) and the test has no wait to lengthen.
- Omitted: consumer flake-list edits and kogen-loop changes: they live outside this repository.

## Tests
- The golden comparison masks `load_average`, `request_offset_ms`, every `*_ms` and every `*_at` key on both sides: update `volatileKeys`/`observeBuild` so TestBuildRequestBudgetOff passes against kogen-loop main (regression)
- A changed timing value compares equal and a changed non-timing value still differs: add a focused test in build/build_request_budget_test.go rewriting a recorded Build's provider attempt record and comparing `observeBuild` results (unit)
- The archived window exists before the hanging request against kogen-loop main and its tampering is refused: update TestAnArchivedWindowThatChangedIsRefused to 44 shell calls (regression)
- kogen-build builds, vets and passes its whole suite against kogen-loop main a5aa5f4: move the replace in go.mod and tidy go.sum; the configured checks run every existing test (integration)

## Feasibility
- kogen-loop main is reachable: `git ls-remote https://github.com/KogenAI/kogen-loop.git` showed `a5aa5f4ee775f8279a442ff02bb52c10355e9f4f refs/heads/main`; `go mod edit -replace kogen-loop=github.com/KogenAI/kogen-loop@a5aa5f4ee775...` and `go mod tidy` resolved `v0.0.0-20261103193244-a5aa5f4ee775` with the go.sum lines quoted above. (scratch copy, exit 0)
- Against kogen-loop main with the unchanged tests, `go test -count=1 -run 'TestBuildRequestBudgetOff$|TestAnArchivedWindowThatChangedIsRefused$' ./build/` failed both: "log line 3 differs at byte 1058 ... "load_average":[22.31,16.71,10.5] ... [22.76,16.89,10.6]" and "context_test.go:118: no archived window"; the full `GOFLAGS=-trimpath go test -count=1 ./...` failed only these two. (scratch, exit 1, 250 s)
- A scripted Build of 50 shell calls showed each 20000-byte shell result is 12302 bytes and the first reset happens on request 41 against main, against 16383 bytes and request 32 with the c6e2360 pin. (scratch, exit 0)
- Prototype (load_average in volatileKeys, `_ms`/`_at` suffix mask, 44 shell calls): `test -z "$(gofmt -l .)"`, `go vet ./...` and `GOFLAGS=-trimpath go test -count=1 ./...` passed (build 258 s, contract 6 s); the two tests passed in `-count=3` and in three separate `-count=1` runs; all four acceptance scenarios passed; on the unchanged tree all four fail. (scratch, exit 0)
- The harness controls are its real outputs: good from the prototype against main; bad-unchanged from the unchanged tree (c6e2360 pin); bad-main-unmasked from main with the unchanged mask; bad-overmask from main with a mask that also hides every number and `outcome`.
- evidence: {"id":"kbvo-observer","kind":"observer","inputs":["features/steps/k_build_volatile_offsets_steps.js","features/steps/k_build_volatile_offsets_test.go.txt",".kogen/intents/k-build-volatile-offsets/acceptance.feature","features/steps/k_build_volatile_offsets_controls/good.txt","features/steps/k_build_volatile_offsets_controls/bad-unchanged.txt","features/steps/k_build_volatile_offsets_controls/bad-main-unmasked.txt","features/steps/k_build_volatile_offsets_controls/bad-overmask.txt"],"observers":["features/steps/k_build_volatile_offsets_steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k_build_volatile_offsets_steps.js --control good","exit":0,"output":"accepted good\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k_build_volatile_offsets_steps.js --control bad","exit":1,"output":"rejected bad-unchanged\nrejected bad-main-unmasked\nrejected bad-overmask\n"}]}
