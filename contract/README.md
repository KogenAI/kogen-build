# Consumer contract

These producer-owned, synthetic tests pin the supplied kogen snapshot `9bf6073`;
no consumer checkout or remote integration gate is required.

- `cli/engine.go` and `cli/main.go`: `consumerCalls` and
  `TestDiagnosticConstruction` compile the Build, Resume, Recover and Diagnostic
  call shapes, including optional executors.
- `cli/response_budget_test.go`, `TestShapeBudgetRolesBuild`: the identically
  named contract copies the three source subtests, respectively covering the
  successful builder/reviewer budget, stalled builder and resume, and stalled
  review and resume. The scripted provider and temporary Git fixture are adapted
  from the producer's `build/response_budget_test.go` and its test helpers.

Deliberate contract changes require updating these producer contracts together
with the paired consumer items.

## Native continuation adapter

The producer exports `build.ContinuationBinding`, `build.ContinuationPlan`, and the
value-receiver methods `ResumeInput.PrepareContinuation` and
`ResumeInput.ExecuteContinuation`. Their exact signatures, JSON evidence fields,
standalone invocation and worker obligations are documented in the README's
“Native continuation evidence for fleet workers” section.

Preparation is model-free evidence from the same native resume preflight, including
partial readback on native refusal. It neither invokes admission nor edits the
checkout or run evidence. Saved approval/contract/checkpoint identities remain
separate from the fleet's caller-supplied request binding. Current candidate and
approval identities are separately named so a hold cannot imply that saved bytes
are current bytes. Inspect availability remains a fact, not authorization.

Execution requires a fresh binding and non-nil trusted admission callback under the
ordinary Build lock, after native revalidation and equality of the prepared evidence.
The consumer must freshly validate/consume the existing fleet fence and ordinary
admission in that callback, rejecting duplicates even after a pre-landing stop.
Native code does not implement the fleet's authorization store or queue. Callback
failure is a held refusal; success delegates to supported native resume without a
lock gap, preserving verification, independent review, provider and finishing gates
and ordinary Outcome population. Remaining work lists are evidence, not gate waivers.
The standalone command only prepares; existing resume is the execution command.
