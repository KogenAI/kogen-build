# Cap optional builder brief context

Slug: k-brief
Model: not applicable — deterministic brief formatting has no meaningful state transition
Acceptance:
- .kogen/intents/k-brief/acceptance.feature
- features/steps/k_brief_steps.js
- features/steps/k_brief_test.go.txt

Add internal native Build settings to compare full versus capped opening context. This research knob targets uncached first-request build input tokens without sacrificing pass rate; this change enables the experiment, not a claimed measurement or pass-rate result. Work only in kogen-build (the former cli/build package is build here); other repositories are follow-ups. Earlier candidate diffs are references, not authority.

## Decisions
- Add integer Deps.BuildBriefMaxFiles and Deps.BuildBriefMaxReadmeBytes, consumed by native builder brief construction; zero/unset retains full output byte-for-byte. No CLI flags or project.yaml schema/configuration changes. Keep external executors unchanged. Internal names settled during shaping. (source: request)
- Positive caps retain the first N entries in the existing git ls-files --cached --others --exclude-standard order, and the first N raw bytes of README.md respectively. Count bytes, not runes; notes are outside the payload limits. Independently apply each cap only when its payload exceeds it. Missing README behavior stays unchanged. Negative values need no new validation; treat nonpositive as uncapped. Boundary mechanics settled during shaping. (source: request)
- Preserve the stable/dynamic split: README remains stable; file listing remains dynamic; do not move any authority or other context between blocks. Cap only README payload and file-list entries, never Intent, acceptance feature, model, testing rules, navigation, baseline/protected paths or configured commands. Unset must match the existing golden without updating it. (source: request)
- When README is cut append a newline then exactly `[README.md cut: N bytes omitted; read_file README.md for the rest.]`, with decimal N replaced by the omitted byte count. When files are cut, follow the retained newline-terminated list with exactly `[File list cut: N entries omitted; use shell git ls-files --cached --others --exclude-standard for the rest.]`. These notes have no trailing newline. Preserve all surrounding existing text. Note wording settled during shaping. (source: request)
- Caps 200/8192 with exactly 500 listed files and a 20480-byte README retain 200 entries and 8192 README bytes, report 300 entries and 12288 bytes omitted, and leave all authority complete. (source: request)
- Use only owned child processes and temporary directories for fixtures, never stop host processes by name/pattern or generate synthetic CPU load. Run all configured checks without weakening them. (source: request)

## Source comparison
- Addressed: request Why/What and repository split via internal Deps settings and native brief formatting in build/builder.go; no coordinator/CLI work or research metric claims are added.
- Addressed: Must stay through unchanged golden, independently scoped payload clipping and exact stable/dynamic comparison in acceptance. Intent, feature, model, rules, protected paths and commands remain byte-complete.
- Addressed: Acceptance reproducer uses 500 Git-visible entries including fixture authority files, and 20 KB means 20480 bytes; expected 200/8192 retained and 300/12288 omitted are executable feature inputs. Required checks remain .kogen/project.yaml commands.
- Addressed: host safety via temporary fixtures and bounded owned child commands. No requested behavior is omitted, conflicting or superseded. Exact omission-note wording and exported internal field names were unspecified and are settled above, not new user-facing configuration.

## Tests
- Unset options retain byte-identical stable and dynamic opening blocks: keep TestOpeningMessageEqualsTheElixirReference unchanged (existing)
- Independent limits retain ordered first entries/raw bytes and report exact omitted counts only when exceeded: add callable brief tests at limit-1, limit, limit+1 and zero for each payload, including multibyte README bytes (unit)
- Native settings reach the real brief/opening path without truncating authority or changing cache block ownership: acceptance overlays TestKBriefAcceptance into build and compares complete stable/dynamic output for the 500-file/20480-byte reproducer; add native initialization wiring coverage for Deps propagation (integration)
- No CLI/yaml change and all existing checks pass: run unchanged configured format, compile, vet and go-test checks (existing)

## Feasibility
- The probe confirmed the existing golden test runs and its verbose PASS line; proposal files are not copied into scratch automatically, so attempted scratch control commands did not run. Final-byte observer controls below must execute successfully during readiness.
- evidence: {"id":"brief-observer","kind":"observer","inputs":["features/steps/k_brief_test.go.txt","features/steps/k_brief_steps.js"],"observers":["features/steps/k_brief_test.go.txt","features/steps/k_brief_steps.js"],"checks":[{"name":"good","command":"node features/steps/k_brief_steps.js good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"node features/steps/k_brief_steps.js bad","exit":1,"output":"observer rejected\n"}]}
