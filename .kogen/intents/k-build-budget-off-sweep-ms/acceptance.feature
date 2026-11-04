Feature: TestBuildRequestBudgetOff treats a plain ms duration as volatile
  kogen-verification 292a2d1 sweeps every verification step and logs one
  {"event":"step_sweep","name":<step>,"stopped":<int>,"ms":<int>} event per step, where ms is the
  sweep's measured duration. TestBuildRequestBudgetOff compares a budget-omitted Build with a
  budget-0 Build after masking wall-clock values, so the plain ms key must be masked too, while a
  difference in any non-timing field still fails it.

  Scenario: Against kogen-verification main 3cb7305, whose step_sweep events carry a measured ms, the test passes five times in a row
    When k-build-budget-off-sweep-ms: go test -count=5 runs TestBuildRequestBudgetOff in package build against kogen-verification v0.0.0-20261103201747-3cb7305a5690
    Then k-build-budget-off-sweep-ms: TestBuildRequestBudgetOff passed in all 5 runs and failed in none

  Scenario: With the committed go.mod pin 01aa16c01767 the test passes
    Given k-build-budget-off-sweep-ms: go.mod requires github.com/KogenAI/kogen-verification at v0.0.0-20261103020618-01aa16c01767 without replacing it
    When k-build-budget-off-sweep-ms: go test -count=1 runs TestBuildRequestBudgetOff in package build with the committed go.mod
    Then k-build-budget-off-sweep-ms: TestBuildRequestBudgetOff passed in all 1 runs and failed in none

  Scenario: Two Builds that differ in a non-timing field of step_sweep still fail the test
    When k-build-budget-off-sweep-ms: go test -count=1 runs TestBuildRequestBudgetOff in package build against kogen-verification v0.0.0-20261103201747-3cb7305a5690 changed so that every step_sweep event's stopped counts up across the test process
    Then k-build-budget-off-sweep-ms: TestBuildRequestBudgetOff failed in its one run on its comparison, reporting a differing line that shows "stopped"
