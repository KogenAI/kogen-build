Feature: host-memory-failure-regression: memory failure classification and the host-or-base stop are covered by regression tests

  This feature checks that build/env_failure_park_test.go pins the memory failure behavior of
  build/envfailure.go: each recognized memory phrase classifies as memory, a check reporting
  different memory phrases in consecutive passes stops the Build quoting the second pass's line,
  and a memory failure after a different environment failure kind does not stop it. Each scenario
  runs the new Go tests against the unchanged build/envfailure.go and against copies with one
  defect, which the tests must reject.

  Scenario: Each recognized memory phrase classifies as memory
    When host-memory-failure-regression: I run the new tests of the host-or-base stop against the unchanged environment failure code
    Then host-memory-failure-regression: every one of those tests ran and passed
    And host-memory-failure-regression: the new tests fail against each of these environment failure code copies:
      | copy                               | defect                                                   |
      | memory-allocate-unrecognised       | "cannot allocate memory" is not classified as memory     |
      | memory-out-of-memory-unrecognised  | "out of memory" is not classified as memory              |
      | memory-killed-process-unrecognised | "Out of memory: Killed process" is not classified memory |

  Scenario: Different memory phrases in consecutive passes stop the Build quoting the second line
    When host-memory-failure-regression: I run the new tests of the host-or-base stop against the unchanged environment failure code
    Then host-memory-failure-regression: every one of those tests ran and passed
    And host-memory-failure-regression: the new tests fail against each of these environment failure code copies:
      | copy                | defect                                                       |
      | full-lines-compared | the stop compares full lines instead of failure kinds        |
      | first-line-quoted   | the stop reports the first pass's line instead of the second |

  Scenario: A memory failure after a different environment failure kind does not stop the Build
    When host-memory-failure-regression: I run the new tests of the host-or-base stop against the unchanged environment failure code
    Then host-memory-failure-regression: every one of those tests ran and passed
    And host-memory-failure-regression: the new tests fail against each of these environment failure code copies:
      | copy                          | defect                                           |
      | any-environment-failure-stops | any two consecutive environment failures stop it |

  Scenario: The environment failure rules stay unchanged and only the park tests change
    Then host-memory-failure-regression: build/envfailure.go keeps its code and no file besides the Intent, its acceptance steps and "build/env_failure_park_test.go" is changed
