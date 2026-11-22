Feature: The kogen-build go-test check finishes inside its bound
  This feature checks that the tests of the build package finish in time.

  Scenario: The build package tests pass inside a 100 second timeout
    Given base-red-kogen-build: the repository checkout
    When base-red-kogen-build: the build package tests run with a 100 second timeout
    Then base-red-kogen-build: the tests pass without a timeout
