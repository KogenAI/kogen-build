Feature: Review gets its own package over a shared core

  The review implementation lives in build/review, the state and services that build and review
  share live in build/core, and build keeps its exported API by delegating to build/review.

  Scenario: build/review depends only on build/core, and build/core on no other kogen-build package
    When the seam lists the Go dependencies, tests included, of "./build/review"
    Then the kogen-build packages among them are exactly:
      | github.com/KogenAI/kogen-build/build/core   |
      | github.com/KogenAI/kogen-build/build/review |
    When the seam lists the Go dependencies, tests included, of "./build/core"
    Then the kogen-build packages among them are exactly:
      | github.com/KogenAI/kogen-build/build/core |
    And the seam's package "./build" imports:
      | github.com/KogenAI/kogen-build/build/core   |
      | github.com/KogenAI/kogen-build/build/review |

  Scenario: The tests of build/core and build/review pass and accept the tests' -update flag
    Given a seam copy of this checkout
    When the seam runs "go test -count=1 ./build/core/... ./build/review/... -update" in the copy
    Then the seam run passes, and a test of build/core or build/review ran

  Scenario Outline: The dependency test refuses a kogen-build import the rule forbids in <package>
    Given a seam copy of this checkout
    And in the seam copy a new package "build/seamprobe" exists and a non-test Go file of "<package>" imports it
    When the seam runs "go test -count=1 ./build/core/... ./build/review/..." in the copy
    Then the seam run fails in a test whose output names "github.com/KogenAI/kogen-build/build/seamprobe"

    Examples:
      | package      |
      | build/review |
      | build/core   |

  Scenario: Review, Resume, the review loop, verdict evaluation and the review tool conversation are in build/review
    Then each of these texts is in a non-test Go file under "build/review" and in no non-test Go file of "build" or under "build/core":
      | kogen: reviewing (review %d)                                                                        |
      | the saved review state cannot be read                                                               |
      | an independent review by an external executor cannot be resumed (resume capability: none)           |
      | The caller explicitly resumed this blocked Build.                                                   |
      | the reviewer could not resolve a substantive blocker within the approved Intent; edits are left in place |
      | The new tree passed checks but has not been reviewed.                                               |
      | Your verdict was not applied: the candidate changed after its contents were delivered to you        |
      | the review ended without calling submit_review                                                      |
      | ignored: the review already has a verdict                                                           |
      | a changes verdict needs at least one blocking finding                                               |
      | the reviewer did not return a valid verdict; edits are left in place                                |
      | kogen: reviewing (review %d, part %d of %d)                                                         |
      | the review material was not completely delivered in a completed round of the reviewer session      |

  Scenario: The persisted-state goldens keep their bytes and their tests pass
    Then these files have these SHA-256 digests:
      | build/testdata/state/fresh/checkpoint.json          | 0677504f24b93c95957b589d75a8f6492c70c6dca1cee2d749d597ea48fd91e6 |
      | build/testdata/state/attempts/checkpoint.json       | b31ae59edd08e94f569eb38ee6575c0844335987abf6b3252c5db1cdb7079a61 |
      | build/testdata/state/review-approve/checkpoint.json | 9a11c904cf8912b8018fe4f320e3cd532a7e996c769cdb1dd1a9f4295ee6e019 |
      | build/testdata/state/review-approve/review-1.json   | 5c3393411e6c7e65132fb823726dcc8f4520be9d7cb24843299aae3197f72bd3 |
      | build/testdata/state/review-changes/checkpoint.json | 628e6f029da299ad824434c73b3b5406a182f7893c585a03d201e14c63314008 |
      | build/testdata/state/review-changes/review-1.json   | 13152a3f5c09f96980983b517aed6cfd2f7038544f0f455ce3637cdf1f609547 |
      | build/testdata/state/resumed/checkpoint.json        | 1933825e102ed1fe6818ed67b98b41af9073f382d634f8b779e85cf6e6796f54 |
    And no file under "build/testdata" is modified, deleted or renamed relative to HEAD
    When the seam runs "go test ./build -count=1 -v -run ^TestStateGolden" in this checkout
    Then the seam run passes, and these tests report PASS:
      | TestStateGoldenFreshCheckpoint          |
      | TestStateGoldenAttemptsCheckpoint       |
      | TestStateGoldenReviewApproveCheckpoint  |
      | TestStateGoldenReviewChangesCheckpoint  |
      | TestStateGoldenResumedCheckpoint        |
      | TestStateGoldenReviewApproveRecord      |
      | TestStateGoldenReviewChangesRecord      |

  Scenario: The Build state a checkpoint persists is encoded by build/core
    Given a seam copy of this checkout
    And in the seam copy every 'json:"round"' in the non-test Go files under "build/core" is replaced by 'json:"rounds"'
    When the seam runs "go test ./build -count=1 -v -run ^TestStateGolden" in the copy
    Then the seam run fails in a test whose output names each of:
      | testdata/state/fresh/checkpoint.json          |
      | testdata/state/attempts/checkpoint.json       |
      | testdata/state/review-approve/checkpoint.json |
      | testdata/state/review-changes/checkpoint.json |
      | testdata/state/resumed/checkpoint.json        |

  Scenario: The review record and the reviewer session a real review persists are encoded by build/review
    Given a seam copy of this checkout
    And in the seam copy every '"effort"' in the non-test Go files under "build/review" is replaced by '"reasoning_effort"'
    And in the seam copy every 'json:"attempts"' in the non-test Go files under "build/review" is replaced by 'json:"verdicts"'
    When the seam runs "go test ./build -count=1 -v -run ^TestStateGolden" in the copy
    Then the seam run fails in a test whose output names each of:
      | testdata/state/review-approve/checkpoint.json |
      | testdata/state/review-approve/review-1.json   |
      | testdata/state/review-changes/checkpoint.json |
      | testdata/state/review-changes/review-1.json   |

  Scenario: A consumer of the build package's exported API still compiles
    When the seam compiles "features/steps/k_review_seam/consumer.go.txt" as a new package of this module
    Then the seam run compiles without a diagnostic

  Scenario: Every existing test and fuzz target is still a test of a build package
    When the seam runs "go test -list .* ./build/..." in this checkout
    Then the seam run lists every name in "features/steps/k_review_seam/existing-tests.txt"

  Scenario: README.md names the two new packages
    Then the seam's README.md has a line that starts with each of:
      | - `build/review`: |
      | - `build/core`:   |
