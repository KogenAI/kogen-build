Feature: Cut sizes are recorded for every cut tool result, wherever the marker sits
  This feature checks that the record of a delivered tool result that carries the omitted-bytes marker has bytes_before_cut and bytes_delivered, and that results without a usable marker have neither.

  Scenario: A marker at byte 15 is recorded
    Given build-cut-fields-follow-the-marker: the kogen-build checkout
    When build-cut-fields-follow-the-marker: the build test "TestCutFieldsAcc1MarkerAtByte15" runs
    Then build-cut-fields-follow-the-marker: the 9000 byte result with the marker at byte 15 and 31000 omitted bytes is recorded with bytes_before_cut 9000 minus the marker length plus 31000 and bytes_delivered 9000

  Scenario: A marker in the former window is recorded as before
    Given build-cut-fields-follow-the-marker: the kogen-build checkout
    When build-cut-fields-follow-the-marker: the build test "TestCutFieldsAcc2MarkerInWindow" runs
    Then build-cut-fields-follow-the-marker: the result with the marker at byte 4050 and the cut of 40960 bytes are recorded as before

  Scenario: Results without a usable marker carry no cut fields
    Given build-cut-fields-follow-the-marker: the kogen-build checkout
    When build-cut-fields-follow-the-marker: the build test "TestCutFieldsAcc3NoCutFields" runs
    Then build-cut-fields-follow-the-marker: the unmarked result, the 16385 byte result and the result with a marker number that is not whole carry neither field

  Scenario: The comment no longer claims a 4096 byte head
    Given build-cut-fields-follow-the-marker: the kogen-build checkout
    When build-cut-fields-follow-the-marker: the comment above cutFields is read
    Then build-cut-fields-follow-the-marker: the comment is not empty and does not mention 4096
