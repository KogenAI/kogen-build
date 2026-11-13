Feature: k-build-approval-removal-failure-visible: a landed Build reports when it could not remove the spent approval file

  This feature covers a Build that lands its commit and removes the approval file of the intent.

  Scenario: Landing reports a failed approval removal once and otherwise reports nothing
    When k-build-approval-removal-failure-visible: I run the landing scenarios of the Build
    Then k-build-approval-removal-failure-visible: the subtest "normal" passed
    And k-build-approval-removal-failure-visible: the subtest "already-gone" passed
    And k-build-approval-removal-failure-visible: the subtest "unremovable" passed
