Feature: Standalone CLI refusals identify the expected input

  Background:
    Given cli-refusal-input-coverage: the standalone kogen-build binary is built

  Scenario: No mode names every supported mode
    When cli-refusal-input-coverage: I run kogen-build without arguments
    Then cli-refusal-input-coverage: I receive a JSON usage refusal listing these supported modes
      | build                |
      | resume               |
      | prepare-continuation |

  Scenario: Malformed binding is refused before preparation
    When cli-refusal-input-coverage: I run prepare-continuation with project ".", ID "abcdef12", and binding file "inputs/binding.json" containing "{}"
    Then cli-refusal-input-coverage: I receive a native hold with continuation evidence for the requested ID
    When cli-refusal-input-coverage: I run prepare-continuation with project ".", ID "abcdef12", and binding file "inputs/binding.json" containing "{"
    Then cli-refusal-input-coverage: I receive a JSON usage refusal identifying the binding path without continuation evidence
