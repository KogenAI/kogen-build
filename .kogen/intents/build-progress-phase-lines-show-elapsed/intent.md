# Show elapsed time on Build phase lines

Slug: build-progress-phase-lines-show-elapsed
Model: not applicable — progress formatting uses an existing clock origin; no new workflow state or decisions
Acceptance:
- .kogen/intents/build-progress-phase-lines-show-elapsed/acceptance.feature
- features/steps/build-progress-phase-lines-show-elapsed__steps.js
- features/steps/build-progress-phase-lines-show-elapsed__driver/fixture_test.go.txt

Append ` (<seconds> s)` to Build stage-boundary progress lines, measuring whole elapsed seconds since the original Build attempt started. Cover preliminary `kogen: checking <name> before the Build changes anything`, `kogen: verifying (pass N)`, `kogen: pass N: ...`, `kogen: reviewing (review N)`, `kogen: review N: ...`, and `kogen: committing`. Apply the suffix to these boundary messages wherever emitted, including unsuccessful verification or review outcomes and resumed review boundaries; retain their existing message bodies. Replace their untimed form, not the Build workflow.

Use `b.T0` and `elapsed()` in build/build.go with one shared suffix helper local to build/. A resumed attempt continues the original timing origin instead of restarting the phase clock. Keep this a narrow Go change at the progress emission/adaptation boundaries (build.go, commit.go, baseline and review/verify progress calls, plus the necessary resume-origin correction). Do not change dependency APIs, add configuration, browser/Rails/Studio tooling, or introduce a new clock abstraction merely for tests.

Per-tool `kogen: building (round N) — <tool> [path] ok|failed` lines must remain byte-for-byte unchanged, as pinned by the k-build-progress-latest-tool Intent. The initial building-round line also remains unchanged. No change to the summary block's format or messages, feedback texts, or decisions, and no timestamp suffix on unrelated output. The summary continues to use its existing calculation; retaining the original T0 is required for resume, not a summary redesign.

Add a focused model-free Go regression test next to the existing build progress tests. Use the existing local Build/executor fixtures and a fake clock to observe stage output through the real Build and Resume entry points. Assert exact whole-second values and unchanged successful/failed tool output. The protected acceptance fixture supplies its own Go test via a temporary Go overlay and runs the compiled test binary; it does not require the Build to invent a named test. Keep useful existing progress tests.

## Decisions
- Derive the suffix from elapsed() divided by time.Second and preserve the original b.T0 on resume, without changing routing, feedback or summary formatting (decision: D1, round 1) (evidence: build/build.go; build/resume.go; build/commit.go) (source: request)
- Reuse local Build fixtures and Go's fake clock for one focused model-free regression and acceptance, preserving the existing per-tool progress contract (decision: D2, round 1) (evidence: build/progress_tool_test.go; build/lineup_test.go; README.md) (source: request)

## Source comparison
- Addressed — Request Expected's six stage-boundary message families and whole-second suffix are specified above and compared exactly in acceptance.feature's fresh Build output; the old untimed form is superseded.
- Addressed — Request's original-start resume requirement is specified above and exercised by the same scenario after a failed commit and a fake-clock advance; retaining b.T0 replaces the reset currently in build/resume.go.
- Addressed — Request Constraints and scope note: tool success/failure output and initial round output remain unchanged in the executable comparison. Summary format, feedback and routing remain unchanged; implementation stays in build/ with one shared suffix helper and a focused Go regression, not a browser or external model suite.
- Addressed — Request's AoR climb release 30cf6619, h11-build-sonnet-high-vs-baseline-30cf6619 / rails-ft-data-export-and-import__baseline__r2 evidence, stderr.txt/result.json, 62 building lines (51 Bash ok), wall_s 1499.443 and summary time: 16m40s are retained truthfully as historical motivation in acceptance.feature, not fabricated as a local benchmark. Shape's checking readiness (205 s) and the 17-minute readability problem are retained there. The executable case uses bounded fake-clock durations instead of waiting for that historical run.
- Addressed — One wrong-implementation control timestamps tool lines too and must fail the exact-output Then. No requested feature is omitted or conflicting; no deliberate scope expansion.

## Tests
- Stage-boundary messages carry whole elapsed seconds without altering their bodies: add a focused fake-clock regression next to build/progress_tool_test.go and compare actual Build stderr in acceptance (regression)
- Resume counts from the original Build start rather than a new start: the same focused test resumes a failed commit after advancing the fake clock and compares the emitted boundary (regression)
- Per-tool success and failure lines and initial round output remain unchanged: exact output comparison in that focused test, with the tool-timestamp wrong control rejected (regression)
- Summary format, feedback texts and routing decisions remain unchanged: retain existing build package coverage and configured checks without weakening assertions (existing)

## Feasibility
- The supplied focused fixture completed Build and Resume with local executors under the fake clock; no model calls were needed. The observer's retained good and bad outputs were generated using the supplied correct and wrong overlays, respectively.
- evidence: {"id":"phase-output-observer","kind":"observer","inputs":[".kogen/intents/build-progress-phase-lines-show-elapsed/acceptance.feature",".kogen/intents/build-progress-phase-lines-show-elapsed/good.json",".kogen/intents/build-progress-phase-lines-show-elapsed/bad.json","features/steps/build-progress-phase-lines-show-elapsed__driver/fixture_test.go.txt"],"observers":["features/steps/build-progress-phase-lines-show-elapsed__steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/build-progress-phase-lines-show-elapsed__steps.js good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/build-progress-phase-lines-show-elapsed__steps.js bad","exit":1,"output":"observer rejected\n"}]}
