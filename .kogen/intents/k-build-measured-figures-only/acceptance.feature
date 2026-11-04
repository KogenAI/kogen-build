Feature: Measured figures the builder writes come from a command it ran
  The builder's standing instructions (build/prompts/builder.md) gain one bullet: a figure
  written into a project file as an observed result is copied from the output of a command run
  in this Build on the final candidate and names that command; an unmeasurable figure is
  written as a target or expectation and the finish summary says it was not measured. Each
  scripted Build runs the real kogen-build program on a temporary fixture project.

  Background:
    Given k-build-measured-figures-only: the rule bullet is
      """
      - A figure you write into a project file as an observed result (a timing, size, count or percentage, in documentation, a report or a code comment) must be copied from the output of a command you ran in this Build on the final candidate, and the text must name that command. When you cannot measure it (no tool or no environment), write the figure as a target or an expectation, not as a measurement, and say in your finish summary that it was not measured. Figures restated from the Intent or from existing files are not affected.
      """

  Scenario: The standing instructions hold the rule exactly once, as one bullet, and no other instruction text changes
    Then k-build-measured-figures-only: build/prompts/builder.md is the instructions of sha256 "00e6225cfcc6520ca3eb18750a632272bfaa543c4f14b6b42d875c3bec24a6fc" with the rule bullet inserted before the bullet starting "- When the implementation is complete, call finish."
    And k-build-measured-figures-only: build/prompts/builder.md has 3777 bytes and sha256 "915b0bc6b4bd3624010f1878143b6215f34fb39c0333e99785b187d8f97511d7"
    And k-build-measured-figures-only: build/testdata/prompts_golden.json records the bytes and sha256 of build/prompts/builder.md
    And k-build-measured-figures-only: the existing test "TestPromptsAreByteEqualToTheElixirFunctions" passes in its own go test run

  Scenario: A scripted builder with a runnable measuring command writes the command's output and names the command
    Then k-build-measured-figures-only: the scripted Build "measured" meets these expectations
      """
      {"command":"wc -c < app.txt | tr -d ' '","exit":0,"figure":"10","note":"Measured effect: app.txt is 10 bytes, measured with `wc -c < app.txt | tr -d ' '` on the final candidate.\n","summary":"Implemented app.txt; its size was measured with `wc -c < app.txt | tr -d ' '`.","builderRequests":3,"reviewRequests":1}
      """

  Scenario: A scripted builder whose measuring command cannot run writes a target and says the effect was not measured
    Then k-build-measured-figures-only: the scripted Build "unavailable" meets these expectations
      """
      {"command":"kmf-measure-size app.txt","exit":127,"figure":"","note":"Expected effect (a target, not measured): app.txt should be about 10 bytes; `kmf-measure-size app.txt` could not run in this environment (exit status 127).\n","summary":"Implemented app.txt. The effect was not measured: `kmf-measure-size app.txt` could not run here.","builderRequests":3,"reviewRequests":1}
      """

  Scenario: A Build without a quantitative claim differs only by the instruction text
    Then k-build-measured-figures-only: with the rule bullet taken out, these golden files that record the instructions keep their bytes
      """
      64370533651781d66eb37ba93c669b70a303437f65ed34a8077f01d6754d26e5 build/testdata/named-feedback-compat.json
      435b5e9bc9f430dfd037ba1258466590e7ac5b98cc41aabb0315d3e7ef9eceb9 build/testdata/context-external-golden.json
      """
    And k-build-measured-figures-only: these golden files keep their bytes
      """
      a7f43907aa439c4d6be11844f59959fe0cdaaa5ca3e1ff55a295f714b8c402ba build/testdata/defs_golden.json
      0677504f24b93c95957b589d75a8f6492c70c6dca1cee2d749d597ea48fd91e6 build/testdata/state/fresh/checkpoint.json
      """
    And k-build-measured-figures-only: the existing test "TestNamedFeedbackCompatibility" passes in its own go test run
    And k-build-measured-figures-only: the existing test "TestContextExternalNativeBytes" passes in its own go test run
    And k-build-measured-figures-only: the existing test "TestDefinitionsMatchTheElixirDefinitions" passes in its own go test run
    And k-build-measured-figures-only: the existing test "TestStateGoldenFreshCheckpoint" passes in its own go test run
