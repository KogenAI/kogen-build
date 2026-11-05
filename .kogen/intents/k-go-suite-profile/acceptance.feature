Feature: k-go-suite-profile: the build package's tests cost less per simulated Build and catch the same defects

  Almost every test of ./build runs one or more simulated Builds against a scripted provider, and
  on the host where the request was measured most of each Build's fixed cost came from the tests'
  own environment: the verification pass of every Build started each toolchain on the host's PATH
  to record its version, and every fixture project wrote a new acceptance script whose first run
  waited for the operating system's check of a new executable. The package now gives its Builds a
  PATH of only the tools its tests use, every fixture project runs one shared, read-only
  acceptance script, and the slow tests that ran alone run in parallel. Each changed test still
  fails, with its own message, against a deliberately broken implementation of what it checks.

  Scenario: Builds started by the build package's tests reach none of the host's other toolchains
    When k-go-suite-profile: I run these Go tests of the build package with stand-ins for the host's other toolchains first on PATH:
      | test                                               |
      | TestTheReviewerApprovesUnchangedCodeWithoutRepairs |
      | TestNamedFeedbackBuilderRepair                     |
    Then k-go-suite-profile: each of those Go tests passed, and no stand-in toolchain ran

  Scenario: Every fixture project runs one shared, read-only acceptance script that still goes red
    When k-go-suite-profile: I run the shared acceptance script check in the build package
    Then k-go-suite-profile: the shared acceptance script check passed

  Scenario: Every changed test still fails, in parallel, against a deliberately broken implementation of what it checks
    When k-go-suite-profile: I run these Go tests of the build package at the same time, each against its broken implementation:
      | broken implementation     | test                                                    | message                                                                                                 |
      | finish-reminder-dropped   | TestWholeIntentRedFeedbackEndsWithTheFinishReminder     | native feedback "Verification pass 1 is not green.                                                      |
      | standalone-checks-dropped | TestStandaloneBuildEvidence                             | "checks":[]                                                                                             |
      | maintained-models-dropped | TestSuccessiveModelIntentsKeepIndependentSpecifications | missing [run .kogen/models/fixture/model.qnt --backend typescript --invariant Safe --seed 007 --max-samples 03 --max-steps 04] |
      | unmoved-notice-dropped    | TestUnmovedScopedFeedback                               | first Stage foundation failed; remain at this stage.                                                    |
      | unmoved-notice-dropped    | TestUnmovedExternalFeedback                             | inputs [{Feedback:} {Feedback:Verification pass 1 is not green.                                         |
      | red-repair-uncounted      | TestRedVerificationIsFedBackToTheSameBuilderAndRepaired | repairs = 0, want 1                                                                                     |
    Then k-go-suite-profile: each of those Go tests ran in parallel and failed with its own message
