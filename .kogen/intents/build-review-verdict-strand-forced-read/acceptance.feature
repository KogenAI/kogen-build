Feature: build-review-verdict-strand-forced-read: the review strand test checks the forced tool against unread pinned parts

  This feature covers kogen-build's TestKReviewVerdictStrandReset, which must accept a forced read_file while pinned parts are unread and a forced submit_review once all are read, using only test changes.

  Scenario: the strand reset test passes offline
    When build-review-verdict-strand-forced-read: I run the Go test "TestKReviewVerdictStrandReset" in "./build"
    Then build-review-verdict-strand-forced-read: the test ran and passed and was not skipped

  Scenario: the forced-tool check is conditional and covers both states
    Then build-review-verdict-strand-forced-read: build/k_review_verdict_strand_test.go checks both "read_file" and "submit_review" as the forced tool depending on unread parts

  Scenario: only test files and fixtures change
    Then build-review-verdict-strand-forced-read: every changed file is a test file or a fixture
