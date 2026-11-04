# Require measured figures to cite a command run

Slug: k-build-measured-figures-only
Model: not applicable — one added bullet in the builder's standing instructions; no state, transition, tool, check or verification rule changes
Acceptance:
- .kogen/intents/k-build-measured-figures-only/acceptance.feature
- features/steps/k-build-measured-figures-only__steps.js

The native builder's standing instructions, `build/prompts/builder.md` (served as `BuilderInstructions()`), gain exactly one bullet under `Rules:`. It goes on its own line, directly after the bullet `- Do not commit, push, stash, reset or switch branches; Kogen commits after verification and review.` and directly before the bullet that starts `- When the implementation is complete, call finish.`. The bullet is exactly this line (plain ASCII, ending with a line feed):

```text
- A figure you write into a project file as an observed result (a timing, size, count or percentage, in documentation, a report or a code comment) must be copied from the output of a command you ran in this Build on the final candidate, and the text must name that command. When you cannot measure it (no tool or no environment), write the figure as a target or an expectation, not as a measurement, and say in your finish summary that it was not measured. Figures restated from the Intent or from existing files are not affected.
```

No other byte of `build/prompts/builder.md` changes. Before the change it is 3246 bytes with sha256 `00e6225cfcc6520ca3eb18750a632272bfaa543c4f14b6b42d875c3bec24a6fc`. After it, it is 3777 bytes with sha256 `915b0bc6b4bd3624010f1878143b6215f34fb39c0333e99785b187d8f97511d7`.

