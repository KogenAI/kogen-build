Feature: A build round without edits is its own condition

  Scenario: A builder that answers every round with text only after a read-only look
    Then build-no-edit-turns-end-build: the no-edit Build fixture "text-only" ends as specified
      """
      {
        "exit": 1,
        "outcome": "no_progress",
        "acceptanceRuns": 1,
        "passes": 1,
        "rounds": 3,
        "feedback": {
          "3": "Round 2 made no edits: the candidate is byte-identical to the tree verification pass 1 checked, so it was not verified again; pass 1 is still red.\nMake an edit toward the first named failure: failed: Export item 1 (line 2).\nIf a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\nAnother round without edits stops the Build as no_progress (no edits)."
        },
        "feedbackStarts": {
          "2": "Verification pass 1 is not green."
        },
        "stdout": [
          "candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none; verification pass 1 is still red and was not run again; edits are left in place\n",
          "  first failure: failed: Export item 1 (line 2)\n",
          "  rounds without edits: 3 of 3\n"
        ],
        "stdoutLacks": [],
        "stderr": [
          "kogen: round 1 made no edits\n",
          "kogen: round 2 made no edits; not verifying the tree of pass 1 again; asked the builder to edit toward failed: Export item 1 (line 2) or report a protected-file blocker\n",
          "kogen: round 3 made no edits\n"
        ],
        "stderrLacks": [
          "kogen: round 4 "
        ]
      }
      """

  Scenario: An external builder that ends every round with a status message and no edits
    Then build-no-edit-turns-end-build: the no-edit Build fixture "external-text-only" ends as specified
      """
      {
        "exit": 1,
        "outcome": "no_progress",
        "acceptanceRuns": 1,
        "passes": 1,
        "rounds": 3,
        "feedback": {
          "3": "Round 2 made no edits: the candidate is byte-identical to the tree verification pass 1 checked, so it was not verified again; pass 1 is still red.\nMake an edit toward the first named failure: failed: Export item 1 (line 2).\nIf a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\nAnother round without edits stops the Build as no_progress (no edits)."
        },
        "feedbackStarts": {
          "2": "Verification pass 1 is not green."
        },
        "stdout": [
          "candidate/no_progress: no edits: round 3 made no edits after the builder was told round 2 made none; verification pass 1 is still red and was not run again; edits are left in place\n",
          "  first failure: failed: Export item 1 (line 2)\n",
          "  rounds without edits: 3 of 3\n"
        ],
        "stdoutLacks": [
          "executor_round_limit"
        ],
        "stderr": [
          "kogen: round 1 made no edits\n",
          "kogen: round 2 made no edits; not verifying the tree of pass 1 again; asked the builder to edit toward failed: Export item 1 (line 2) or report a protected-file blocker\n",
          "kogen: round 3 made no edits\n"
        ],
        "stderrLacks": [
          "kogen: round 4 "
        ]
      }
      """

  Scenario: A builder that makes one small edit in round 3 and otherwise answers with text only
    Then build-no-edit-turns-end-build: the no-edit Build fixture "late-edit" ends as specified
      """
      {
        "exit": 1,
        "outcome": "no_progress",
        "acceptanceRuns": 2,
        "passes": 2,
        "rounds": 5,
        "feedback": {
          "3": "Round 2 made no edits: the candidate is byte-identical to the tree verification pass 1 checked, so it was not verified again; pass 1 is still red.\nMake an edit toward the first named failure: failed: Export item 1 (line 2).\nIf a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\nAnother round without edits stops the Build as no_progress (no edits).",
          "5": "Round 4 made no edits: the candidate is byte-identical to the tree verification pass 2 checked, so it was not verified again; pass 2 is still red.\nMake an edit toward the first named failure: failed: Export item 2 (line 4).\nIf a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\nAnother round without edits stops the Build as no_progress (no edits)."
        },
        "feedbackStarts": {
          "2": "Verification pass 1 is not green.",
          "4": "Verification pass 2 is not green."
        },
        "stdout": [
          "candidate/no_progress: no edits: round 5 made no edits after the builder was told round 4 made none; verification pass 2 is still red and was not run again; edits are left in place\n",
          "  first failure: failed: Export item 2 (line 4)\n",
          "  rounds without edits: 4 of 5\n"
        ],
        "stdoutLacks": [],
        "stderr": [
          "kogen: round 1 made no edits\n",
          "kogen: round 2 made no edits; not verifying the tree of pass 1 again; asked the builder to edit toward failed: Export item 1 (line 2) or report a protected-file blocker\n",
          "kogen: round 4 made no edits; not verifying the tree of pass 2 again; asked the builder to edit toward failed: Export item 2 (line 4) or report a protected-file blocker\n",
          "kogen: round 5 made no edits\n"
        ],
        "stderrLacks": [
          "kogen: round 3 made no edits",
          "kogen: round 6 "
        ]
      }
      """

  Scenario: A builder that reports a protected-file blocker after rounds without edits
    Then build-no-edit-turns-end-build: the no-edit Build fixture "blocker" ends as specified
      """
      {
        "exit": 1,
        "outcome": "build_blocked",
        "acceptanceRuns": 1,
        "passes": 1,
        "rounds": 3,
        "feedback": {
          "3": "Round 2 made no edits: the candidate is byte-identical to the tree verification pass 1 checked, so it was not verified again; pass 1 is still red.\nMake an edit toward the first named failure: failed: Export item 1 (line 2).\nIf a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\nAnother round without edits stops the Build as no_progress (no edits)."
        },
        "feedbackStarts": {
          "2": "Verification pass 1 is not green."
        },
        "stdout": [
          "candidate/build_blocked: BUILDER-REPORTED protected-baseline contradiction; edits are left in place; this report is not independently proven\n",
          "  paths: .kogen/intents/fixture/acceptance.feature\n",
          "  reason: The export scenarios require a file the import scenarios forbid.\n"
        ],
        "stdoutLacks": [
          "no_progress",
          "no edits:"
        ],
        "stderr": [
          "kogen: round 2 made no edits; not verifying the tree of pass 1 again; asked the builder to edit toward failed: Export item 1 (line 2) or report a protected-file blocker\n"
        ],
        "stderrLacks": [
          "kogen: round 4 "
        ]
      }
      """

  Scenario: A builder that edits every round without reducing the failing set
    Then build-no-edit-turns-end-build: the no-edit Build fixture "edits" ends as specified
      """
      {
        "exit": 1,
        "outcome": "no_progress",
        "acceptanceRuns": 4,
        "passes": 4,
        "rounds": 6,
        "feedback": {
          "6": "Round 5 made no edits: the candidate is byte-identical to the tree verification pass 4 checked, so it was not verified again; pass 4 is still red.\nMake an edit toward the first named failure: failed: Export item 1 (line 2).\nIf a correct implementation cannot make it pass because a protected file is wrong, report a protected-file blocker with its paths, the contradiction and your observed evidence instead.\nAnother round without edits stops the Build as no_progress (no edits)."
        },
        "feedbackStarts": {
          "2": "Verification pass 1 is not green.",
          "3": "Failing set did not move for 2 consecutive red passes. Last round changed: unrelated.txt. None of the failing items changed.\nCommon failure text (address first): the requested output was never produced\n\nVerification pass 2 is not green.",
          "4": "Failing set did not move for 3 consecutive red passes. Last round changed: unrelated.txt. None of the failing items changed.\nCommon failure text (address first): the requested output was never produced\n\nVerification pass 3 is not green.",
          "5": "Failing set did not move for 4 consecutive red passes. Last round changed: unrelated.txt. None of the failing items changed.\nCommon failure text (address first): the requested output was never produced\n\nVerification pass 4 is not green."
        },
        "stdout": [
          "candidate/no_progress: no edits: round 6 made no edits after the builder was told round 5 made none; verification pass 4 is still red and was not run again; edits are left in place\n",
          "  first failure: failed: Export item 1 (line 2)\n",
          "  rounds without edits: 2 of 6\n"
        ],
        "stdoutLacks": [],
        "stderr": [
          "kogen: round 5 made no edits; not verifying the tree of pass 4 again; asked the builder to edit toward failed: Export item 1 (line 2) or report a protected-file blocker\n",
          "kogen: round 6 made no edits\n"
        ],
        "stderrLacks": [
          "kogen: round 1 made no edits",
          "kogen: round 2 made no edits",
          "kogen: round 3 made no edits",
          "kogen: round 4 made no edits",
          "kogen: round 7 "
        ]
      }
      """
