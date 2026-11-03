# Carry the read ledger across native re-opens

Slug: k-resetcarry
Model: not applicable — the ledger is a pure function of the input a reset replaces, and the prefill rule is a condition of the input being sent; nothing new is checkpointed and the option is off by default
Acceptance:
- .kogen/intents/k-resetcarry/acceptance.feature
- features/steps/k_resetcarry_steps.js
- features/steps/k_resetcarry_test.go.txt
- features/steps/k_resetcarry_controls/good.json
- features/steps/k_resetcarry_controls/bad.json

What the code does now (`build/context.go` `prepareContext`, `build/material.go` `prefillSource`, `build/review.go` `converse`). When a native request outgrows the 524288-byte window, Kogen archives the input. It then sends three things: the owner's mandatory opening messages, a `Local working-window reset` message, and the newest exchange (the last response's calls and their outputs). For a native review in the parts mode, every review request also gets a native pinned-source data message (`prefillMarker`). That message carries the longest run of unread pinned parts that keeps the request 32768 bytes under the ceiling. So after every reset, Kogen refills the window with part bodies to just under the ceiling. This re-prefill is what the S-LEVERS and S-HISTSUPER reviews ran into.

What the product does after this change. A new internal option, `Deps.ReadLedger bool` (`build/hooks.go`, beside `BuildRequestBudget`), is off by default: its zero value is off. It has no CLI flag and no `.kogen/project.yaml` key, and the checkpoint does not record it. `Build` and `Resume` read it from their own `Input.Deps` / `ResumeInput.Deps`.

With the option on:

1. A working-window reset of the native builder (owner `build`) or of the native reviewer (owner `review`) re-opens the conversation with exactly:
   - the owner's mandatory opening messages;
   - the reset message, with today's content rules;
   - one read-ledger message.

   The newest exchange is not carried. Everything else about the reset stays as it is:
   - the archive file and its history ref;
   - the observations (`unfinished`), `history`, `history_in_catalog` and `review_delivery` facts;
   - the `window_reset` event and the `kogen: <owner> local reset ...` progress line;
   - the `context_budget_exceeded` refusal when the reduced window does not fit.
2. For the review only:
   - Every output of the newest exchange that today's `deferrable` selects (a complete `read_file` read of a current pinned part that no earlier exchange or archived window delivered) is replaced in the archived input by today's not-delivered notice. Its part numbers go in the reset message's `deferred_reads`, as today's deferral does. A read that never reached the model is therefore never counted as delivered.
   - Today's "defer last first until it fits" loop is not used, because nothing of the newest exchange is carried.
   - `review_delivery` is computed with no kept exchange, so every image there has `in_window` false.
3. The read-ledger message follows kogen-loop's L-LEDGER continuation format. It is `{"role": "user", "content": <JSON text>}`, where the JSON text is the compact `encodeJSON` of `{"type": "read_ledger", "dropped": D, "entries": [...]}`; key order is not significant. Its entries come from the input the reset archives (after rule 2), in input order:
   - A `read_ledger` message in that input contributes its entries at its position.
   - Every other source of an entry is a `read_file` call whose output qualifies. The call is a `function_call` named `read_file` with a string `path` argument. A `function_call_output` with the same `call_id` follows it. That output is a string that does not begin with `error:`, `refused:` or `not delivered:`.
   - Such a call contributes `{"tool": "read_file", "path": <the argument as spelled>, "start_line": S, "end_line": E, "sha256": <lowercase hex SHA-256 of the output string>, "note": N}`.
   - S and E are the numbers of the first and last lines of the output that have the form `<digits>\t<text>`; both are 0 when no line has that form.
   - N is the `<text>` of the first such line whose text, trimmed of Unicode white space, is not empty. It is trimmed and cut to its first 80 Unicode scalars, with no suffix. N is `""` when there is no such line.
   - Repeated reads are separate entries.
   - D starts at the carried ledger's `dropped` value, or 0 without one. While the content exceeds 8192 UTF-8 bytes and entries remain, the oldest entry is removed and D increases by 1.
   - There are no `view_image` entries, and no output text beyond the notes.

   Example (acceptance): `fixtures/pad-1.txt` has 1000 lines, so its `read_file` result is cut to 16384 bytes. Its entry is `{"tool": "read_file", "path": "fixtures/pad-1.txt", "start_line": 1, "end_line": 1000, "sha256": <SHA-256 of that cut result>, "note": "pad-1 line 0001 of the read ledger fixture"}`.
