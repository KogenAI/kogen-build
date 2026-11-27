Feature: Command-level coverage of the standalone reconstruction modes
  This feature checks the command package tests rather than the command itself. The tests must pass on the unmodified tree. They must also fail on a test assertion when cmd/kogen-build/main.go is deliberately broken in one way, so that testing only the argument parser or only the library result would not be enough.

  Scenario: Readback of an unrecorded item must exit 1
    Given standalone-reconstruction-json-contract: the kogen-build checkout
    When standalone-reconstruction-json-contract: the command package tests run unmodified and with the "the readback exits 0 for an item with no recorded reconstruction" mutation of main.go
    Then standalone-reconstruction-json-contract: the unmodified tests pass and the mutated tests fail on a test assertion

  Scenario: Readback of an unrecorded item must not write into the checkout
    Given standalone-reconstruction-json-contract: the kogen-build checkout
    When standalone-reconstruction-json-contract: the command package tests run unmodified and with the "the readback creates custody state in the checkout" mutation of main.go
    Then standalone-reconstruction-json-contract: the unmodified tests pass and the mutated tests fail on a test assertion

  Scenario: An interrupted reconstruct run must report state interrupted
    Given standalone-reconstruction-json-contract: the kogen-build checkout
    When standalone-reconstruction-json-contract: the command package tests run unmodified and with the "reconstruct reports an interrupted run as held" mutation of main.go
    Then standalone-reconstruction-json-contract: the unmodified tests pass and the mutated tests fail on a test assertion

  Scenario: An interrupted reconstruct run must stop before any attempt or wake
    Given standalone-reconstruction-json-contract: the kogen-build checkout
    When standalone-reconstruction-json-contract: the command package tests run unmodified and with the "reconstruct ignores the stop-after value" mutation of main.go
    Then standalone-reconstruction-json-contract: the unmodified tests pass and the mutated tests fail on a test assertion

  Scenario: An invalid stop-after must be refused with usage exit 2
    Given standalone-reconstruction-json-contract: the kogen-build checkout
    When standalone-reconstruction-json-contract: the command package tests run unmodified and with the "the command accepts an invalid stop-after value" mutation of main.go
    Then standalone-reconstruction-json-contract: the unmodified tests pass and the mutated tests fail on a test assertion

  Scenario: An invalid stop-after must not create files before refusing
    Given standalone-reconstruction-json-contract: the kogen-build checkout
    When standalone-reconstruction-json-contract: the command package tests run unmodified and with the "the command creates the archive before refusing an invalid stop-after value" mutation of main.go
    Then standalone-reconstruction-json-contract: the unmodified tests pass and the mutated tests fail on a test assertion
