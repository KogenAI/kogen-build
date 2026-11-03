Feature: An approval binds exactly the contents the reviewer was given
  Each scenario runs one real Build of a fixture project whose builder changes app.txt from
  "baseline" to "candidate", through the build package's own test environment. A write
  "outside" the native reviewer is made while its request is in flight; the external reviewer's
  edits are made inside its round, where Kogen cannot tell them from any other writer. An
  approval commits only contents whose bytes reached the reviewer: the source of its review,
  the native reviewer's own write_file and shell edits, or changes Kogen delivered to it as a
  diff. The reviewer's project instructions are the README of the Build's start, never the
  builder's edit of it.

  Scenario: A native approval of contents that changed while its request was in flight does not commit
    When the review-bind harness runs the case "native-outside-then-changes"
    Then the review-bind observations include:
      """
      {"exit": 1, "outcome": "review_blocked", "commits": "0", "reviewer_requests": 2, "checkout_app": "changed outside\n",
       "review_progress": "kogen: review 1: approve, 0 blocking\nkogen: review 1: the candidate changed after its contents were delivered; returning the changes to the reviewer\nkogen: review 1: changes, 1 blocking"}
      """
    And the review-bind observation "second_request_feedback" contains:
      """
      Your verdict was not applied: the candidate changed after its contents were delivered to you, other than through your own Kogen tool calls: app.txt.
      The exact diff of these changes against the contents delivered to you:

      diff --git a/app.txt b/app.txt
      """
    And the review-bind observation "second_request_feedback" contains:
      """
      @@ -1 +1 @@
      -candidate
      +changed outside
      """
    And the review-bind observation "second_request_feedback" contains:
      """
      The new tree passed checks but has not been reviewed. Inspect these changes and submit_review for the current contents; do not restart the whole critique.
      """

  Scenario: A native reviewer's fresh approval of the delivered changes commits them
    When the review-bind harness runs the case "native-outside-then-approve"
    Then the review-bind observations include:
      """
      {"exit": 0, "outcome": "landed", "commits": "1", "committed_app": "changed outside", "reviewer_requests": 2,
       "review_progress": "kogen: review 1: approve, 0 blocking\nkogen: review 1: the candidate changed after its contents were delivered; returning the changes to the reviewer\nkogen: review 1: approve, 0 blocking"}
      """
    And the review-bind observation "second_request_feedback" contains:
      """
      @@ -1 +1 @@
      -candidate
      +changed outside
      """

  Scenario: A native reviewer's own write_file repair is approved and lands as today
    When the review-bind harness runs the case "native-write-repair"
    Then the review-bind observations include:
      """
      {"exit": 0, "outcome": "landed", "commits": "1", "committed_app": "reviewer repair", "reviewer_requests": 1,
       "review_progress": "kogen: review 1: approve, 0 blocking"}
      """

  Scenario: A native reviewer's own shell repair is approved and lands as today
    When the review-bind harness runs the case "native-shell-repair"
    Then the review-bind observations include:
      """
      {"exit": 0, "outcome": "landed", "commits": "1", "committed_app": "reviewer shell repair", "reviewer_requests": 1,
       "review_progress": "kogen: review 1: approve, 0 blocking"}
      """

  Scenario: An external approval of contents that changed during its round does not commit
    When the review-bind harness runs the case "external-edit-then-changes"
    Then the review-bind observations include:
      """
      {"exit": 1, "outcome": "review_blocked", "commits": "0", "review_rounds": 2, "checkout_app": "changed outside\n",
       "review_progress": "kogen: review 1: approve, 0 blocking\nkogen: review 1: the candidate changed after its contents were delivered; returning the changes to the reviewer\nkogen: review 1: changes, 1 blocking"}
      """
    And the review-bind observation "second_round_feedback" contains:
      """
      Your verdict was not applied: the candidate changed after its contents were delivered to you, other than through your own Kogen tool calls: app.txt.
      The exact diff of these changes against the contents delivered to you:

      diff --git a/app.txt b/app.txt
      """
    And the review-bind observation "second_round_feedback" contains:
      """
      @@ -1 +1 @@
      -candidate
      +changed outside
      """

  Scenario: An external reviewer's edit lands after its explicit approval of the delivered diff
    When the review-bind harness runs the case "external-edit-then-approve"
    Then the review-bind observations include:
      """
      {"exit": 0, "outcome": "landed", "commits": "1", "committed_app": "changed outside", "review_rounds": 2,
       "review_progress": "kogen: review 1: approve, 0 blocking\nkogen: review 1: the candidate changed after its contents were delivered; returning the changes to the reviewer\nkogen: review 1: approve, 0 blocking"}
      """
    And the review-bind observation "second_round_feedback" contains:
      """
      @@ -1 +1 @@
      -candidate
      +changed outside
      """

  Scenario: A builder's README edit is candidate data, not the native reviewer's instructions
    When the review-bind harness runs the case "native-readme"
    Then the review-bind observations include:
      """
      {"exit": 0, "outcome": "landed", "commits": "1", "reviewer_requests": 1, "authority_found": true,
       "authority_readme_is_start_readme": true, "authority_has_injected_text": false, "source_has_injected_diff_line": true}
      """

  Scenario: A builder's README edit is candidate data, not the external reviewer's instructions
    When the review-bind harness runs the case "external-readme"
    Then the review-bind observations include:
      """
      {"exit": 0, "outcome": "landed", "commits": "1", "review_rounds": 1, "authority_found": true,
       "authority_readme_is_start_readme": true, "authority_has_injected_text": false, "source_has_injected_diff_line": true}
      """
