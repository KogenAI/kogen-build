Feature: The host-or-base stop matches the kind of an environment failure
  This feature checks that a check failing with the same kind of environment failure in two consecutive builder passes stops the Build, whatever its first line names.

  Scenario: Disk failures naming different files stop the Build
    Given build-host-stop-matches-failure-kind: the kogen-build checkout
    When build-host-stop-matches-failure-kind: the Build test "TestHostStopAcc1DiskLinesNamingDifferentFiles" runs in the build package
    Then build-host-stop-matches-failure-kind: the Build stops after pass 2 quoting the second disk line and asks for no further round

  Scenario: Failures of different kinds do not stop the Build
    Given build-host-stop-matches-failure-kind: the kogen-build checkout
    When build-host-stop-matches-failure-kind: the Build test "TestHostStopAcc2DifferentKinds" runs in the build package
    Then build-host-stop-matches-failure-kind: the Build does not stop for the host or base after a disk failure then a memory failure

  Scenario: An ordinary failure in the previous pass does not stop the Build
    Given build-host-stop-matches-failure-kind: the kogen-build checkout
    When build-host-stop-matches-failure-kind: the Build test "TestHostStopAcc3OrdinaryFailureThenDisk" runs in the build package
    Then build-host-stop-matches-failure-kind: the Build does not stop for the host or base after an ordinary failure then a disk failure

  Scenario: Disk failures in different checks do not stop the Build
    Given build-host-stop-matches-failure-kind: the kogen-build checkout
    When build-host-stop-matches-failure-kind: the Build test "TestHostStopAcc4DifferentChecks" runs in the build package
    Then build-host-stop-matches-failure-kind: the Build does not stop for the host or base when the disk failures are in different checks

  Scenario: Identical disk lines still stop the Build with the existing message and event
    Given build-host-stop-matches-failure-kind: the kogen-build checkout
    When build-host-stop-matches-failure-kind: the Build test "TestHostStopAcc5IdenticalLines" runs in the build package
    Then build-host-stop-matches-failure-kind: the Build still stops with the existing message and event when the disk lines are identical
