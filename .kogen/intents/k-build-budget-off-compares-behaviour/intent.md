# Compare observed behaviour in the budget-off test

Slug: k-build-budget-off-compares-behaviour
Model: not applicable — only one test's comparison changes; no product state, transition or output changes
Acceptance:
- .kogen/intents/k-build-budget-off-compares-behaviour/acceptance.feature
- features/steps/k-build-budget-off-compares-behaviour__steps.js

`TestBuildRequestBudgetOff` (build/build_request_budget_test.go) checks that a Build with the request budget switched off behaves like a Build without a budget. Today it compares the two Builds' whole decoded requests, `log.jsonl`, `checkpoint.json`, `result.json` and commit message with `reflect.DeepEqual`, after `observeBuild` masks a hand-kept list of volatile keys (`volatileKeys`, `volatileKey`), path and run-id texts, cache identities and hashes. Any field a producer adds that differs between two otherwise identical Builds turns it red. Examples are timing offsets, host load, sweep durations, and the `status` of a toolchain in `check_identity`. In a scratch run under load, `node` showed `"status":"timed_out"` in one Build and `"ok"` in the other. Each time, the fix has been one more key in the list.

After this change the test compares only what a user or caller of Build observes, taken from the Build's documented outputs. It no longer compares whole files, logs or requests. The masking list, `observeBuild` and the tests of that list are removed. No product code, timeout or limit changes. Only `build/build_request_budget_test.go` changes.

**Builds.** The two Builds keep today's fixture and order. The test first runs the budget-omitted Build, then the budget-off Build (`in.BuildRequestBudget = 0`), one after the other. Each Build runs in its own `newEnv(t)`, created right before it. Before each Build, the test queues today's script on that env's fake provider with `f.then`: five `read_file` calls, `redTry(f, 1)`, `redTry(f, 2)`, then `landScript(f)`. The test runs each Build through `e.build(...)`, and the mutate function also sets `in.Deps.Outcome` to a fresh `Outcome` so the test can read the stop code. The test records `e.head()` before each Build as that Build's start. It runs no further Builds, resumes, waits or retries.

**Reference.** The budget-omitted Build must end with exit code 0 and 8 builder requests. If not, the test stops with `t.Fatalf`. The budget-off Build has no separate assertions of its own. It is judged only by comparison with the reference and by the request-budget check below. So a budget-off Build that stops or differs is reported, not fatal.

**Observations.** The test takes these observations from each Build. The names in the first column are the exact names used in failure messages.

| Name | Value |
| --- | --- |
| `exit code` | the exit code `Build` returned (int) |
| `stop code` | `<Class>/<Reason>` of `Outcome.Error`, as the standalone command's stdout `error.category`/`kind`; `""` when `Outcome.Error` is nil (string) |
| `outcome` | `outcome` of the run's `result.json` (string) |
| `commits` | `git rev-list --count <start>..HEAD` (int) |
| `parent` | whether HEAD's first parent is the start commit, read from `git rev-list --parents -n 1 HEAD` so a parentless HEAD does not fail (bool) |
| `tree` | `git diff-tree -r --no-commit-id <start> HEAD`: the tree change from the start commit, comparable across the two temporary projects (string) |
| `message title` | `git log -1 --format=%s` (string) |
| `trailer` | `git log -1 --format=%(trailers)` (string) |
| `status` | `git status --porcelain` (string) |
| `builder requests` | `f.count() - len(f.reviewRequests())` (int) |
| `review requests` | `len(f.reviewRequests())` (int) |
| `events` | the `event` names of the run's `log.jsonl` lines, in order, keeping only the documented Build events `start`, `request`, `tool_call`, `pass`, `review`, `commit` and `outcome` (string list) |
| `phase` | `phase` of the run's facts record `.kogen/local/facts/<run>.json` (string) |
| `reason` | `reason` of the same facts record, `""` when null (string) |

The `events` set is the run record that `Run` documents in build/core_run.go (requests, tool calls, verification passes, reviews and the outcome), plus the Build's `start` and `commit` boundaries. Kogen-build logs all of these events itself. Producer diagnostics are not observations: `response_budget`, `provider_attempt`, `phase_duration`, `check_identity`, `command`, `step_sweep`, `command_result` and any new event name. For the unchanged fixture, both Builds give: exit code 0; stop code `""`; outcome `landed`; 1 commit whose parent is the start; message title `Preserve the fixture`; trailer `Kogen-Intent: fixture`; empty status; 8 builder requests; 0 review requests; phase `committing`; reason `landed`.

