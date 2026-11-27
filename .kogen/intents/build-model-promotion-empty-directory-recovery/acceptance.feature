Feature: Promote the approved model into an empty model directory
  This feature runs disposable kogen-build fixtures: an Intent with an approved model is built and
  interrupted, the Build is resumed natively, and the maintained model directory is observed.

  Background:
    Given build-model-promotion-empty-directory-recovery: the disposable Build fixture driver is available

  Scenario: Resume the original Build over the empty directory native recovery left
    When build-model-promotion-empty-directory-recovery: I run the fixture tests "EmptyDirectory"
    Then build-model-promotion-empty-directory-recovery: the fixture tests "EmptyDirectory" ran and passed

  Scenario: Promote a refined model from the newly approved input
    When build-model-promotion-empty-directory-recovery: I run the fixture tests "RefinedModel"
    Then build-model-promotion-empty-directory-recovery: the fixture tests "RefinedModel" ran and passed

  Scenario: Keep complete maintained models byte for byte
    When build-model-promotion-empty-directory-recovery: I run the fixture tests "CompleteModelIntact,RepeatedPromotion"
    Then build-model-promotion-empty-directory-recovery: the fixture tests "CompleteModelIntact,RepeatedPromotion" ran and passed

  Scenario: Refuse partial, conflicting and drifted state
    When build-model-promotion-empty-directory-recovery: I run the fixture tests "RefusesUnsafeDirectories,RefusesDrift"
    Then build-model-promotion-empty-directory-recovery: the fixture tests "RefusesUnsafeDirectories,RefusesDrift" ran and passed

  Scenario: Publish the pair as one complete update
    When build-model-promotion-empty-directory-recovery: I run the fixture tests "AtomicPublication"
    Then build-model-promotion-empty-directory-recovery: the fixture tests "AtomicPublication" ran and passed

  Scenario: Refuse a mismatched Build identity without changes
    When build-model-promotion-empty-directory-recovery: I run the fixture tests "MismatchedIdentity,UnknownBuildRefused"
    Then build-model-promotion-empty-directory-recovery: the fixture tests "MismatchedIdentity,UnknownBuildRefused" ran and passed

  Scenario: Keep unrelated models and model-less Builds working
    When build-model-promotion-empty-directory-recovery: I run the fixture tests "UnrelatedModelKept,ModelFreeBuild,MissingDirectory"
    Then build-model-promotion-empty-directory-recovery: the fixture tests "UnrelatedModelKept,ModelFreeBuild,MissingDirectory" ran and passed
