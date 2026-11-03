Feature: Each configured check runs once per tree and environment within a Build

  # Every mode runs whole Builds through Build and Resume (the package's scripted provider and stub
  # reviewer) in a temporary repository. Its checks append their names to an execution log outside the
  # repository; "executions" is that log in order. "runs" are the Build's runs in order (the first run, then
  # its resume or the next Build), each with its result.json outcome, the names of its check_reuse events
  # ("reused"), whether every such event has a 64-hex key and the message "baseline reused: <check> <key>",
  # and the last pass its checkpoint holds (checks in configured order as [name, status]).
  # "commits" counts the commits added to the base.

  Scenario: The baseline is reused on an unchanged tree and run again on a changed one
    # Check lint: echo lint >> "$KBO_LOG". In mode tree the builder writes app.txt before it finishes;
    # in mode unchanged it finishes without an edit, so pass 1 verifies the tree the baseline ran on.
    Then the baseline-once harness in mode "tree" observes:
      """
      {
        "mode": "tree",
        "executions": ["lint", "lint"],
        "commits": "1",
        "runs": [
          {"last_pass": {"checks": [["lint", 0]], "failing": [], "green": true, "n": 1, "not_run": []}, "outcome": "landed", "reuse_records_well_formed": true, "reused": []}
        ]
      }
      """
    And the baseline-once harness in mode "unchanged" observes:
      """
      {
        "mode": "unchanged",
        "executions": ["lint"],
        "commits": "1",
        "runs": [
          {"last_pass": {"checks": [["lint", 0]], "failing": [], "green": true, "n": 1, "not_run": []}, "outcome": "landed", "reuse_records_well_formed": true, "reused": ["lint"]}
        ]
      }
      """

  Scenario: A toolchain change between two check points runs the check again on the same tree
    # Checks lint: kbo-lint (a script in a folder on the checks' PATH) and gate: echo gate >> "$KBO_LOG";
    # test -f "$KBO_OPEN". The builder writes app.txt and finishes; pass 1 fails gate; the Build is interrupted
    # at its next builder request. The gate is opened and kbo-lint is rewritten; the resumed builder finishes
    # without an edit, so pass 2 verifies pass 1's tree with another toolchain: gate runs first, then lint.
    Then the baseline-once harness in mode "toolchain" observes:
      """
      {
        "mode": "toolchain",
        "executions": ["lint", "gate", "lint", "gate", "gate", "lint"],
        "commits": "1",
        "runs": [
          {"last_pass": {"checks": [["lint", 0], ["gate", 1]], "failing": ["gate"], "green": false, "n": 1, "not_run": []}, "outcome": "interrupted", "reuse_records_well_formed": true, "reused": []},
          {"last_pass": {"checks": [["lint", 0], ["gate", 0]], "failing": [], "green": true, "n": 2, "not_run": []}, "outcome": "landed", "reuse_records_well_formed": true, "reused": []}
        ]
      }
      """

  Scenario: A command change between two check points runs the check again on the same tree
    # As the toolchain mode, but kbo-lint is unchanged and the interrupted checkpoint's recorded result of lint
    # names the command "kbo-lint --previous", as if lint's command had changed since it passed.
    Then the baseline-once harness in mode "command" observes:
      """
      {
        "mode": "command",
        "executions": ["lint", "gate", "lint", "gate", "gate", "lint"],
        "commits": "1",
        "runs": [
          {"last_pass": {"checks": [["lint", 0], ["gate", 1]], "failing": ["gate"], "green": false, "n": 1, "not_run": []}, "outcome": "interrupted", "reuse_records_well_formed": true, "reused": []},
          {"last_pass": {"checks": [["lint", 0], ["gate", 0]], "failing": [], "green": true, "n": 2, "not_run": []}, "outcome": "landed", "reuse_records_well_formed": true, "reused": []}
        ]
      }
      """

  Scenario: A resumed Build reuses the passed result its first run recorded and runs the failed check again
    # As the toolchain mode, with nothing changed but the opened gate.
    Then the baseline-once harness in mode "resume" observes:
      """
      {
        "mode": "resume",
        "executions": ["lint", "gate", "lint", "gate", "gate"],
        "commits": "1",
        "runs": [
          {"last_pass": {"checks": [["lint", 0], ["gate", 1]], "failing": ["gate"], "green": false, "n": 1, "not_run": []}, "outcome": "interrupted", "reuse_records_well_formed": true, "reused": []},
          {"last_pass": {"checks": [["lint", 0], ["gate", 0]], "failing": [], "green": true, "n": 2, "not_run": []}, "outcome": "landed", "reuse_records_well_formed": true, "reused": ["lint"]}
        ]
      }
      """

  Scenario: A fix loop runs the failing check first and the rest of the suite once on the final tree
    # Checks first: echo first >> "$KBO_LOG"; second: echo second >> "$KBO_LOG"; grep -qx fixed app.txt.
    # Round 1 writes app.txt "half" (pass 1, in configured order, fails second); round 2 writes "fixed":
    # pass 2 runs second (the check pass 1 failed) first, then first, once each.
    Then the baseline-once harness in mode "fix-loop" observes:
      """
      {
        "mode": "fix-loop",
        "executions": ["first", "second", "first", "second", "second", "first"],
        "commits": "1",
        "runs": [
          {"last_pass": {"checks": [["first", 0], ["second", 0]], "failing": [], "green": true, "n": 2, "not_run": []}, "outcome": "landed", "reuse_records_well_formed": true, "reused": []}
        ]
      }
      """

  Scenario: A Build whose full suite fails after its targeted check passes does not land
    # Checks first: echo first >> "$KBO_LOG"; ! grep -qx broken app.txt; second: echo second >> "$KBO_LOG";
    # grep -qx broken app.txt. Round 1 writes "half" (pass 1 fails second); round 2 writes "broken": pass 2
    # runs second first (it passes), then first (it fails). The provider refuses the next builder request.
    Then the baseline-once harness in mode "final-fails" observes:
      """
      {
        "mode": "final-fails",
        "executions": ["first", "second", "first", "second", "second", "first"],
        "commits": "0",
        "runs": [
          {"last_pass": {"checks": [["first", 1], ["second", 0]], "failing": ["first"], "green": false, "n": 2, "not_run": []}, "outcome": "refused", "reuse_records_well_formed": true, "reused": []}
        ]
      }
      """

  Scenario: The per-base baseline reuse across Builds stays as it is beside the reuse within a Build
    # Checks closed: ":" and lint: echo lint >> "$KBO_LOG". The provider refuses the first Build's first
    # request, so it stops after its baseline on the unchanged base. A second Build on the same base reuses
    # closed through the existing per-base evidence (one baseline_reuse event naming the first Build's
    # log.jsonl and a 64-hex identity, no closed command before the change), runs lint in its own baseline,
    # and its pass 1 reuses lint from that baseline.
    Then the baseline-once harness in mode "per-base" observes:
      """
      {
        "mode": "per-base",
        "executions": ["lint", "lint"],
        "commits": "1",
        "second_build_baseline_reuse": [{"evidence_is_first_builds_log": true, "identity_well_formed": true, "name": "closed"}],
        "second_build_closed_runs_before_change": 0,
        "runs": [
          {"last_pass": null, "outcome": "refused", "reuse_records_well_formed": true, "reused": []},
          {"last_pass": {"checks": [["closed", 0], ["lint", 0]], "failing": [], "green": true, "n": 1, "not_run": []}, "outcome": "landed", "reuse_records_well_formed": true, "reused": ["lint"]}
        ]
      }
      """
