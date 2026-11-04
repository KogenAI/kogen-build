'use strict';
// Real `go test -json` rows that qualify the run judge of k-build-review-round-limit-repairs__steps.js
// (its good and bad controls). Each list is every row other than the "output" rows, verbatim and in
// order, of the stdout of `go test -json -count=1 -run '^TestClaudeLineupReviewRoundLimit$' ./build`
// on the unchanged checkout (kogen-build 32c65a7).

// passing: with the committed go.mod (kogen-review v0.0.0-20261103124347-cfc2fb29917c); go test exited 0.
const passing = [
  '{"Time":"2026-11-04T13:28:49.362071+03:00","Action":"start","Package":"github.com/KogenAI/kogen-build/build"}',
  '{"Time":"2026-11-04T13:28:49.916102+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit"}',
  '{"Time":"2026-11-04T13:28:49.916271+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit"}',
  '{"Time":"2026-11-04T13:28:49.916281+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit"}',
  '{"Time":"2026-11-04T13:28:49.916304+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/default"}',
  '{"Time":"2026-11-04T13:28:49.916343+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/default"}',
  '{"Time":"2026-11-04T13:28:49.916352+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/one"}',
  '{"Time":"2026-11-04T13:28:49.916397+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/one"}',
  '{"Time":"2026-11-04T13:28:49.916406+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/two"}',
  '{"Time":"2026-11-04T13:28:49.916433+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/two"}',
  '{"Time":"2026-11-04T13:28:49.916442+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/zero"}',
  '{"Time":"2026-11-04T13:28:49.916495+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/zero"}',
  '{"Time":"2026-11-04T13:28:49.916503+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/default"}',
  '{"Time":"2026-11-04T13:28:49.916522+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/two"}',
  '{"Time":"2026-11-04T13:28:49.916538+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/one"}',
  '{"Time":"2026-11-04T13:28:49.917059+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/zero"}',
  '{"Time":"2026-11-04T13:28:52.706588+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/one","Elapsed":1.67}',
  '{"Time":"2026-11-04T13:28:53.235425+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/two","Elapsed":2.79}',
  '{"Time":"2026-11-04T13:28:54.031712+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/default","Elapsed":3.32}',
  '{"Time":"2026-11-04T13:28:54.031743+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/zero","Elapsed":4.12}',
  '{"Time":"2026-11-04T13:28:54.031751+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit","Elapsed":0}',
  '{"Time":"2026-11-04T13:28:54.032452+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Elapsed":4.67}',
];

// identityBlind: against a copy of kogen-review v0.0.0-20261103212337-81d3baef1d48 whose
// review/independent.go has `slices.Equal(ids, r.sess.FailureIdentities)` replaced by `len(ids) > 0`;
// go test exited 1.
const identityBlind = [
  '{"Time":"2026-11-04T13:29:19.003862+03:00","Action":"start","Package":"github.com/KogenAI/kogen-build/build"}',
  '{"Time":"2026-11-04T13:29:19.581414+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit"}',
  '{"Time":"2026-11-04T13:29:19.581631+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit"}',
  '{"Time":"2026-11-04T13:29:19.581651+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit"}',
  '{"Time":"2026-11-04T13:29:19.58167+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/default"}',
  '{"Time":"2026-11-04T13:29:19.581786+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/default"}',
  '{"Time":"2026-11-04T13:29:19.581794+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/one"}',
  '{"Time":"2026-11-04T13:29:19.581825+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/one"}',
  '{"Time":"2026-11-04T13:29:19.581837+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/two"}',
  '{"Time":"2026-11-04T13:29:19.581858+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/two"}',
  '{"Time":"2026-11-04T13:29:19.581864+03:00","Action":"run","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/zero"}',
  '{"Time":"2026-11-04T13:29:19.581917+03:00","Action":"pause","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/zero"}',
  '{"Time":"2026-11-04T13:29:19.581925+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/default"}',
  '{"Time":"2026-11-04T13:29:19.581937+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/two"}',
  '{"Time":"2026-11-04T13:29:19.582125+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/one"}',
  '{"Time":"2026-11-04T13:29:19.582137+03:00","Action":"cont","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/zero"}',
  '{"Time":"2026-11-04T13:29:22.190616+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/two","Elapsed":1.95}',
  '{"Time":"2026-11-04T13:29:22.58725+03:00","Action":"fail","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/default","Elapsed":2.62}',
  '{"Time":"2026-11-04T13:29:22.614443+03:00","Action":"pass","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/one","Elapsed":3.01}',
  '{"Time":"2026-11-04T13:29:22.619252+03:00","Action":"fail","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit/zero","Elapsed":3.04}',
  '{"Time":"2026-11-04T13:29:22.61926+03:00","Action":"fail","Package":"github.com/KogenAI/kogen-build/build","Test":"TestClaudeLineupReviewRoundLimit","Elapsed":0}',
  '{"Time":"2026-11-04T13:29:22.620293+03:00","Action":"fail","Package":"github.com/KogenAI/kogen-build/build","Elapsed":3.616}',
];

module.exports = { passing, identityBlind };
