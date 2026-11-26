Feature: Display file index paths unambiguously
  This feature makes the builder file index show distinct repository paths differently when they contain backslashes or control characters.
  A local Go fixture calls the real file index builder with synthetic path listings. It makes no provider or model call and does not touch the filesystem.

  Scenario: A literal backslash n filename displays differently from a newline n filename
    Given unambiguous-file-index-paths: the focused Go fixture is available
    When unambiguous-file-index-paths: I run the fixture test "TestAcceptanceUnambiguousPathsLiteralBackslashN"
    Then unambiguous-file-index-paths: the fixture test passed

  Scenario: Backslashes are escaped even without a control character
    Given unambiguous-file-index-paths: the focused Go fixture is available
    When unambiguous-file-index-paths: I run the fixture test "TestAcceptanceUnambiguousPathsBackslashWithoutControl"
    Then unambiguous-file-index-paths: the fixture test passed

  Scenario: Ordinary paths stay unchanged and control characters keep deterministic escapes
    Given unambiguous-file-index-paths: the focused Go fixture is available
    When unambiguous-file-index-paths: I run the fixture test "TestAcceptanceUnambiguousPathsOrdinaryAndControl"
    Then unambiguous-file-index-paths: the fixture test passed
