Feature: Show the latest completed Build tool without output
  Scenario: Latest completion replaces the earlier summary
    When k-build-progress-latest-tool: I run a fake builder with "two tools"
    Then k-build-progress-latest-tool: progress reports "read_file" then "shell" with statuses "ok" and "failed" and path "progress-input.txt"

  Scenario: Tool result content stays out of the console
    When k-build-progress-latest-tool: I run a fake builder with "two tools"
    Then k-build-progress-latest-tool: console output excludes "TOOL_RESULT_PRIVATE_7081"

  Scenario: No tool events preserve the existing progress
    When k-build-progress-latest-tool: I run a fake builder with "none"
    Then k-build-progress-latest-tool: progress remains "kogen: building (round 1)" without tool events
