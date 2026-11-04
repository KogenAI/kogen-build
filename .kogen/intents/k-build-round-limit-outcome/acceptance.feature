Feature: Rounds that call a write tool keep the round-limit outcomes kogen's spec states

  # kogen spec/features/04-building.feature:105 (LOOP-7): a native builder that writes the same
  # wrong file and calls finish every round keeps repairing; every round is verified until the
  # scripted model stops answering after 25 rounds.
  Scenario: Ordinary implementation failures keep repairing until the caller stops it
    Then k-build-round-limit-outcome: the round-limit Build fixture "native-rewrites" ends as specified
      """
      {
        "redRounds": 25,
        "exit": 4,
        "outcome": "refused",
        "passes": 25,
        "passEvents": 25,
        "rounds": 26,
        "reviewedPasses": [],
        "feedbackStarts": {
          "2": "Verification pass 1 is not green.",
          "3": "Verification pass 2 is not green.",
          "26": "Verification pass 25 is not green."
        },
        "stdout": ["provider/refused: ChatGPT refused the request: 400 scripted provider refusal\n"],
        "stdoutLacks": ["no_progress"],
        "stderrLacks": ["made no edits"],
        "app": "red\n"
      }
      """

  # kogen spec/features/04-building.feature:321 (LOOP-14): an external builder that writes the
  # same file with its Write tool every round under a check that stays red stops at the third
  # identical red pass, and its checkpoint cannot be resumed.
  Scenario: Identical red passes leave the candidate in place and the external checkpoint cannot be resumed
    Then k-build-round-limit-outcome: the round-limit Build fixture "external-rewrites" ends as specified
      """
      {
        "exit": 1,
        "outcome": "executor_round_limit",
        "passes": 3,
        "passEvents": 3,
        "rounds": 3,
        "reviewedPasses": [],
        "feedbackStarts": {
          "2": "Verification pass 1 is not green.",
          "3": "Verification pass 2 is not green."
        },
        "stdout": [
          "candidate/executor_round_limit: the executor used its 3 rounds without a verified, reviewed candidate; the candidate is left in place\n",
          "  edited: app.txt\n"
        ],
        "stdoutLacks": ["no_progress"],
        "stderrLacks": ["made no edits"],
        "app": "red\n",
        "resume": {
          "exit": 1,
          "stdout": ["candidate/resume_refused"],
          "rounds": 3,
          "app": "red\n"
        }
      }
      """

  # kogen spec/features/06-reviewing.feature:53 (REV-1): the builder receives the feedback of
  # verification pass 3, and no review starts while verification is red.
  Scenario: No review while verification is red
    Then k-build-round-limit-outcome: the round-limit Build fixture "native-rewrites" ends as specified
      """
      {
        "redRounds": 3,
        "exit": 4,
        "outcome": "refused",
        "passes": 3,
        "passEvents": 3,
        "rounds": 4,
        "reviewedPasses": [],
        "feedbackStarts": {
          "2": "Verification pass 1 is not green.",
          "3": "Verification pass 2 is not green.",
          "4": "Verification pass 3 is not green."
        },
        "stdout": [],
        "stdoutLacks": ["no_progress"],
        "stderrLacks": ["made no edits"],
        "app": "red\n"
      }
      """

  Scenario: A native round with only a read-only look is still a round without edits
    Then k-build-round-limit-outcome: the round-limit Build fixture "native-read-only" ends as specified
      """
      {
        "exit": 1,
        "outcome": "no_progress",
        "passes": 1,
        "passEvents": 1,
        "rounds": 3,
        "reviewedPasses": [],
        "feedbackStarts": {
          "2": "Verification pass 1 is not green.",
          "3": "Round 2 made no edits: the candidate is byte-identical to the tree verification pass 1 checked, so it was not verified again; pass 1 is still red.\nMake an edit toward the first named failure: failed: First (line 2).\nIf a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\nAnother round without edits stops the Build as no_progress (no edits)."
        },
        "stdout": [
          "candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none; verification pass 1 is still red and was not run again; edits are left in place\n",
          "  rounds without edits: 2 of 3\n"
        ],
        "stdoutLacks": ["executor_round_limit"],
        "stderrLacks": ["kogen: round 4 "],
        "app": "red\n"
      }
      """

  Scenario: An external round that ends with a status message and no write call is still a round without edits
    Then k-build-round-limit-outcome: the round-limit Build fixture "external-status-only" ends as specified
      """
      {
        "exit": 1,
        "outcome": "no_progress",
        "passes": 1,
        "passEvents": 1,
        "rounds": 3,
        "reviewedPasses": [],
        "feedbackStarts": {
          "2": "Verification pass 1 is not green.",
          "3": "Round 2 made no edits: the candidate is byte-identical to the tree verification pass 1 checked, so it was not verified again; pass 1 is still red.\nMake an edit toward the first named failure: failed: First (line 2).\nIf a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\nAnother round without edits stops the Build as no_progress (no edits)."
        },
        "stdout": [
          "candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none; verification pass 1 is still red and was not run again; edits are left in place\n",
          "  rounds without edits: 2 of 3\n"
        ],
        "stdoutLacks": ["executor_round_limit"],
        "stderrLacks": ["kogen: round 4 "],
        "app": "red\n"
      }
      """
