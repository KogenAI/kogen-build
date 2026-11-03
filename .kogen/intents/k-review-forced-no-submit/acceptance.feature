Feature: Forced verdict provider failure
  Scenario: Both bounded reviews ignore forced submit_review
    Then ignored forced verdict requests stop with actionable provider diagnostics without extra requests
