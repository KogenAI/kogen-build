Feature: Build applies the scoped Studio rule snapshot
  This feature runs the real Build entry against a loopback fake authenticated Studio rules endpoint with synthetic tokens and a scripted provider.

  Background:
    Given build-use-studio-rule-snapshot: the focused Go fixture is available

  Scenario: live rules reach the model request and the receipt
    When build-use-studio-rule-snapshot: I run the fixture test "TestStudioRulesLive"
    Then build-use-studio-rule-snapshot: field "exit" is "0"
    And build-use-studio-rule-snapshot: field "receipt.origin" is "live"
    And build-use-studio-rule-snapshot: field "receipt.rules.0.id" is "rule-1"
    And build-use-studio-rule-snapshot: field "receipt.rules.0.revision" is "3"
    And build-use-studio-rule-snapshot: the model request contains "- rule-1 r3 [tests] Run the focused test before finishing."
    And build-use-studio-rule-snapshot: field "receipt.content_hash" matches "^sha256:[0-9a-f]{64}$"
    And build-use-studio-rule-snapshot: field "studio_requests" is "1"
    And build-use-studio-rule-snapshot: field "token_leaked" is "false"

  Scenario: a changed revision reaches the next run
    When build-use-studio-rule-snapshot: I run the fixture test "TestStudioRulesChanged"
    Then build-use-studio-rule-snapshot: field "receipt.rules.0.revision" is "4"
    And build-use-studio-rule-snapshot: the model request contains "- rule-1 r4 [tests] Run every test twice."
    And build-use-studio-rule-snapshot: the model request does not contain "Run the focused test before finishing."
    And build-use-studio-rule-snapshot: field "receipt.origin" is "live"

  Scenario: an outage reports the cached revision
    When build-use-studio-rule-snapshot: I run the fixture test "TestStudioRulesOutage"
    Then build-use-studio-rule-snapshot: field "receipt.origin" is "cached"
    And build-use-studio-rule-snapshot: field "receipt.fallback_reason" is "http_503"
    And build-use-studio-rule-snapshot: field "receipt.rules.0.revision" is "3"
    And build-use-studio-rule-snapshot: the model request contains "- rule-1 r3 [tests] Run the focused test before finishing."

  Scenario: a revoked token never reuses the cache
    When build-use-studio-rule-snapshot: I run the fixture test "TestStudioRulesRevoked"
    Then build-use-studio-rule-snapshot: field "exit" is "non-zero"
    And build-use-studio-rule-snapshot: field "stderr" contains "rules/refused"
    And build-use-studio-rule-snapshot: field "stderr" contains "http_401"
    And build-use-studio-rule-snapshot: field "provider_requests" is "0"

  Scenario: unavailable without a cache is named
    When build-use-studio-rule-snapshot: I run the fixture test "TestStudioRulesUnavailable"
    Then build-use-studio-rule-snapshot: field "exit" is "non-zero"
    And build-use-studio-rule-snapshot: field "stderr" contains "rules/hold"
    And build-use-studio-rule-snapshot: field "provider_requests" is "0"

  Scenario: a cache of another scope is never used
    When build-use-studio-rule-snapshot: I run the fixture test "TestStudioRulesForeign"
    Then build-use-studio-rule-snapshot: field "exit" is "non-zero"
    And build-use-studio-rule-snapshot: field "stderr" contains "rules/hold"
    And build-use-studio-rule-snapshot: field "provider_requests" is "0"

  Scenario: an unconfigured project keeps standalone behavior
    When build-use-studio-rule-snapshot: I run the fixture test "TestStudioRulesUnconfigured"
    Then build-use-studio-rule-snapshot: field "exit" is "0"
    And build-use-studio-rule-snapshot: field "receipt" is absent
    And build-use-studio-rule-snapshot: field "studio_requests" is "0"

  Scenario: a resumed Build keeps the recorded snapshot
    When build-use-studio-rule-snapshot: I run the fixture test "TestStudioRulesResume"
    Then build-use-studio-rule-snapshot: field "exit" is "0"
    And build-use-studio-rule-snapshot: field "receipt.origin" is "live"
    And build-use-studio-rule-snapshot: field "receipt.rules.0.revision" is "3"
    And build-use-studio-rule-snapshot: field "checkpoint_receipt.rules.0.revision" is "3"
    And build-use-studio-rule-snapshot: field "studio_requests_after_resume" is "0"
    And build-use-studio-rule-snapshot: the model request does not contain "Run every test twice."

  Scenario: a review finding links the matching applied rule and revision
    When build-use-studio-rule-snapshot: I run the fixture test "TestStudioRulesReviewFinding"
    Then build-use-studio-rule-snapshot: field "finding0.applied_rule.id" is "rule-1"
    And build-use-studio-rule-snapshot: field "finding0.applied_rule.revision" is "3"
    And build-use-studio-rule-snapshot: field "finding1.applied_rule" is absent
