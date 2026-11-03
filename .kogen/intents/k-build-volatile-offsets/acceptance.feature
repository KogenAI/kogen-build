Feature: kogen-build runs against kogen-loop main and its request-budget golden masks timing and host-load values

  Scenario: kogen-build requires kogen-loop main a5aa5f4
    Then go.mod replaces "kogen-loop" with "github.com/KogenAI/kogen-loop" at "v0.0.0-20261103193244-a5aa5f4ee775"
    And go.sum records "github.com/KogenAI/kogen-loop v0.0.0-20261103193244-a5aa5f4ee775 h1:0NcEoKZu/6sYsUyUUKGRm1vtRU9rHD9TDbJHavp8Go8=" and no line for "c6e2360290d4"

  Scenario: Both broken tests pass three times in a row against kogen-loop main
    # Measured on the unchanged tree with kogen-loop main: TestBuildRequestBudgetOff fails at log line 3
    # ("load_average":[22.31,16.71,10.5] against [22.76,16.89,10.6]) and TestAnArchivedWindowThatChangedIsRefused
    # fails with "no archived window" (its 36 shell results of 12302 bytes stay under the 524288-byte window).
    Then go.mod replaces "kogen-loop" with "github.com/KogenAI/kogen-loop" at "v0.0.0-20261103193244-a5aa5f4ee775"
    And "TestBuildRequestBudgetOff" and "TestAnArchivedWindowThatChangedIsRefused" each pass 3 times in a row in ./build

  Scenario: A Build's provider attempt record carries the request offset and the host load
    Then the first provider attempt record of a Build carries "request_offset_ms" and "load_average"

  Scenario: The golden comparison masks timing and host-load values and compares everything else
    # Each row rewrites one value of the first provider_attempt event's "provider" record in the run's
    # log.jsonl (row "event at": the event's own "at") and compares observeBuild of the rewritten run
    # with observeBuild of the recorded run. "new timing keys" compares two rewritten runs that add
    # spawn_offset_ms, queue_ms and ended_at with different values; "new non-timing key" two that add
    # attempts_seen 1 and 2.
    Then rewriting one value of a Build's first provider attempt record gives these observeBuild comparisons:
      | rewrite             | comparison   |
      | request_offset_ms   | equal        |
      | load_average        | equal        |
      | load_average null   | equal        |
      | every _ms number    | equal        |
      | timestamp           | equal        |
      | provider_request_id | differs: log |
      | pass                | differs: log |
      | outcome             | differs: log |
      | stream_bytes        | differs: log |
      | event at            | equal        |
      | new timing keys     | equal        |
      | new non-timing key  | differs: log |
