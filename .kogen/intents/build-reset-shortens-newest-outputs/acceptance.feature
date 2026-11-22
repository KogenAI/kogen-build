Feature: A Build window reset shortens the oversized outputs of the newest exchange
  This feature checks prepareContext of the build owner through focused Go tests compiled into the build test binary.

  Background:
    Given build-reset-shortens-newest-outputs: the focused Go fixture is available

  Scenario: A newest exchange over the room shortens only the later oversized output
    When build-reset-shortens-newest-outputs: I run the focused Go test "TestShortenLaterOutputFirst"
    Then build-reset-shortens-newest-outputs: the focused Go test passed

  Scenario: Every call, assistant item and call identifier of the newest exchange stays unchanged
    When build-reset-shortens-newest-outputs: I run the focused Go test "TestShortenKeepsCallsAndIdentifiers"
    Then build-reset-shortens-newest-outputs: the focused Go test passed

  Scenario: The archive written for the reset holds the full original input
    When build-reset-shortens-newest-outputs: I run the focused Go test "TestShortenArchiveHoldsTheFullInput"
    Then build-reset-shortens-newest-outputs: the focused Go test passed

  Scenario: An unfit mandatory window is still refused with today's text and no archive
    When build-reset-shortens-newest-outputs: I run the focused Go test "TestShortenStillRefusesWhenTheMandatoryWindowDoesNotFit"
    Then build-reset-shortens-newest-outputs: the focused Go test passed

  Scenario: A reset that already fits leaves every output unchanged
    When build-reset-shortens-newest-outputs: I run the focused Go test "TestShortenLeavesAFittingWindowAlone"
    Then build-reset-shortens-newest-outputs: the focused Go test passed
