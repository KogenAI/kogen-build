Feature: Build keeps the applicable Studio rules through resume
  This feature runs the real Build and resume entries against a loopback fake authenticated Studio rules endpoint with synthetic tokens and local fake executors; it makes no provider calls.

  Background:
    Given build-retain-rules-through-resume: the focused Go fixture is available

  Scenario: a fresh conversation after a usage limit carries the recorded rule block
    When build-retain-rules-through-resume: I run the fixture test "TestRetainRulesUsageLimit"
    Then build-retain-rules-through-resume: field "exit" is "0"
    And build-retain-rules-through-resume: the model request contains "- rule-1 r3 [tests] Run the focused test before finishing."
    And build-retain-rules-through-resume: the model request does not contain "Run every test twice."
    And build-retain-rules-through-resume: field "receipt.origin" is "live"
    And build-retain-rules-through-resume: field "receipt.rules.0.id" is "rule-1"
    And build-retain-rules-through-resume: field "receipt.rules.0.revision" is "3"
    And build-retain-rules-through-resume: field "receipt.content_hash" matches "^sha256:[0-9a-f]{64}$"
    And build-retain-rules-through-resume: field "studio_requests_after_resume" is "0"

  Scenario: an external-executor review round after resume carries the recorded rule block
    When build-retain-rules-through-resume: I run the fixture test "TestRetainRulesReview"
    Then build-retain-rules-through-resume: field "exit" is "0"
    And build-retain-rules-through-resume: the model request contains "- rule-1 r3 [tests] Run the focused test before finishing."
    And build-retain-rules-through-resume: the model request does not contain "Run every test twice."
    And build-retain-rules-through-resume: field "receipt.origin" is "live"
    And build-retain-rules-through-resume: field "receipt.rules.0.revision" is "3"
    And build-retain-rules-through-resume: field "studio_requests_after_resume" is "0"
