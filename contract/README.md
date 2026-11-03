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
