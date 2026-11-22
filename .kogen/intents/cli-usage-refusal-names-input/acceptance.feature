Feature: kogen-build usage refusals name the offending input
  This feature compiles the standalone kogen-build program and runs it with invalid command lines.

  Scenario: An unknown mode is named in the refusal
    Given cli-usage-refusal-names-input: the kogen-build program is compiled
    When cli-usage-refusal-names-input: it runs with the arguments "frobnicate --project ."
    Then cli-usage-refusal-names-input: stdout is one JSON usage refusal and the exit status is 2
    Then cli-usage-refusal-names-input: the error message contains "frobnicate"

  Scenario: An unknown flag is named in the refusal
    Given cli-usage-refusal-names-input: the kogen-build program is compiled
    When cli-usage-refusal-names-input: it runs with the arguments "build --slug demo --bogus-flag"
    Then cli-usage-refusal-names-input: stdout is one JSON usage refusal and the exit status is 2
    Then cli-usage-refusal-names-input: the error message contains "bogus-flag"

  Scenario: An extra positional argument is named in the refusal
    Given cli-usage-refusal-names-input: the kogen-build program is compiled
    When cli-usage-refusal-names-input: it runs with the arguments "resume --id demo stray-argument"
    Then cli-usage-refusal-names-input: stdout is one JSON usage refusal and the exit status is 2
    Then cli-usage-refusal-names-input: the error message contains "stray-argument"

  Scenario: A build without a slug is refused as JSON
    Given cli-usage-refusal-names-input: the kogen-build program is compiled
    When cli-usage-refusal-names-input: it runs with the arguments "build --project ."
    Then cli-usage-refusal-names-input: stdout is one JSON usage refusal and the exit status is 2

  Scenario: A resume without an id is refused as JSON
    Given cli-usage-refusal-names-input: the kogen-build program is compiled
    When cli-usage-refusal-names-input: it runs with the arguments "resume --project ."
    Then cli-usage-refusal-names-input: stdout is one JSON usage refusal and the exit status is 2
