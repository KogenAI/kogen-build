Feature: TestContextExternalNativeBytes compares the protected files with .kogen/models.yaml present or absent equally
  kogen-coordinator 4826fb1 made project.Baseline list .kogen/models.yaml right after
  .kogen/project.yaml, so the protected files the Build names in its opening requests gained that
  entry. TestContextExternalNativeBytes asserts what kogen-build contributes to those requests, so
  it passes with the coordinator go.mod pins (ffca7072fe15), with 4826fb1 and with main (03d7341),
  and its frozen golden keeps its bytes, without the coordinator's lineup file.

  Scenario: With the committed go.mod pin the test runs all eight subtests and passes
    When k-build-context-bytes-unpinned: go test runs TestContextExternalNativeBytes in package build with the committed go.mod
    Then k-build-context-bytes-unpinned: TestContextExternalNativeBytes passed with exactly these subtests passing:
      | subtest                           |
      | native/native_without_context     |
      | native/native_with_context        |
      | native/external_without_context   |
      | native/external_with_context      |
      | external/native_without_context   |
      | external/external_without_context |
      | external/native_with_context      |
      | external/external_with_context    |
    And k-build-context-bytes-unpinned: go.mod requires github.com/KogenAI/kogen-coordinator at v0.0.0-20261103113131-ffca7072fe15 without replacing it

  Scenario Outline: Against kogen-coordinator <commit>, whose baseline lists .kogen/models.yaml, the test passes
    When k-build-context-bytes-unpinned: go test runs TestContextExternalNativeBytes in package build against kogen-coordinator <version>
    Then k-build-context-bytes-unpinned: TestContextExternalNativeBytes passed with exactly these subtests passing:
      | subtest                           |
      | native/native_without_context     |
      | native/native_with_context        |
      | native/external_without_context   |
      | native/external_with_context      |
      | external/native_without_context   |
      | external/external_without_context |
      | external/native_with_context      |
      | external/external_with_context    |

    Examples:
      | commit                    | version                            |
      | 4826fb1, the first bad    | v0.0.0-20261103201314-4826fb104954 |
      | 03d7341, the current main | v0.0.0-20261103214825-03d7341feab7 |

  Scenario: The frozen golden keeps its bytes and does not name the coordinator's lineup file
    Then k-build-context-bytes-unpinned: the file build/testdata/context-external-golden.json has SHA-256 9b753a918827ef61e9072fea1e8af6ec132aed43b64f3a5a921e104cfe698790
    And k-build-context-bytes-unpinned: the file build/testdata/context-external-golden.json does not contain ".kogen/models.yaml"

  Scenario: Rewriting the golden with -update against coordinator main records the same bytes
    When k-build-context-bytes-unpinned: go test -update runs TestContextExternalNativeBytes in package build of a scratch copy of the checkout against kogen-coordinator v0.0.0-20261103214825-03d7341feab7
    Then k-build-context-bytes-unpinned: TestContextExternalNativeBytes passed with exactly these subtests passing:
      | subtest                           |
      | native/native_without_context     |
      | native/native_with_context        |
      | native/external_without_context   |
      | native/external_with_context      |
      | external/native_without_context   |
      | external/external_without_context |
      | external/native_with_context      |
      | external/external_with_context    |
    And k-build-context-bytes-unpinned: in the scratch copy, the file build/testdata/context-external-golden.json has SHA-256 9b753a918827ef61e9072fea1e8af6ec132aed43b64f3a5a921e104cfe698790
