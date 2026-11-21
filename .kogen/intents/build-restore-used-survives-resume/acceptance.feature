Feature: A resumed Build keeps whether its one baseline restore was used
  This feature runs the real Build and Resume against a scripted provider in a temporary checkout,
  through driver tests run by name.

  Scenario: The used restore survives a resume
    When build-restore-used-survives-resume: I run the driver test "TestAccRestoreUsedKeptByResume"
    Then build-restore-used-survives-resume: that driver test ran and passed
    And build-restore-used-survives-resume: the persisted state type declares the JSON key "restore_used"

  Scenario: An unused restore is still available after a resume
    When build-restore-used-survives-resume: I run the driver test "TestAccUnusedRestoreAvailableAfterResume"
    Then build-restore-used-survives-resume: that driver test ran and passed

  Scenario: A record without restore_used resumes with the restore available
    When build-restore-used-survives-resume: I run the driver test "TestAccRecordWithoutRestoreUsedResumes"
    Then build-restore-used-survives-resume: that driver test ran and passed

  Scenario: The used value survives two resumes
    When build-restore-used-survives-resume: I run the driver test "TestAccRestoreUsedKeptByTwoResumes"
    Then build-restore-used-survives-resume: that driver test ran and passed