The recorded goldens that hold the builder's instruction bytes change only by this bullet:
- `build/testdata/prompts_golden.json` records the new length and sha256;
- `build/testdata/named-feedback-compat.json` (the first request of a passing scripted Build) and `build/testdata/context-external-golden.json` (the native builder's opening parts) hold the new instruction text. Their only difference from today's bytes is the bullet line, as escaped JSON text.

These goldens are frozen. This Intent is the deliberate reason they change. Regenerate them with `go test ./build -count=1 -update -run '^(TestPromptsAreByteEqualToTheElixirFunctions|TestNamedFeedbackCompatibility|TestContextExternalNativeBytes)$'`, then confirm that the diff is only the bullet. If any other golden or recorded value turns out to derive from the instruction bytes, update it the same way: its only difference may be the bullet.

Unchanged:
- the tool definitions (`finish`, `report_blocker`, `defs_golden.json`) and `SubmissionSchema`;
- the reviewer and its behaviour;
- verification, checks and gates;
- the checkpoint and result schemas (`state/fresh/checkpoint.json`);
- the `kogen-build` command's flags and its stdout result;
- the instructions that external executors receive from their own executor package. `build/prompts/builder.md` is the builder policy this module owns.

No new tool, check, verification rule, setting or code path is added. Do not change the dependencies.

## Decisions
- The rule is one bullet in the builder's standing instructions, stated once. No new tool, check or verification rule is added, and review's behaviour stays as it is (source: request)
- A quantitative claim written into a project file as an observed result (timing, size, count, percentage) must be copied from the output of a command run in this Build on the final candidate, and must name that command. When it cannot be measured, it is written as a target or expectation, and the finish message (the `finish` tool's `summary`) says so. Figures restated from the Intent or from existing files are not affected (source: request)
- The bullet's exact wording and its place (directly before the finish bullet, which it refers to) are fixed above, so "no other instruction text changes" can be checked byte for byte (source: request)

## Source comparison
- addressed: Need (the standing instruction rule) and Synthetic acceptance 4 ("the rule exactly once, as one bullet; no other instruction text changes"). Scenario 1 checks:
  - the bullet is exactly one line, and its text occurs exactly once;
  - it sits directly before the finish bullet;
  - removing it gives today's sha256 `00e6225c…`;
  - the new file is 3777 bytes with sha256 `915b0bc6…`;
  - `prompts_golden.json` records those values;
  - the existing TestPromptsAreByteEqualToTheElixirFunctions passes in its own run.
- addressed: Synthetic acceptance 1 (runnable measuring command; the note's figure equals the command's output and names the command, checked against the command log). Scenario "measured" runs the real program: the step file builds `./cmd/kogen-build` from this checkout and runs `kogen-build build --project <fixture> --slug fixture` on a temporary Git project. That project has an approved fixture Intent and a real Cucumber acceptance. A scripted local provider (`KOGEN_PROVIDER_URL`) stands in for ChatGPT and answers in order:
  1. builder: write app.txt;
  2. builder: run `wc -c < app.txt | tr -d ' '` through the shell tool;
  3. builder: write NOTE.md from the shell result that the Build actually returned (`exit status: 0` then `10`), and finish;
  4. reviewer: `submit_review` approve.

  The step checks:
  - every builder request carries the checkout's `build/prompts/builder.md`, with the rule bullet exactly once;
  - the command log shows exit 0 and output `10`;
  - the committed NOTE.md is exactly `Measured effect: app.txt is 10 bytes, measured with `wc -c < app.txt | tr -d ' '` on the final candidate.`;
  - the note's only figure equals the command log's output, and the note names the command;
  - the program exits 0 with `error` null, and lands one commit.
- addressed: Synthetic acceptance 2 (the measuring command exits with an environment error; the note gives a target, and the finish message says not measured). Scenario "unavailable": the command `kmf-measure-size app.txt` exits 127 (`command not found`). The committed note is exactly `Expected effect (a target, not measured): app.txt should be about 10 bytes; `kmf-measure-size app.txt` could not run in this environment (exit status 127).` and has no `Measured effect`. The finish summary says `The effect was not measured`.
- addressed: Synthetic acceptance 3 (no quantitative claim: the instruction text is the only difference, and a scripted Build is unchanged). Scenario 4 takes the rule bullet out of `named-feedback-compat.json` (the first request of the existing passing scripted Build `landScript`) and of `context-external-golden.json`. Each must then equal today's bytes (`64370533…`, `435b5e9b…`) and must have held the bullet. `defs_golden.json` and `state/fresh/checkpoint.json` keep their bytes. The existing tests TestNamedFeedbackCompatibility, TestContextExternalNativeBytes, TestDefinitionsMatchTheElixirDefinitions and TestStateGoldenFreshCheckpoint each pass in their own `go test -run '^Name$'` run.
- honest limit: a scripted builder writes only what its script writes. The two scripted Builds show three things: the rule reaches the builder in the Build's requests; the Build runs the measuring command and returns its exact output, which a rule-following builder can copy; and the note lands in the commit. They do not prove that a real model obeys the rule, and no Kogen check enforces it, as the request asks.
- addressed: the delivery decisions (a step that names a Go test must point to one that exists and that its `-run` pattern selects; acceptance must not depend on tests the Build has to write). The scripted Builds run the real `kogen-build` program and name no Go test. The only Go tests named are the four existing golden tests and TestPromptsAreByteEqualToTheElixirFunctions, all of which exist and pass on the unchanged tree.
- omitted: external executors' own instruction text (kogen-executor), the reviewer's instructions, and any new tool, check or verification rule, all out of scope by the request. No external protocol or library facts are involved, so no documentation lookup was needed.

## Tests
- The rule bullet is in `build/prompts/builder.md` exactly once, directly before the finish bullet, with no other instruction text changed: regenerate `build/testdata/prompts_golden.json` (3777 bytes, sha256 `915b0bc6…`), checked by TestPromptsAreByteEqualToTheElixirFunctions (existing) and acceptance scenario 1 (scenario)
- A scripted Build's requests carry the rule, and a measured figure copied from the command log lands in the commit naming the command: acceptance scenario "measured", through the `kogen-build build` CLI (scenario)
- A scripted Build whose measuring command cannot run lands a target note, and its finish summary says the effect was not measured: acceptance scenario "unavailable", through the `kogen-build build` CLI (scenario)
- A Build without a quantitative claim differs only by the instruction text: regenerate `named-feedback-compat.json` and `context-external-golden.json`, checked by TestNamedFeedbackCompatibility and TestContextExternalNativeBytes; TestDefinitionsMatchTheElixirDefinitions and TestStateGoldenFreshCheckpoint stay unchanged (existing)

## Feasibility
- In a scratch copy of the unchanged checkout, the step file built `./cmd/kogen-build` and ran `kogen-build build` on fresh fixture projects with the scripted provider. The real reviewer approved with a single `submit_review` request, and the fixture's Cucumber acceptance ran from `KOGEN_CUCUMBER`, falling back to `testdata/tools/cucumber/node_modules`. Results:
  - with an existing instructions bullet standing in for the rule, both modes landed one commit and every expectation held (`observer accepted`, exit 0);
  - the shell tool's real result has the form `exit status: 0\n10\n` and `exit status: 127\n/bin/sh: kmf-measure-size: command not found\n`, and the step parses that observed form;
  - with an invented figure, both modes were rejected by the note checks (`observer rejected`, exit 1);
  - with the real rule on the unchanged tree, both modes failed only with `kmf-reject: instructions: builder request N carries the rule bullet 0 times` (missing-feature red);
  - with the bullet inserted into builder.md in the scratch copy, both modes passed.
- In the same scratch copy, the bullet was inserted before the finish bullet, and `go test ./build -count=1 -update -run '^(TestPromptsAreByteEqualToTheElixirFunctions|TestNamedFeedbackCompatibility|TestContextExternalNativeBytes)$'` passed (exit 0, 9 s):
  - it changed only builder.md, `prompts_golden.json` (3777 bytes, `915b0bc6…`), `named-feedback-compat.json` (the bullet once) and `context-external-golden.json` (the bullet 4 times, escaped twice);
  - removing the escaped bullet restored both frozen goldens' current sha256.
- evidence: {"id":"measured-figures-observer","kind":"observer","inputs":["features/steps/k-build-measured-figures-only__steps.js",".kogen/intents/k-build-measured-figures-only/acceptance.feature"],"observers":["features/steps/k-build-measured-figures-only__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-measured-figures-only__steps.js control-good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-measured-figures-only__steps.js control-bad","exit":1,"output":"observer rejected\n"}]}

The controls run the real program on the unchanged tree with the same step bytes, and read their expectations from the feature. An existing bullet (`- Do not commit, push…`) stands in for the rule, so the deeper assertions are reached even though the missing rule masks them:
- control-good: the scripted builder copies the command log, and both modes pass;
- control-bad: the scripted builder states an invented figure (12, or `Measured effect` after a failed command), and both modes must be rejected by the note checks.

The controls qualify the observer; they are not evidence that the feature is implemented.
