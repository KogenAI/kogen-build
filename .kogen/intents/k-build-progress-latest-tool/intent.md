# Show the latest completed Build tool

Slug: k-build-progress-latest-tool
Model: not applicable — this is progress-output wiring from completed tool events, not a new workflow or persisted state machine
Acceptance:
- .kogen/intents/k-build-progress-latest-tool/acceptance.feature
- features/steps/k-build-progress-latest-tool__progress.js

During a Build, enrich the existing `kogen: building (round N)` console progress line with the latest completed tool's name, path when present, and status. Update on completion, while the round is still running, rather than waiting for round completion. Replace the apparently inactive round-only display when tool completions occur; retain the existing display when they do not. Keep tool result bodies out of console output. Keep run logging, checkpoints, verification, review, stdout JSON and Build outcomes unchanged. This request is tracked in Studio issue c6fbe7f5-1a01-4546-b9e1-cf04d2364a84 (KOGEN-005 — Visibility while shaping is in flight); this contract changes Build visibility, not the Shape workflow.

## Decisions
- Show the latest completed tool name, path if any, and status on the existing console progress line, updating as events arrive: “the console progress line also shows the latest completed tool event (tool name, path if any, status), updated as events arrive”. (source: request)
- Do not print tool result content: “without printing tool output” and “no tool result content is printed”. (source: request)
- Preserve the no-event display: “output is unchanged when no tool events occur”. (source: request)
- Exercise two fake tool completions and cover each requested outcome: “with a fake runner emitting two tool events the progress shows the latest tool name and status” and “tests cover each”. (source: request)

Implementation boundary: use the existing native AfterCall completion boundary in `build/executor_native.go` and the existing external executor event sink where applicable, rather than polling log.jsonl. Do not treat tool starts or non-tool events as completed tools. Reuse the existing progress output channel and round label. Display `ok` for a successful completed operation and `failed` for a failed operation (including a nonzero shell exit). Choose punctuation freely; do not include arguments, command strings or result fingerprints as a substitute for status. A later pathless tool must not retain the previous tool's path. Native `read_file` supplies its path from the call arguments; shell need not invent one. Do not add settings, provider calls, output streaming, or changes to tool execution.

## Source comparison
- Addressed: request problem “console stays at 'building (round 1)' while .kogen/local/runs/<id>/log.jsonl records completed tool calls with results” by the outcome and completion-time wiring boundary above and the first acceptance scenario's between-request snapshots.
- Addressed: requested latest name/path/status and two fake events by acceptance scenario `Latest completion replaces the earlier summary`, with `read_file` of `progress-input.txt` followed by failed `shell`; scenario values define concrete success/failure labels, not test outcomes.
- Addressed: “no tool result content is printed” by the separate result-content scenario and sentinel `TOOL_RESULT_PRIVATE_7081` emitted by both fake operations.
- Addressed: “output is unchanged when no tool events occur” by the no-tool scenario's unchanged `kogen: building (round 1)` line. Existing unrelated progress remains intact.
- Addressed: “tests cover each” by all three executable scenarios and implementation regression coverage below. The fake runner is the repository's scripted local native provider, driving the real Build entry point without credentials or remote inference.
- Addressed: retry delivery instructions by prefixing all bindings, using the specified slug and file names, keeping Decisions request-sourced, and not including probe output or test outcomes as product expectations.
- No requested behavior is omitted or superseded. The Studio issue's shaping wording is retained as provenance, not expanded into a separate Shape change. The README's stderr human-progress/stdout-JSON contract and configured checks remain unchanged.

## Tests
- Latest completed tool updates progress during the same round and supersedes the previous name/path/status: add a Build regression using two scripted local tool calls and capture progress before each following provider request; acceptance latest-completion scenario runs the real Build entry point (regression)
- Tool result content never reaches console progress: add sentinel-result regression coverage for successful read and failed shell; acceptance result-content scenario inspects the complete captured output independently of latest-line assertions (regression)
- No tool events leave existing output unchanged: add a no-tool scripted-run regression retaining the round-only line; acceptance no-event scenario checks the existing line (regression)
- Completed external tool events feed the same progress summary without displaying results: add focused coverage using the existing fake external executor and its event sink, preserving existing external behavior tests (integration)

## Feasibility
- evidence: {"id":"progress-observer","kind":"observer","inputs":["features/steps/k-build-progress-latest-tool__progress.js"],"observers":["features/steps/k-build-progress-latest-tool__progress.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k-build-progress-latest-tool__progress.js --control good","exit":0,"output":"observer controls accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k-build-progress-latest-tool__progress.js --control bad","exit":1,"output":"observer controls rejected\n"}]}
