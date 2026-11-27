Feature: facts-history-cap-regression: the last-20 facts history limit is covered by regression tests

  This feature checks that build/facts_test.go pins the last-20 history limit of build/facts.go:
  inheriting a previous run's facts keeps the newest 20 verification entries and, independently,
  the newest 20 review entries, in order; recording a 21st evidence entry keeps the newest 20 and
  sets evidence_truncated to true. Each scenario runs the Go tests against the unchanged
  build/facts.go and against copies with one defective limit, which the tests must reject.

  Scenario: Inheriting more than 20 verification entries keeps exactly the newest 20 in order
    When facts-history-cap-regression: I run the Go tests of build/facts_test.go against the unchanged build/facts.go
    Then facts-history-cap-regression: every one of those tests ran and passed
    And facts-history-cap-regression: the Go tests of build/facts_test.go fail against each of these build/facts.go copies:
      | copy                   | defect                                              |
      | verification-oldest    | inheriting keeps the oldest 20 verification entries |
      | verification-unbounded | inheriting keeps every verification entry          |

  Scenario: Inheriting more than 20 review entries keeps the newest 20 independently of verification
    When facts-history-cap-regression: I run the Go tests of build/facts_test.go against the unchanged build/facts.go
    Then facts-history-cap-regression: every one of those tests ran and passed
    And facts-history-cap-regression: the Go tests of build/facts_test.go fail against each of these build/facts.go copies:
      | copy             | defect                                                         |
      | review-oldest    | inheriting keeps the oldest 20 review entries                  |
      | review-unbounded | inheriting keeps every review entry                            |
      | shared-limit     | one limit of 20 is shared by the verification and review lists |

  Scenario: Recording a 21st evidence entry keeps the newest 20 and reports the truncation
    When facts-history-cap-regression: I run the Go tests of build/facts_test.go against the unchanged build/facts.go
    Then facts-history-cap-regression: every one of those tests ran and passed
    And facts-history-cap-regression: the Go tests of build/facts_test.go fail against each of these build/facts.go copies:
      | copy               | defect                                                 |
      | evidence-silent    | entries are dropped without setting evidence_truncated |
      | evidence-oldest    | evidence keeps the oldest 20 entries                   |
      | evidence-unbounded | evidence keeps every entry                             |

  Scenario: The facts rule stays unchanged and only the facts tests change
    Then facts-history-cap-regression: build/facts.go keeps its last-20 limit code and no file besides the Intent, its acceptance steps and "build/facts_test.go" is changed
