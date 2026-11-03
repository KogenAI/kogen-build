Feature: Standalone developer Build entry point
  Scenario: Build preserves approval refusal and reports the checkout
    Then the standalone Build command refuses an unapproved fixture with exit 5 and kind "not_approved"

  Scenario: Resume preserves unsupported checkpoint refusal
    Then the standalone Resume command refuses checkpoint "00000000" with exit 1 and kind "resume_unsupported"
