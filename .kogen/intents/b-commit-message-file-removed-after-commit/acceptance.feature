Feature: b-commit-message-file-removed-after-commit: kogen-build no longer depends on the commit message file after Commit returns

  kogen-coordinator's pending change removes the commit message file at git.CommitSpec.MessageFile
  after the commit, whether it succeeded, failed or was interrupted; its current main leaves the
  file behind. kogen-build's tests and Builds pass with either coordinator, and the test that checks
  the commit message reads it from the commit (git log -1 --format=%B).

  Scenario: kogen-build's commit tests pass whether the coordinator leaves the message file behind or removes it
    When b-commit-message-file-removed-after-commit: I run every Go test of "./consumes/kogen-coordinator" against the "pinned" kogen-coordinator
    Then b-commit-message-file-removed-after-commit: every test of that run passed and none was skipped, including:
      | test                                             |
      | TestCommitLandsTheReviewedTree                   |
      | TestCommitFailureLeavesHeadAndTheIndexRestorable |
      | TestCommitUndoesACommitAHookChanged              |
    When b-commit-message-file-removed-after-commit: I run every Go test of "./consumes/kogen-coordinator" against the "removed" kogen-coordinator
    Then b-commit-message-file-removed-after-commit: every test of that run passed and none was skipped, including:
      | test                                             |
      | TestCommitLandsTheReviewedTree                   |
      | TestCommitFailureLeavesHeadAndTheIndexRestorable |
      | TestCommitUndoesACommitAHookChanged              |
    When b-commit-message-file-removed-after-commit: I run these Go tests of "./build" against the "removed" kogen-coordinator:
      | test                                                                     |
      | TestBuildLandsOneCommitWithTheIntentTrailer                              |
      | TestACommitHookThatRejectsLeavesEditsAndRestoresTheStagedIntentIndex     |
      | TestAHookThatEditsTheCommitIsUndoneAndTheIndexRestored                   |
      | TestADefinitelyFailedCommitIsCheckpointedAsResumableAndResumeCommitsOnly |
      | TestASecondCommitFailureOnResumeLeavesAResumableCheckpointAgain          |
      | TestAHookUndoneCommitStaysAnUncertainBoundary                            |
    Then b-commit-message-file-removed-after-commit: every test of that run passed and none was skipped, including:
      | test                                                                     |
      | TestBuildLandsOneCommitWithTheIntentTrailer                              |
      | TestACommitHookThatRejectsLeavesEditsAndRestoresTheStagedIntentIndex     |
      | TestAHookThatEditsTheCommitIsUndoneAndTheIndexRestored                   |
      | TestADefinitelyFailedCommitIsCheckpointedAsResumableAndResumeCommitsOnly |
      | TestASecondCommitFailureOnResumeLeavesAResumableCheckpointAgain          |
      | TestAHookUndoneCommitStaysAnUncertainBoundary                            |

  Scenario: The coordinator tests check the committed message from the commit, not from the message file
    When b-commit-message-file-removed-after-commit: I run every Go test of "./consumes/kogen-coordinator" against the "changed-and-removed" kogen-coordinator
    Then b-commit-message-file-removed-after-commit: that run failed, a failing test reporting the committed message "Add the app (changed)" and no failing test reporting a missing file
