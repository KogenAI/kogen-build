Feature: kogen-build passes with the K-MODELS-FILE coordinator and keeps its pin
  K-MODELS-FILE (kogen-coordinator candidate 2a4c69a6) moves the model lineup from the models:
  key of .kogen/project.yaml to .kogen/models.yaml and refuses a project.yaml that still has
  models:. kogen-build's tests pass both with the coordinator its committed go.mod pins
  (c4a49a4) and against that candidate, which the harness makes from the pinned coordinator and
  the candidate's project changes and uses through a temporary go.mod, never the committed one.
  The persisted-state goldens keep their bytes.

  Scenario: With the committed pin, the named tests and the state goldens pass and go.mod is unchanged
    When these tests of package build run with the committed go.mod:
      | test                                                    |
      | TestExternalCheckpointRefusesResume                     |
      | TestExternalRoundLimitDefaultsToTheExecutorsOwn         |
      | TestExternalRoundLimitKeepsTheCandidate                 |
      | TestExternalHeadMoveStops                               |
      | TestExternalNeedsNoProviderLoginForTheBuildRole         |
      | TestExternalProtectedMutationStopsWithBaselineChanged   |
      | TestExternalTaskAndRunRecords                           |
      | TestExternalFinishClaimOnARedTreeGetsFeedbackNotACommit |
      | TestNativeConformance                                   |
      | TestNativeConformance/finish_after_write                |
      | TestNativeConformance/interrupt_kills_children          |
      | TestStateGoldenFreshCheckpoint                          |
      | TestStateGoldenAttemptsCheckpoint                       |
      | TestStateGoldenReviewApproveCheckpoint                  |
      | TestStateGoldenReviewChangesCheckpoint                  |
      | TestStateGoldenResumedCheckpoint                        |
      | TestStateGoldenReviewChangesRecord                      |
      | TestStateGoldenReviewApproveRecord                      |
    Then each of those tests passed
    And go.mod still requires github.com/KogenAI/kogen-coordinator at v0.0.0-20261102193107-c4a49a4fe84f without replacing it

  Scenario: Against kogen-coordinator 2a4c69a6, the tests the landing check named pass
    When these tests of package build run against kogen-coordinator 2a4c69a6:
      | test                                                    |
      | TestExternalCheckpointRefusesResume                     |
      | TestExternalRoundLimitDefaultsToTheExecutorsOwn         |
      | TestExternalRoundLimitKeepsTheCandidate                 |
      | TestExternalHeadMoveStops                               |
      | TestExternalNeedsNoProviderLoginForTheBuildRole         |
      | TestExternalProtectedMutationStopsWithBaselineChanged   |
      | TestExternalTaskAndRunRecords                           |
      | TestExternalFinishClaimOnARedTreeGetsFeedbackNotACommit |
      | TestNativeConformance                                   |
      | TestNativeConformance/finish_after_write                |
      | TestNativeConformance/interrupt_kills_children          |
    Then each of those tests passed

  Scenario: Against kogen-coordinator 2a4c69a6, the persisted-state golden tests pass
    When these tests of package build run against kogen-coordinator 2a4c69a6:
      | test                                   |
      | TestStateGoldenFreshCheckpoint         |
      | TestStateGoldenAttemptsCheckpoint      |
      | TestStateGoldenReviewApproveCheckpoint |
      | TestStateGoldenReviewChangesCheckpoint |
      | TestStateGoldenResumedCheckpoint       |
      | TestStateGoldenReviewChangesRecord     |
      | TestStateGoldenReviewApproveRecord     |
    Then each of those tests passed

  Scenario: Against kogen-coordinator 2a4c69a6, lineups parsed from text and Elixir resumes pass
    When these tests of package build run against kogen-coordinator 2a4c69a6:
      | test                                                               |
      | TestClaudeLineupNativeCompatibility                                |
      | TestNativeSelectionIgnoresDepsExternal                             |
      | TestALegacyBuilderCheckpointResumesTheSameConversationAndLandsOnce |
      | TestLegacyResumeRefusals                                           |
      | TestMain76BuildsResumeAndLand                                      |
    Then each of those tests passed

  Scenario: Against kogen-coordinator 2a4c69a6, the recorded-request comparisons pass
    When these tests of package build run against kogen-coordinator 2a4c69a6:
      | test                                   |
      | TestKRevodGatePartsUnchanged           |
      | TestKReviewRetryUnchangedValidRequests |
      | TestNamedFeedbackCompatibility         |
      | TestContextExternalFirstPrompt         |
    Then each of those tests passed

  Scenario: Against kogen-coordinator 2a4c69a6, tests that set a review, build or loop role pass
    When these tests of package build run against kogen-coordinator 2a4c69a6:
      | test                                                            |
      | TestExternalReviewResumeStartsOneFreshReviewOfTheBase           |
      | TestReviewInPartsLands                                          |
      | TestKReviewRetrySafetyBoundaries                                |
      | TestAnAnthropicBuildRequestRecordsCacheCreationAsCacheWrite     |
      | TestBuildAnthropicInvalidProviderFailureReachesResultAndOutcome |
      | TestLoopKnobsReview                                             |
      | TestUsageLimitResumeSafety                                      |
      | TestExternalNoProgressStopsOnThirdIdenticalRed                  |
    Then each of those tests passed

  Scenario: The persisted-state goldens keep their bytes and the moved layout's review records are recorded beside them
    Then these files have exactly these SHA-256 digests:
      | path                                                     | sha256                                                           |
      | build/testdata/state/fresh/checkpoint.json               | 0677504f24b93c95957b589d75a8f6492c70c6dca1cee2d749d597ea48fd91e6 |
      | build/testdata/state/attempts/checkpoint.json            | b31ae59edd08e94f569eb38ee6575c0844335987abf6b3252c5db1cdb7079a61 |
      | build/testdata/state/resumed/checkpoint.json             | 1933825e102ed1fe6818ed67b98b41af9073f382d634f8b779e85cf6e6796f54 |
      | build/testdata/state/review-approve/checkpoint.json      | 9a11c904cf8912b8018fe4f320e3cd532a7e996c769cdb1dd1a9f4295ee6e019 |
      | build/testdata/state/review-approve/review-1.json        | 5c3393411e6c7e65132fb823726dcc8f4520be9d7cb24843299aae3197f72bd3 |
      | build/testdata/state/review-changes/checkpoint.json      | 628e6f029da299ad824434c73b3b5406a182f7893c585a03d201e14c63314008 |
      | build/testdata/state/review-changes/review-1.json        | 13152a3f5c09f96980983b517aed6cfd2f7038544f0f455ce3637cdf1f609547 |
      | build/testdata/context-external-golden.json              | 435b5e9bc9f430dfd037ba1258466590e7ac5b98cc41aabb0315d3e7ef9eceb9 |
      | build/testdata/named-feedback-compat.json                | 64370533651781d66eb37ba93c669b70a303437f65ed34a8077f01d6754d26e5 |
      | build/testdata/k-revod-gate/parts-requests.json          | 1050f0ab89bea443963f3cacf9b7a1caa27f76c3f1d304b2605d13a8256c0b92 |
      | build/testdata/reviewretry/approve.wire.json             | 9c93283cd1f444e6d1c0a5be031b2057f3963e6dca45439179bd6a01732967ab |
      | build/testdata/reviewretry/changes.wire.json             | 9c93283cd1f444e6d1c0a5be031b2057f3963e6dca45439179bd6a01732967ab |
      | build/testdata/state/review-approve/review-1-moved.json  | dfa2037eb6a1b6117061757626edbae0202fca4bd38632b794d311e10e4fe9b3 |
      | build/testdata/state/review-changes/review-1-moved.json  | f833308eef3041d076f11983e4a3a25fab77e835d2c66b64d46e678fc441ea04 |
