Feature: kogen-build's review-parts tests assert outcomes, not kogen-review's prompt layout

  The eight review-parts tests of package build pass with the kogen-review version go.mod pins, with
  kogen-review main (fee5268) and with a stand-in for the layout of kogen-review 7d005ed6, made from
  main: the opening's source block begins with "# Review task" and carries no pinned-part catalog,
  and a changes verdict is no longer sent back while parts are unread. With the stand-in, the
  approval test still catches a review that accepts an approval with an unread part, or whose
  refusal does not name the unread part.

  Background:
    Given k-build-review-parts-unpinned: the review-parts tests are:
      | test                                                                |
      | TestAReviewerThatNeverReadsIsStopped                                |
      | TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation   |
      | TestChangesAfterEveryPartWasReadStaysBlocked                        |
      | TestChangesWhilePartsAreUnreadIsSentBackToRead                      |
      | TestKReviewRetrySeparateAllowances                                  |
      | TestKReviewVerdictStrandReset                                       |
      | TestKRevodGatePartsUnchanged                                        |
      | TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval  |
    And k-build-review-parts-unpinned: their required subtests are:
      | subtest                                                                                                         |
      | TestKReviewRetrySeparateAllowances/S-DECREL:_unread_rejections,_prose,_then_the_forced_verdict_in_the_same_conversation |
      | TestKReviewRetrySeparateAllowances/reverse_order_in_a_fresh_review:_prose,_then_a_rejected_verdict,_then_a_valid_one    |
      | TestKRevodGatePartsUnchanged/empty-source_controls                                                              |
    And k-build-review-parts-unpinned: the 7d005ed6 layout stand-in is kogen-review v0.0.0-20261104082000-fee52689bd61 with these edits:
      | file                  | from                                                                                     | to                                                                                                                      | count |
      | review/independent.go | input, err := env.Prepare(pass.Tree, m, role, ReviewerInstructions(), tools, diagnostic) | input, err := env.Prepare(pass.Tree, m, role, ReviewerInstructions(), tools, diagnostic); input = standInOpening(input) | 1     |
      | review/independent.go | if verdict == "changes" && !r.external {                                                 | if verdict == "changes" && !r.external && false {                                                                       | 1     |
    And k-build-review-parts-unpinned: the stand-in adds the file review/standin_layout.go:
      """go
      package review

      import "strings"

      // standInOpening is the acceptance stand-in for the layout of kogen-review 7d005ed6: the source
      // block of a fresh review's opening begins with "# Review task" and no longer carries Kogen's
      // pinned-part catalog. Kogen's own custody of the parts is untouched.
      func standInOpening(input []any) []any {
        if len(input) == 0 {
          return input
        }
        first, ok := input[0].(map[string]any)
        if !ok {
          return input
        }
        blocks, ok := first["content"].([]any)
        if !ok || len(blocks) < 2 {
          return input
        }
        block, ok := blocks[1].(map[string]any)
        if !ok {
          return input
        }
        text, _ := block["text"].(string)
        const heading = "# Pinned complete candidate source evidence\n"
        at := strings.Index(text, heading)
        if at < 0 {
          return input
        }
        _, after, _ := strings.Cut(text[at+len(heading):], "\n")
        changed := map[string]any{}
        for k, v := range block {
          changed[k] = v
        }
        changed["text"] = "# Review task\n\nKogen delivers this candidate's pinned source parts; read each one before a verdict.\n\n" + text[:at] + after
        content := append([]any(nil), blocks...)
        content[1] = changed
        opening := map[string]any{}
        for k, v := range first {
          opening[k] = v
        }
        opening["content"] = content
        out := append([]any(nil), input...)
        out[0] = opening
        return out
      }
      """

  Scenario: With the kogen-review version go.mod pins, every review-parts test passes
    Then k-build-review-parts-unpinned: go.mod requires github.com/KogenAI/kogen-review at v0.0.0-20261103124347-cfc2fb29917c without replacing it
    When k-build-review-parts-unpinned: go test runs the review-parts tests in package build with the committed go.mod
    Then k-build-review-parts-unpinned: go test reported every review-parts test and required subtest passing, and no test failing or skipped

  Scenario: Against kogen-review main, every review-parts test passes
    When k-build-review-parts-unpinned: go test runs the review-parts tests in package build against kogen-review v0.0.0-20261104082000-fee52689bd61
    Then k-build-review-parts-unpinned: go test reported every review-parts test and required subtest passing, and no test failing or skipped

  Scenario: Against the 7d005ed6 layout stand-in, every review-parts test passes
    When k-build-review-parts-unpinned: go test runs the review-parts tests in package build against the stand-in
    Then k-build-review-parts-unpinned: go test reported every review-parts test and required subtest passing, and no test failing or skipped

  Scenario Outline: With the stand-in's layout, the approval test still catches a review where <fault>
    When k-build-review-parts-unpinned: go test runs TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation in package build against the stand-in
    Then k-build-review-parts-unpinned: go test reported TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation passing
    When k-build-review-parts-unpinned: go test runs TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation in package build against the stand-in with this further edit:
      | file                  | from   | to   | count |
      | review/independent.go | <from> | <to> | 1     |
    Then k-build-review-parts-unpinned: go test reported TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation failing

    Examples:
      | fault                                         | from                                                                                       | to                                                                                           |
      | an approval with an unread part is accepted   | if verdict == "approve" && !r.external {                                                   | if verdict == "approve" && !r.external && false {                                            |
      | the refusal does not name the unread part     | "Read all pinned source evidence before approving: %d unread part(s); use read_file on %s", | "Read all pinned source evidence before approving: %d unread part(s); use read_file on %.0s", |

  Scenario: The review-request byte goldens are removed and the README no longer lists them
    Then k-build-review-parts-unpinned: the file build/testdata/k-revod-gate/parts-requests.json does not exist
    And k-build-review-parts-unpinned: the file build/testdata/k-revod-gate/parts-requests-moved.json does not exist
    And k-build-review-parts-unpinned: the file README.md does not contain "k-revod-gate/parts-requests.json"

  Scenario: The change touches no production file of kogen-build
    Then k-build-review-parts-unpinned: the candidate changes no non-test Go file, go.mod or go.sum
