Feature: k-silent-shell-test-fast: the silent-shell tests run on controlled time

  The two silent-shell tests of ./build drive the loop's real shell wait on controlled time
  instead of a real 47-second sleep. A plain landing test of the same harness is the baseline:
  each silent-shell test takes less than 2 seconds longer than it. Each test still fails when the
  loop stops a silent command at its wait, and still passes when the loop's waits change.

  Scenario: The Build's silent-shell test is fast and still exercises the loop's yield
    When k-silent-shell-test-fast: I run the Go tests "TestSilentBuildShellNotStopped" and "TestBuildLandsOneCommitWithTheIntentTrailer" of the build package one at a time
    Then k-silent-shell-test-fast: the Go tests "TestSilentBuildShellNotStopped" and "TestBuildLandsOneCommitWithTheIntentTrailer" both passed
    And k-silent-shell-test-fast: the Go test "TestSilentBuildShellNotStopped" took less than 2 seconds longer than "TestBuildLandsOneCommitWithTheIntentTrailer"
    When k-silent-shell-test-fast: I run the Go test "TestSilentBuildShellNotStopped" against a kogen-loop whose shell wait stops the command when it elapses
    Then k-silent-shell-test-fast: the Go test "TestSilentBuildShellNotStopped" failed
    When k-silent-shell-test-fast: I run the Go test "TestSilentBuildShellNotStopped" against a kogen-loop whose shell waits are 90 and 60 seconds
    Then k-silent-shell-test-fast: the Go test "TestSilentBuildShellNotStopped" passed

  Scenario: The review's silent-shell test is fast and still exercises the loop's yield
    When k-silent-shell-test-fast: I run the Go tests "TestSilentReviewShellNotStopped" and "TestTheReviewerApprovesUnchangedCodeWithoutRepairs" of the build package one at a time
    Then k-silent-shell-test-fast: the Go tests "TestSilentReviewShellNotStopped" and "TestTheReviewerApprovesUnchangedCodeWithoutRepairs" both passed
    And k-silent-shell-test-fast: the Go test "TestSilentReviewShellNotStopped" took less than 2 seconds longer than "TestTheReviewerApprovesUnchangedCodeWithoutRepairs"
    When k-silent-shell-test-fast: I run the Go test "TestSilentReviewShellNotStopped" against a kogen-loop whose shell wait stops the command when it elapses
    Then k-silent-shell-test-fast: the Go test "TestSilentReviewShellNotStopped" failed
    When k-silent-shell-test-fast: I run the Go test "TestSilentReviewShellNotStopped" against a kogen-loop whose shell waits are 90 and 60 seconds
    Then k-silent-shell-test-fast: the Go test "TestSilentReviewShellNotStopped" passed