**Comparison.** For each observation whose values differ, the test calls `t.Errorf`. It reports every differing observation, not just the first, with this message:
`budget-off Build differs in <name>: budget omitted <omitted value>, budget off <off value>`
Values are printed with `%d` for ints, `%t` for the bool, and `%q` for strings and string lists. Examples:
- `budget-off Build differs in builder requests: budget omitted 8, budget off 9`
- `budget-off Build differs in stop code: budget omitted "", budget off "provider/refused"`
- `budget-off Build differs in commits: budget omitted 1, budget off 2`

**No request-budget names.** For each Build, the test decodes each of the run's `log.jsonl` (line by line), `checkpoint.json` and `result.json`. It flags a name when either:
- an object key, at any depth, contains `request_budget`;
- the string value of an `event` key contains `request_budget`.

Each such name is reported with `t.Errorf` and this message:
`request-budget name "<name>" in <file> of the <budget-omitted|budget-off> Build`
For example: `request-budget name "build_request_budget" in checkpoint.json of the budget-off Build`. Here `<file>` is `log.jsonl`, `checkpoint.json` or `result.json`. `response_budget` and `budget_ms` do not contain `request_budget` and are not flagged.

**Mutation control.** The mutation control stays as a focused test without a Build. It applies the test's request-budget check to small JSON documents and asserts that the check reports the injected name. The documents are: a checkpoint with `build_request_budget` nested under `state`, a result with a nested `request_budget_left` key, and a log line with event `build_request_budget`. Each one, without its injection, reports nothing.

**Removed.** These go from `build/build_request_budget_test.go`:
- `budgetObservation`, `volatileKeys`, `volatileKey` and `TestVolatileKey`;
- `cacheIdentity`, `hexHash` and `observeBuild` (with its `walk`, `replace` and `norm`);
- `TestTheGoldenMaskCatchesTimingAndHostLoadOnly`;
- the whole-observation `reflect.DeepEqual` comparison and its `<part> line <n> differs at byte <k>` report.

`TestBuildRequestBudgetAcrossPasses`, `TestBuildRequestBudgetResume`, `TestBuildRequestBudgetReview` and the helpers they use (`redTry`, `redContent`, `budgetOf`, `budgetEvents`) stay unchanged. Every other test file, `helper_test.go` included, stays unchanged. The acceptance harness adds one call line right after each of these anchors, which must stay byte for byte:
- in build/core_run.go: the openings of `Run.Event` (through its nil check), `Run.Write` and `Run.PublishCheckpoint`;
- in build/helper_test.go: the opening of `newFakeProvider` (through `f := &fakeProvider{t: t}`), the opening of `(*fakeProvider).then`, and `env.build`'s `a.exit, a.err = Build(context.Background(), s, in)` line.

## Decisions
- The test compares exactly the observations the request lists. These are the exit code, stop code and outcome; the commits, parent, tree, message title, trailer and porcelain status; the builder and review request counts; the documented event-name order; and the ended phase and reason. No whole files, logs or requests are compared, and nothing is masked (source: request)
- Neither Build's log, checkpoint or result may carry a field or event whose name mentions the request budget. A failure names the field and the file (source: request)
- A failure names which observation differed (source: request)
- A mutation control stays: an injected request-budget name turns the check red and is named (source: request)
- No product code, timeout or limit changes, and the test's runtime does not grow (source: request)

