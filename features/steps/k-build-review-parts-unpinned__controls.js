'use strict';
// Real `go test -json` rows that qualify the run judge of k-build-review-parts-unpinned__steps.js (its
// good and bad controls). Each list is every row other than the "output" rows, verbatim and in order,
// of the stdout of one run on the unchanged checkout (kogen-build 5aaef04).

// passing: `go test -json -count=1 -run '^(<the eight review-parts tests>)$' ./build` with the committed
// go.mod (kogen-review v0.0.0-20261103124347-cfc2fb29917c); go test exited 0.
const passing = [
  "{\"Time\":\"2026-11-04T12:00:36.740352+03:00\",\"Action\":\"start\",\"Package\":\"github.com/KogenAI/kogen-build/build\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296153+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewVerdictStrandReset\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296372+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewVerdictStrandReset\"}",
  "{\"Time\":\"2026-11-04T12:00:37.29638+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296406+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296414+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKRevodGatePartsUnchanged\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296437+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKRevodGatePartsUnchanged\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296449+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296521+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296551+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296602+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296609+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestChangesWhilePartsAreUnreadIsSentBackToRead\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296639+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestChangesWhilePartsAreUnreadIsSentBackToRead\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296645+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestAReviewerThatNeverReadsIsStopped\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296663+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestAReviewerThatNeverReadsIsStopped\"}",
  "{\"Time\":\"2026-11-04T12:00:37.29667+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestChangesAfterEveryPartWasReadStaysBlocked\"}",
  "{\"Time\":\"2026-11-04T12:00:37.29669+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestChangesAfterEveryPartWasReadStaysBlocked\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296695+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewVerdictStrandReset\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296707+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296727+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestAReviewerThatNeverReadsIsStopped\"}",
  "{\"Time\":\"2026-11-04T12:00:37.296835+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKRevodGatePartsUnchanged\"}",
  "{\"Time\":\"2026-11-04T12:00:39.772142+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKRevodGatePartsUnchanged/empty-source_controls\"}",
  "{\"Time\":\"2026-11-04T12:00:40.013884+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation\",\"Elapsed\":2.72}",
  "{\"Time\":\"2026-11-04T12:00:40.013905+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances\"}",
  "{\"Time\":\"2026-11-04T12:00:40.014029+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/S-DECREL:_unread_rejections,_prose,_then_the_forced_verdict_in_the_same_conversation\"}",
  "{\"Time\":\"2026-11-04T12:00:40.014064+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/S-DECREL:_unread_rejections,_prose,_then_the_forced_verdict_in_the_same_conversation\"}",
  "{\"Time\":\"2026-11-04T12:00:40.014068+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/reverse_order_in_a_fresh_review:_prose,_then_a_rejected_verdict,_then_a_valid_one\"}",
  "{\"Time\":\"2026-11-04T12:00:40.014077+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/reverse_order_in_a_fresh_review:_prose,_then_a_rejected_verdict,_then_a_valid_one\"}",
  "{\"Time\":\"2026-11-04T12:00:40.01408+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance\"}",
  "{\"Time\":\"2026-11-04T12:00:40.014104+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance/second_rejected_verdict,_then_second_prose-only_turn\"}",
  "{\"Time\":\"2026-11-04T12:00:40.014115+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance/second_rejected_verdict,_then_second_prose-only_turn\"}",
  "{\"Time\":\"2026-11-04T12:00:40.014119+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance/rejected_then_prose_then_prose,_and_prose_then_rejected_then_rejected\"}",
  "{\"Time\":\"2026-11-04T12:00:40.014128+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance/rejected_then_prose_then_prose,_and_prose_then_rejected_then_rejected\"}",
  "{\"Time\":\"2026-11-04T12:00:40.014131+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval\"}",
  "{\"Time\":\"2026-11-04T12:00:40.813504+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestAReviewerThatNeverReadsIsStopped\",\"Elapsed\":3.52}",
  "{\"Time\":\"2026-11-04T12:00:40.813519+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestChangesAfterEveryPartWasReadStaysBlocked\"}",
  "{\"Time\":\"2026-11-04T12:00:41.569061+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKRevodGatePartsUnchanged/empty-source_controls\",\"Elapsed\":1.8}",
  "{\"Time\":\"2026-11-04T12:00:41.58151+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKRevodGatePartsUnchanged\",\"Elapsed\":4.28}",
  "{\"Time\":\"2026-11-04T12:00:41.581518+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestChangesWhilePartsAreUnreadIsSentBackToRead\"}",
  "{\"Time\":\"2026-11-04T12:00:41.845141+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance/second_rejected_verdict,_then_second_prose-only_turn\"}",
  "{\"Time\":\"2026-11-04T12:00:44.11109+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewVerdictStrandReset\",\"Elapsed\":4.55}",
  "{\"Time\":\"2026-11-04T12:00:44.111124+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance/second_rejected_verdict,_then_second_prose-only_turn\",\"Elapsed\":2.27}",
  "{\"Time\":\"2026-11-04T12:00:44.111127+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance/rejected_then_prose_then_prose,_and_prose_then_rejected_then_rejected\"}",
  "{\"Time\":\"2026-11-04T12:00:46.5178+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance/rejected_then_prose_then_prose,_and_prose_then_rejected_then_rejected\",\"Elapsed\":2.41}",
  "{\"Time\":\"2026-11-04T12:00:46.517814+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/a_second_spending_ends_only_its_own_allowance\",\"Elapsed\":0}",
  "{\"Time\":\"2026-11-04T12:00:46.517817+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/S-DECREL:_unread_rejections,_prose,_then_the_forced_verdict_in_the_same_conversation\"}",
  "{\"Time\":\"2026-11-04T12:00:50.181922+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval\",\"Elapsed\":10.17}",
  "{\"Time\":\"2026-11-04T12:00:50.18193+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/reverse_order_in_a_fresh_review:_prose,_then_a_rejected_verdict,_then_a_valid_one\"}",
  "{\"Time\":\"2026-11-04T12:00:51.526971+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestChangesAfterEveryPartWasReadStaysBlocked\",\"Elapsed\":9.65}",
  "{\"Time\":\"2026-11-04T12:00:51.542138+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestChangesWhilePartsAreUnreadIsSentBackToRead\",\"Elapsed\":9.95}",
  "{\"Time\":\"2026-11-04T12:00:53.617647+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/S-DECREL:_unread_rejections,_prose,_then_the_forced_verdict_in_the_same_conversation\",\"Elapsed\":5.02}",
  "{\"Time\":\"2026-11-04T12:00:53.617686+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances/reverse_order_in_a_fresh_review:_prose,_then_a_rejected_verdict,_then_a_valid_one\",\"Elapsed\":3.44}",
  "{\"Time\":\"2026-11-04T12:00:53.617696+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestKReviewRetrySeparateAllowances\",\"Elapsed\":6.5}",
  "{\"Time\":\"2026-11-04T12:00:53.620496+03:00\",\"Action\":\"pass\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Elapsed\":16.88}",
];

