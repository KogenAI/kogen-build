Feature: Direct file-index accounting and determinism regressions
  This feature adds tests only in build/file_index_test.go. The existing renderer
  remains unchanged. Tests call fileIndex with a large NUL-delimited listing and
  required paths, and inspect its returned index. Scratch renderer faults prove
  that the regression assertions are effective rather than merely passing today.

  Background:
    Given file-index-omission-accounting: the file-index source and existing regression file are available

  Scenario: Omitted paths form a disjoint partition
    Then file-index-omission-accounting: the regression tests reject overlapping and dropped omission counts

  Scenario: Input order does not affect the index
    Then file-index-omission-accounting: the regression tests reject input-order-dependent output

  Scenario: The regression fixture keeps its existing guards
    Then file-index-omission-accounting: the regression tests reject required-path loss and oversized output
