Feature: Build has a single implementation path
  This feature checks that removed Build forms stay removed and old checkpoints are refused.

  Scenario: Removed names are absent from Go code and tests
    Given simplify-build-single-path: the repository checkout
    Then simplify-build-single-path: no Go file mentions "BuildRequestBudget", "ReadLedger", "legacyConv" or "legacy checkpoint"

  Scenario: The legacy decoder file is gone
    Given simplify-build-single-path: the repository checkout
    Then simplify-build-single-path: the file "build/legacy.go" does not exist