4. `prefillSource` appends nothing to a review request whose input carries a working-window reset message: a `user` message whose string content begins with `Local working-window reset\n`. Before a review's first reset, the prefill is unchanged; the on_demand mode already appends nothing. The model re-reads parts on demand through its existing `read_file` tool, and every hash-bound part stays readable at its path.
5. In the parts mode, the reset's `review_delivery.note` replaces the sentence `Kogen also appends unread pinned parts as native source data messages; they count once a completed response follows them. ` with `After a working-window reset Kogen appends no pinned parts to this review: read the unread parts with read_file; the read_ledger message lists the reads this conversation made, and every pinned part can be read again. ` The following stay as they are: the rest of that note, the on_demand note, the reviewer instructions and the catalog guidance.
6. Each such reset logs one run event: `{"event": "read_ledger_reset", "owner": "build"|"review", "entries": N, "bytes": B, "dropped": D, "prefilled_bytes": P}`.
   - N is the number of entries the carried ledger has, and D its `dropped` value.
   - B is the UTF-8 byte count of the ledger message's content.
   - P is the total UTF-8 bytes of the content of the messages that the request's `Prefill` appended to that re-opened request. It is 0 when there are none, and by rule 4 there are none.
   - The event is logged after that reset's `window_reset` event and before that request's `request` event.

With the option off, behaviour is byte-identical to today: no ledger message, no new event, and the newest exchange, prefill and note exactly as now.

Unchanged in both modes:
- the delivery rule (`delivered`, `materialUnread`), approval and verdict validation, review independence and every custody check;
- the external (Claude Code) reviewer and builder;
- every limit, timeout and retry, and every CLI command, flag and configuration key;
- the `kogen-loop` pin in `go.mod`. Neither the pin nor kogen-loop itself is edited.

The acceptance harness uses these build package test helpers and evidence fields, which the Build keeps:
- `newEnv`, `env.write`, `env.run`, `env.approve`, `env.head`, `env.gitOut`, `env.build`, `env.onlyRun`, `env.events`, `env.runPath`, `env.root`, `env.prov`, `attempt.exit`, `attempt.shown`;
- `fakeProvider` with `then`, `calls`, `requests`, `reviewRequests` and `toolOutputs`;
- `useRealReviewer`, `writeApp`, `submit`, `requestInput`, `requestInstructions`, `pinnedOf`, `inWindow`, `prefilledParts`, `resetHandoff`, `eventsNamed`, `hashHex`, `prefillMarker`, `Input` with its embedded `Deps`;
- the events `window_reset` (`owner`), `review_source_prefill` and `review_material` (`parts`, `parts_read`), and the `verdict` of `review-<n>.json`.

## Decisions
- The option is internal and off by default, with no CLI and no project.yaml key until research's benchmark (source: request)
- With the option on, a native builder or review re-open continues from the read ledger (path, range, sha256, note) instead of re-prefilling pinned parts; the model re-reads on demand through its existing read tools (source: request)
- Every re-open is recorded as a run event with the ledger bytes carried and the bytes prefilled (source: request)

## Source comparison
- Why (S-LEVERS Build d0fe4001: review requests of 480–523 KB against a 524 KB window, two resets each re-prefilling about 490 KB, ended without a verdict after 14 requests, 181 parts and 1.08 MB; S-DECREL the same). This is the motivation; the cause is located in `prepareContext` plus `prefillSource`. The acceptance review-off scenario reproduces the re-prefill on a synthetic fixture: the re-open carries the newest exchange and a pinned-source data message with part bodies. No paid run was repeated.
- Item 1 (internal option, off by default, no CLI, no project.yaml key): addressed by `Deps.ReadLedger bool`. Scenarios 2 and 4 set it off; the harness sets it through the `Input`'s `Deps`.
- Item 2 (on: a re-open continues with the L-LEDGER read-ledger continuation instead of re-prefilling; the ledger lists path, range, sha256 and note; the model re-reads on demand; hash-bound parts stay readable): addressed by rules 1–5.
  - L-LEDGER's continuation never carries old conversation items, so the newest exchange is not carried (rule 1).
  - Its undelivered part reads are deferred (rule 2), so delivery custody is not weakened.
  - Prefill stops after a reset (rule 4), so no refill follows the re-open.
  - Scenario 1 re-reads part 1 after the reset and checks that the read equals the part's file text, whose SHA-256 is the catalog's.
- Item 3 (run events record each re-open with the ledger bytes carried and the bytes prefilled): addressed by the `read_ledger_reset` event (rule 6; scenarios 1 and 3).
- Must stay:
  - Off is byte-identical: scenarios 2 and 4, and the existing tests unchanged.
  - Review independence, hash-bound material and verdict validation are unchanged. Scenario 1 approves only after every part is delivered, with no invalid verdict.
  - No timeout, limit or retry is raised, and there is no CLI change.
- Acceptance:
  - "a fake provider and a forced window reset in a review, option on: the re-open carries the ledger and no part bodies, and the review still reaches a valid verdict after re-reading a part" is scenario 1.
  - "option off: today's re-prefill" is scenario 2.
  - "all configured checks pass" is the configured checks.
  - The builder half of item 2 is scenarios 3 and 4.
