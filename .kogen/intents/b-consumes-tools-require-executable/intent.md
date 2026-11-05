# Pass TestToolsRequire with executable node rule

Slug: b-consumes-tools-require-executable
Model: not applicable — only a consumer contract test and its unused fixture helper change; kogen-build gains no state or rules
Acceptance:
- .kogen/intents/b-consumes-tools-require-executable/acceptance.feature
- features/steps/b-consumes-tools-require-executable__steps.js
- features/steps/b-consumes-tools-require-executable__control.js

Paired consumer row of kogen-verification's Build broken-kogen-install-blamed-on-intent (candidate 7b03b35, Build 4677e906). That change deliberately changes the exported `verify.Tools.Require`: node must be executable, not just a regular file, and the refusal text changes from "node is not available at X; reinstall Kogen" to "node is not available: X …" (the candidate also adds `RequireInstall` for new install checks, which this repository does not use). Its consumer contract check (5 Nov 09:51Z) refused it with one new failure here:

```
kogen-build TestToolsRequire tools_test.go:23: verify.Tools.Require: both tools present: environment/tool_missing: node is not available: node is no…
```

because `TestToolsRequire` (`consumes/kogen-verification/tools_test.go`) builds its node fixture as a plain non-executable file (`regular`, `consumes/kogen-verification/helper_test.go`, mode 0644) and pins the old exact `Detail` sentences. After this change `TestToolsRequire` passes both against the kogen-verification `go.mod` pins today (`github.com/KogenAI/kogen-verification v0.0.0-20261104204404-74798e7aa09e`, whose `Tools.Require` in `verify/env.go` wants node and quint to be regular files and refuses with `"<tool> is not available at "+path+"; reinstall Kogen"`) and against one whose `Tools.Require` has the new rule and wording. This row lands first; the producer is then re-shaped on top.

Required end state of `TestToolsRequire`:
1. Fixtures are real executables: the node and quint tools are made with the existing `script` helper (`#!/bin/sh` script, mode 0755), for example `script(t, "node", "exit 0\n")` and `script(t, "cli.js", "exit 0\n")`; Cucumber stays a `t.TempDir()` folder.
2. Kept as they are: the compile-time signature/field assertions; both tools present → `Require` returns nil; Tools without Cucumber → nil; the empty `verify.Tools{}` → an error.
3. Missing node (`Node` is a path that does not exist, `Quint` the executable fixture): the error is a `sys.Error` (through `failWith`) with `Class == "environment"`, `Reason == "tool_missing"`, `Exit == 3`, and a `Detail` that names the tool (contains `node`) and contains the missing path exactly. Missing quint (`Node` the executable fixture, `Quint` a missing path): the same, with the detail naming `quint` and containing that missing path. The test no longer compares `Detail` with a whole sentence and does not pin the words around the path (neither "is not available at …; reinstall Kogen" nor "is not available: …").
4. Node is judged first and a folder is not a tool: for `Node` a folder (`t.TempDir()`) and `Quint` a missing path, the refusal names node and the folder's path (and not the missing quint path); it no longer checks the prefix `"node is not available at "`.
5. Every failure message still says which case failed and shows the error, as the current messages do.
6. `regular` in `consumes/kogen-verification/helper_test.go` has no caller left and is deleted; nothing else in that file changes.
7. Unchanged: `go.mod` and `go.sum` (every pin), every non-test file (including `build/build.go`, which calls `in.Tools.Require()`), every other test and file under `consumes/` (including `consumes/README.md`, whose inventory still lists `verify.Tools.Require` under `TestToolsRequire`), and all other behaviour. No test is skipped.

## Decisions
- TestToolsRequire passes both against the pinned kogen-verification and against one whose `Tools.Require` wants node executable and words the refusal "node is not available: X …"; no pin or behaviour changes and no other test changes. (source: request)
- The tool fixtures are real executables; the test still checks class environment, reason tool_missing, exit 3, and that the detail names the tool and its path, without pinning the sentence around the path; the now unused `regular` helper goes with its last caller. (source: request)

