Feature: Reset message lists recent history only
  This feature checks that a working-window reset message lists at most eight archive references and summarises the rest.

  Background:
    Given build-reset-lists-recent-history-only: the Go acceptance test of the reset history has run in a copy of the checkout

  Scenario: The reset message lists the new archive and the seven before it
    Then build-reset-lists-recent-history-only: the subtest "s1" passed

  Scenario: The omitted windows are summarised by count and first and last paths
    Then build-reset-lists-recent-history-only: the subtest "s2" passed

  Scenario: The saved working context keeps every reference
    Then build-reset-lists-recent-history-only: the subtest "s3" passed

  Scenario: The reset message does not grow with the number of resets
    Then build-reset-lists-recent-history-only: the subtest "s4" passed

  Scenario: A window of at most 8 references is unchanged and has no omitted object
    Then build-reset-lists-recent-history-only: the subtest "s5a" passed

  Scenario: Eight prior references list eight and omit one
    Then build-reset-lists-recent-history-only: the subtest "s5b" passed

  Scenario: The golden reset comparison passes unchanged
    Then build-reset-lists-recent-history-only: the existing golden reset test passed
