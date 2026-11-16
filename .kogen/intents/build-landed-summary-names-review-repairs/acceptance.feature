Feature: The landed summary names what the review repaired
  This feature runs focused Build fixtures through the landing and reads the landed summary.

  Background:
    Given build-landed-summary-names-review-repairs: the focused Go fixture is available

  Scenario: An external reviewer repair is named in the landed summary
    When build-landed-summary-names-review-repairs: I run the fixture test "TestRepairSummaryExternalRepair"
    Then build-landed-summary-names-review-repairs: the output has these adjacent lines, each a regular expression:
      """
      ^  review: approved$
      ^  review repaired: app\.txt$
      ^  repairs: 1 
      """

  Scenario: A builder repair with an unchanged review is not named
    When build-landed-summary-names-review-repairs: I run the fixture test "TestRepairSummaryBuilderRepairOnly"
    Then build-landed-summary-names-review-repairs: the output has a line matching "^  repairs: 1 "
    And build-landed-summary-names-review-repairs: the output has no line matching "^  review repaired:"

  Scenario: More than five repaired paths are capped
    When build-landed-summary-names-review-repairs: I run the fixture test "TestRepairSummaryManyPaths"
    Then build-landed-summary-names-review-repairs: the output has the exact line "  review repaired: app.txt, r1.txt, r2.txt, r3.txt, r4.txt, and 2 more"

  Scenario: A native reviewer repair is named in the landed summary
    When build-landed-summary-names-review-repairs: I run the fixture test "TestRepairSummaryNativeRepair"
    Then build-landed-summary-names-review-repairs: the output has these adjacent lines, each a regular expression:
      """
      ^  review: approved$
      ^  review repaired: app\.txt$
      ^  repairs: 1 
      """
    When build-landed-summary-names-review-repairs: I run the fixture test "TestRepairSummaryResumedReview"
    Then build-landed-summary-names-review-repairs: the output has the exact line "  review repaired: app.txt"
