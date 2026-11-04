Feature: Tie a builder's blocker claim to a failure verification showed
  A builder-reported protected contradiction stops the Build only when one of its paths
  appears in a failure of the latest verification pass of the current tree. Each fixture
  is a real local Build with a scripted builder, run in its own go test process.

  Scenario: A first-round claim with no verification pass does not stop the Build
    Then k-build-blocker-claim-grounded: the Build fixture "first-round" meets these expectations
      """
      {"outcome":"no_progress","message":"blocker not backed by verification: no verification pass has run on the current tree yet, so no failure names features/steps/integration_steps.js; no protected edit is granted; make the failing items pass, or finish so verification runs on the current tree and report again with a protected path that a failure names","paths":["features/steps/integration_steps.js"],"passes":1,"requests":4}
      """

  Scenario: A claim naming a protected file no failure of the red pass mentions does not stop the Build
    Then k-build-blocker-claim-grounded: the Build fixture "unnamed" meets these expectations
      """
      {"outcome":"no_progress","message":"blocker not backed by verification: no failure of verification pass 1 names features/steps/integration_steps.js; no protected edit is granted; make the failing items pass, or report again with a protected path that a failure names","paths":["features/steps/integration_steps.js"],"passes":1,"requests":4}
      """

  Scenario: A claim backed by a failure in the protected step file stops the Build as before
    Then k-build-blocker-claim-grounded: the Build fixture "backed" meets these expectations
      """
      {"outcome":"build_blocked","paths":["features/steps/integration_steps.js"],"passes":1,"requests":2}
      """

  Scenario: A claim naming two protected files, one of them named by a failure, stops the Build
    Then k-build-blocker-claim-grounded: the Build fixture "two-paths" meets these expectations
      """
      {"outcome":"build_blocked","paths":[".kogen/intents/fixture/intent.md","features/steps/integration_steps.js"],"passes":1,"requests":2}
      """

  Scenario: Builds without a blocker report keep their requests and results
    Then k-build-blocker-claim-grounded: these golden files keep their bytes
      """
      64370533651781d66eb37ba93c669b70a303437f65ed34a8077f01d6754d26e5 build/testdata/named-feedback-compat.json
      ffb3f1bb161a428fdf15691e17fc836a78a3d51c51c05e028b18c49994bd1545 build/testdata/prompts_golden.json
      a7f43907aa439c4d6be11844f59959fe0cdaaa5ca3e1ff55a295f714b8c402ba build/testdata/defs_golden.json
      435b5e9bc9f430dfd037ba1258466590e7ac5b98cc41aabb0315d3e7ef9eceb9 build/testdata/context-external-golden.json
      0677504f24b93c95957b589d75a8f6492c70c6dca1cee2d749d597ea48fd91e6 build/testdata/state/fresh/checkpoint.json
      """
    And k-build-blocker-claim-grounded: the existing test "TestNamedFeedbackCompatibility" passes in its own go test run
    And k-build-blocker-claim-grounded: the existing test "TestPromptsAreByteEqualToTheElixirFunctions" passes in its own go test run
    And k-build-blocker-claim-grounded: the existing test "TestDefinitionsMatchTheElixirDefinitions" passes in its own go test run
    And k-build-blocker-claim-grounded: the existing test "TestContextExternalNativeBytes" passes in its own go test run
    And k-build-blocker-claim-grounded: the existing test "TestStateGoldenFreshCheckpoint" passes in its own go test run