## Source comparison
- Addressed: "exit code, stop code and outcome". These are the `exit code`, `stop code` (`Outcome.Error` class/reason, as the stdout `error` of `cmd/kogen-build`) and `outcome` (`result.json`) observations. The `provider-refusal` acceptance row refuses the budget-off Build's first request and expects `budget-off Build differs in stop code: budget omitted "", budget off "provider/refused"`.
- Addressed: "Git facts: number of commits, parent, tree, message title and trailer, and the porcelain status". These are the `commits`, `parent`, `tree`, `message title`, `trailer` and `status` observations. `tree` is the change from the start commit, because the two temporary projects have different start trees (their configuration holds temporary paths). The `second-commit` row adds an empty commit after the budget-off Build and expects `budget-off Build differs in commits: budget omitted 1, budget off 2`.
- Addressed: "the number of builder requests and the number of review requests". These are `builder requests` and `review requests`. The `extra-builder-request` row scripts one more `read_file` request for the budget-off Build and expects `budget-off Build differs in builder requests: budget omitted 8, budget off 9`.
- Addressed: "the order of the documented run-log event names and the phase and reason each ended in". These are `events` (the documented Build events listed above), and `phase` and `reason` from the facts record (`localFacts`, build/facts.go). The phase and reason values are those the Build records: for the fixture `committing`/`landed`, and for a refused Build `building`/`refused`. The specification's model of a Build is not in this repository. Its names are taken from the Build's own record, which `kogen context` and `intent inspect` read.
- Addressed: "neither Build's log, checkpoint or result contains a field or event whose name mentions the request budget". This is the request-budget name check. The `budget-in-checkpoint`, `budget-in-result` and `budget-in-log` rows inject `build_request_budget` into the checkpoint state, `request_budget_left` into the result, and a `build_request_budget` event into the log. Each row expects the name, the file and the budget-off Build in the message.
- Addressed: "Equality of whole files, logs or requests, the list of masked keys and the tests of that list should go", and "no helper that rewrites or masks log lines remains in the test package". See the removals above. A scenario checks that no `build/*_test.go` file contains `volatileKeys`, `volatileKey(`, `observeBuild`, `budgetObservation`, `<volatile>`, `TestVolatileKey` or `TestTheGoldenMaskCatchesTimingAndHostLoadOnly`.
- Addressed: "Two scripted Builds that differ only in timing values, host load figures, step durations and the status of a toolchain probe: the test passes, with no masking list". In the `noise` scenario, both Builds get random `clock_skew` and `host_load` on every log line, and a random `took` on `command`, `command_result` and `step_sweep`. The `check_identity` lines also get `toolchain_probe` `ok` or `timed_out`, and only the budget-off Build logs extra `toolchain_probe_timeout` events. `result.json` and `checkpoint.json` get random load and duration fields as well.
- Addressed: "A mutation control stays". A focused test without a Build covers it, along with the three injection rows.
- Addressed: "The rest of the package's tests pass unchanged, and the test's own runtime does not grow". The configured `go-test` check runs every test. The test still runs only the same two Builds. The first scenario requires the test to pass within 40 s when run alone; it took 8.98 s in a scratch run of today's test.
- Addressed: "No product code, timeout or limit changes". The last scenario works without Git history, because Kogen's fresh acceptance copy has a single commit. Its feature table lists the SHA-256 of every Go file, go.mod and go.sum of the shaped tree (HEAD 3dcdb86), 150 files, except `build/build_request_budget_test.go`. Each of those files must keep its hash, and no other Go file may exist. In `build/build_request_budget_test.go`, the definitions `redTry`, `redContent`, `budgetOf`, `budgetEvents`, `TestBuildRequestBudgetAcrossPasses`, `TestBuildRequestBudgetResume` and `TestBuildRequestBudgetReview` must keep their shaped-tree SHA-256, which keeps their budget limits. If another Intent changes one of the listed files before this one is built, this scenario names that file and the Intent needs Refine.
- Superseded: the historical acceptance of k-build-budget-off-sweep-ms (`differs at byte` with `"stopped"`) and k-build-volatile-offsets (the `observeBuild` comparison table). Both depended on the removed whole-log comparison. Their files stay as they are.
- Omitted: none.

## Tests
- The budget-off Build is compared only on the listed observations, so differences in timing, host load, step duration or toolchain probe status do not fail it: rewrite TestBuildRequestBudgetOff (acceptance scenarios "unchanged fixture" and "noise") (regression)
- A differing observation fails the test and names it: TestBuildRequestBudgetOff's comparison report (acceptance rows extra-builder-request, provider-refusal, second-commit) (integration)
- A request-budget name in either Build's log, checkpoint or result fails the test and names the name, the file and the Build: TestBuildRequestBudgetOff's request-budget check (acceptance rows budget-in-checkpoint, budget-in-result, budget-in-log) (integration)
- The mutation control: add a focused test in build/build_request_budget_test.go that applies the request-budget check to a checkpoint, a result and a log line, each with and without an injected name, and asserts the reported names (unit)
- The masking list, `observeBuild` and the tests of that list are removed: delete `TestVolatileKey` and `TestTheGoldenMaskCatchesTimingAndHostLoadOnly` (acceptance static scenario) (regression)
- No product code, timeout or limit changes: the acceptance scenario that checks the shaped-tree SHA-256 of every Go file, go.mod and go.sum except the budget test file, and of the other budget test definitions (regression)
- The other budget tests and the rest of the package still pass: the configured go-test check (existing)

