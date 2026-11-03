Feature: go test ./... -update regenerates the build package's goldens

  Scenario: A drifted golden fails without -update, -update rewrites it from the current output, and the next plain run passes
    Given a scratch copy of this checkout, whose goldens have these SHA-256 digests:
      | golden                                           | sha256                                                           |
      | build/testdata/context-external-golden.json      | 435b5e9bc9f430dfd037ba1258466590e7ac5b98cc41aabb0315d3e7ef9eceb9 |
      | build/testdata/context-golden.json               | 2f002dfa8ed655d0136c8c149ee47387542298d429155fd029593ed7e06110f7 |
      | build/testdata/k-revod-gate/parts-requests.json  | 1050f0ab89bea443963f3cacf9b7a1caa27f76c3f1d304b2605d13a8256c0b92 |
      | build/testdata/reset-golden.json                 | 8c63eaf99abca3370b451d46749f5e66105195295d2f8cf4bf141f8c57b00870 |
      | build/testdata/review/material-golden.json       | caf0623631f774d90cf91b2d5939e9c1a94c3c8b20b84dc7a766bd5753e2bc01 |
      | build/testdata/reviewretry/approve.wire.json     | 9c93283cd1f444e6d1c0a5be031b2057f3963e6dca45439179bd6a01732967ab |
      | build/testdata/reviewretry/changes.wire.json     | 9c93283cd1f444e6d1c0a5be031b2057f3963e6dca45439179bd6a01732967ab |
      | build/testdata/named-feedback-compat.json        | 64370533651781d66eb37ba93c669b70a303437f65ed34a8077f01d6754d26e5 |
    And the golden tests are:
      | TestContextExternalNativeBytes                 |
      | TestOpeningMessageEqualsTheElixirReference     |
      | TestNativeOpeningInputEqualsTheOpeningMessage  |
      | TestKRevodGatePartsUnchanged                   |
      | TestTheLocalResetEqualsTheElixirReference      |
      | TestTheMaterialEqualsTheElixirReference        |
      | TestKReviewRetryUnchangedValidRequests         |
      | TestNamedFeedbackCompatibility                 |
    And in the copy "DRIFT " is inserted at the start of the first string value of each golden's key:
      | golden                                           | key            |
      | build/testdata/context-external-golden.json      | builder.stable |
      | build/testdata/context-golden.json               | stable         |
      | build/testdata/k-revod-gate/parts-requests.json  | sha256         |
      | build/testdata/reset-golden.json                 | sha256         |
      | build/testdata/review/material-golden.json       | checks         |
      | build/testdata/reviewretry/approve.wire.json     | text           |
      | build/testdata/reviewretry/changes.wire.json     | text           |
      | build/testdata/named-feedback-compat.json        | feedback       |
    When "go test ./... -count=1 -v -run <golden-tests>" runs in the copy with KOGEN_REVOD_GATE_GOLDEN set to "write"
    Then the run fails, each golden test reports FAIL, and no golden in the copy changed
    When "go test ./... -update -count=1 -v -run <golden-tests>" runs in the copy
    Then the run exits 0, each golden test reports PASS or SKIP, and every golden in the copy has its recorded digest again
    When "go test ./... -count=1 -v -run <golden-tests>" runs in the copy
    Then the run exits 0 and each golden test reports PASS

  Scenario: The update switch is test-only and documented
    Then README.md documents "go test ./... -update"
    And the non-test code of the module imports no "flag" package and no non-test Go file contains any of:
      | "update"                |
      | KOGEN_REVOD_GATE_GOLDEN |
