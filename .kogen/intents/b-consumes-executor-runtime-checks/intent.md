# Pass first-prompt test with runtime checks

Slug: b-consumes-executor-runtime-checks
Model: not applicable — only one test's stand-in runtime and its environment change; kogen-build gains no state or rules
Acceptance:
- .kogen/intents/b-consumes-executor-runtime-checks/acceptance.feature
- features/steps/b-consumes-executor-runtime-checks__steps.js
- features/steps/b-consumes-executor-runtime-checks__control.js

Paired consumer row of kogen-executor's Build kogen-runs-unverified-claude-code-runtime (candidate 626c4649, Build e62962a9). That change makes the executor check the managed Claude Code runtime before every start: a regular, executable, non-empty file whose size equals the pinned size for the platform, owned by the current user and not writable by group or others. Its consumer contract check (5 Nov 18:58Z) refused it with these new failures here:

```
kogen-build TestContextExternalFirstPrompt/context_false  context_external_test.go:303: exit 1 err <nil>
kogen-build TestContextExternalFirstPrompt/context_true   context_external_test.go:303: exit 1 err <nil>
```

because `TestContextExternalFirstPrompt` (`build/context_external_test.go`) starts the real Claude Code adapter (`claudecode.New`) on a stand-in runtime, `fakeClaudeScript` written by `withClaudeProcess` at `claudecode.RuntimePath(home)`, which is about a thousand bytes; the new size check refuses it as `executor/unavailable` before the first prompt. It is the only test in this repository that stands in for the managed runtime.

