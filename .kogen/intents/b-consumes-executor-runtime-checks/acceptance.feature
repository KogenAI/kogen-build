Feature: b-consumes-executor-runtime-checks: TestContextExternalFirstPrompt passes whether or not kogen-executor checks the managed runtime's size, owner and mode

  kogen-executor's pending change kogen-runs-unverified-claude-code-runtime makes the executor
  check, before every start, that the managed Claude Code runtime is a regular, executable,
  non-empty file of the pinned size for the platform, owned by the current user and not writable
  by group or others; with a test origin (KOGEN_CLAUDE_CODE_URL) that supplies its own manifest
  the size is unknown and that check is skipped. kogen-build's TestContextExternalFirstPrompt
  passes with the pinned kogen-executor and with one carrying those checks, still proves what the
  first prompt contains with context on and off, and nothing else changes.

  Scenario: TestContextExternalFirstPrompt passes against both executors and still proves the first prompt
    Then b-consumes-executor-runtime-checks: the file "go.mod" has the SHA-256 "bfcfa998e3171f3fcb64eda9257d1f1950e38b356152df6ef3602f492b363934"
    And b-consumes-executor-runtime-checks: the file "go.sum" has the SHA-256 "44690a0e668ede21a20209c6a69c27de2b4771bff8e1e0dbdf0a47faccbe8ace"
    And b-consumes-executor-runtime-checks: the files under these folders other than "build/context_external_test.go" have the digest "fd22641bf72f1c34cd849d8075c40989ce413d4f69dd71008be6f4b4a73dc771":
      | folder   |
      | build    |
      | cmd      |
      | consumes |
      | contract |
    When b-consumes-executor-runtime-checks: I run the Go test "TestContextExternalFirstPrompt" against the "pinned" kogen-executor
    Then b-consumes-executor-runtime-checks: that run passed these subtests and no test was skipped:
      | subtest                                      |
      | TestContextExternalFirstPrompt/context_true  |
      | TestContextExternalFirstPrompt/context_false |
    And b-consumes-executor-runtime-checks: the Go test "TestContextExternalFirstPrompt" fails against each of these kogen-executor copies:
      | copy            | defect                                                                 |
      | strictseam      | the size is checked against the pin even with a test origin            |
      | doubledprompt   | the first prompt carries the stable authority and diagnostic twice    |
      | blankprompt     | the first prompt is the role's framing only, without the brief         |
      | extradiagnostic | the first prompt ends with the words Caller diagnostic data            |
    When b-consumes-executor-runtime-checks: I run the Go test "TestContextExternalFirstPrompt" against the "checked" kogen-executor
    Then b-consumes-executor-runtime-checks: that run passed these subtests and no test was skipped:
      | subtest                                      |
      | TestContextExternalFirstPrompt/context_true  |
      | TestContextExternalFirstPrompt/context_false |
    When b-consumes-executor-runtime-checks: I run the Go test "TestContextExternalFirstPrompt" against the "checked" kogen-executor with umask "002"
    Then b-consumes-executor-runtime-checks: that run passed these subtests and no test was skipped:
      | subtest                                      |
      | TestContextExternalFirstPrompt/context_true  |
      | TestContextExternalFirstPrompt/context_false |
