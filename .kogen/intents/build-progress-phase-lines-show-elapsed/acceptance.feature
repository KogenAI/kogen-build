Feature: Show elapsed seconds at Build phase boundaries
  This feature fixes Build stage timing without changing tool progress, summaries,
  feedback texts or routing decisions. The local Go fixture runs Build and Resume
  using the existing fake executors and Go's fake clock, with no model calls.
  Historical motivation: AoR climb, release 30cf6619,
  h11-build-sonnet-high-vs-baseline-30cf6619 cell
  rails-ft-data-export-and-import__baseline__r2, stderr.txt and result.json:
  wall_s 1499.443 (wall 1499 s), Build summary time: 16m40s, 62 building
  round-1 lines, 51 Bash ok, followed by verification, approval and committing.
  Unlike Shape's kogen: checking readiness (205 s), those Build lines had no time.
  A 17-minute Build therefore could not be read for where the time went.

  Scenario: Timestamp stage boundaries but preserve per-tool progress across resume
    Given build-progress-phase-lines-show-elapsed: the focused Go fixture is available
    When build-progress-phase-lines-show-elapsed: I run the model-free Build and resume with a fake clock
    Then build-progress-phase-lines-show-elapsed: stage suffixes and unchanged tool lines are exactly:
      """
      {
        "fresh": "kogen: checking test before the Build changes anything (0 s)\nkogen: building (round 1)\nkogen: building (round 1) — Read input.txt ok\nkogen: building (round 1) — Bash failed\nkogen: verifying (pass 1) (205 s)\nkogen: pass 1: all checks passed (205 s)\nkogen: reviewing (review 1) (205 s)\nkogen: review 1: approve, 0 blocking (223 s)\nkogen: committing (223 s)\n",
        "resumed": "kogen: committing (323 s)\n"
      }
      """
