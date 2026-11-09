Feature: build-consumes-executor-claude-fixture: Build's context/external tests use the signed Executor Claude fixture

  This feature covers kogen-build only. Its tests that stand in for the Claude Code runtime use the
  test fixture of the signed kogen-executor main revision v0.0.0-20261109203850-f64de62e3831
  (testkit/clauderuntime) instead of their own copy of the runtime script, a fake runtime result
  that is malformed is refused with a control that fails when the adapter tolerates it, and
  everything else about Build stays as it is.

  Scenario: Build resolves the signed Executor main revision that carries the fixture
    Given build-consumes-executor-claude-fixture: the checkout of kogen-build
    Then build-consumes-executor-claude-fixture: go.mod requires kogen-executor "v0.0.0-20261109203850-f64de62e3831" and go.sum holds these lines:
      | line                                                                                                                      |
      | github.com/KogenAI/kogen-executor v0.0.0-20261109203850-f64de62e3831 h1:u1MnL3CRQivv/gU8BDKtQyx2AIcrI40G0tL8ncS3sKc=      |
      | github.com/KogenAI/kogen-executor v0.0.0-20261109203850-f64de62e3831/go.mod h1:dpPbIVCNEfFubxLT6iw/yCDVRO0ltgHm6d5ET6nV4hs= |
    And build-consumes-executor-claude-fixture: go.mod keeps these other requirements:
      | module                                  | version                                   |
      | github.com/KogenAI/kogen-coordinator    | v0.0.0-20261107115359-b1f2988eb753        |
      | github.com/KogenAI/kogen-loop           | v0.0.0-20261104223449-7e80986bcd04        |
      | github.com/KogenAI/kogen-review         | v0.0.0-20261104231132-7f6ef7d1c0a3        |
      | github.com/KogenAI/kogen-verification   | v0.0.0-20261104204404-74798e7aa09e        |
    And build-consumes-executor-claude-fixture: go resolves kogen-executor to "v0.0.0-20261109203850-f64de62e3831" and that module has the file "testkit/clauderuntime/fixture.go"
    And build-consumes-executor-claude-fixture: the tests of "./build" import "github.com/KogenAI/kogen-executor/testkit/clauderuntime"
    And build-consumes-executor-claude-fixture: no non-test package of "./build" and "./cmd/kogen-build" imports "github.com/KogenAI/kogen-executor/testkit/clauderuntime"

  Scenario: The duplicated runtime script is gone and the in-process fakes stay
    Given build-consumes-executor-claude-fixture: the checkout of kogen-build
    Then build-consumes-executor-claude-fixture: no Go file under these folders writes its own Claude runtime script:
      | folder   |
      | build    |
      | cmd      |
      | consumes |
      | contract |
    And build-consumes-executor-claude-fixture: the files contain these texts:
      | file                                | text                                                            |
      | build/context_external_test.go      | github.com/KogenAI/kogen-executor/testkit/clauderuntime         |
      | build/context_external_test.go      | clauderuntime.Install(                                          |
      | build/context_external_test.go      | claudecode.New(                                                 |
      | build/context_external_test.go      | func TestContextExternalFirstPrompt(                            |
      | build/context_external_test.go      | func TestContextExternalReviewMatrix(                           |
      | build/context_external_test.go      | func TestContextExternalNativeBytes(                            |
      | build/context_external_test.go      | # Protected paths                                               |
      | build/context_external_test.go      | Caller diagnostic data                                          |
      | build/context_external_test.go      | # Files                                                         |
      | build/context_external_test.go      | must not depend on a later round                                |
      | build/context_external_test.go      | newLineupExec(t)                                                |
      | build/review_lineup_test.go         | type lineupExec struct                                          |
      | build/executor_test.go              | type fakeExecutor struct                                        |
    And build-consumes-executor-claude-fixture: the files under these folders other than those matching "^build/context_external[^/]*_test\.go$" have the digest "e6d7298abb58cc1b40a86daab3c04e9707aa8e060ac123ee4cf4ee83eec3ce6e":
      | folder   |
      | build    |
      | cmd      |
      | consumes |
      | contract |

  Scenario: The focused tests pass on the fixture and fail when the adapter tolerates a malformed result
    When build-consumes-executor-claude-fixture: I run the focused context and external tests of the build package
    Then build-consumes-executor-claude-fixture: that run passed these tests and no test was skipped:
      | test                                          |
      | TestContextExternalFirstPrompt                |
      | TestContextExternalFirstPrompt/context_true   |
      | TestContextExternalFirstPrompt/context_false  |
      | TestContextExternalReviewMatrix               |
      | TestContextExternalNativeBytes                |
    And build-consumes-executor-claude-fixture: the focused context and external tests fail against each of these kogen-executor copies:
      | copy             | defect                                                    | failing                        |
      | ignoresmalformed | an event line that is not a JSON object is ignored       | any test                       |
      | doubledprompt    | the first prompt carries the stable authority twice       | TestContextExternalFirstPrompt |
