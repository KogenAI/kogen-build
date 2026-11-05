Feature: k-suite-load-reliable: the build package's held-child tests wait on events, not on wall-clock deadlines

  Twelve tests of ./build start a Build or a native round, wait until its held child (a setup
  command, a baseline check, a builder or reviewer shell call, a stage gate, a verification check or
  a post-commit hook) is running, and then interrupt it. They waited under fixed wall-clock deadlines
  of 5 to 120 seconds, so on a busy host a Build that was still working when the deadline passed
  failed its test ("timed out waiting for .../hook-waiting"). A git that waits before every call
  stands in for that host. Each test now waits until its held child runs or its Build ends, and still
  fails, with its own assertion's message, against a deliberately broken implementation of what it
  checks.

  Scenario: Held-child tests pass when every git call takes longer, as on a busy host
    When k-suite-load-reliable: I run these Go tests of the build package at the same time, with every git call delayed:
      | seconds | test                                           |
      | 0.25    | TestACommitInterruptedDuringPostcommitIsUndone |
      | 0.25    | TestResumeRefusesAnUncertainToolBoundary       |
      | 0.5     | TestNativeShellContextCancellationStops        |
    Then k-suite-load-reliable: each of those Go tests passed, and git calls were delayed

  Scenario: Every changed test still fails against a deliberately broken implementation of what it checks
    When k-suite-load-reliable: I run these Go tests of the build package at the same time, each against its broken implementation:
      | broken implementation      | test                                                                                | message                                                                                       |
      | interrupted-commit-kept    | TestACommitInterruptedDuringPostcommitIsUndone                                      | head = "                                                                                      |
      | uncertain-boundary-resumed | TestAnInterruptedStartupRestoresTheIndexAndCannotResume                             | resume lacks "uncertain startup boundary"                                                     |
      | uncertain-boundary-resumed | TestResumeRefusesAnUncertainToolBoundary                                            | resume lacks "uncertain tool boundary; inspect effects before starting a freshly approved Build" |
      | uncertain-boundary-resumed | TestAnInterruptedTargetedRepairReportsPendingReviewAndChangedVerification           | resume lacks "candidate/resume_refused: Build                                                 |
      | every-stage-refused        | TestInterruptingAStageGateRetainsNoCompletedEvidenceAndResumeRerunsSetupAndCommands | resume exit = 1, want 4                                                                       |
      | every-stage-refused        | TestExternalBuilderReviewStageResumes                                               | resume exit = 1, want 0                                                                       |
      | every-stage-refused        | TestAnInterruptedVerificationOfAReviewerRepairResumesWithoutANewRequest             | uncertain review_verify boundary                                                              |
      | baseline-stop-unnamed      | TestInterruptedBaselineCheckNamesTheBaseline                                        | stdout lacks "stopped fixture: interrupted (Build                                             |
      | baseline-stop-unnamed      | TestErrorStopDuringBaselineCheckNamesTheBaseline                                    | want command_no_progress stopped during baseline                                              |
      | native-interrupt-lost      | TestNativeRoundMapsCancellationButKeepsTheSessionsInterrupt                         | error the interrupt was lost                                                                  |
      | native-interrupt-lost      | TestChecktimeClockReadiness                                                         | the interrupt was lost                                                                        |
      | native-cancel-succeeds     | TestNativeShellContextCancellationStops                                             | cancellation returned success                                                                 |
    Then k-suite-load-reliable: each of those Go tests failed with its own message