## Source comparison
- Addressed: "node must be executable, not just a regular file": rule 1; acceptance runs TestToolsRequire against the `executable` copy, whose node check is `os.Stat` regular with some execute bit, reproducing the reported `tools_test.go:23 … both tools present: environment/tool_missing: node is not available: node is no…` at HEAD.
- Addressed: "refusal text changes … to 'node is not available: X …'": rules 3, 4; the `executable` copy words its refusals `"<tool> is not available: <tool> is not <an executable file|a regular file> at <path>; the Kogen installation is broken, reinstall Kogen"` (the producer's exact tail is not available to this repository; the test must accept any sentence that names the tool and the path).
- Addressed: "still checks class environment, reason tool_missing, exit 3, and that the detail names the tool and its path": rule 3; acceptance requires TestToolsRequire to fail against the `wrongclass`, `wrongreason`, `wrongexit`, `pathless` and `toolless` copies.
- Addressed: existing "Node is judged first, and a folder is not a tool" check kept: rule 4; acceptance requires a failure against `quintfirst`.
- Addressed: "passes … against the kogen-verification this repo pins today": acceptance runs it against `pinned`.
- Addressed: "No other test, pin (go.mod/go.sum) or behaviour changes": rule 7; acceptance checks the SHA-256 of `go.mod` and `go.sum` and a digest of every file under `consumes/` other than `tools_test.go` and `helper_test.go`.
- Addressed: "Keep every existing test green": the configured checks (pinned kogen-verification).
- Omitted: `RequireInstall`, which this repository does not consume; no test of it is added.
- Context, asserted nowhere: candidate 7b03b35 and Build 4677e906 of kogen-verification are not readable from this checkout; the new rule is reproduced by the `executable` copy instead.

## Tests
- TestToolsRequire accepts the executable-node rule and the new refusal wording, and still the pinned rule and wording: update `TestToolsRequire` (executable fixtures; detail checked for tool name and path, not the sentence); acceptance runs it against the `pinned` and `executable` kogen-verification (regression)
- TestToolsRequire still rejects a refusal with the wrong class, reason or exit, without the path or the tool name, or with quint judged before node: acceptance runs it against `wrongclass`, `wrongreason`, `wrongexit`, `pathless`, `toolless` and `quintfirst` and requires it to fail (regression)
- No pin or other consumer test changes: acceptance's byte checks of `go.mod`, `go.sum` and `consumes/` (regression)
- The rest of the suite stays green: the configured go-test check (existing)

## Feasibility
- The pinned kogen-verification's `verify/env.go` holds `Tools.Require` exactly as `PINNED` in the control script, once. At HEAD, TestToolsRequire passes against `pinned` (exit 0) and fails against `executable` with `tools_test.go:23: verify.Tools.Require: both tools present: environment/tool_missing: node is not available: node is not an executable file at …/001/node; …`, the reported failure; against each defective copy it fails too, each copy compiling and refusing with its own class, reason, exit, wording or order. The copies are made outside the checkout and used through `-modfile` with a `replace`; the checkout stays clean. (probe: a scratch copy of the control script, eight runs, about 5 s each)
- evidence: {"id":"verification-run-verdict","kind":"observer","inputs":[],"observers":["features/steps/b-consumes-tools-require-executable__control.js"],"checks":[{"name":"good","command":"\"${KOGEN_NODE:-node}\" features/steps/b-consumes-tools-require-executable__control.js pinned TestToolsRequire","exit":0,"output":"passed: TestToolsRequire\n"},{"name":"bad","command":"\"${KOGEN_NODE:-node}\" features/steps/b-consumes-tools-require-executable__control.js pathless TestToolsRequire","exit":1,"output":"rejected: TestToolsRequire: failed: TestToolsRequire\n"}]}
