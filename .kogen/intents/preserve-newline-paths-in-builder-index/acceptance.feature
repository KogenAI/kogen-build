Feature: Newline filenames in the builder file index
  This feature checks that the builder file index keeps a filename containing a newline as one escaped path.

  Scenario: A newline filename is one escaped entry with true counts
    Given preserve-newline-paths-in-builder-index: the focused Go fixture is available
    When preserve-newline-paths-in-builder-index: I run the fixture test "TestNewlineFilenameIsOneEscapedEntryWithTrueCounts"
    Then preserve-newline-paths-in-builder-index: the fixture test passed

  Scenario: A required newline path stays one entry when the listing exceeds the limit
    Given preserve-newline-paths-in-builder-index: the focused Go fixture is available
    When preserve-newline-paths-in-builder-index: I run the fixture test "TestRequiredNewlinePathStaysOneEntryWhenOversized"
    Then preserve-newline-paths-in-builder-index: the fixture test passed

  Scenario: Ordinary paths keep their display and oversized listings stay bounded
    Given preserve-newline-paths-in-builder-index: the focused Go fixture is available
    When preserve-newline-paths-in-builder-index: I run the fixture test "TestOrdinaryPathsKeepDisplayAndOversizedStaysBounded"
    Then preserve-newline-paths-in-builder-index: the fixture test passed
