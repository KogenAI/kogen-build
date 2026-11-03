Feature: The check baseline records its wall time and names itself in a stop

  Scenario: The baseline record carries each check's start, end and wall time
    # Checks, in configured order: slow = "sleep 1; echo SLOW-MARKER" (exit 0), fast = "echo FAST-MARKER; exit 3".
    # Each baseline command_result and the baseline_checks event (with its ordered timings) carry
    # started_at, ended_at and wall_ms; checks 2 and failing [fast] are unchanged; result.json names no baseline stop.
    Then a Build whose baseline runs check "slow" then failing check "fast" records each check's start, end and wall time, with "slow" at least 1000 ms

  Scenario: A Build that ends while a baseline check runs names the baseline
    # The outcome event and result.json carry stopped_during "baseline" and baseline_check {name, command, started_at, stopped_at, wall_ms};
    # stdout keeps "stopped fixture: interrupted (Build <id>)" and adds
    # "  stopped during the check baseline: check slow (<command>) ran <wall_ms> ms (started <started_at>)"; exit stays 143.
    Then a Build interrupted by SIGTERM at least 400 ms into baseline check "slow" names the baseline and that check in its stop record
    And a Build interrupted by SIGTERM during setup names no baseline in its stop record
