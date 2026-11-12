Feature: Bounded, type-checked reading of the approval record and saved index copy
  This feature checks that the Build's comparison of the approval record and the index copy
  refuses oversized, non-regular and unreadable files with a distinct message naming the path.

  Background:
    Given k-build-approval-record-read-bounded: the driver is available
    When k-build-approval-record-read-bounded: the comparisons run on prepared files

  Scenario: Normal approval files compare as before
    Then k-build-approval-record-read-bounded: approval_same reports equal
    And k-build-approval-record-read-bounded: approval_different reports the approval changed

  Scenario: Normal index copies compare as before
    Then k-build-approval-record-read-bounded: index_same reports equal

  Scenario: An oversized approval record is refused as oversized
    Then k-build-approval-record-read-bounded: approval_oversized is refused as oversized naming the path

  Scenario: A named pipe in place of the approval is refused without blocking
    Then k-build-approval-record-read-bounded: approval_fifo is refused as irregular naming the path

  Scenario: An unreadable approval reports unreadable
    Then k-build-approval-record-read-bounded: approval_unreadable is refused as unreadable naming the path

  Scenario: An oversized index copy is refused as oversized
    Then k-build-approval-record-read-bounded: index_oversized is refused as oversized naming the path

  Scenario: A named pipe in place of the index is refused without blocking
    Then k-build-approval-record-read-bounded: index_fifo is refused as irregular naming the path
