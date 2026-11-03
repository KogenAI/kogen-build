Feature: Land and enforce maintained models
  Scenario: Land a model and enforce it on later Builds
    Then synthetic Builds maintain and verify models in "modeled" mode

  Scenario: Preserve model-free Builds
    Then synthetic Builds maintain and verify models in "empty" mode
