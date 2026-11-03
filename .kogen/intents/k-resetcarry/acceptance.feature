Feature: A native conversation re-opened after a working-window reset continues from the read ledger

  The harness features/steps/k_resetcarry_test.go.txt runs a whole Build in a disposable fixture
  repository against the build package's scripted provider (fakeProvider) as the native builder and
  the native reviewer: no process, model or account. It sets the internal option Deps.ReadLedger
  on or off. The fixture repository starts with README.md, app.txt and the project configuration,
  then commits fixtures/pad-1.txt, pad-2.txt and pad-3.txt: 1000 lines each, line i of pad-n being
  "pad-n line iiii of the read ledger fixture", so each read_file result is cut to 16384 bytes.
  In the review modes the builder writes a 13000-line app.txt, so the review source is pinned in
  parts and Kogen prefills the first review request; the reviewer's first response reads the three
  pad files, which forces the reset. After it the reviewer reads part 1 again, then every part it
  does not know was delivered, up to eight per response, and approves. "The re-open" is the first
  request of the conversation that carries the working-window reset message.

  Scenario: With the read ledger on, the review re-opens from the ledger without part bodies and still reaches a valid verdict
    Given the reset-carry fixture "review-on": the read ledger on, the reviewer's first response reads fixtures/pad-1.txt, pad-2.txt and pad-3.txt, and after the reset it reads part 1 again
    When the reset-carry harness builds the fixture
    Then the reset-carry observations include:
      """
      {"exit": 0, "commits": "1", "option": "on", "verdict": "approve", "invalid_verdicts": 0, "all_parts_read": true,
       "resets": 1, "prefills_before_reset": 1, "prefills_after_reset": 0,
       "reopen": {"ledger_messages": 1, "ledger_type": "read_ledger", "ledger_dropped": 0,
         "ledger_entries": [
           {"tool": "read_file", "path": "fixtures/pad-1.txt", "start_line": 1, "end_line": 1000, "sha256": "<sha256 of the read output>", "note": "pad-1 line 0001 of the read ledger fixture"},
           {"tool": "read_file", "path": "fixtures/pad-2.txt", "start_line": 1, "end_line": 1000, "sha256": "<sha256 of the read output>", "note": "pad-2 line 0001 of the read ledger fixture"},
           {"tool": "read_file", "path": "fixtures/pad-3.txt", "start_line": 1, "end_line": 1000, "sha256": "<sha256 of the read output>", "note": "pad-3 line 0001 of the read ledger fixture"}],
         "carries_exchange": false, "carries_part_bodies": false, "prefill_messages": 0},
       "reread_part_1": {"read_after_reset": true, "hash_bound": true},
       "read_ledger_reset_events": [{"event": "read_ledger_reset", "owner": "review", "entries": 3, "bytes": "<bytes of the carried ledger>", "dropped": 0, "prefilled_bytes": 0}]}
      """

  Scenario: With the read ledger off, the review re-open keeps the newest exchange and re-prefills pinned parts as today
    Given the reset-carry fixture "review-off": the read ledger off, the reviewer's first response reads fixtures/pad-1.txt, pad-2.txt and pad-3.txt, and after the reset it reads part 1 again
    When the reset-carry harness builds the fixture
    Then the reset-carry observations include:
      """
      {"exit": 0, "commits": "1", "option": "off", "verdict": "approve", "invalid_verdicts": 0, "all_parts_read": true,
       "resets": 1, "prefills_before_reset": 1, "prefills_after_reset": 1,
       "reopen": {"ledger_messages": 0, "carries_exchange": true, "carries_part_bodies": true, "prefill_messages": 1},
       "reread_part_1": {"read_after_reset": true, "hash_bound": true},
       "read_ledger_reset_events": []}
      """

  Scenario: With the read ledger on, the builder re-opens from the ledger instead of its newest exchange
    Given the reset-carry fixture "build-on": the read ledger on, the builder reads fixtures/pad-1.txt, pad-2.txt and pad-3.txt and writes two 300000-byte scratch files over two responses
    When the reset-carry harness builds the fixture
    Then the reset-carry observations include:
      """
      {"exit": 0, "commits": "1", "option": "on", "resets": 1, "prefills_before_reset": 0, "prefills_after_reset": 0,
       "reopen": {"ledger_messages": 1, "ledger_type": "read_ledger", "ledger_dropped": 0,
         "ledger_entries": [
           {"tool": "read_file", "path": "fixtures/pad-1.txt", "start_line": 1, "end_line": 1000, "sha256": "<sha256 of the read output>", "note": "pad-1 line 0001 of the read ledger fixture"},
           {"tool": "read_file", "path": "fixtures/pad-2.txt", "start_line": 1, "end_line": 1000, "sha256": "<sha256 of the read output>", "note": "pad-2 line 0001 of the read ledger fixture"},
           {"tool": "read_file", "path": "fixtures/pad-3.txt", "start_line": 1, "end_line": 1000, "sha256": "<sha256 of the read output>", "note": "pad-3 line 0001 of the read ledger fixture"}],
         "carries_exchange": false, "carries_part_bodies": false, "prefill_messages": 0},
       "read_ledger_reset_events": [{"event": "read_ledger_reset", "owner": "build", "entries": 3, "bytes": "<bytes of the carried ledger>", "dropped": 0, "prefilled_bytes": 0}]}
      """

  Scenario: With the read ledger off, the builder re-open keeps its newest exchange as today
    Given the reset-carry fixture "build-off": the read ledger off, the builder reads fixtures/pad-1.txt, pad-2.txt and pad-3.txt and writes two 300000-byte scratch files over two responses
    When the reset-carry harness builds the fixture
    Then the reset-carry observations include:
      """
      {"exit": 0, "commits": "1", "option": "off", "resets": 1, "prefills_before_reset": 0, "prefills_after_reset": 0,
       "reopen": {"ledger_messages": 0, "carries_exchange": true, "carries_part_bodies": false, "prefill_messages": 0},
       "read_ledger_reset_events": []}
      """