- Depends on: CONFLICTING. The request says kogen main vendors kogen-loop e5054c8 with L-LEDGER b9e36b4, and asks to use `loop.ReadLedger` and its `Continuation` from the pinned copy. This repository pins `github.com/KogenAI/kogen-loop v0.0.0-20261102055344-c6e2360290d4` (`go.mod` replace). That copy's `CHANGELOG.md` records "Remove the unused experimental levers: ... the read ledger (`loop.Round.ReadLedger`, `ReadLedger`, `NewReadLedger`, `Continuation`)". Its `conformance/queued_levers_test.go` requires those names to be undeclared. There is no `cli/third_party` in this repository.
  - The continuation is therefore implemented in package `build`, with L-LEDGER's message shape, entry fields, note rule, 8192-byte FIFO cap and reset event (from `.kogen/intents/l-ledger/intent.md` in the pinned module).
  - L-LEDGER's `view_image` entries are omitted: the request lists text fields only, and the review's images stay in `review_delivery.images`.
  - The pin is not changed: an older loop would drop later loop changes.
- Gate (research, after landing: S-LEVERS d0fe4001 re-review with 0 refills over 100 KB): out of this Build's acceptance. With the option on, rule 4 appends no pinned source after a reset. The Kogen program has no setting for the option (no CLI); a benchmark that wires `Deps.ReadLedger` in the kogen repository is a follow-up there.
- Priority (before K-REVOD-GATE): noted; no effect on scope.
- Host safety: the harness uses temp dirs and the in-process scripted provider. It starts and stops no processes by name or pattern and creates no synthetic CPU load.
- Repository split: built in kogen-build only (package `build`). Nothing in kogen, kogen-shaping, kogen-verification, kogen-coordinator, kogen-executor or kogen-loop changes. Earlier candidate diffs are not used.

## Tests
- With the option on, a review reset re-opens with the mandatory messages, the reset message and the ledger only. No prefill follows it, and the review reaches a valid approval after re-reading part 1: acceptance scenario 1 (scenario)
- With the option off, the review re-open keeps the newest exchange and re-prefills pinned parts: acceptance scenario 2 (scenario)
- With the option on, a builder reset re-opens from the ledger without the newest exchange; off keeps today's window: acceptance scenarios 3 and 4 (scenario)
- Ledger entries: the path as spelled, start/end lines from numbered lines (0/0 without any), the SHA-256 of the output, the note trimmed and cut to 80 Unicode scalars ("" without one), outputs beginning `error:`, `refused:` or `not delivered:` left out, and repeated reads kept: a new test that calls `prepareContext` with crafted over-budget inputs and the option on (unit)
- A second reset carries the earlier ledger's entries first; the 8192-byte cap evicts oldest entries below, at and above the bound, and `dropped` is cumulative: a new test with two successive `prepareContext` resets (unit)
- On a review reset with the option on, an undelivered complete part read in the newest exchange is deferred in the archive, listed in `deferred_reads` and still unread for `materialUnread`, while an earlier delivered part stays delivered: a new test over a pinned review (integration)
- With the option on, the reset's parts-mode delivery note states that no parts are appended after a reset; off keeps today's sentence: a new `reviewDelivery` test (unit)
- The `read_ledger_reset` event per reset, with owner, entries, bytes, dropped and prefilled_bytes, logged after `window_reset`: acceptance scenarios 1 and 3 (scenario)
- Off is byte-identical: the existing context, prefill, delivery, budget and review tests pass unchanged (existing)

## Feasibility
- These were scratch checks in this read-only external session, outside Kogen's probe records and outside the checkout.
  - The pinned kogen-loop has no read ledger (its CHANGELOG and `conformance/queued_levers_test.go`), so the contract implements it in package `build`.
  - On the unchanged tree the harness builds with `go test -c -vet=off -overlay` and runs all four modes in about 3 s each. Cucumber fails all 4 scenarios on missing-feature assertions (`option` is `"absent"`).
  - A scratch prototype of this contract in a copy of HEAD passed all 4 scenarios (12 steps) in about 21 s. With the prototype, `go test -count=1 ./build -run 'Prefill|Delivery|Budget|Reset|Window|Context|Material|ReviewInParts|Native'` passed (78 s); the full suite was not run on it.
  - Prototype request sizes stay well under the 524288-byte ceiling: review-on at most about 312 KB after the reset, the review-off re-open about 373 KB. Repeated runs gave identical observations.
- The observer controls take recorded real harness runs: good from that prototype, bad from the unchanged tree. The good control accepts them against the feature's expected values. The bad control rejects:
  - the unchanged tree, a failed run and a run that did not start;
  - a missing or doubled observation line, a harness test that did not pass, the other mode's observation and an altered fixture description;
  - the twin mode's run relabelled (an implementation that ignores the option);
  - every expected leaf altered or removed.

  A weakened observer that ignores `reopen` made the bad control fail. These are observer controls only, not product evidence.
- evidence: {"id":"reset-carry-observer","kind":"observer","inputs":["features/steps/k_resetcarry_controls/good.json","features/steps/k_resetcarry_controls/bad.json","features/steps/k_resetcarry_test.go.txt",".kogen/intents/k-resetcarry/acceptance.feature"],"observers":["features/steps/k_resetcarry_steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k_resetcarry_steps.js --control good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k_resetcarry_steps.js --control bad","exit":1,"output":"rejected\n"}]}
