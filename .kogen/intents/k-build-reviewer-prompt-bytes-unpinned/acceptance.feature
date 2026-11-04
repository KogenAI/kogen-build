Feature: TestContextExternalNativeBytes does not pin the reviewer's prompt wording
  kogen-review e5005fa reworded the reviewer instructions, added the Requirements rule and gave
  submit_review a requirements parameter. TestContextExternalNativeBytes failed because its golden
  froze those bytes. The reviewer's wording is kogen-review's: the test now compares the reviewer
  parts with what kogen-review exports at the version it is built with (review.ReviewerInstructions(),
  review.ReviewerToolDefinitions(review.SubmitReviewTool), review.LimitRaises), plus what kogen-build
  contributes. So it passes with the pinned kogen-review (cfc2fb29917c), with e5005fa, with main
  (81d3bae) and with a kogen-review whose reviewer wording alone differs, and its golden holds no
  reviewer wording. It still fails when the reviewer is given different material.

  Scenario: With the committed go.mod pin the test runs all eight subtests and passes
    When k-build-reviewer-prompt-bytes-unpinned: go test runs TestContextExternalNativeBytes in package build with the committed go.mod
    Then k-build-reviewer-prompt-bytes-unpinned: go test reported TestContextExternalNativeBytes as pass with these subtest results:
      | subtest                           | result |
      | native/native_without_context     | pass   |
      | native/native_with_context        | pass   |
      | native/external_without_context   | pass   |
      | native/external_with_context      | pass   |
      | external/native_without_context   | pass   |
      | external/external_without_context | pass   |
      | external/native_with_context      | pass   |
      | external/external_with_context    | pass   |
    And k-build-reviewer-prompt-bytes-unpinned: go.mod requires github.com/KogenAI/kogen-review at v0.0.0-20261103124347-cfc2fb29917c without replacing it

  Scenario Outline: Against kogen-review <commit> the test passes
    When k-build-reviewer-prompt-bytes-unpinned: go test runs TestContextExternalNativeBytes in package build against kogen-review <version>
    Then k-build-reviewer-prompt-bytes-unpinned: go test reported TestContextExternalNativeBytes as pass with these subtest results:
      | subtest                           | result |
      | native/native_without_context     | pass   |
      | native/native_with_context        | pass   |
      | native/external_without_context   | pass   |
      | native/external_with_context      | pass   |
      | external/native_without_context   | pass   |
      | external/external_without_context | pass   |
      | external/native_with_context      | pass   |
      | external/external_with_context    | pass   |

    Examples:
      | commit               | version                            |
      | e5005fa, first bad   | v0.0.0-20261103161351-e5005fa1daf8 |
      | 81d3bae, main        | v0.0.0-20261103212337-81d3baef1d48 |

  Scenario: Against a kogen-review whose reviewer wording alone differs the test passes
    When k-build-reviewer-prompt-bytes-unpinned: go test runs TestContextExternalNativeBytes in package build against kogen-review v0.0.0-20261103124347-cfc2fb29917c with these edits:
      | file                       | from                                                                        | to                                                                          | count |
      | review/prompts/reviewer.md | You review code that another model wrote                                    | You review the code another model wrote                                     | 1     |
      | review/prompts/reviewer.md | run your focused probes (stated limits, empty or zero values, error paths) | run every focused probe (stated limits, empty or zero values, error paths) | 1     |
      | review/policy.go           | is not flagged by this rule.                                                | is not a finding under this rule.                                           | 1     |
      | review/policy.go           | Submit the review: verdict approve or changes, with findings.               | Submit the review: the verdict and its findings.                            | 1     |
      | review/parts.go            | # Review task                                                               | # The review task                                                           | 2     |
      | review/parts.go            | Finish with the structured verdict:                                         | End with the structured verdict:                                            | 1     |
    Then k-build-reviewer-prompt-bytes-unpinned: go test reported TestContextExternalNativeBytes as pass with these subtest results:
      | subtest                           | result |
      | native/native_without_context     | pass   |
      | native/native_with_context        | pass   |
      | native/external_without_context   | pass   |
      | native/external_with_context      | pass   |
      | external/native_without_context   | pass   |
      | external/external_without_context | pass   |
      | external/native_with_context      | pass   |
      | external/external_with_context    | pass   |

  Scenario: An external reviewer that is not given the caller diagnostic still fails the test
    When k-build-reviewer-prompt-bytes-unpinned: go test runs TestContextExternalNativeBytes in package build against kogen-review v0.0.0-20261103124347-cfc2fb29917c with these edits:
      | file               | from                          | to                                | count |
      | review/external.go | range in.Evidence.Diagnostic { | range in.Evidence.Diagnostic[:0] { | 1     |
    Then k-build-reviewer-prompt-bytes-unpinned: go test reported TestContextExternalNativeBytes as fail with these subtest results:
      | subtest                           | result |
      | native/native_without_context     | pass   |
      | native/native_with_context        | pass   |
      | native/external_without_context   | pass   |
      | native/external_with_context      | fail   |
      | external/native_without_context   | pass   |
      | external/external_without_context | pass   |
      | external/native_with_context      | pass   |
      | external/external_with_context    | pass   |

  Scenario: A native reviewer that is not given the caller diagnostic still fails the test
    When k-build-reviewer-prompt-bytes-unpinned: go test runs TestContextExternalNativeBytes in package build against kogen-review v0.0.0-20261103124347-cfc2fb29917c with these edits:
      | file                  | from                                        | to                                   | count |
      | review/independent.go | ReviewerInstructions(), tools, diagnostic) | ReviewerInstructions(), tools, nil) | 1     |
    Then k-build-reviewer-prompt-bytes-unpinned: go test reported TestContextExternalNativeBytes as fail with these subtest results:
      | subtest                           | result |
      | native/native_without_context     | pass   |
      | native/native_with_context        | fail   |
      | native/external_without_context   | pass   |
      | native/external_with_context      | pass   |
      | external/native_without_context   | pass   |
      | external/external_without_context | pass   |
      | external/native_with_context      | fail   |
      | external/external_with_context    | pass   |

  Scenario: The golden holds kogen-build's parts and no reviewer wording
    Then k-build-reviewer-prompt-bytes-unpinned: the golden build/testdata/context-external-golden.json holds exactly these parts, fingerprinted by the SHA-256 of their text or, for a reviewer part, of its JSON with sorted keys:
      | entry                             | part            | sha256                                                           |
      | external/external without context | builder.dynamic | a19da629adb2062c7086e21e30e53dc61a34543baeb165599aaafebed9760a2b |
      | external/external without context | builder.stable  | 9b7646d93a41bc3e74ba32e1e69f2f5f71a3491192c6de185477bb0da7ea718b |
      | external/native without context   | builder.dynamic | a19da629adb2062c7086e21e30e53dc61a34543baeb165599aaafebed9760a2b |
      | external/native without context   | builder.stable  | 9b7646d93a41bc3e74ba32e1e69f2f5f71a3491192c6de185477bb0da7ea718b |
      | external/native without context   | reviewer        | 4c384b391441b9e29e106298d9184484dc94e9ee1be167f81b368ab84638d686 |
      | native/external with context      | builder         | ab61779f5f078b833420282712ec96264c7e840e744512d89c1b3add4f60d615 |
      | native/external without context   | builder         | 4f5a54d8659fa59b4099e356389ee0afc28ada34bd327d380176b02545a17b02 |
      | native/native with context        | builder         | ab61779f5f078b833420282712ec96264c7e840e744512d89c1b3add4f60d615 |
      | native/native with context        | reviewer        | 6927575fff3d00419edf95d7e5b04dfe44220d2f56293f7cbc13541f27407e96 |
      | native/native without context     | builder         | 4f5a54d8659fa59b4099e356389ee0afc28ada34bd327d380176b02545a17b02 |
      | native/native without context     | reviewer        | 4c384b391441b9e29e106298d9184484dc94e9ee1be167f81b368ab84638d686 |
    And k-build-reviewer-prompt-bytes-unpinned: the file build/testdata/context-external-golden.json contains none of these texts:
      | text                                     |
      | You review code that another model wrote |
      | run your focused probes                  |
      | Limit raises:                            |
      | Requirements: before any verdict         |
      | Submit the review:                       |
      | # Review task                            |
      | Finish with the structured verdict       |

  Scenario Outline: -update against kogen-review <commit> records the golden with the same bytes
    When k-build-reviewer-prompt-bytes-unpinned: go test -update runs TestContextExternalNativeBytes in package build of a scratch copy of the checkout against kogen-review <version>
    Then k-build-reviewer-prompt-bytes-unpinned: go test reported TestContextExternalNativeBytes as pass with these subtest results:
      | subtest                           | result |
      | native/native_without_context     | pass   |
      | native/native_with_context        | pass   |
      | native/external_without_context   | pass   |
      | native/external_with_context      | pass   |
      | external/native_without_context   | pass   |
      | external/external_without_context | pass   |
      | external/native_with_context      | pass   |
      | external/external_with_context    | pass   |
    And k-build-reviewer-prompt-bytes-unpinned: in the scratch copy, the file build/testdata/context-external-golden.json has the same bytes as in the checkout

    Examples:
      | commit                  | version                            |
      | cfc2fb29917c, the pin   | v0.0.0-20261103124347-cfc2fb29917c |
      | 81d3bae, main           | v0.0.0-20261103212337-81d3baef1d48 |
