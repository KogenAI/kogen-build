Feature: kogen-build's loop knob tests follow kogen-loop's line-aligned output cut

  # Every step runs, from the checkout, without writing it:
  #   go test -modfile=<scratch go.mod> -count=1 -v -run '^(TestLoopKnobsNativeBuilder|TestLoopKnobsReview)$' ./build
  # where the scratch go.mod replaces kogen-loop with a scratch copy of the pinned kogen-loop module
  # (go.mod's replace, github.com/KogenAI/kogen-loop v0.0.0-20261102055344-c6e2360290d4) whose tool-result cut is:
  #   pinned        unchanged: a 4096-byte head, the marker "\n[... N bytes omitted ...]\n" and a tail, 16384 bytes in all
  #   line-aligned  kogen-loop eed48c4's cutText: the head and the tail back off to complete lines
  #                 (the knob shell result "exit status: 0\n" + 30000 "a" is delivered as 12303 bytes, the marker after 15)
  #   head-only     a head of 16384 bytes less the marker, then the marker, which ends the text
  #   over-limit    the pinned cut at 20480 bytes instead of 16384
  #   unmarked      the first 16384 bytes, with no omission marker
  # "pass" needs exit 0 and both "--- PASS: TestLoopKnobsNativeBuilder (" and "--- PASS: TestLoopKnobsReview (";
  # "fail" needs a nonzero exit and both "--- FAIL: TestLoopKnobsNativeBuilder (" and "--- FAIL: TestLoopKnobsReview (".

  Scenario: The loop knob tests pass with kogen-loop's line-aligned cut
    Then kogen-build's loop knob tests pass with the "pinned" kogen-loop cut
    And kogen-build's loop knob tests pass with the "line-aligned" kogen-loop cut

  Scenario: The loop knob tests pin only the bound, the omission marker and the delivery
    Then kogen-build's loop knob tests fail with the "over-limit" kogen-loop cut
    And kogen-build's loop knob tests fail with the "unmarked" kogen-loop cut
    And kogen-build's loop knob tests pass with the "head-only" kogen-loop cut
