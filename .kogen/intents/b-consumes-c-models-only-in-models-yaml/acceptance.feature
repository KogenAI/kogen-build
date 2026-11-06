Feature: b-consumes-c-models-only-in-models-yaml: kogen-build follows kogen-coordinator's models-only-in-models-yaml contract

  kogen-coordinator's c-models-only-in-models-yaml (commit d74daae36f56, "Read model roles only
  from the models files") takes every model role only from .kogen/models.yaml and
  .kogen/local/models.yaml, and refuses a models: key in .kogen/project.yaml with
  environment/project_config_invalid. kogen-build pins that version in go.mod, its tests and
  fixtures describe only that contract, and every test passes against it.

  Scenario: go.mod and go.sum pin the producer's new main and nothing else changes in them
    Then b-consumes-c-models-only-in-models-yaml: the file "go.mod" has the SHA-256 "a63db072fee111d33b02af63d608e63b66ce6d07da0f676945d881bf49079864"
    And b-consumes-c-models-only-in-models-yaml: the file "go.sum" has the SHA-256 "83363473bb53b70cb3f557bec513d9e8621bebd822dde80b71ab3f7b5ceb43bf"

  Scenario: The coordinator consumer suite reads the roles from .kogen/models.yaml
    When b-consumes-c-models-only-in-models-yaml: I run these Go tests of "./consumes/kogen-coordinator":
      | test                                       |
      | TestBaselineAndHash                        |
      | TestLoadConfigReadsTheProjectConfiguration |
      | TestLoadConfigDefaults                     |
      | TestWritePackageFacts                      |
    Then b-consumes-c-models-only-in-models-yaml: every test of that run passed and none was skipped, including:
      | test                                       |
      | TestBaselineAndHash                        |
      | TestLoadConfigReadsTheProjectConfiguration |
      | TestLoadConfigDefaults                     |
      | TestWritePackageFacts                      |
    And b-consumes-c-models-only-in-models-yaml: that run used kogen-coordinator "v0.0.0-20261106110407-d74daae36f56" without a replacement

  Scenario: Builds whose roles a test sets take them from .kogen/models.yaml
    When b-consumes-c-models-only-in-models-yaml: I run these Go tests of "./build":
      | test                                                                    |
      | TestAnAnthropicBuildRequestRecordsCacheCreationAsCacheWrite             |
      | TestAnInvalidExternalBlockerClaimWithoutEditsGetsTheNoteThenTheNotice   |
      | TestBuildAnthropicInvalidProviderFailureReachesResultAndOutcome         |
      | TestClaudeLineupNativeCompatibility                                     |
      | TestClaudeLineupReviewExactTree                                         |
      | TestClaudeLineupReviewMaterialDelivery                                  |
      | TestClaudeLineupReviewRefusesResume                                     |
      | TestClaudeLineupReviewRoundLimit                                        |
      | TestContextExternalFirstPrompt                                          |
      | TestContextExternalNativeBytes                                          |
      | TestContextExternalReviewMatrix                                         |
      | TestExternalBlockerFromADeclaringExecutorGoesThroughTheNativePath       |
      | TestExternalBuilderReviewStageResumes                                   |
      | TestExternalCheckpointRefusesResume                                     |
      | TestExternalDescriptorMustFitTheContract                                |
      | TestExternalFailedRoundStillDetectsCustodyChanges                       |
      | TestExternalFinishClaimOnARedTreeGetsFeedbackNotACommit                 |
      | TestExternalHeadMoveStops                                               |
      | TestExternalInterruptKeepsTheInterruptExitCode                          |
      | TestExternalInterruptReportedAsAnExecutorFailureStillExitsWithTheSignal |
      | TestExternalNeedsNoProviderLoginForTheBuildRole                         |
      | TestExternalNoCallEndingGetsFeedbackLikeAFinishClaim                    |
      | TestExternalNoProgressResetsWhenTheTreeChanges                          |
      | TestExternalNoProgressStopsOnThirdIdenticalRed                          |
      | TestExternalProtectedMutationStopsWithBaselineChanged                   |
      | TestExternalReviewCheckpointsRecordTheGreenBase                         |
      | TestExternalReviewResumeRefusals                                        |
      | TestExternalReviewResumeStartsOneFreshReviewOfTheBase                   |
      | TestExternalRoundLimitDefaultsToTheExecutorsOwn                         |
      | TestExternalRoundLimitKeepsTheCandidate                                 |
      | TestExternalRoundsWithoutEditsGetANoticeThenStopAsNoProgress            |
      | TestExternalStartFailureStopsTheBuild                                   |
      | TestExternalTaskAndRunRecords                                           |
      | TestExternalUnsupportedEndingsAreProtocolFailures                       |
      | TestExternalWithoutAProvidedExecutorIsUnavailable                       |
      | TestKReviewRetry2ForcedReset                                            |
      | TestKReviewRetryReminderChoice                                          |
      | TestKReviewRetrySafetyBoundaries                                        |
      | TestLoopKnobsNativeBuilder                                              |
      | TestLoopKnobsReview                                                     |
      | TestNativeSelectionIgnoresDepsExternal                                  |
      | TestReviewInPartsBlockStopsLaterParts                                   |
      | TestReviewInPartsCustody                                                |
      | TestReviewInPartsLands                                                  |
      | TestReviewInPartsMandatoryMaterialStopsBeforeSending                    |
      | TestReviewInPartsOmitsAChangedFilesFullText                             |
      | TestReviewInPartsRepairIsVerifiedBeforeTheNextPart                      |
      | TestReviewInPartsVerdictlessPartRestartsFromPartOne                     |
      | TestUnmovedExternalFeedback                                             |
      | TestUsageLimitResumeSafety                                              |
      | TestWholeIntentRedFeedbackEndsWithTheFinishReminder                     |
    Then b-consumes-c-models-only-in-models-yaml: every test of that run passed and none was skipped, including:
      | test                                                                    |
      | TestAnAnthropicBuildRequestRecordsCacheCreationAsCacheWrite             |
      | TestAnInvalidExternalBlockerClaimWithoutEditsGetsTheNoteThenTheNotice   |
      | TestBuildAnthropicInvalidProviderFailureReachesResultAndOutcome         |
      | TestClaudeLineupNativeCompatibility                                     |
      | TestClaudeLineupReviewExactTree                                         |
      | TestClaudeLineupReviewMaterialDelivery                                  |
      | TestClaudeLineupReviewRefusesResume                                     |
      | TestClaudeLineupReviewRoundLimit                                        |
      | TestContextExternalFirstPrompt                                          |
      | TestContextExternalNativeBytes                                          |
      | TestContextExternalReviewMatrix                                         |
      | TestExternalBlockerFromADeclaringExecutorGoesThroughTheNativePath       |
      | TestExternalBuilderReviewStageResumes                                   |
      | TestExternalCheckpointRefusesResume                                     |
      | TestExternalDescriptorMustFitTheContract                                |
      | TestExternalFailedRoundStillDetectsCustodyChanges                       |
      | TestExternalFinishClaimOnARedTreeGetsFeedbackNotACommit                 |
      | TestExternalHeadMoveStops                                               |
      | TestExternalInterruptKeepsTheInterruptExitCode                          |
      | TestExternalInterruptReportedAsAnExecutorFailureStillExitsWithTheSignal |
      | TestExternalNeedsNoProviderLoginForTheBuildRole                         |
      | TestExternalNoCallEndingGetsFeedbackLikeAFinishClaim                    |
      | TestExternalNoProgressResetsWhenTheTreeChanges                          |
      | TestExternalNoProgressStopsOnThirdIdenticalRed                          |
      | TestExternalProtectedMutationStopsWithBaselineChanged                   |
      | TestExternalReviewCheckpointsRecordTheGreenBase                         |
      | TestExternalReviewResumeRefusals                                        |
      | TestExternalReviewResumeStartsOneFreshReviewOfTheBase                   |
      | TestExternalRoundLimitDefaultsToTheExecutorsOwn                         |
      | TestExternalRoundLimitKeepsTheCandidate                                 |
      | TestExternalRoundsWithoutEditsGetANoticeThenStopAsNoProgress            |
      | TestExternalStartFailureStopsTheBuild                                   |
      | TestExternalTaskAndRunRecords                                           |
      | TestExternalUnsupportedEndingsAreProtocolFailures                       |
      | TestExternalWithoutAProvidedExecutorIsUnavailable                       |
      | TestKReviewRetry2ForcedReset                                            |
      | TestKReviewRetryReminderChoice                                          |
      | TestKReviewRetrySafetyBoundaries                                        |
      | TestLoopKnobsNativeBuilder                                              |
      | TestLoopKnobsReview                                                     |
      | TestNativeSelectionIgnoresDepsExternal                                  |
      | TestReviewInPartsBlockStopsLaterParts                                   |
      | TestReviewInPartsCustody                                                |
      | TestReviewInPartsLands                                                  |
      | TestReviewInPartsMandatoryMaterialStopsBeforeSending                    |
      | TestReviewInPartsOmitsAChangedFilesFullText                             |
      | TestReviewInPartsRepairIsVerifiedBeforeTheNextPart                      |
      | TestReviewInPartsVerdictlessPartRestartsFromPartOne                     |
      | TestUnmovedExternalFeedback                                             |
      | TestUsageLimitResumeSafety                                              |
      | TestWholeIntentRedFeedbackEndsWithTheFinishReminder                     |
    And b-consumes-c-models-only-in-models-yaml: that run used kogen-coordinator "v0.0.0-20261106110407-d74daae36f56" without a replacement

  Scenario: Resuming an Elixir Build whose project still has models: in .kogen/project.yaml is refused
    When b-consumes-c-models-only-in-models-yaml: I run these Go tests of "./build":
      | test                                                                         |
      | TestALegacyBuilderCheckpointResumesTheSameConversationAndLandsOnce           |
      | TestALegacyBuilderVerifyCheckpointRunsTheFixersAndThePassThenLands           |
      | TestALegacyRetryCheckpointSendsTheBlockerAndTheFreshResultsToTheSameReviewer |
      | TestALegacyReviewCheckpointContinuesTheSameReviewerConversation              |
      | TestALegacyReviewVerifyCheckpointJudgesTheSavedVerdictWithoutAskingAgain     |
      | TestLegacyResumeRefusals                                                     |
      | TestMain76BuildsResumeAndLand                                                |
      | TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer         |
    Then b-consumes-c-models-only-in-models-yaml: every test of that run passed and none was skipped, including:
      | test                                                                         |
      | TestALegacyBuilderCheckpointResumesTheSameConversationAndLandsOnce           |
      | TestALegacyBuilderVerifyCheckpointRunsTheFixersAndThePassThenLands           |
      | TestALegacyRetryCheckpointSendsTheBlockerAndTheFreshResultsToTheSameReviewer |
      | TestALegacyReviewCheckpointContinuesTheSameReviewerConversation              |
      | TestALegacyReviewVerifyCheckpointJudgesTheSavedVerdictWithoutAskingAgain     |
      | TestLegacyResumeRefusals                                                     |
      | TestMain76BuildsResumeAndLand                                                |
      | TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer         |
    And b-consumes-c-models-only-in-models-yaml: that run used kogen-coordinator "v0.0.0-20261106110407-d74daae36f56" without a replacement

  Scenario: The review records are one golden each, recorded from a project.yaml without models:
    When b-consumes-c-models-only-in-models-yaml: I run these Go tests of "./build":
      | test                               |
      | TestStateGoldenReviewApproveRecord |
      | TestStateGoldenReviewChangesRecord |
    Then b-consumes-c-models-only-in-models-yaml: every test of that run passed and none was skipped, including:
      | test                               |
      | TestStateGoldenReviewApproveRecord |
      | TestStateGoldenReviewChangesRecord |
    And b-consumes-c-models-only-in-models-yaml: that run used kogen-coordinator "v0.0.0-20261106110407-d74daae36f56" without a replacement
    And b-consumes-c-models-only-in-models-yaml: the file "build/testdata/state/review-approve/review-1.json" has the SHA-256 "67751b6165e4493afe1cc17c586b50cee3975e36152c0aeaf5973e2b05025b08"
    And b-consumes-c-models-only-in-models-yaml: the file "build/testdata/state/review-changes/review-1.json" has the SHA-256 "49a5dcc93376c0a0509411200282a12091f2f1ce10398270055f5714466ca66e"
    And b-consumes-c-models-only-in-models-yaml: these files do not exist:
      | file                                                    |
      | build/testdata/state/review-approve/review-1-moved.json |
      | build/testdata/state/review-changes/review-1-moved.json |
    And b-consumes-c-models-only-in-models-yaml: the files under "build/testdata" other than these have the digest "0f4f3a55bebcac564e3069c5c7ce8a544e3d02a2571cfb381e63f25e72f29592":
      | file                                                    |
      | build/testdata/state/review-approve/review-1.json       |
      | build/testdata/state/review-changes/review-1.json       |
      | build/testdata/state/review-approve/review-1-moved.json |
      | build/testdata/state/review-changes/review-1-moved.json |

  Scenario: No test or code keeps the former lineup layout
    Then b-consumes-c-models-only-in-models-yaml: no Go file under these folders contains "lineupMoved":
      | folder   |
      | build    |
      | cmd      |
      | consumes |
      | contract |
    And b-consumes-c-models-only-in-models-yaml: no Go file under these folders contains "models_moved":
      | folder   |
      | build    |
      | cmd      |
      | consumes |
      | contract |
    And b-consumes-c-models-only-in-models-yaml: no Go file under these folders contains "review-1-moved":
      | folder   |
      | build    |
      | cmd      |
      | consumes |
      | contract |
