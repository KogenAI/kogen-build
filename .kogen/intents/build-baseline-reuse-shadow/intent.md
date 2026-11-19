# Observe baseline check reuse in shadow mode

Slug: build-baseline-reuse-shadow
Model: not applicable — the shadow only observes and keeps no state the Build depends on; its receipts are an append-only log
Acceptance:
- .kogen/intents/build-baseline-reuse-shadow/acceptance.feature
- features/steps/build-baseline-reuse-shadow__steps.js
- features/steps/build-baseline-reuse-shadow__tests/shadow_acceptance.go.tmpl

Build's check baseline gains an always-on shadow mode that only observes whether an earlier result could have been reused. For each configured check of the baseline it computes the reuse key through the Verification snapshot exported by the dependency row verify-export-check-reuse-key (one snapshot per baseline, not one per check), looks the key up in a receipt file, still executes the check exactly as today, and logs one `reuse_shadow` event. It appends a receipt after each baseline execution and, after each whole-Intent verification pass, a receipt for every check that ran in that pass, with the `reuse_sha256` Verification put on that check's command event, so the next Build starting on the landed tree finds it. No check is ever skipped, and no flag turns reuse on.

The pinned kogen-verification in go.mod is updated to the commit that landed that dependency row, and the consumes suite (consumes/kogen-verification) is extended for the new exported names. The key covers the check, its command, the start tree and the check environment, and ignores `KOGEN_ATTEMPT_ID`, as Verification's key does.

Receipt file: `kogen-reuse-shadow`, directly under the repository's git common directory (a sibling of `kogen-baselines`). One append-only JSON line per receipt, bounded to the newest 2000 lines. A receipt has `key`, `name`, `status` (exit status), `wall_ms`, `source` (`baseline` or `pass`) and `log` (the run log path).

`reuse_shadow` event, one per configured check at baseline, written after the check ran: `name`, `reuse_sha256` (the key), `would_reuse` (true only when a receipt with the same key has status 0), `receipt` (that receipt's `log`, empty string if none), `actual_status`, `actual_wall_ms`, `agrees` (false only when `would_reuse` is true and `actual_status` is not 0; otherwise true). An `agrees` false is also printed as one line to stderr. It never changes a verdict, a check order, an exit code, an existing event or timing beyond the key computation.

Any shadow failure (receipt file unwritable or corrupt, a probe error) is recorded as a `reuse_shadow_unavailable` event with a non-empty `reason`, at most once per run log, and the Build carries on unchanged.

Scope: build/baseline.go, one new file build/reuse_shadow.go for the shadow, go.mod, the consumes suite and Go test files in the build package. Every other file, and all documentation, README and design files, stay unchanged.

The dependency row is not in the kogen-verification version currently pinned (this checkout holds no Verification snapshot export), so the Build must pin the commit that landed it. If that commit cannot be found, the Build reports a blocker rather than inventing the export.

## Decisions
- The reuse key comes from the Verification snapshot of the dependency row, computed once per baseline; pass receipts use the `reuse_sha256` Verification puts on each check's command event. (source: request)
- go.mod pins the kogen-verification commit that landed the dependency row, and the consumes suite names the new exports. The names are unknown in this checkout, so the acceptance checks only that the pin changed and that the consumes suite for kogen-verification mentions reuse or the snapshot. (evidence: go.mod; consumes/README.md) (assumption: the exported names and commit cannot be read in this checkout, so the dependency row must have landed when the Build starts) (source: request)
- `agrees` is present on every `reuse_shadow` event and `reuse_shadow_unavailable` is logged at most once per run log, so the log has exactly one such event. (assumption: the request names agrees only for the false case) (source: request)
- The receipt file sits next to `kogen-baselines`, which baseline_cache.go places under the git common directory. (evidence: build/baseline_cache.go) (source: request)
- Every Build the acceptance test starts is approved first (`e.approve()`), because a landed Build consumes its approval and a second Build without it is refused with `intent/not_approved` and creates no run; the test then reads the one run that Build newly created, not the last run folder. This only repairs the test's own helper: the request's second-Build scenarios (a later Build finds an earlier receipt) are unchanged. (evidence: build/helper_test.go approve) (source: request)
- The count of Verification snapshot computations (one per baseline of two checks) is asserted by a Go test in the build package that the Build writes, since the counting hook depends on the unknown export. (assumption: the export offers a point where computations can be counted) (source: request)

## Source comparison
- Shadow events, receipts, the 2000-line bound, would_reuse semantics, stderr line, unavailable event and the no-skip constraint: addressed above.
- Verification snapshot key, the updated kogen-verification pin and the extended consumes suite: addressed above and in acceptance scenario 8.
- Scenarios 1 to 7 of the request: covered by the Go acceptance test run by acceptance.feature, with one subtest each, except the snapshot count of scenario 1 (see Decisions). Scenario 6 stores the golden list captured from the tree before this change (a two-check baseline, one green, one failing, on an empty receipt store): the complete list of event names of that run log, which the test compares, once `reuse_shadow` events are filtered out, with the list recorded in the test.
- Correction (builder-reported, independently reproduced from build/helper_test.go): addressed — the helper `shadowBuild` of shadow_acceptance.go.tmpl now approves the Intent before each Build and returns the single run that Build created, so s1, s4 and s7 observe the second Build's own events and receipt. Conflicting/omitted/superseded: none; every scenario, subtest, expectation, the golden list of s6, the receipt and event contract and the scope of the original request are unchanged, and no production behavior is relaxed.

## Tests
- would_reuse true after an earlier green receipt, and the check still runs: TestReuseShadowAcceptance/s1 (integration)
- exactly one Verification snapshot computation per baseline of two checks: a Go test in the build package written by the Build (unit)
- a changed tree gives would_reuse false and an empty receipt: TestReuseShadowAcceptance/s2 (integration)
- a failed earlier receipt is not reusable: TestReuseShadowAcceptance/s3 (integration)
- a would-reuse hit whose check fails logs agrees false and keeps the real failing verdict: TestReuseShadowAcceptance/s4 (integration)
- an unwritable or corrupt receipt file yields reuse_shadow_unavailable and an unchanged Build: TestReuseShadowAcceptance/s5 (integration)
- the baseline's other events, order and verdicts stay unchanged: TestReuseShadowAcceptance/s6 (regression)
- a whole-Intent pass receipt is found by the next Build on the landed tree: TestReuseShadowAcceptance/s7 (integration)
- the kogen-verification pin moves and the consumes suite names the new exports: scenario 8 of acceptance.feature (integration)

## Feasibility
- none
