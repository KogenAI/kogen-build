Feature: A no-edit round after an environment red is verified again
  This feature checks that a builder round that leaves the tree of an environment-failed red pass unchanged is verified again, so the host-or-base rule stops the Build, while an ordinary red keeps the no-edit notice.

  Scenario: Environment timeout repeated after a no-edit round stops as host or base
    Given build-env-red-no-edit-host-stop: the kogen-build checkout
    When build-env-red-no-edit-host-stop: the Build test "TestEnvRedAcc1TimeoutRepeatedAfterANoEditRound" runs in the build package
    Then build-env-red-no-edit-host-stop: the Build stops as environment host_or_base_failure with exit 3 after two passes and no no-edit notice

  Scenario: Ordinary failure in the second pass is ordinary feedback
    Given build-env-red-no-edit-host-stop: the kogen-build checkout
    When build-env-red-no-edit-host-stop: the Build test "TestEnvRedAcc2OrdinaryFailureInTheSecondPassIsOrdinaryFeedback" runs in the build package
    Then build-env-red-no-edit-host-stop: the Build gives the ordinary feedback of pass 2 and stops for neither the host nor no_progress

  Scenario: Ordinary red keeps the notice and the no_progress stop
    Given build-env-red-no-edit-host-stop: the kogen-build checkout
    When build-env-red-no-edit-host-stop: the Build test "TestEnvRedAcc3OrdinaryRedKeepsTheNoticeAndTheNoProgressStop" runs in the build package
    Then build-env-red-no-edit-host-stop: the first no-edit round gets the notice and the second stops as candidate no_progress without another pass

  Scenario: Disk failure repeated after a no-edit round stops as host or base
    Given build-env-red-no-edit-host-stop: the kogen-build checkout
    When build-env-red-no-edit-host-stop: the Build test "TestEnvRedAcc4DiskFailureRepeatedAfterANoEditRound" runs in the build package
    Then build-env-red-no-edit-host-stop: the Build stops as environment host_or_base_failure naming check test with exit 3 after two passes and no no-edit notice

  Scenario: An ordinary failing check next to a disk failing check is verified again
    Given build-env-red-no-edit-host-stop: the kogen-build checkout
    When build-env-red-no-edit-host-stop: the Build test "TestEnvRedAcc6OrdinaryAndDiskFailingChecksVerifyAgain" runs in the build package
    Then build-env-red-no-edit-host-stop: a round 2 without edits after pass 1 failing check vet ordinarily and check test with a disk failure is verified again without the notice

  Scenario: Stage gate timeout repeated after a no-edit round stops as host or base
    Given build-env-red-no-edit-host-stop: the kogen-build checkout
    When build-env-red-no-edit-host-stop: the Build test "TestEnvRedAcc5StageGateTimeoutRepeatedAfterANoEditRound" runs in the build package
    Then build-env-red-no-edit-host-stop: the Build stops as environment host_or_base_failure naming the stage check with exit 3 after two passes and no no-edit notice
