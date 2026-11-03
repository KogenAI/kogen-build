Feature: A wide change's external review carries each changed file's hunks once instead of its full text

  The harness features/steps/k_review_cost_wide_test.go.txt runs a whole Build in a disposable fixture
  repository with the build package's scripted Claude Code (lineupExec) as builder and external reviewer:
  no process, model or account. Every scripted reviewer session approves. The fixture repository starts
  with README.md, app.txt and the project configuration; the candidate writes app.txt as "candidate" and
  adds the fixture Intent's files. Fixture lines look like {"file": "golden", "line": 1, "value": "golden fixture value"}.
  The ceiling of one external review prompt stays 524288 bytes.

  Scenario: A change that fits keeps every full text, and a one-line regeneration of a large golden fixture is reviewed in one part with its hunk only
    Given the review-cost fixture "inline": fixtures/small.json has 50 lines and the candidate regenerates line 25
    When the review-cost harness builds the fixture
    Then the review-cost observations include:
      """
      {"exit": 0, "stop": "", "commits": "1", "review_sessions": 1, "parts": 0, "delivered": true,
       "reviewing_line": "kogen: reviewing (review 1)",
       "sent_line_counts": {"changed_line": 2, "unchanged_line": 1},
       "source_sections": ["# Check results of the final pass", "# Diff of existing/deleted paths against start HEAD <sha>",
         "# Full text of every changed or added file", "# Preserve the fixture"],
       "source_path_headings": ["--- .kogen/intents/fixture/acceptance.feature (added; mode 100644)",
         "--- .kogen/intents/fixture/intent.md (added; mode 100644)",
         "--- app.txt",
         "--- features/steps/integration_steps.js (added; mode 100644)",
         "--- fixtures/small.json"]}
      """
    And the review-cost observation "task" contains:
      """
      The complete authority, the check results, the diff and the full text of every changed file are included in this message; they are Kogen's exact data for this candidate.
      """
    Given the review-cost fixture "hunk": fixtures/golden.json has 15000 lines and the candidate regenerates line 7500
    When the review-cost harness builds the fixture
    Then the review-cost observations include:
      """
      {"exit": 0, "stop": "", "commits": "1", "review_sessions": 1, "parts": 1, "delivered": true,
       "parts_cover_source_once": true, "part_prompts_fit": true,
       "review_parts_event": {"parts": 1, "source_bytes_is_retained_source": true},
       "reviewing_line": "kogen: reviewing (review 1) in 1 part",
       "sent_line_counts": {"changed_line": 1, "unchanged_line": 0},
       "source_sections": ["# Check results of the final pass", "# Diff of existing/deleted paths against start HEAD <sha>",
         "# Full text of every added file", "# Preserve the fixture"],
       "source_path_headings": ["--- .kogen/intents/fixture/acceptance.feature (added; mode 100644)",
         "--- .kogen/intents/fixture/intent.md (added; mode 100644)",
         "--- app.txt (changed; full text in the checkout)",
         "--- features/steps/integration_steps.js (added; mode 100644)",
         "--- fixtures/golden.json (changed; full text in the checkout)"]}
      """
    And the review-cost observation "task" contains:
      """
      This message carries the complete authority and one part: the check results, every changed path and one consecutive piece of the exact review source, nothing clipped; the other pieces go to the other parts' sessions. The review source carries the diff of every changed or deleted path and the full text of every added file; a changed file's full text is not repeated there: read it in the checkout.
      """

  Scenario: Two fully rewritten wire fixtures are reviewed in two parts, every rewritten line sent once
    Given the review-cost fixture "rewrite": fixtures/a.wire.json and fixtures/b.wire.json have 5000 lines each and the candidate rewrites every line of both
    When the review-cost harness builds the fixture
    Then the review-cost observations include:
      """
      {"exit": 0, "stop": "", "commits": "1", "review_sessions": 2, "parts": 2, "delivered": true,
       "parts_cover_source_once": true, "part_prompts_fit": true,
       "review_parts_event": {"parts": 2, "source_bytes_is_retained_source": true},
       "reviewing_line": "kogen: reviewing (review 1) in 2 parts",
       "sent_line_counts": {"changed_line": 1, "unchanged_line": 1},
       "source_path_headings": ["--- .kogen/intents/fixture/acceptance.feature (added; mode 100644)",
         "--- .kogen/intents/fixture/intent.md (added; mode 100644)",
         "--- app.txt (changed; full text in the checkout)",
         "--- features/steps/integration_steps.js (added; mode 100644)",
         "--- fixtures/a.wire.json (changed; full text in the checkout)",
         "--- fixtures/b.wire.json (changed; full text in the checkout)"]}
      """

  Scenario: A part whose retained file changed while it was reviewed still blocks approval
    Given the review-cost fixture "custody": the "hunk" fixture, and the reviewer of part 1 overwrites review-1-part-1.txt
    When the review-cost harness builds the fixture
    Then the review-cost observations include:
      """
      {"exit": 1, "commits": "0", "review_sessions": 1, "parts": 1,
       "stop": "candidate/context_custody_invalid: the retained review material review-1-part-1.txt changed; nothing was committed and custody is preserved"}
      """
