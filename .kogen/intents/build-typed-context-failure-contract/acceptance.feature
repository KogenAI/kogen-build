Feature: Type the Build diagnostic context failure contract
  This feature observes the focused test TestDiagnosticDataIsBoundedOriginalUTF8 in build/unit_test.go from outside. Each step runs only that test against the Build as it is, or against a variant of build/context.go with one property changed, and checks whether the test passes or fails.

  Scenario: The focused test passes on the Build as it is
    Then build-typed-context-failure-contract: the focused test passes on the Build as it is

  Scenario: The focused test checks the structured failure identity
    Then build-typed-context-failure-contract: the focused test rejects each of these Build variants:
      | a plain error that only spells intent/context_unavailable |
      | failure class build                                       |
      | failure kind context_missing                              |
      | exit code 1                                               |

  Scenario: The focused test keeps the exact size and UTF-8 checks
    Then build-typed-context-failure-contract: the focused test rejects each of these Build variants:
      | size counted in runes    |
      | text UTF-8 not checked   |

  Scenario: The focused test keeps its successful cases
    Then build-typed-context-failure-contract: the focused test rejects each of these Build variants:
      | text containing a carriage return refused |

  Scenario: The focused test leaves the wording to the CLI
    Then build-typed-context-failure-contract: the focused test still passes on each of these Build variants:
      | different failure wording |

  Scenario: Only the focused test file changes
    Then build-typed-context-failure-contract: build/unit_test.go is the only file changed outside this Intent
