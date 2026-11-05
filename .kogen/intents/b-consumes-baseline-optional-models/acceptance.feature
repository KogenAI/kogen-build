Feature: b-consumes-baseline-optional-models: kogen-build's tests pass whether or not the coordinator lists an absent .kogen/models.yaml in the baseline

  kogen-coordinator's pending change coord-baseline-optional-files-when-present makes
  project.Baseline leave .kogen/models.yaml out when the project has no such file (tracked, or
  untracked and not ignored); the pinned version always lists it right after .kogen/project.yaml.
  kogen-build's tests pass with either coordinator: the review and named-feedback captures run on a
  fixture that tracks .kogen/models.yaml, and the Elixir checkpoints' baseline is brought up to
  whatever the coordinator in use lists for their fixture project, which has no lineup file.

  Scenario: The baseline tests pass against both coordinators and still reject a misplaced lineup file
    When b-consumes-baseline-optional-models: I run these Go tests of "./build" against the "absent" kogen-coordinator:
      | test                                                                |
      | TestKReviewRetryUnchangedValidRequests                              |
      | TestLegacyResumeRefusals                                            |
      | TestMain76BuildsResumeAndLand                                       |
      | TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer |
      | TestNamedFeedbackCompatibility                                      |
    Then b-consumes-baseline-optional-models: every test of that run passed and none was skipped, including:
      | test                                                                |
      | TestKReviewRetryUnchangedValidRequests                              |
      | TestLegacyResumeRefusals                                            |
      | TestMain76BuildsResumeAndLand                                       |
      | TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer |
      | TestNamedFeedbackCompatibility                                      |
    When b-consumes-baseline-optional-models: I run these Go tests of "./build" against the "pinned" kogen-coordinator:
      | test                                                                |
      | TestKReviewRetryUnchangedValidRequests                              |
      | TestLegacyResumeRefusals                                            |
      | TestMain76BuildsResumeAndLand                                       |
      | TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer |
      | TestNamedFeedbackCompatibility                                      |
    Then b-consumes-baseline-optional-models: every test of that run passed and none was skipped, including:
      | test                                                                |
      | TestKReviewRetryUnchangedValidRequests                              |
      | TestLegacyResumeRefusals                                            |
      | TestMain76BuildsResumeAndLand                                       |
      | TestMain76ExternallyRepairedBlockedReviewReopensUnderTheSameReviewer |
      | TestNamedFeedbackCompatibility                                      |
    When b-consumes-baseline-optional-models: I run these Go tests of "./build" against the "misplaced" kogen-coordinator:
      | test                                   |
      | TestKReviewRetryUnchangedValidRequests |
      | TestNamedFeedbackCompatibility         |
    Then b-consumes-baseline-optional-models: that run failed, with each of these tests failing:
      | test                                   |
      | TestKReviewRetryUnchangedValidRequests |
      | TestNamedFeedbackCompatibility         |

  Scenario: Only the named-feedback golden changes, by listing the tracked lineup file, and the pins stay
    Then b-consumes-baseline-optional-models: the recorded request of "build/testdata/named-feedback-compat.json" lists exactly these project files:
      """
      # Files

      .kogen/intents/fixture/acceptance.feature
      .kogen/intents/fixture/intent.md
      features/steps/integration_steps.js
      .gitignore
      .kogen/models.yaml
      .kogen/project.yaml
      README.md
      app.txt
      """
    And b-consumes-baseline-optional-models: the file "build/testdata/named-feedback-compat.json" has the SHA-256 "41f9a57de40764f38c54e5f90ffa8b0d8b073a2ef3f42429c6b798e437508389"
    And b-consumes-baseline-optional-models: the file "build/testdata/reviewretry/approve.wire.json" has the SHA-256 "5bf61f6b23171c679f76d81434c9346ddf50e61edff0b56caf3a5357579bbb80"
    And b-consumes-baseline-optional-models: the file "build/testdata/reviewretry/changes.wire.json" has the SHA-256 "5bf61f6b23171c679f76d81434c9346ddf50e61edff0b56caf3a5357579bbb80"
    And b-consumes-baseline-optional-models: the files under "build/testdata" other than "build/testdata/named-feedback-compat.json" have the digest "0a2b96b1305a76df1fe7a724e92e7d537bf24d1018ad694f26e009231a2f56e5"
    And b-consumes-baseline-optional-models: the file "go.mod" has the SHA-256 "bfcfa998e3171f3fcb64eda9257d1f1950e38b356152df6ef3602f492b363934"
    And b-consumes-baseline-optional-models: the file "go.sum" has the SHA-256 "44690a0e668ede21a20209c6a69c27de2b4771bff8e1e0dbdf0a47faccbe8ace"