## Feasibility
- These checks ran in this read-only external session, in scratch copies outside the checkout (a `git clone --local` of HEAD 3dcdb86). They are not Kogen probe records.
  - Two fixture Builds were run and inspected. Both gave exit 0, 8 builder requests and 0 review requests, and 1 commit whose parent is the start. Both had title `Preserve the fixture`, trailer `Kogen-Intent: fixture`, empty porcelain status, and facts phase `committing` with reason `landed`. Their log event names were equal. The documented ones, in order, were `start`, 5×(`request`, `tool_call`), then 3×(`request`, `tool_call`, `tool_call`, `pass`), then `review`, `commit` and `outcome`. The start trees differ between the two projects, but `git diff-tree -r --no-commit-id <start> HEAD` is identical.
  - A Build whose first request is refused gave exit 4 with `Outcome.Error` `provider`/`refused`. It still wrote `log.jsonl`, `checkpoint.json` and `result.json` (outcome `refused`), and its facts record had phase `building` and reason `refused`. Its events were `start response_budget phase_duration provider_attempt outcome phase_duration`, with no `request` event.
  - The acceptance harness compiled (about 2 s with a warm cache) and ran on today's test. Without injection, the test passed in 8.98 s. Every injection reached the second Build: `noise` failed on `log line 0 differs`; `budget-in-checkpoint` and `budget-in-log` failed with `checkpoint has`/`log has "build_request_budget"`; `budget-in-result` failed on a result diff showing `request_budget_left`; `extra-builder-request` failed with `requests 8 and 9, want 8`; `provider-refusal` failed with `exit 4 ... provider/refused`; `second-commit` failed on `commit line 0 differs`. Without injection, under 4 parallel runs, the real `check_identity` of one Build recorded `node` as `timed_out` and the other as `ok`, which is the flake the request describes. All seven injections took about 22 s at four at a time.
  - Kogen ran readiness on the unchanged tree twice. In the first run, the no-injection scenario failed on the real `check_identity` difference: `log line 23`, with `node` `"status":"timed_out"` in the budget-omitted Build and `"ok"` in the budget-off Build. In the second run, that scenario passed. So today's test is flaky there, for the reason the request names. Also in the second run, the `second-commit` row hit the same flake (`go` and `node` `timed_out` in one Build) before its commit-message difference. In both runs, every other red scenario failed with today's messages, as listed above.
  - A Cucumber `--dry-run --strict` of the feature with the real tools found every step defined, with none ambiguous.
  - I probed the no-change steps in a fresh single-commit copy (`git archive HEAD` plus the proposed files, then one commit). On the unchanged tree, both passed. After appending to `build/core_run.go`, adding `build/zz_new.go` and changing `redTry`, they failed and named all three.
- evidence: {"id":"budget-off-run-judges","kind":"observer","inputs":["features/steps/k-build-budget-off-compares-behaviour__steps.js"],"observers":["features/steps/k-build-budget-off-compares-behaviour__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-budget-off-compares-behaviour__steps.js good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-budget-off-compares-behaviour__steps.js bad","exit":1,"output":"rejected\n"}]}
- The observer's known-good inputs are real `-test.v` output from the harness's test binary on today's test: the passing run without injection, and the failing `extra-builder-request` run with its printed line `requests 8 and 9, want 8`. The bad inputs are variants of those:
  - for the pass judge: a nonzero exit, a failing run, no test run, a run over 40 s, a failed subtest, and a killed run;
  - for the failure judge: a passing run, a zero exit, a missing expected report, no test run, and a panic.
  The controls qualify the run judges only, not product behaviour or the injections. The injections were checked by the harness runs above.
