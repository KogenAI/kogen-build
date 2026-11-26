Feature: Regression coverage for control characters in the file index
  This feature adds one test function only in build/file_index_test.go. It uses
  the real fileIndex with a NUL-delimited listing containing both kinds of path.
  The documented escape text must appear on one complete line, with separate checks
  for each exact escape and for absence of each raw character within the path.
  An ordinary UTF-8 path must appear unchanged in the output of the same call.
  It does not change path ordering, omission selection, or the file index size policy.
  The unit separator and ordinary path below are representative examples.

  Scenario: Control path regression passes normally and catches rendering faults
    Given file-index-control-path-coverage: the documented escape cases and ordinary path
      """json
      {
        "cases": [
          {"name": "newline", "raw": "\n", "escaped": "\\n"},
          {"name": "tab", "raw": "\t", "escaped": "\\t"},
          {"name": "carriage return", "raw": "\r", "escaped": "\\r"},
          {"name": "backslash", "raw": "\\", "escaped": "\\\\"},
          {"name": "unit separator", "raw": "\u001f", "escaped": "\\x1f"}
        ],
        "ordinary": "docs/café.txt"
      }
      """
    Then file-index-control-path-coverage: exactly one added test runs and passes with the unchanged renderer
    And file-index-control-path-coverage: the added test rejects each raw byte, a rewritten ordinary path, and a bypass in fileIndex
