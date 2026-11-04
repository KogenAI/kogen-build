Feature: A red pass after a partial finish reminds the builder that the whole Intent must work through its real entry points
  Builder-bound red feedback of a whole-Intent verification pass ends with one fixed reminder line,
  after a blank line, and the builder's standing instructions (build/prompts/builder.md) state the
  same once, as one bullet directly before the finish bullet. Review feedback, verification and
  green decisions are unchanged. The scripted Builds run the real kogen-build program on a
  temporary fixture project whose Intent adds --loud behind its existing entry point greet.js.

  Background:
    Given k-build-finish-whole-intent-reminder: the finish bullet is
      """
      - Finish means the whole approved Intent is implemented: every behavior it requires works through the real entry points that a user or an acceptance scenario reaches. A new helper or internal function with its own passing test, an entry point that still behaves the old way, or a part of the Intent not yet started is not complete, and a finish with such work is verified red again.
      """
    And k-build-finish-whole-intent-reminder: the reminder line is
      """
      Reminder: finish means the whole approved Intent is implemented and every behavior it requires works through the real entry points that a user or an acceptance scenario reaches; helper-only or partially wired work will be red again.
      """

  Scenario: A builder that adds only an unused helper with its passing unit test and finishes gets a red pass with the reminder, then lands once it wires the entry point
    Then k-build-finish-whole-intent-reminder: the scripted Build "partial-then-wired" meets these expectations
      """
      {"requests":["builder","builder","reviewer"],"commitsAtRequest":[1,1,1],"checks":[["unit-tests",0]],"builderFeedback":"Verification pass 1 is not green. Fix the implementation (the approved acceptance files cannot change), then call finish again.\nacceptance (NODE_PATH=\"$KOGEN_CUCUMBER\" \"$KOGEN_NODE\" \"$KOGEN_CUCUMBER/@cucumber/cucumber/bin/cucumber.js\" --strict --require 'features/steps/*.js' --format \"message:$KOGEN_ACCEPTANCE_REPORT\" \"$KOGEN_ACCEPTANCE_FEATURES\"): failed, exit status 1\n  failed: Greeting loudly shouts (line 5)\n  acceptance.feature line 6: Then greet.js run with \"--loud\" prints \"HELLO\"\n  greet.js printed \"hello\\n\", want \"HELLO\\n\"\nnot run because an earlier item failed: unit-tests"}
      """

  Scenario: A red pass at the review stage sends the reviewer's repair its ordinary feedback without the reminder
    Then k-build-finish-whole-intent-reminder: the scripted Build "review-repair" meets these expectations
      """
      {"requests":["builder","reviewer","reviewer"],"commitsAtRequest":[1,1,1],"checks":[["unit-tests",0]],"reviewerFeedback":"Final verification of your repairs failed. Fix these failures and regressions from your edits; do not restart a whole-change critique. If observed evidence demonstrates a protected fixture defect, use submit_review changes with a blocking finding at that protected path instead of a production workaround, including a test-facing hidden comment.\nVerification pass 2 is not green. Fix the implementation (the approved acceptance files cannot change), then call submit_review again.\nacceptance (NODE_PATH=\"$KOGEN_CUCUMBER\" \"$KOGEN_NODE\" \"$KOGEN_CUCUMBER/@cucumber/cucumber/bin/cucumber.js\" --strict --require 'features/steps/*.js' --format \"message:$KOGEN_ACCEPTANCE_REPORT\" \"$KOGEN_ACCEPTANCE_FEATURES\"): failed, exit status 1\n  failed: Greeting loudly shouts (line 5)\n  acceptance.feature line 6: Then greet.js run with \"--loud\" prints \"HELLO\"\n  greet.js printed \"hello\\n\", want \"HELLO\\n\"\nnot run because an earlier item failed: unit-tests"}
      """

  Scenario: The builder's standing instructions state the finish statement exactly once, and no other instruction text changes
    Then k-build-finish-whole-intent-reminder: the existing test "TestPromptsAreByteEqualToTheElixirFunctions" passes in its own go test run
    And k-build-finish-whole-intent-reminder: build/prompts/builder.md is the instructions of sha256 "915b0bc6b4bd3624010f1878143b6215f34fb39c0333e99785b187d8f97511d7" with the finish bullet inserted before the bullet starting "- When the implementation is complete, call finish."
    And k-build-finish-whole-intent-reminder: build/prompts/builder.md has 4160 bytes and sha256 "2729f3c4e8bd5754252fd4d6a292178f844e2a037a09bd332f31e56ed557e4a2"
    And k-build-finish-whole-intent-reminder: build/testdata/prompts_golden.json records the bytes and sha256 of build/prompts/builder.md

  Scenario: Ordinary feedback, requests and recorded state differ from today's only by the reminder line and the finish bullet
    Then k-build-finish-whole-intent-reminder: the existing test "TestNamedFeedbackCompatibility" passes in its own go test run
    And k-build-finish-whole-intent-reminder: the existing test "TestContextExternalNativeBytes" passes in its own go test run
    And k-build-finish-whole-intent-reminder: the existing test "TestDefinitionsMatchTheElixirDefinitions" passes in its own go test run
    And k-build-finish-whole-intent-reminder: the existing test "TestStateGoldenFreshCheckpoint" passes in its own go test run
    And k-build-finish-whole-intent-reminder: the existing test "TestKReviewRetryUnchangedValidRequests" passes in its own go test run
    And k-build-finish-whole-intent-reminder: the existing test "TestOpeningMessageEqualsTheElixirReference" passes in its own go test run
    And k-build-finish-whole-intent-reminder: these golden files keep their bytes
      """
      a7f43907aa439c4d6be11844f59959fe0cdaaa5ca3e1ff55a295f714b8c402ba build/testdata/defs_golden.json
      0677504f24b93c95957b589d75a8f6492c70c6dca1cee2d749d597ea48fd91e6 build/testdata/state/fresh/checkpoint.json
      9c93283cd1f444e6d1c0a5be031b2057f3963e6dca45439179bd6a01732967ab build/testdata/reviewretry/approve.wire.json
      9c93283cd1f444e6d1c0a5be031b2057f3963e6dca45439179bd6a01732967ab build/testdata/reviewretry/changes.wire.json
      2f002dfa8ed655d0136c8c149ee47387542298d429155fd029593ed7e06110f7 build/testdata/context-golden.json
      8c63eaf99abca3370b451d46749f5e66105195295d2f8cf4bf141f8c57b00870 build/testdata/reset-golden.json
      """
    And k-build-finish-whole-intent-reminder: with the finish bullet and the reminder line taken out, these golden files keep their bytes
      """
      97712b5c31f227402e39cd295fea7be77cdda1059dd8dd7b037c0815f24a6497 build/testdata/named-feedback-compat.json
      6863f74fe124642d48245fedf3660df6559d6a2750fd6524eb2a816941227441 build/testdata/context-external-golden.json
      """
