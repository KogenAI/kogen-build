Feature: Confine Run.Write to the run folder
  This feature checks that Run.Write keeps its writes inside the run folder. A local Go fixture starts a real run folder in a temporary directory and calls Run.Write with names that escape it and with a valid name. It makes no provider or model call.

  Scenario: A name that escapes through parent-directory components is rejected
    Given run-write-path-confinement: the Go fixture is available
    When run-write-path-confinement: I run the fixture test "TestAcceptRunWriteRejectsParentEscape"
    Then run-write-path-confinement: the fixture test passed

  Scenario: An absolute name is rejected and its target is unchanged
    Given run-write-path-confinement: the Go fixture is available
    When run-write-path-confinement: I run the fixture test "TestAcceptRunWriteRejectsAbsolutePath"
    Then run-write-path-confinement: the fixture test passed

  Scenario: A valid single-component name is still written as JSON and a newline
    Given run-write-path-confinement: the Go fixture is available
    When run-write-path-confinement: I run the fixture test "TestAcceptRunWriteKeepsSingleComponentNames"
    Then run-write-path-confinement: the fixture test passed
