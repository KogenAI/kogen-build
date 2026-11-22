Feature: Standalone resume refusal contract
  This feature checks that the build package's standalone tests cover the refusal of resume for an unrecorded Build through the built executable.

  Scenario: The standalone resume refusal is tested and the test rejects contract violations
    Given standalone-resume-json-contract: the repository checkout
    When standalone-resume-json-contract: the standalone tests of the build package run
    Then standalone-resume-json-contract: a test added to the standalone test file passes
    Then standalone-resume-json-contract: the added test fails when stdout holds a second JSON value
    Then standalone-resume-json-contract: the added test fails when the refusal creates a run
    Then standalone-resume-json-contract: the added test fails when the structured error is missing