The kogen-executor `go.mod` pins today (`github.com/KogenAI/kogen-executor v0.0.0-20261104160936-8a4fecd1ce0a`) exports no pinned size and no test helper (its `contract/fake_claude_test.go` `newFake` is a `_test.go` file of another module and also writes a small script). What it already provides is the test download origin `claudecode.SeamEnv` = `KOGEN_CLAUDE_CODE_URL` (`http://127.0.0.1:<port>` or `http://localhost:<port>`, read from the test process's environment), whose own manifest (`GET <origin>/<Version>/manifest.json`, `{"platforms":{"<platform>":{"checksum":"<sha256 hex>","size":<bytes>}}}`) replaces the pin. With such an origin set the size is unknown to the new check and that check is skipped. This change uses that origin.

Required end state of `TestContextExternalFirstPrompt` (and its helpers `withClaudeProcess` and `fakeClaudeScript` in the same file):
1. Each of its two subtests (`context true`, `context false`) starts its own local test origin with `net/http/httptest.NewServer` (closed when the subtest ends). It answers `GET /` + `claudecode.Version` + `/manifest.json` with `{"platforms":{"<platform>":{"checksum":"<lowercase hex SHA-256 of the stand-in's bytes>","size":<the stand-in's byte length>}}}`, where `<platform>` is the first result of `claudecode.Supported()` (for example `darwin-arm64`), and `GET /` + `claudecode.Version` + `/<platform>/claude` with the stand-in's exact bytes; any other path is 404.
2. Before its Build, each subtest sets `claudecode.SeamEnv` to that origin's URL (`server.URL`) with `t.Setenv`. Go refuses `t.Setenv` in a parallel test or one with a parallel ancestor, so `TestContextExternalFirstPrompt` and its two subtests no longer call `t.Parallel`; the variable is restored when each subtest ends.
3. The stand-in stays the `fakeClaudeScript` of the subtest's state folder, written by the test at `claudecode.RuntimePath(home)` before the Build exactly as today (the executor then installs nothing), owned by the current user, with its mode set to exactly 0755 after writing (`os.Chmod`), whatever the process umask is. `claudecode.New(claudecode.Options{Home: home, HookCommand: []string{"/bin/true"}})` is unchanged; the stand-in is not padded to the pinned size and no size constant is copied into this repository.
4. Everything the test proves today stays, with the same messages: the Build exits 0 with no error; the first prompt (`prompt-1.bin`) contains `# Protected paths`; without context it lacks `Caller diagnostic data`; with context it holds the exact labelled diagnostic message once, after `# Protected paths`, followed by `\n\n# Files\n\n`, and the stand-in ran exactly one round (`calls` is `1`).
5. On a platform `claudecode.Supported()` refuses, the test fails as it does today; no skip is added.
6. Unchanged: `go.mod` and `go.sum` (every pin), every other file under `build/`, `cmd/`, `consumes/` and `contract/` (including `TestMain` in `build/main_test.go`, `writeFile`, the goldens and every other test), and all behaviour. Only `build/context_external_test.go` changes.

## Decisions
- TestContextExternalFirstPrompt passes both against the kogen-executor this repository pins and against one that also checks the runtime's pinned size, owner and mode; no other test, pin or behaviour changes. (source: request)
- The stand-in satisfies the new rules through what the pinned executor already provides: a local test origin named by `KOGEN_CLAUDE_CODE_URL` that supplies its own manifest, so the size is unknown and that check is skipped; the stand-in is owned by the current user and has mode 0755. (source: request)
- The test still proves what the first prompt contains with context on and off. (source: request)

## Source comparison
- Addressed: "a regular, executable, non-empty file whose size equals the pinned size … owned by the current user and not writable by group or others": rules 1–3; acceptance runs the test against the `checked` kogen-executor copy, whose `runtimeUsable` adds exactly these checks (size against the platform pin unless `KOGEN_CLAUDE_CODE_URL` is set), at the default umask and at umask 002. At HEAD that copy reproduces the reported `context_external_test.go:303: exit 1 err <nil>` for both subtests, with `executor/unavailable: Kogen's Claude Code at …/claude is 1046 bytes, not the pinned 275988512 bytes; …` (the copy's own wording; the producer's exact wording is not available to this repository and is not asserted).
- Addressed: "in the way the producer's own tests do … or a test origin that supplies its own manifest so the size is unknown and that check is skipped. Use whichever the pinned executor already provides": the pinned executor provides only the test origin (`claudecode.SeamEnv`); rules 1–2. Acceptance requires the test to fail against `strictseam`, which checks the size against the pin even with a test origin, so a stand-in padded to the pinned size is told apart.
- Addressed: "The test must still prove what it proves today": rule 4; acceptance requires failure against `doubledprompt` (diagnostic twice), `blankprompt` (no brief, so no `# Protected paths`) and `extradiagnostic` (prompt mentions `Caller diagnostic data` without context).
- Addressed: "passes … against the kogen-executor this repo pins today": acceptance runs it against `pinned`, both subtests passing, nothing skipped.
- Addressed: "No other test, pin (go.mod/go.sum) or behaviour changes": rule 6; acceptance checks the SHA-256 of `go.mod` and `go.sum` and a digest of every file under `build`, `cmd`, `consumes` and `contract` other than `build/context_external_test.go`.
- Addressed: "(and any other test in this repo that stands in for the managed runtime)": a search finds no other `claudecode.RuntimePath` or runtime stand-in in this repository.
- Addressed: "Keep every existing test green": the configured checks (pinned kogen-executor).
- Context, asserted nowhere: candidate 626c4649 and Build e62962a9 of kogen-executor are not readable from this checkout; the new rules are reproduced by the `checked` copy. "This row lands first; the producer is then re-shaped on top" is sequencing outside this change.

## Tests
- TestContextExternalFirstPrompt passes with the size, owner and mode checks and with the pinned executor: update `TestContextExternalFirstPrompt` (test origin with its own manifest set through `t.Setenv`, stand-in mode 0755); acceptance runs it against `pinned`, `checked` and `checked` under umask 002 (regression)
- It relies on the test origin's unknown size, not on matching the pinned size: acceptance requires failure against `strictseam` (regression)
- It still proves the first prompt's contents with context on and off: acceptance requires failure against `doubledprompt`, `blankprompt` and `extradiagnostic` (regression)
- No pin or other test changes: acceptance's byte checks of `go.mod`, `go.sum` and the other files under `build`, `cmd`, `consumes`, `contract` (regression)
- The rest of the suite stays green: the configured go-test check (existing)

## Feasibility
- At HEAD, TestContextExternalFirstPrompt passes against `pinned` (exit 0, also under umask 002) and fails against `checked` with the reported `context_external_test.go:303: exit 1 err <nil>` in both subtests, the executor refusing the 1046/1048-byte stand-in as not the pinned 275988512 bytes (darwin/arm64); it fails against `strictseam` the same way, against `doubledprompt` at line 317 (diagnostic 2 times), against `blankprompt` at line 310 in both subtests (prompt lacks `# Protected paths`) and against `extradiagnostic` at line 312 (prompt has `Caller diagnostic data`). Each copy compiles; the copies are made outside the checkout and used through `-modfile` with a `replace`; the checkout stays clean. (probe: a scratch copy of the control script, eight runs, about 5–15 s each)
- evidence: {"id":"executor-run-verdict","kind":"observer","inputs":[],"observers":["features/steps/b-consumes-executor-runtime-checks__control.js"],"checks":[{"name":"good","command":"\"${KOGEN_NODE:-node}\" features/steps/b-consumes-executor-runtime-checks__control.js pinned - TestContextExternalFirstPrompt TestContextExternalFirstPrompt/context_true TestContextExternalFirstPrompt/context_false","exit":0,"output":"passed: TestContextExternalFirstPrompt, TestContextExternalFirstPrompt/context_true, TestContextExternalFirstPrompt/context_false\n"},{"name":"bad","command":"\"${KOGEN_NODE:-node}\" features/steps/b-consumes-executor-runtime-checks__control.js doubledprompt - TestContextExternalFirstPrompt TestContextExternalFirstPrompt/context_true TestContextExternalFirstPrompt/context_false","exit":1,"output":"rejected: TestContextExternalFirstPrompt, TestContextExternalFirstPrompt/context_true, TestContextExternalFirstPrompt/context_false: failed: TestContextExternalFirstPrompt, TestContextExternalFirstPrompt/context_true\n"}]}
