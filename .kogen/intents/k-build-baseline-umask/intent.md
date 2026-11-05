# Make baseline identity tests umask-independent

Slug: k-build-baseline-umask
Model: not applicable — only the fixtures of two existing tests change; baselineIdentity and every other product state stay as they are
Acceptance:
- .kogen/intents/k-build-baseline-umask/acceptance.feature
- features/steps/k-build-baseline-umask__steps.js

This is a test-only fix of `TestBaselinePerBaseIdentity` and `TestBaselinePerBaseCustody` in build/baseline_cache_test.go (package `build`). Both tests build a fixture repository, commit it, add a second checkout with `git worktree add -q --detach <other> HEAD`, and require `baselineIdentity` (build/baseline_cache.go) to give both checkouts the same identity. `baselineIdentity` hashes each path's full `os.FileMode` (`put(uint32(st.Mode()))`), including the permission bits. Today the tests make the main checkout's files and directories with fixed modes: `os.WriteFile(..., 0644)` and `os.Mkdir(..., 0755)`. Git makes the worktree's files at `0666 &^ umask` and its directories at `0777 &^ umask`. So the two checkouts differ whenever the umask is not 022.

Reported on Linux (kogen-build main 92a5c7d, git 2.43.0, host kogen-linux-macbook as lanes): umask 0002 FAIL, 077 FAIL, 022 ok. It surfaced as a red consumer check when landing kogen-coordinator 99d3057. The delivery hosts now build under umask 022 (hostbin 803f139), so it no longer blocks landings. Reproduced on this checkout (macOS, git 2.54.0):
- under 0002, `TestBaselinePerBaseIdentity` fails at :159 with `worktree identity differs: <a> <b>`, and `TestBaselinePerBaseCustody` fails at :398 (`second events ...`: the worktree child ran the check fresh instead of reusing the baseline);
- under 077, `TestBaselinePerBaseIdentity` fails at :201 with `restored tree did not regain identity`, because the chmod step restores `tracked` to a hard-coded 0644 while the committed fixture had 0600;
- under 022, both pass.

After the change, both tests pass under umask 0002, 022 and 077, and keep detecting every change they detect today.

## The rule
- The fixtures give the main checkout the same file and directory modes that git gives a checkout of the same commit under the umask the test process runs with. Examples: create the committed files and directories through git (remove them after the commit and restore them with `git checkout -- .`), or derive the fixture modes from the process umask. The choice is the Build's, as long as the comparison stays a real one between the main checkout and the `git worktree add` checkout.
- The tests do not change the process umask (no `syscall.Umask`). `TestBaselinePerBaseCustody` runs `t.Parallel()` in the shared test process, and the requirement is to pass under the umask the tests are run with.
- `TestBaselinePerBaseIdentity` keeps every check it makes today, in the same sense:
  - main checkout and worktree identities are equal;
  - each of these changes the identity: different content of `tracked`, an untracked file, an ignored file, a chmod of `tracked` to 0755, the `link` symlink pointing elsewhere, different check command bytes, a changed environment variable, different executable bytes (child process);
  - restoring the tree regains the base identity.
  The chmod step restores the file's mode from before the chmod, whatever the umask made it, instead of a hard-coded 0644.
- `TestBaselinePerBaseCustody` keeps every check it makes today:
  - the first child in the main checkout runs the check (`command_result`, no `baseline_reuse`);
  - the child in the worktree reuses it (`baseline_reuse`, no `command_result`);
  - one cache entry is written under `.git/kogen-baselines`, without the secret;
  - an independent repository with the same content does not reuse it;
  - concurrent readers see either a reuse or a fresh run, never both;
  - broken entries are not reused.
- Product code does not change. build/baseline_cache.go keeps its exact bytes: hashing permission bits is intended, since the tests require a chmod to change the identity. Nothing else changes either: no other test, golden, go.mod/go.sum, README, CLI command or flag, or timeout. No model runs.

## Decisions
- The fix is in the two tests' fixtures; baselineIdentity's behaviour, including hashing permission bits, stays unchanged because the evidence shows the mismatch comes from the fixture, not the product (source: request)
- The tests pass under the umask they run with rather than forcing one (source: request)
- Both tests still detect the real changes they check today (source: request)

