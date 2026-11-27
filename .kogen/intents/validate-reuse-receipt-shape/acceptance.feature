Feature: Reuse receipt shape
  This feature checks that incomplete reuse receipt records are corrupt while complete ones stay readable.

  Background:
    Given validate-reuse-receipt-shape: the Go acceptance test of the receipt shape has run in a copy of the checkout

  Scenario: A receipt line with only a key and status zero is corrupt
    Then validate-reuse-receipt-shape: the subtest "r1" passed

  Scenario: A Build meeting an incomplete receipt records one unavailable event and runs every baseline check
    Then validate-reuse-receipt-shape: the subtest "r2" passed

  Scenario: Complete baseline and pass receipts remain readable and produce observations
    Then validate-reuse-receipt-shape: the subtest "r3" passed
