Feature: Build restores an edited acceptance file once
  This feature makes the Build baseline guard restore an approved acceptance file that a shell call changed or added, once per attempt.
  A local Go fixture runs the real scripted native Build in a temporary repository with an approved Intent. It makes no model call.

  Scenario: An approved controls file the Build appended to is restored and the Build continues
    Given build-restores-edited-acceptance-file: the focused Go fixture is available
    When build-restores-edited-acceptance-file: I run the fixture test "TestAcceptanceRestoreAppendedControlsFile"
    Then build-restores-edited-acceptance-file: the fixture test passed

  Scenario: A file the Build added to the Intent folder is removed and the Build continues
    Given build-restores-edited-acceptance-file: the focused Go fixture is available
    When build-restores-edited-acceptance-file: I run the fixture test "TestAcceptanceRestoreRemovesAddedIntentFile"
    Then build-restores-edited-acceptance-file: the fixture test passed

  Scenario: A second acceptance edit in the same attempt fails the attempt
    Given build-restores-edited-acceptance-file: the focused Go fixture is available
    When build-restores-edited-acceptance-file: I run the fixture test "TestAcceptanceSecondEditFailsTheAttempt"
    Then build-restores-edited-acceptance-file: the fixture test passed