## Source comparison
- Addressed: "Both tests pass under umask 0002, 022 and 077". The acceptance Scenario Outline runs `go test -json -count=1 -run '^(TestBaselinePerBaseIdentity|TestBaselinePerBaseCustody)$' ./build` under `umask 0002`, `umask 022` and `umask 077`. Each run requires exactly one passing result for each test, exit 0 and a passing package.
- Addressed: "and still detect the real changes they check today". Three acceptance scenarios run the same command under umask 0002 with build/baseline_cache.go replaced, through a `go test -overlay`, by a copy whose baselineIdentity has one change:
  - it ignores permission bits (`put(uint32(st.Mode()))` → `put(uint32(st.Mode().Type()))`): TestBaselinePerBaseIdentity must fail;
  - it ignores file contents (`put(hex.EncodeToString(ch.Sum(nil)))` → `put("")`): TestBaselinePerBaseIdentity must fail;
  - it depends on the checkout path (`put(common)` followed by `put(root)`): both tests must fail.
  The checks listed in the rule above are kept as well; the review checks them.
- Addressed: "Product behaviour of baselineIdentity stays unchanged unless the change shows it is wrong". It is not wrong. The last scenario requires build/baseline_cache.go to keep sha256 `30291a24583d86f33582611b1aa13d67518e7d747f511aa121c5b832551452c7`.
- Addressed: "Test-only unless evidence says otherwise; no model runs; no new CLI commands or flags; no timeout raised" (see the rule above).
- Preserved literals: test names, file build/baseline_cache_test.go, the message `worktree identity differs`, umasks 0002/022/077, `0666 &^ umask = 0664`, kogen-build 92a5c7d, kogen-coordinator 99d3057, hostbin 803f139, git 2.43.0, host kogen-linux-macbook.
- Narrowed by observation: on macOS the 077 failure is TestBaselinePerBaseIdentity only (restore to 0644); TestBaselinePerBaseCustody passed under 077 there. The requirement covers both tests under all three umasks regardless.

## Tests
- TestBaselinePerBaseIdentity passes under umask 0002, 022 and 077: update its fixture so the main checkout's modes match git's checkout under the process umask, and restore the pre-chmod mode instead of 0644 (acceptance Scenario Outline) (regression)
- TestBaselinePerBaseCustody passes under umask 0002, 022 and 077: update its fixture (`.kogen` directory and `.kogen/keep`) the same way (acceptance Scenario Outline) (regression)
- The tests still detect permission, content and cross-checkout identity changes: unchanged assertions of both tests, checked by the three mutation scenarios under umask 0002 (existing)
- baselineIdentity is unchanged: TestBaselineCacheTreeAndCustody and the sha256 scenario for build/baseline_cache.go (existing)

## Feasibility
- Ran in this read-only external session in scratch locations outside the checkout (not Kogen probe records):
  - unchanged tree, `-run '^(TestBaselinePerBaseIdentity|TestBaselinePerBaseCustody)$'`: umask 0002 both FAIL (:159 `worktree identity differs`, :398 `second events`); 022 both PASS; 077 Identity FAIL at :201 `restored tree did not regain identity`, Custody PASS. Each run about 2 s after compilation.
  - `go test -overlay` with the three baselineIdentity changes under umask 022 on the unchanged tree: permission bits → Identity FAIL at :183, Custody PASS; file contents → Identity FAIL at :168, Custody PASS; checkout path → both FAIL (:159, :398). About 5 s each with compilation.
  - An overlay key with an absolute path did not apply when the checkout path is reached through a symlink (`/tmp` → `/private/tmp`), because go matches against its logical working directory. The step file therefore uses the relative key `build/baseline_cache.go`, which applied in both cases.
- evidence: {"id":"umask-run-judge","kind":"observer","inputs":["features/steps/k-build-baseline-umask__steps.js"],"observers":["features/steps/k-build-baseline-umask__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-baseline-umask__steps.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-baseline-umask__steps.js bad","exit":1,"output":"rejected\n"}]}
- The observer's inputs are real `go test -json` runs under umask 022 in the checkout:
  - known-good: the passing run of both tests, and the run in which both fail with the checkout-path change;
  - known-bad for the pass judge: that failing run, a run of Identity alone, a run with `-run '^$'` (no test), the permission-bits run (Identity failed);
  - known-bad for the fail judge: the passing run, the permission-bits run judged for Custody too, and a run whose overlay does not compile.
  In a scratch copy, good printed `accepted` (exit 0) and bad printed `rejected` (exit 1), in about 10 s each. The controls qualify the judges only, not the fix.
