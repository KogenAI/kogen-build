Feature: Baseline reuse shadow
  This feature checks that Build's baseline observes reuse without skipping or changing anything.

  Background:
    Given build-baseline-reuse-shadow: the Go acceptance test of the shadow has run in a copy of the checkout

  Scenario: A green earlier receipt is reported as would reuse while the checks run again
    Then build-baseline-reuse-shadow: the subtest "s1" passed

  Scenario: A changed start tree is not reusable
    Then build-baseline-reuse-shadow: the subtest "s2" passed

  Scenario: A failed earlier receipt is not reusable
    Then build-baseline-reuse-shadow: the subtest "s3" passed

  Scenario: A reuse hit whose real check fails keeps the real verdict
    Then build-baseline-reuse-shadow: the subtest "s4" passed

  Scenario: An unwritable or corrupt receipt file changes nothing but the log
    Then build-baseline-reuse-shadow: the subtest "s5" passed

  Scenario: Without receipts the baseline logs what it logged before
    Then build-baseline-reuse-shadow: the subtest "s6" passed

  Scenario: A whole-Intent pass receipt is found by the next Build
    Then build-baseline-reuse-shadow: the subtest "s7" passed

  Scenario: The kogen-verification pin moves and the consumes suite covers the new exports
    Then build-baseline-reuse-shadow: go.mod pins another kogen-verification version than the previous one
    And build-baseline-reuse-shadow: the kogen-verification consumes suite mentions the reuse key or snapshot
