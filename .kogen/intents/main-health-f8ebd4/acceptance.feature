Feature: Bound the CPU use of the model-promotion atomic-publication fixture
  This feature compiles the existing model-promotion fixture against the current build package
  and observes the atomic-publication test as a process.

  Background:
    Given main-health-f8ebd4: the model promotion fixture driver is available

  Scenario: The atomic publication check does not scale its observers with the host CPU count
    When main-health-f8ebd4: I run the atomic publication fixture
    Then main-health-f8ebd4: the fixture passed with an observer count independent of the host CPU count

  Scenario: The atomic publication check still rejects a non-atomic publication
    When main-health-f8ebd4: I run the atomic publication fixture against a non-atomic publication
    Then main-health-f8ebd4: the fixture rejected the publication as partial
