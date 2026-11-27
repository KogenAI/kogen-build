# Bound the promotion fixture's observer CPU use

Slug: main-health-f8ebd4
Model: not applicable — the change bounds the CPU use of a test fixture and has no state to model
Acceptance:
- .kogen/intents/main-health-f8ebd4/acceptance.feature
- features/steps/main-health-f8ebd4__steps.js

The main-health qualification of the current main head ran every assembly scenario to a pass (271 of 271) yet the suite check failed, and the only text that identifies the failure is its duration, 11m 14.679s wall for 44m 2.495s of executing code, i.e. the four parallel workers were saturated. The only commit new on main since the last green tuple is 1af18550, which added the model-promotion fixture driver. Its `TestBMPEDAtomicPublication` starts one busy-polling observer per host CPU (`runtime.NumCPU()`) for each of three attempts; the observers never yield, so on a host that is already saturated by parallel workers they compete with the Build under test and with the other workers for CPU. The change bounds that CPU use: the atomic-publication fixture observes with a fixed small number of observers, independent of the host CPU count, that yield the processor when the pair is not yet visible, and it still detects a non-atomic publication. No scenario is removed, skipped or weakened, and `build/maintained.go` and the release scripts are not changed.

## Decisions
- The cause that this repository can demonstrate is the unbounded observer fan-out of the atomic-publication fixture added by 1af18550; the exact wall-clock budget of the assembly suite check and the other repositories' share of the 11m 14s are not observable from this repository, so if the next main-health run still fails for the same duration, the cause lies in the assembly repository (kogen), which owns the suite check and its time budget. (decision: D1, round 1) (evidence: features/steps/build-model-promotion-empty-directory-recovery__driver/fixture_test.go.txt; build/maintained.go) (assumption: the failure is a resource or time-budget failure of the suite check, not an assertion failure; the assembly run output shows 271 of 271 scenarios passed and only the duration line identifies it) (source: request)
- The atomic-publication fixture uses two observers instead of one per CPU and each yields with `runtime.Gosched()` while `model.qnt` is not yet visible; the three attempts, the checks of the observed directory and every other fixture test stay as they are. (decision: D2, round 1) (evidence: features/steps/build-model-promotion-empty-directory-recovery__driver/fixture_test.go.txt) (assumption: two observers detect a non-atomic publication reliably; observed in scratch runs on a 4-CPU Linux host only) (source: request)
- The CPU budget of the atomic-publication fixture is 7 CPU seconds (user plus system) for one run of its test; observed in scratch runs on a 4-CPU host: about 10 before the change, about 6 after. (decision: D3, round 1) (evidence: features/steps/build-model-promotion-empty-directory-recovery__steps.js) (assumption: the bound of 7 CPU seconds is a measurement-based choice on a 4-CPU host and may need adjustment on other hosts) (source: request)
- The fixture file belongs to the earlier Intent build-model-promotion-empty-directory-recovery and is edited in place; that Intent's acceptance still runs the same tests with the same assertions and its step file is not touched. (decision: D4, round 1) (evidence: features/steps/build-model-promotion-empty-directory-recovery__steps.js) (source: request)
- No flag, setting or environment variable is added and nothing outside this repository changes. (decision: D5, round 1) (source: request)
- No numeric CPU budget is asserted by the acceptance: the 7 CPU seconds figure is a scratch measurement only, and the acceptance instead asserts what the change is, that the fixture's observer count does not depend on the host CPU count (`runtime.NumCPU` is no longer used by the atomic-publication fixture), that the fixture test still passes, and that it still rejects a non-atomic publication. (decision: D6, round 2) (revises: D3 because a numeric bound is a literal that neither the request nor a tracked file states) (evidence: features/steps/build-model-promotion-empty-directory-recovery__driver/fixture_test.go.txt) (source: request)
- controls.json is deferred because the fixture to change lives in the steps directory, which a control overlay may not write, so no correct or wrong implementation overlay can express the change. (decision: D7, round 2) (evidence: features/steps/build-model-promotion-empty-directory-recovery__steps.js) (source: request)

## Source comparison
- Addressed: the failing 'suite' check of the main-health run, through the one repository-owned candidate cause (commit 1af18550) by scenario 1 (the fixture observes independently of the host CPU count and passes) and scenario 2 (the fixture still rejects a non-atomic publication, so the guard is not weakened).
- Omitted deliberately: the assembly suite and its time budget cannot be run or observed here; the Build cannot run it and the next main-health run decides. The report names the assembly repository as the owner if the failure persists.
- Omitted deliberately: other intents' fixtures were not measured; they are unchanged.

## Tests
- The atomic-publication fixture no longer scales its observers with the host CPU count and still passes: acceptance scenario 1 compiles and runs the real fixture test (integration)
- The fixture still fails a publication that is not atomic: acceptance scenario 2 compiles the same fixture against a non-atomic copy of build/maintained.go and expects the partial-model rejection (integration)
- The existing fixture tests and build tests keep passing unchanged: the repository's maintained checks (existing)
- none: the change does not process a collection of records, so there is no record-count scenario

## Feasibility
- none
