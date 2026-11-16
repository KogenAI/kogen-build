# Retain Studio rules through Build resume

Slug: build-retain-rules-through-resume
Model: not applicable — the change recovers the already recorded snapshot at resume; its only state is the existing checkpointed receipt, observed in the scenarios
Acceptance:
- .kogen/intents/build-retain-rules-through-resume/acceptance.feature
- features/steps/build-retain-rules-through-resume__steps.js
- features/steps/build-retain-rules-through-resume__driver/fixture_test.go.txt

A configured project's resumed Build must give its models the applicable Studio rule block at the two request boundaries the earlier landed Intent build-use-studio-rule-snapshot left uncovered: (1) the fresh-conversation continuation after a usage-limit stop (StageUsageLimit, resumeStage in build/builder.go), and (2) a review round after resume of a Build whose builder was an external executor (build/host.go reviewInput). Today a resumed Build has no in-memory snapshot (`Attempt.studio` is nil), so only the receipt (State.StudioRules) survives and the block is absent from those requests.

On resume of a Build whose checkpoint records `studio_rules`, before any model request, Build recovers the applicable rules through the existing kogen-coordinator `rules.Load` entry with an `Options.Client` whose transport always fails, so that Load makes no network request and returns the producer's verified same-scope cache (the existing protected scoped snapshot/cache contract; Build adds no HTTP client, cache reader, scheduler or cache path code). The recovered snapshot must match the recorded receipt exactly: same scope, same `content_hash`, same rule ids and revisions in order; the receipt itself is left unchanged (origin, retrieved_at and fallback_reason stay as recorded). The recovered block is then used exactly as a fresh Build uses it (`studioMessage`, `studioDiagnostic`). If the token variable is missing or the cache is absent, foreign-scope, unverifiable or no longer matches the receipt, the resume stops before any provider or executor request with a `rules` class error naming the reason (a missing token is `rules/refused` `token_missing`; otherwise `rules/hold`). A resume makes no Studio request. Full rule text and tokens stay out of receipts, logs and exported artifacts; the receipt format is unchanged. A Build with no `studio_rules` receipt (unconfigured projects) resumes exactly as before. Native approval, checkpoints, model selection, protected paths and changed-revision-on-next-fresh-run behavior are unchanged; the runtime-rules group stays partial and nothing is enabled or repinned.

## Decisions
- Recover the block at resume through `rules.Load` with an always-failing HTTP transport, so the existing producer's verified same-scope cache is the only source and Build holds no cache or HTTP code (decision: D1, round 1) (evidence: build/studio_rules.go; build/resume.go; build/builder.go; build/executor_external.go; build/host.go) (assumption: the pinned coordinator commit 3b8df0c076a5 was not resolvable in the module mirror; the behavior of Options.Client and the cache fallback was read in the sibling version v0.0.0-20261116203159-d0dab3d6a948 and must match the pin) (source: request)
- A recovered snapshot that differs from the recorded receipt (hash, scope, ids, revisions) stops the resume with `rules/hold` before any request, and the receipt is never rewritten (decision: D2, round 1) (evidence: build/studio_rules.go; build/core_state.go) (assumption: stopping rather than continuing without the block is the intended outcome when the cache was overwritten by a later Build) (source: request)
- Acceptance overlays one Go fixture file into ./build using the existing test helpers and a fake executor and reviewer, as the previous Intent does; the review boundary is observed at the Build's review input (`reviewInput` Diagnostic, from which the review's request is made); controls.json is deferred because the correct implementation spans resume, state and request assembly (decision: D3, round 1) (evidence: build/usage_limit_resume_test.go; build/studio_rules_test.go; build/hooks_test.go; features/steps/build-use-studio-rule-snapshot__steps.js) (assumption: the fake review boundary stands in for the outgoing review model request; no provider or model call is made) (source: request)

## Source comparison
- Fresh-conversation continuation after usage limit carries the applicable block and a matching receipt: scenario usage limit.
- External-executor review round after resume carries the applicable block and a matching receipt: scenario review; it fails if the block is omitted while the receipt remains.
- Public receipts keep provenance only; no second HTTP/cache client; unconfigured standalone; refusal, same-scope offline and foreign-scope behavior of the producer: unchanged and preserved (existing tests build/studio_rules_test.go).
- Omitted: assembled/public release pins, repinning studies, enabling customer profiles, Shape and Review repository wiring.

## Tests
- A usage-limit fresh conversation carries the recorded block and receipt without a Studio request: scenario usage limit (scenario)
- An external-executor review round after resume carries the recorded block and receipt: scenario review (scenario)
- A resume whose cache no longer matches the receipt, or whose token is missing, stops before any request: build/studio_rules_test.go (integration)
- Existing snapshot, resume and unconfigured behavior: existing Build tests (existing)

## Feasibility
- none
