Feature: TestClaudeLineupReviewRoundLimit reaches the executor's review cap whatever repair stop kogen-review has
  kogen-review 932c061 stops the reviewer's repair loop with candidate/review_blocked after three
  consecutive final verifications that fail with the same set of stage-qualified failing items.
  TestClaudeLineupReviewRoundLimit scripted every review round to fail acceptance the same way, so
  with kogen-review main (81d3bae) its cap-6 subtests stopped before the executor's review cap. The
  scripted rounds now alternate between a failing acceptance and a failing configured check, so the
  cap ends the review role with candidate/executor_round_limit against the pinned kogen-review
  (cfc2fb29917c), against main and against a stricter no-progress stop. The test still fails when every
  red final verification counts toward the stop, and it names none of kogen-review's stop.

  Scenario: With the committed go.mod pin the test passes all four subtests
    When k-build-review-round-limit-repairs: go test runs TestClaudeLineupReviewRoundLimit in package build with the committed go.mod
    Then k-build-review-round-limit-repairs: go test reported TestClaudeLineupReviewRoundLimit as pass with these subtest results:
      | subtest | result |
      | default | pass   |
      | one     | pass   |
      | two     | pass   |
      | zero    | pass   |
    And k-build-review-round-limit-repairs: go.mod requires github.com/KogenAI/kogen-review at v0.0.0-20261103124347-cfc2fb29917c without replacing it

  Scenario: Against kogen-review main 81d3bae, which stops repeated identical final failures, the test passes
    When k-build-review-round-limit-repairs: go test runs TestClaudeLineupReviewRoundLimit in package build against kogen-review v0.0.0-20261103212337-81d3baef1d48
    Then k-build-review-round-limit-repairs: go test reported TestClaudeLineupReviewRoundLimit as pass with these subtest results:
      | subtest | result |
      | default | pass   |
      | one     | pass   |
      | two     | pass   |
      | zero    | pass   |

  Scenario: Against a kogen-review that stops after two identical final failures in a row the test passes
    When k-build-review-round-limit-repairs: go test runs TestClaudeLineupReviewRoundLimit in package build against kogen-review v0.0.0-20261103212337-81d3baef1d48 with these edits:
      | file                  | from               | to                 | count |
      | review/independent.go | FailureStreak >= 3 | FailureStreak >= 2 | 1     |
    Then k-build-review-round-limit-repairs: go test reported TestClaudeLineupReviewRoundLimit as pass with these subtest results:
      | subtest | result |
      | default | pass   |
      | one     | pass   |
      | two     | pass   |
      | zero    | pass   |

  Scenario: Against a kogen-review that stops after any three red final verifications in a row the cap-6 subtests fail
    When k-build-review-round-limit-repairs: go test runs TestClaudeLineupReviewRoundLimit in package build against kogen-review v0.0.0-20261103212337-81d3baef1d48 with these edits:
      | file                  | from                                       | to           | count |
      | review/independent.go | slices.Equal(ids, r.sess.FailureIdentities) | len(ids) > 0 | 1     |
    Then k-build-review-round-limit-repairs: go test reported TestClaudeLineupReviewRoundLimit as fail with these subtest results:
      | subtest | result |
      | default | fail   |
      | one     | pass   |
      | two     | pass   |
      | zero    | fail   |

  Scenario: The test keeps asserting the cap's outcome and names none of kogen-review's stop
    Then k-build-review-round-limit-repairs: the body of TestClaudeLineupReviewRoundLimit in build/review_lineup_test.go contains each of these texts:
      | text                                               |
      | wantStop(t, a, "candidate/executor_round_limit")   |
      | len(f.rounds(executor.RoleReview)); got != tc.max  |
      | len(f.starts[executor.RoleReview]) != 1            |
      | len(f.starts[executor.RoleBuild]) != 1             |
      | len(f.rounds(executor.RoleBuild)) != 1             |
      | eq(t, "HEAD", e.head(), start)                     |
      | "preserved work"                                   |
      | int(x["rounds"].(float64)) != tc.max               |
    And k-build-review-round-limit-repairs: the body of TestClaudeLineupReviewRoundLimit in build/review_lineup_test.go contains none of these texts:
      | text               |
      | review_blocked     |
      | repeated identical |
      | hard cap           |
      | FailureStreak      |
      | failure_streak     |
