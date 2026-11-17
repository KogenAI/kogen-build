Feature: Build leaves shared test support unedited
  This feature makes the Build brief tell the builder not to edit files every test loads, and makes the landed summary name such files when the Build changed them.
  A local Go fixture runs the real scripted native Build in a temporary repository. It makes no model call.

  Scenario: The Build brief states the shared test support rule
    Given build-leaves-shared-test-support-unedited: the focused Go fixture is available
    When build-leaves-shared-test-support-unedited: I run the fixture test "TestAcceptanceSharedSupportBriefStatesTheRule"
    Then build-leaves-shared-test-support-unedited: the fixture test passed

  Scenario: A changed file the README lists as test support is named in the landed summary
    Given build-leaves-shared-test-support-unedited: the focused Go fixture is available
    When build-leaves-shared-test-support-unedited: I run the fixture test "TestAcceptanceSharedSupportListedFileChangedIsNamed"
    Then build-leaves-shared-test-support-unedited: the fixture test passed

  Scenario: Without a README listing the files the unchanged tests require first and the manifest are named
    Given build-leaves-shared-test-support-unedited: the focused Go fixture is available
    When build-leaves-shared-test-support-unedited: I run the fixture test "TestAcceptanceSharedSupportRequiredFirstFilesAndManifestAreNamed"
    Then build-leaves-shared-test-support-unedited: the fixture test passed

  Scenario: A Build that only adds new test files has no shared test support line
    Given build-leaves-shared-test-support-unedited: the focused Go fixture is available
    When build-leaves-shared-test-support-unedited: I run the fixture test "TestAcceptanceSharedSupportNewTestFilesOnlyHasNoLine"
    Then build-leaves-shared-test-support-unedited: the fixture test passed
