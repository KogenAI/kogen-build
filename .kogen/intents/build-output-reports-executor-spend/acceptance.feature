Feature: Report executor spend in every Build outcome
  This feature makes the Build's landed summary and stop report show what the Build spent.
  A local Go fixture runs real Build entry points with scripted fake executors and no model.
  Historical motivation: in the AoR climb, 148 of 157 landed summaries read tokens: 0 above
  executor lines (for example repairs: 1 · tokens: 0 · time: 13m07s, then
  executor: claude-code (harness-reported tokens input=6607784 cached=6464136 output=44938 reasoning=unknown, estimate $3.3403)
  and review executor: claude-code (harness-reported tokens input=445080 cached=396139 output=5682 reasoning=unknown, estimate $0.2922)),
  and all 13 build_blocked claude-code Builds (for example failed rails-ft-saas-billing: build_blocked (Build 76e139e5),
  not run because an earlier item failed: test, still failing: acceptance) printed no spend line.

  Scenario: Landed external builder and reviewer omit the zero native token total
    Given build-output-reports-executor-spend: the focused Go fixture is available
    When build-output-reports-executor-spend: I run the fixture test "TestSpendReportLandedExternal"
    Then build-output-reports-executor-spend: the output has the exact line "  repairs: 0 · time: 3m25s"
    And build-output-reports-executor-spend: the output has no line containing "tokens: 0"
    And build-output-reports-executor-spend: the output has a line matching "^  executor: claude-code "
    And build-output-reports-executor-spend: the output has a line matching "^  review executor: claude-code "

  Scenario: A landed native Build keeps its native token total
    Given build-output-reports-executor-spend: the focused Go fixture is available
    When build-output-reports-executor-spend: I run the fixture test "TestSpendReportLandedNative"
    Then build-output-reports-executor-spend: the output has a line matching "^  repairs: 0 · tokens: 2 · time: [0-9]+s$"
    And build-output-reports-executor-spend: the output has no line containing "executor:"

  Scenario: An external builder's round-limit stop reports its spend before the run log
    Given build-output-reports-executor-spend: the focused Go fixture is available
    When build-output-reports-executor-spend: I run the fixture test "TestSpendReportRoundLimit"
    Then build-output-reports-executor-spend: the output has these adjacent lines, each a regular expression:
      """
      ^  edited: app\.txt$
      ^  repairs: \d+ · time: [0-9hms]+$
      ^  executor: claude-code \(harness-reported tokens input=200 cached=0 output=80 reasoning=unknown, estimate \$0\.5000\)$
      ^  run log: 
      """
    And build-output-reports-executor-spend: the output has the exact line "  edited: app.txt"
    And build-output-reports-executor-spend: the output has the exact line "  executor: claude-code (harness-reported tokens input=200 cached=0 output=80 reasoning=unknown, estimate $0.5000)"

  Scenario: An external reviewer's blocking verdict stop reports both executors
    Given build-output-reports-executor-spend: the focused Go fixture is available
    When build-output-reports-executor-spend: I run the fixture test "TestSpendReportReviewBlocked"
    Then build-output-reports-executor-spend: the output has these adjacent exact lines:
      """
        executor: claude-code (harness-reported tokens input=100 cached=0 output=40 reasoning=unknown, estimate $0.5000)
        review executor: claude-code (harness-reported tokens input=50 cached=10 output=5 reasoning=unknown, estimate $0.2500)
      """
    And build-output-reports-executor-spend: the output has a line matching "^candidate/review_blocked: "

  Scenario: A native Build's stop reports native tokens and no executor line
    Given build-output-reports-executor-spend: the focused Go fixture is available
    When build-output-reports-executor-spend: I run the fixture test "TestSpendReportNativeStop"
    Then build-output-reports-executor-spend: the output has no line containing "executor:"
    And build-output-reports-executor-spend: the output has no line containing "  executor: native"
    And build-output-reports-executor-spend: the output has a line matching "^  repairs: [0-9]+ · tokens: "

  Scenario: A startup stop prints no spend lines
    Given build-output-reports-executor-spend: the focused Go fixture is available
    When build-output-reports-executor-spend: I run the fixture test "TestSpendReportStartupStop"
    Then build-output-reports-executor-spend: the output has no line matching "^  repairs: "
    And build-output-reports-executor-spend: the output has a line matching "stopped in startup: baseline_changed"
    And build-output-reports-executor-spend: the output has no line containing "executor:"
