Feature: Reopen a blocked independent review after an external repair
  A Build whose independent review ended with a blocking verdict is repaired outside the Build and resumed
  with `kogen intent resume`. The resume verifies the repaired tree fresh and returns the original finding to
  the same reviewer, for a current (version 4) and a previous (version 3) checkpoint; custody drift and
  checkpoint versions this Kogen cannot read are refused.

  Scenario: A blocked review of a current checkpoint, repaired outside the Build, is verified fresh and approved by the same reviewer
    # A version 4 checkpoint of a blocking verdict; app.txt is repaired outside the Build; the resume verifies the repaired tree, the same
    # reviewer gets its original finding with the repair's diff and the fresh results, approves, and the verified repaired tree lands.
    Then k-review-reopen-after-external-repair: the reopen harness in mode "current-fixed" observes:
      """
      {
        "app_txt": "repaired\n",
        "approval_kept": false,
        "at_decision": {
          "pass_green": true,
          "pass_tree_is_repaired": true,
          "verified": [
            "baseline",
            "candidate",
            "repaired"
          ]
        },
        "blocked": {
          "exit": 1,
          "stage": "review_verify",
          "stop": "candidate/review_blocked",
          "verdict": "changes",
          "version": 4
        },
        "child_stage": "landed",
        "child_verdict": null,
        "commits": 1,
        "committed_app": "repaired",
        "exit": 0,
        "finding_returned": true,
        "mode": "current-fixed",
        "new_runs": 1,
        "outcome": "landed",
        "refusal": "",
        "repair_delivered": true,
        "requests": 1,
        "same_reviewer": true,
        "stop": "",
        "verified": [
          "baseline",
          "candidate",
          "repaired"
        ]
      }
      """

  Scenario: A blocked review of a version 3 checkpoint, repaired outside the Build, resumes under the same rules
    # A version 3 (Elixir Kogen) checkpoint of a blocking verdict, stage review_verify with the verdict changes, repaired the same way:
    # the same fresh pass, the same reviewer and its finding, and the approval lands; no saved verdict or tree is replayed first.
    Then k-review-reopen-after-external-repair: the reopen harness in mode "previous-fixed" observes:
      """
      {
        "app_txt": "repaired\n",
        "approval_kept": false,
        "at_decision": {
          "pass_green": true,
          "pass_tree_is_repaired": true
        },
        "blocked": {
          "stage": "review_verify",
          "verdict": "changes",
          "version": 3
        },
        "child_stage": "landed",
        "child_verdict": null,
        "commits": 1,
        "committed_app": "repaired",
        "exit": 0,
        "finding_returned": true,
        "mode": "previous-fixed",
        "new_runs": 1,
        "outcome": "landed",
        "refusal": "",
        "repair_delivered": true,
        "requests": 1,
        "same_reviewer": true,
        "stop": ""
      }
      """

  Scenario: A repair that does not fix the finding keeps the review blocked
    # The repair leaves the problem; the reopened reviewer maintains its finding after the fresh pass: the review stays blocked, nothing
    # is committed, the repair stays in place and the new checkpoint is again a blocking verdict.
    Then k-review-reopen-after-external-repair: the reopen harness in mode "current-unfixed" observes:
      """
      {
        "app_txt": "still wrong\n",
        "approval_kept": true,
        "at_decision": {
          "pass_green": true,
          "pass_tree_is_repaired": true,
          "verified": [
            "baseline",
            "candidate",
            "still wrong"
          ]
        },
        "blocked": {
          "exit": 1,
          "stage": "review_verify",
          "stop": "candidate/review_blocked",
          "verdict": "changes",
          "version": 4
        },
        "child_stage": "review_verify",
        "child_verdict": "changes",
        "commits": 0,
        "committed_app": null,
        "exit": 1,
        "finding_returned": true,
        "mode": "current-unfixed",
        "new_runs": 1,
        "outcome": "review_blocked",
        "refusal": "",
        "repair_delivered": true,
        "requests": 1,
        "same_reviewer": true,
        "stop": "candidate/review_blocked",
        "verified": [
          "baseline",
          "candidate",
          "still wrong"
        ]
      }
      """

  Scenario: A version 3 blocked review whose repair does not fix the finding stays blocked
    # The same as the previous scenario, from a version 3 checkpoint.
    Then k-review-reopen-after-external-repair: the reopen harness in mode "previous-unfixed" observes:
      """
      {
        "app_txt": "still wrong\n",
        "approval_kept": true,
        "at_decision": {
          "pass_green": true,
          "pass_tree_is_repaired": true
        },
        "blocked": {
          "stage": "review_verify",
          "verdict": "changes",
          "version": 3
        },
        "child_stage": "review_verify",
        "child_verdict": "changes",
        "commits": 0,
        "committed_app": null,
        "exit": 1,
        "finding_returned": true,
        "mode": "previous-unfixed",
        "new_runs": 1,
        "outcome": "review_blocked",
        "refusal": "",
        "repair_delivered": true,
        "requests": 1,
        "same_reviewer": true,
        "stop": "candidate/review_blocked"
      }
      """

  Scenario: A stale approval recorded before the repair does not clear the finding
    # The checkpoint records a reviewer approval of the blocked tree (as the latest review and as an earlier attempt); after the repair
    # the resume still verifies fresh and asks the same reviewer, who maintains the finding: nothing is committed.
    Then k-review-reopen-after-external-repair: the reopen harness in mode "current-stale-approval" observes:
      """
      {
        "app_txt": "repaired\n",
        "approval_kept": true,
        "at_decision": {
          "pass_green": true,
          "pass_tree_is_repaired": true,
          "verified": [
            "baseline",
            "candidate",
            "repaired"
          ]
        },
        "blocked": {
          "exit": 1,
          "stage": "review_verify",
          "stop": "candidate/review_blocked",
          "verdict": "changes",
          "version": 4
        },
        "child_stage": "review_verify",
        "child_verdict": "changes",
        "commits": 0,
        "committed_app": null,
        "exit": 1,
        "finding_returned": true,
        "mode": "current-stale-approval",
        "new_runs": 1,
        "outcome": "review_blocked",
        "refusal": "",
        "repair_delivered": true,
        "requests": 1,
        "same_reviewer": true,
        "stop": "candidate/review_blocked",
        "verified": [
          "baseline",
          "candidate",
          "repaired"
        ]
      }
      """

  Scenario: A repair that also changes custody-relevant files is refused as custody drift
    # A protected (baseline) file changed with the repair, and, separately, the repair staged in the index: each resume is refused
    # as today, before any run, request or change, naming the drift.
    Then k-review-reopen-after-external-repair: the reopen harness in mode "current-custody" observes:
      """
      {
        "index": {
          "app_txt": "repaired\n",
          "approval_kept": true,
          "at_decision": null,
          "blocked": {
            "exit": 1,
            "stage": "review_verify",
            "stop": "candidate/review_blocked",
            "verdict": "changes",
            "version": 4
          },
          "child_stage": null,
          "child_verdict": null,
          "commits": 0,
          "committed_app": null,
          "exit": 1,
          "new_runs": 0,
          "outcome": null,
          "refusal": "Build ID: index/staged custody changed; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build",
          "requests": 0,
          "stop": "candidate/resume_refused",
          "verified": [
            "baseline",
            "candidate"
          ]
        },
        "mode": "current-custody",
        "protected": {
          "app_txt": "repaired\n",
          "approval_kept": true,
          "at_decision": null,
          "blocked": {
            "exit": 1,
            "stage": "review_verify",
            "stop": "candidate/review_blocked",
            "verdict": "changes",
            "version": 4
          },
          "child_stage": null,
          "child_verdict": null,
          "commits": 0,
          "committed_app": null,
          "exit": 1,
          "new_runs": 0,
          "outcome": null,
          "refusal": "Build ID: baseline changed: features/steps/integration_steps.js; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build",
          "requests": 0,
          "stop": "candidate/resume_refused",
          "verified": [
            "baseline",
            "candidate"
          ]
        }
      }
      """

  Scenario: A version 3 blocked review whose repair changes a protected file is refused as custody drift
    # The version 3 project protects accept.sh; changing it with the repair is refused as today.
    Then k-review-reopen-after-external-repair: the reopen harness in mode "previous-custody" observes:
      """
      {
        "app_txt": "repaired\n",
        "approval_kept": true,
        "at_decision": null,
        "blocked": {
          "stage": "review_verify",
          "verdict": "changes",
          "version": 3
        },
        "child_stage": null,
        "child_verdict": null,
        "commits": 0,
        "committed_app": null,
        "exit": 1,
        "mode": "previous-custody",
        "new_runs": 0,
        "outcome": null,
        "refusal": "Build ID: baseline changed: accept.sh; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build",
        "requests": 0,
        "stop": "candidate/resume_refused"
      }
      """

  Scenario: A checkpoint version this Kogen cannot read is refused with the version, the reason and the next step
    # The blocked checkpoint is rewritten as version 2: the refusal names the version and its incompatibility and says what to do next,
    # before any run or request.
    Then k-review-reopen-after-external-repair: the reopen harness in mode "unsupported-version" observes:
      """
      {
        "app_txt": "repaired\n",
        "approval_kept": true,
        "at_decision": null,
        "blocked": {
          "exit": 1,
          "stage": "review_verify",
          "stop": "candidate/review_blocked",
          "verdict": "changes",
          "version": 4
        },
        "child_stage": null,
        "child_verdict": null,
        "commits": 0,
        "committed_app": null,
        "exit": 1,
        "mode": "unsupported-version",
        "new_runs": 0,
        "outcome": null,
        "refusal": "Build ID: incompatible checkpoint version 2; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build",
        "requests": 0,
        "stop": "candidate/resume_refused",
        "verified": [
          "baseline",
          "candidate"
        ]
      }
      """
