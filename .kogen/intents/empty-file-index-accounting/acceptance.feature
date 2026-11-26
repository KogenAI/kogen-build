Feature: Account for empty file inventories
  Scenario: Empty listing
    Given empty-file-index-accounting: a Git listing encoded as hex ""
    Then empty-file-index-accounting: the index reports 0 total, 0 listed, 0 omitted and lists no paths

  Scenario: NUL-only listing
    Given empty-file-index-accounting: a Git listing encoded as hex "00"
    Then empty-file-index-accounting: the index reports 0 total, 0 listed, 0 omitted and lists no paths

  Scenario: One-path listing
    Given empty-file-index-accounting: a Git listing encoded as hex "6100" with required path "a"
    Then empty-file-index-accounting: the index reports 1 total, 1 listed, 0 omitted and lists "a" exactly once