// faultFailing: `go test -modfile=<temporary go.mod> -json -count=1 -run
// '^(TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation)$' ./build` against a copy of
// kogen-review v0.0.0-20261104082000-fee52689bd61 with the 7d005ed6 layout stand-in and the unread
// approval check removed; go test exited 1.
const faultFailing = [
  "{\"Time\":\"2026-11-04T12:00:55.338847+03:00\",\"Action\":\"start\",\"Package\":\"github.com/KogenAI/kogen-build/build\"}",
  "{\"Time\":\"2026-11-04T12:00:55.946651+03:00\",\"Action\":\"run\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation\"}",
  "{\"Time\":\"2026-11-04T12:00:55.946798+03:00\",\"Action\":\"pause\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation\"}",
  "{\"Time\":\"2026-11-04T12:00:55.946802+03:00\",\"Action\":\"cont\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation\"}",
  "{\"Time\":\"2026-11-04T12:00:59.881262+03:00\",\"Action\":\"fail\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Test\":\"TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation\",\"Elapsed\":3.93}",
  "{\"Time\":\"2026-11-04T12:00:59.882178+03:00\",\"Action\":\"fail\",\"Package\":\"github.com/KogenAI/kogen-build/build\",\"Elapsed\":4.543}",
];

module.exports = { passing, faultFailing };
