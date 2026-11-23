Feature: kogen-build refuses flags that do not belong to the selected mode
  This feature compiles the standalone kogen-build program and runs it with mode-specific flags against an empty, non-Git project directory.

  Scenario: A build with --id is refused
    Given reject-mode-inappropriate-cli-flags: the kogen-build program is compiled
    When reject-mode-inappropriate-cli-flags: it runs with the arguments "build --slug demo --id some-build"
    Then reject-mode-inappropriate-cli-flags: stdout is one JSON usage refusal and the exit status is 2
    Then reject-mode-inappropriate-cli-flags: the refusal message names "--id" and not "--slug"

  Scenario: A resume with --slug is refused
    Given reject-mode-inappropriate-cli-flags: the kogen-build program is compiled
    When reject-mode-inappropriate-cli-flags: it runs with the arguments "resume --id some-build --slug demo"
    Then reject-mode-inappropriate-cli-flags: stdout is one JSON usage refusal and the exit status is 2
    Then reject-mode-inappropriate-cli-flags: the refusal message names "--slug" and not "--id"

  Scenario: A build with --slug is accepted
    Given reject-mode-inappropriate-cli-flags: the kogen-build program is compiled
    When reject-mode-inappropriate-cli-flags: it runs with the arguments "build --slug demo"
    Then reject-mode-inappropriate-cli-flags: the program was not refused as a usage error

  Scenario: A resume with --id is accepted
    Given reject-mode-inappropriate-cli-flags: the kogen-build program is compiled
    When reject-mode-inappropriate-cli-flags: it runs with the arguments "resume --id some-build"
    Then reject-mode-inappropriate-cli-flags: the program was not refused as a usage error
