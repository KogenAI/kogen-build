Feature: Bound the initial Build file index
  This feature makes the builder brief carry a bounded, truthful file index for very large repositories.
  A local Go fixture builds a synthetic repository of 25000 mixed tracked and untracked long paths and runs the real builder brief and opening message. It makes no provider or model call.

  Scenario: A 25000-path repository gives a bounded index and bounded complete composition
    Given build-bound-initial-file-index: the focused Go fixture is available
    When build-bound-initial-file-index: I run the fixture test "TestBoundedIndexLargeRepoStaysBounded"
    Then build-bound-initial-file-index: the fixture test passed

  Scenario: Counts, directories and required paths are truthful
    Given build-bound-initial-file-index: the focused Go fixture is available
    When build-bound-initial-file-index: I run the fixture test "TestBoundedIndexMetadataIsTruthfulAndRequiredPathsListed"
    Then build-bound-initial-file-index: the fixture test passed

  Scenario: An omitted but real path stays accessible
    Given build-bound-initial-file-index: the focused Go fixture is available
    When build-bound-initial-file-index: I run the fixture test "TestBoundedIndexOmittedPathStaysAccessible"
    Then build-bound-initial-file-index: the fixture test passed

  Scenario: The bounded selection is deterministic
    Given build-bound-initial-file-index: the focused Go fixture is available
    When build-bound-initial-file-index: I run the fixture test "TestBoundedIndexSelectionIsDeterministic"
    Then build-bound-initial-file-index: the fixture test passed

  Scenario: A small repository keeps its exact listing
    Given build-bound-initial-file-index: the focused Go fixture is available
    When build-bound-initial-file-index: I run the fixture test "TestBoundedIndexSmallRepoIsUnchanged"
    Then build-bound-initial-file-index: the fixture test passed

  Scenario: Irreducible authority fails with the typed budget error before any provider call
    Given build-bound-initial-file-index: the focused Go fixture is available
    When build-bound-initial-file-index: I run the fixture test "TestBoundedIndexIrreducibleAuthorityFailsBeforeAnyProvider"
    Then build-bound-initial-file-index: the fixture test passed
