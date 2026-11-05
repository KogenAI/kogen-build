Feature: b-consumes-tools-require-executable: TestToolsRequire passes whether kogen-verification wants node to be a regular file or an executable

  kogen-verification's pending change broken-kogen-install-blamed-on-intent makes Tools.Require
  want node to be executable, not just a regular file, and words its refusal "node is not
  available: X ..." instead of "node is not available at X; reinstall Kogen". kogen-build's
  consumer test TestToolsRequire passes with the pinned kogen-verification and with one carrying
  that rule, still checks class environment, reason tool_missing, exit 3 and that the refusal
  names the tool and its path, and changes nothing else.

  Scenario: TestToolsRequire passes against both rules and still rejects a defective refusal
    Then b-consumes-tools-require-executable: the file "go.mod" has the SHA-256 "bfcfa998e3171f3fcb64eda9257d1f1950e38b356152df6ef3602f492b363934"
    And b-consumes-tools-require-executable: the file "go.sum" has the SHA-256 "44690a0e668ede21a20209c6a69c27de2b4771bff8e1e0dbdf0a47faccbe8ace"
    And b-consumes-tools-require-executable: the files under "consumes" other than these have the digest "25ea7d206fc22cad8eb4d0989c570ccb608259466d5268b4ea60847fcb5c444d":
      | file                                       |
      | consumes/kogen-verification/tools_test.go  |
      | consumes/kogen-verification/helper_test.go |
    When b-consumes-tools-require-executable: I run the Go test "TestToolsRequire" against the "pinned" kogen-verification
    Then b-consumes-tools-require-executable: that run passed and no test was skipped
    And b-consumes-tools-require-executable: the Go test "TestToolsRequire" fails against each of these kogen-verification copies:
      | copy        | defect                                         |
      | wrongclass  | the refusal's class is configuration           |
      | wrongreason | the refusal's reason is tool_unusable          |
      | wrongexit   | the refusal's exit code is 1                   |
      | pathless    | the refusal does not name the tool's path      |
      | toolless    | the refusal does not name the tool             |
      | quintfirst  | quint is judged before node                    |
    When b-consumes-tools-require-executable: I run the Go test "TestToolsRequire" against the "executable" kogen-verification
    Then b-consumes-tools-require-executable: that run passed and no test was skipped
