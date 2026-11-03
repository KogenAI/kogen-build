# Run each check once per tree within a Build

Slug: k-baselineonce
Model: .kogen/intents/k-baselineonce/model.qnt invariants=ReuseOnlyPassed,GreenOnFullSuite,LandsOnFullSuite seed=7 samples=2000 steps=12
Acceptance:
- .kogen/intents/k-baselineonce/acceptance.feature
- features/steps/k_baselineonce_steps.js
- features/steps/k_baselineonce_test.go.txt
- features/steps/k_baselineonce_controls/good.json
- features/steps/k_baselineonce_controls/bad.json

Today a Build runs every configured check at startup (`recordBaseline` in `build/baseline.go`, "checking <name> before the Build changes anything"). It runs every configured check again, in configured order, in every whole-Intent verification pass (`(*Attempt).Verify` in `build/build.go`, which calls kogen-verification's `verify.Verify`). It does this even when the pass verifies a tree on which the check already passed in this Build: the tree the baseline ran on, the tree of a pass whose builder round made no edit, or a resume of the same tree.

After this change, a Build keeps a ledger of the checks that passed in it, keyed by the check, the tree and the environment that can change the result. A whole-Intent pass reuses a recorded pass on the same key instead of running the check, and the run log records each reuse. In fix and repair loops, the checks the previous pass failed run first, and the rest of the configured suite runs after them, once, on the same tree. A pass is green, and a Build lands, only when every configured check passed on that pass's own tree and environment in this Build. The ledger is part of the persisted Build state, so a resume of the same Build reuses it.

Boundaries:
- The reuse is within one Build and its resumes only. A new Build starts with an empty ledger.
- The existing per-base baseline reuse across Builds (`baselineIdentity`, the `baseline_reuse` event and its evidence under `kogen-baselines`, only for checks whose command is exactly `:`) stays exactly as it is.
- The startup baseline still runs every configured check once per Build. It records into the ledger and never reuses from it.
- Stage gates (`verifyStage`) neither reuse nor record.
- No CLI command, flag or `project.yaml` key changes.
- kogen-verification, kogen-coordinator, kogen-review, kogen-executor and kogen-loop do not change. The change lives in kogen-build's `build` package and its tests.

## Rules

1. The ledger. `State` (in `build/core_state.go`, the type the Build saves and restores) gains `CheckResults []checkResult`, JSON key `check_results`, `omitempty`. Each record has the string fields `name`, `command`, `tree`, `environment`, `key` and `output`. The ledger is saved in every checkpoint and restored by `kogen intent resume` like every other `State` field. A checkpoint without the key decodes to an empty ledger. A state without records encodes exactly as today, so the state goldens do not change. A record is only ever made from an execution in this Build that exited 0. It holds at most one record per configured check: recording a check replaces that check's previous record.
2. The tree of a check point is `repo.Tree(b.Start, b.Excluded)`: every tracked file and every untracked file that is not ignored, with content and mode, without other Intents' folders. This is the tree id that `verify.Verify` reports as `pass.Tree`.
3. The environment of a check is a lowercase hex SHA-256 over three parts:
   - (a) The environment the check's command gets: the session's variables (`sys.Session.Env`, where a later duplicate wins, as in `LookupEnv`), plus `KOGEN_NODE`, `KOGEN_QUINT` and `KOGEN_CUCUMBER` from the Build's `verify.Tools` when they are not empty, sorted by name.
   - (b) The toolchain on that environment's `PATH`: for each directory in `PATH` order, each entry sorted by name, with its mode, size and modification time after following symbolic links. An unreadable directory or entry is recorded as unreadable.
   - (c) Each whitespace-separated word of the check's command that, without surrounding single or double quotes, is the absolute path of a readable regular file: that path and the SHA-256 of the file's content. For example, the configured `go-test` check names `heavy.sh` and `go-test.sh` this way.

   (a) and (b) are computed once per check point, and (c) once per check. This is the toolchain identity the request asks for. It is not exhaustive: a version switch that changes none of these inputs (for example, a global configuration file outside `PATH` that a shim reads) is not detected.
4. The key of a check is the lowercase hex SHA-256 (64 characters) of its name, command text, tree and environment.
5. The baseline records. `recordBaseline` computes the tree and the environment before its first check and again after its last. When both are unchanged, it records each check that executed in the baseline and exited 0, with its capped output (`verify.Cap`). A check reused through the per-base evidence (`baseline_reuse`) did not execute and is not recorded. The baseline's check commands get the same `KOGEN_NODE`, `KOGEN_QUINT` and `KOGEN_CUCUMBER` variables (non-empty ones only) that a pass gives its checks, so that the recorded environment is the one the check saw. Nothing else about the baseline changes: its events, timings, stdout lines, `check_baseline` and the per-base path stay as they are. An `Attempt` built without a repository, as the per-base tests build it, must still work: those tests call `recordBaseline` twice and expect two executions of every non-`:` check.
6. A pass reuses. In `(*Attempt).Verify`, which serves the builder loop's passes, the routed passes and the reviewer's repair passes (`reviewEnv.Verify`), the Build computes the tree and the environment before it calls `verify.Verify`. A configured check is reused when the ledger holds a record whose `name`, `command`, `tree` and `environment` each equal the check's current ones. For each reused check, in configured order, the Build:
   - logs the run-log event `{"event": "check_reuse", "name": <check>, "command": <command>, "key": <key>, "tree": <tree>, "pass": <pass number>, "message": "baseline reused: <check> <key>"}`;
   - prints the progress line `kogen: baseline reused: <check> <key>` on stderr.

   The event is never named `baseline_reuse`: that name stays reserved for the per-base reuse.
7. Targeted first. The configured checks that are not reused run in this order. First come those that the Build's last whole-Intent pass (`b.LastPass`, including a pass restored by a resume) recorded with a nonzero status, in configured order. Then come the rest, in configured order. With no previous pass, every check runs in configured order. `verify.Verify` gets a copy of the configuration whose `Checks` are this list; `Setup`, `Acceptance` and every other field are unchanged. So the Intent's acceptance scenarios, its model and the maintained models still run first, and the first red item still stops the pass. These targeted tests are the Intent's acceptance and model plus the checks the previous pass failed. The rest of the configured suite runs after them on the same tree, in the same pass, and only when they pass. Each configured check therefore runs at most once per tree and environment.
8. The pass the Build keeps (`b.LastPass`, and through it the builder's and reviewer's feedback, the review, the checkpoint, `result.json`, the facts and the comparison):
   - `Checks` are in configured order. A reused check is `{name, command, status 0, output: the recorded output}`. An executed check is its result. A check that neither ran nor was reused is absent.
   - `NotRun` is verify's entries that are not configured checks, in its order, then the absent configured checks in configured order.
   - `Failing` is verify's entries that are not configured checks, in its order, then the configured checks with a nonzero status in configured order.
   - `Green` is verify's `Green`, and the pass tree equals the tree the reuse was decided on, and no configured check is absent. When `pass.Tree` differs from that tree, the reused checks count as not run (named in `NotRun`) and the pass is not green.
   - The `pass` run-log event and the pass summary line that `verify.Verify` writes itself stay verify's own account of the checks it ran.
9. A pass records. When `pass.Tree` equals the tree the reuse was decided on and `pass.Moved` is empty, each configured check that executed in the pass and exited 0 is recorded with its output, replacing that check's previous record. A failed result is never recorded, so a failing or flaky check runs again at every check point, and the host-or-base rule (`hostOrBaseStop`) and the no-progress watch see the same failures as today.
10. The comparison finds results by name. `comparison` (used by `comparisonLines`, `comparisonRecord` and the stop report) and `reviewBaselineMessage` find a check's result in the pass by its name instead of by its position. The baseline side stays positional, as today. A check absent from the pass shows "not run" as today.
11. Per-base reuse is unchanged. `baselineIdentity`, `readBaselineEvidence`, `publishBaselineEvidence`, the `baseline_reuse` event (`name`, `identity`, `evidence`), the `:` eligibility and the evidence directory stay byte-for-byte as they are. kogen's `TestBaselinePerBaseInstalled` and `TestBaselinePackagingIsolation` expect a second Build on one base to log exactly one `baseline_reuse` event that names the first Build's log, and to run no `closed` command before the change. That stays so.

## Decisions
- Reuse is keyed by check name, command text, tree and environment (rules 2–4). Any change to the tree, the toolchain or the command runs the check again. (source: request)
- The run log records each reuse as a `check_reuse` event whose `message` is "baseline reused: <check> <key>" (rule 6). (source: request)
- Only results that passed are recorded and reused, so a check is never skipped on a tree it has not passed on (rules 1, 6 and 9). (source: request)
- The targeted tests of a fix or repair loop are the Intent's acceptance and model (already first in every pass) plus the configured checks the previous whole-Intent pass failed. The full configured suite then runs once on that final tree. A pass is never green on the targeted tests alone (rules 7–8). The request forbids new configuration and gives no other source of targeted tests. (source: request)
- Within one Build and its resumes only; the per-base reuse across Builds is unchanged (rule 11; the boundaries above). (source: request)
- Flaky-check handling and failure output are unchanged: failures are never reused, failure text and the comparison lines are the same, and the order of an unreused, untargeted pass is today's (rules 8–10). (source: request)

## Source comparison
- Why (a Build reruns the configured checks on an unchanged tree; the startup baseline in `build.go` "checking <name> before the Build changes anything"; 9–26% of build time): addressed by rules 1–9. In this repository the startup line is in `build/baseline.go` (`recordBaseline`, called from `begin` in `build/build.go`). The baseline runs once per Build. The repeats on an unchanged tree are the whole-Intent passes after it, and those passes reuse.
- What to build 1 (record each check's result keyed by tree plus toolchain versions and command text; a later run on the same key reuses and logs "baseline reused: <check> <key>"; any change reruns): addressed by rules 1–6 and 9. Acceptance scenarios 1–4 cover it: unchanged and changed tree, toolchain change, command change, and resume.
- What to build 2 (fix and repair loops run the targeted tests first, then the full configured suite once at the end on the final tree; never pass on targeted tests alone): addressed by rules 7–8, acceptance scenarios 5–6 and the model invariants `GreenOnFullSuite` and `LandsOnFullSuite`.
- Must stay, no CLI command, flag or project.yaml key: addressed (Boundaries).
- Must stay, a check is never skipped on a tree it has not passed on: addressed by rules 1, 6 and 9, and the model invariant `ReuseOnlyPassed`.
- Must stay, flaky-check handling and failure output unchanged: addressed by rules 8–10 and the existing failing-check and host-or-base tests.
- Must stay, per-base `baseline_reuse` across Builds exactly as it is: addressed by rule 11, acceptance scenario 7 and the existing `TestBaselinePerBase*` tests.
- Must stay, no change under `cli/third_party/kogen-loop/**` or `cli/loop-pin.json`: these paths are not in this repository, and kogen-loop is not changed.
- Acceptance, "unchanged tree: one execution and a reused record": scenario 1, mode `unchanged`.
- Acceptance, "changed tree reruns; toolchain or command change reruns": scenario 1, mode `tree`, and scenarios 2 and 3.
- Acceptance, "fix loop: targeted first, full suite exactly once at the end; a failing final suite does not pass": scenarios 5 and 6.
- Acceptance, "all configured checks pass": the configured checks `format`, `compile`, `vet` and `go-test`.
- Landing attempt 3 Nov 07:41Z (Build 894bd9fb; the second Build ran the baseline again and logged no `baseline_reuse`): addressed by rule 11 and scenario 7. The scenario observes exactly one `baseline_reuse` that names the first Build's log and has a 64-hex identity, and no `closed` command before the change. The earlier candidate diff is not used.
- Earlier attempt (Build 7d18c4e0 kept its ledger on a type that moved, so `b.CheckLedger` did not compile after the rebase): addressed by rule 1. The ledger is a field of `State` in `build/core_state.go`, the type that now holds the Build's durable state. It is reached as `b.CheckResults` through the embedded `State` of `Attempt` in `build/build.go`.
- Host safety (shared host; stop only pids you started; no synthetic CPU load; temp dirs only): the harness and the new tests use temporary directories, the in-process scripted provider and short shell checks. The resume fixture interrupts only its own session (`Interrupt("SIGINT")` on the Build it started).
- Repository split (built in kogen-build; other repositories get follow-up items): this contract changes only kogen-build. `verify.Verify` is used unchanged through a copy of the configuration.

## Tests
- A pass on the tree and environment of a passed baseline reuses it: one execution of the check, a `check_reuse` event with a 64-hex `key` and the message "baseline reused: <check> <key>", a green pass with the check passed, and a landed Build. Acceptance scenario 1, mode `unchanged` (scenario)
- The same reuse through `Build` with the scripted provider, asserting one execution, the `check_reuse` event and its message: a new test in `build/checkreuse_test.go` (integration)
- A changed tree runs the check again: scenario 1, mode `tree` (scenario)
- A changed tree runs the check again and logs no `check_reuse`: a case of the new test in `build/checkreuse_test.go` (integration)
- A changed PATH tool runs the check again on the same tree: scenario 2 (scenario)
- A record whose command differs from the check's command is not reused: scenario 3 (scenario)
- The ledger survives a resume, and a resumed pass on the same tree reuses it: scenario 4 (scenario)
- The ledger is saved in the checkpoint's `check_results` and reused after `Resume`: a new test in `build/checkreuse_test.go` that interrupts at a builder request and resumes (integration)
- The environment identity changes when a session variable, a `PATH` entry's file or a command-named file changes, and stays the same otherwise. The key changes with the name, the command, the tree and the environment: a new unit test in `build/checkreuse_test.go` of the identity and key computation (unit)
- Checks the last pass failed run first, and the rest run once after them on the same tree: scenario 5 (scenario)
- The execution order of a fix loop through `Build`: a new test in `build/checkreuse_test.go` (integration)
- A pass whose targeted check passes and whose later full-suite check fails is not green, and the Build does not land: scenario 6 (scenario)
- The kept pass lists checks in configured order, with reused results passed and carrying their recorded outputs: the existing `TestFailingCheckBuildRegression`, whose second pass reuses `old` and must still show `old` as passed with its empty output (existing)
- The comparison finds a check's result by name: a new unit test of `comparison` with a pass whose checks are not a prefix of the configuration (unit)
- Per-base reuse is unchanged next to the reuse within a Build: scenario 7 (scenario)
- Per-base reuse is unchanged: the existing `TestBaselinePerBaseEligibility`, `TestBaselinePerBaseIdentity`, `TestBaselinePerBaseFailures`, `TestBaselinePerBaseCustody` and `TestBaselineCacheTreeAndCustody` pass unchanged (existing)
- The checkpoint format of states without records, the goldens and every other behavior are unchanged: the existing state-golden, failing-check, baseline-wall, resume and review tests and all configured checks (`format`, `compile`, `vet`, `go-test`) pass without `-update` (existing)

## Feasibility
- These are scratch checks from this read-only external session, run outside the checkout in temporary copies of HEAD 137633f. They are not Kogen probe records.
  - On the unchanged tree, the harness builds with `go test -c -vet=off -overlay` in about 2 s with a warm cache, and each mode runs in 1–3 s. The fixtures reach the intended paths through `Build` and `Resume`: a Build with no edit lands; an interrupted builder request resumes and lands; a provider refusal after a red pass stops without a commit; and a second Build on one base logs exactly one `baseline_reuse` naming the first Build's log. Cucumber runs 7 scenarios. All 7 fail on missing-feature observations, for example `executions` `["lint","lint"]` instead of `["lint"]` and no `check_reuse` events. The mode `tree` step, the counterexample, passes and is exercised.
  - A scratch prototype of rules 1–10 in a copy of HEAD passed all 7 scenarios (about 20 s). With the prototype, the whole package passed: `go test -count=1 ./build`, ok in 309 s, run through the host's `heavy.sh`. That run includes the state goldens, the per-base tests and the failing-check tests. Before the prototype kept outputs in the ledger, a reused result carried a placeholder output instead of the recorded one, and `TestFailingCheckBuildRegression` failed on it. That failure is why rules 1 and 8 keep the recorded output.
  - The Quint model typechecks and `quint run --backend=typescript --invariant='ReuseOnlyPassed and GreenOnFullSuite and LandsOnFullSuite' --seed=7 --max-samples=2000 --max-steps=12` finds no violation. In a copy whose reuse ignores the environment, the same run finds a violation.
- The observer controls take recorded real harness runs. The good control holds the prototype's observation for every mode, and it accepts all of them against the feature's expected values. The bad control rejects every recorded unchanged-tree observation and every altered observation of a plausible wrong implementation: a key that ignores the tree, the toolchain or the command; a malformed reuse record; a Build that lands on its targeted check alone; a fix loop that runs the targeted check twice; a resume that loses the ledger; a second Build that runs the per-base baseline again; and within-Build results carried into the next Build. These are observer controls only, not product evidence.
- evidence: {"id":"baselineonce-observer","kind":"observer","inputs":["features/steps/k_baselineonce_controls/good.json","features/steps/k_baselineonce_controls/bad.json","features/steps/k_baselineonce_test.go.txt",".kogen/intents/k-baselineonce/acceptance.feature"],"observers":["features/steps/k_baselineonce_steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k_baselineonce_steps.js --control good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k_baselineonce_steps.js --control bad","exit":1,"output":"observer rejected\n"}]}
