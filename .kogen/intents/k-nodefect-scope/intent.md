# Scope the no-defect review filter to on_demand

Slug: k-nodefect-scope
Model: not applicable — one stateless validation rule of a single submit_review call, keyed on a configuration value and the finding's text
Acceptance:
- .kogen/intents/k-nodefect-scope/acceptance.feature
- features/steps/k_nodefect_scope_steps.js
- features/steps/k_nodefect_scope/harness.js
- features/steps/k_nodefect_scope/review_test.go.txt

K-REVOD-GATE (Build f4f97201 review) added `reviewNoDefectRe` / `reviewNoDefect` in `build/review.go`. Right now `(*reviewRun).validate` applies it in every review mode, to the whole text of every blocking finding. A concrete finding that only quotes such output, for example `The scanner prints 'no defects found' for broken input`, comes back as invalid feedback (`a blocking finding that asserts no product defect (such as inspection not completed) is not a verdict on the candidate; ...`). A Build whose reviewer keeps it ends `review_invalid` instead of `review_blocked`. The filter was meant only for the on_demand review gate: a blocking finding whose only claim is that inspection was incomplete.

After this change:

1. **Scope by mode.** `validate` applies the no-defect check only when the project's `review_material` mode is `on_demand` (`b.onDemand()`, i.e. `Config.ReviewMaterial.Mode == "on_demand"`). The key is the configured mode only. That holds for the native and the external reviewer alike, as both use `validate` today. In `parts` mode, and with no configuration (which means parts), no finding is checked for a no-defect claim. A blocking finding there is handled exactly as before K-REVOD-GATE: a `changes` verdict with it is a valid verdict, and the Build stops `review_blocked` with the branch unchanged.
2. **Judge the finding's own claim (on_demand).** A no-defect phrase counts only when it appears outside quoted text in the finding's message. Quoted text is any span enclosed by a matching pair of:
   - straight double quotes `"…"`
   - curly double quotes `“…”`
   - curly single quotes `‘…’`
   - backticks `` `…` ``
   - straight single quotes `'…'`, where the opening `'` is at the start of the message or follows a character that is not a letter or digit, and the closing `'` is the next `'` that is followed by the end of the message or by a character that is not a letter or digit. So apostrophes such as `scanner's` or `doesn't` neither open nor close a quote.

   A quote character with no closing partner encloses nothing. The existing `reviewNoDefectRe` pattern is unchanged and is matched against the message with its quoted spans removed. It is neither broadened nor narrowed. So, in on_demand mode:
   - `The scanner prints 'no defects found' for broken input: ...` is a blocker: the phrase is only quoted.
   - `The required inspection of the complete pinned candidate source was not completed... no product defect is asserted by this finding.` is still invalid feedback, with the unchanged problem text, and never becomes `review_blocked`.
   - `No defect was found, but the inspection is not complete.` is still invalid feedback.
   - A message with the phrase both quoted and unquoted is invalid, because the unquoted occurrence counts.
3. **Unchanged.**
   - The protected-path exemption: a blocker at a path in `b.Baseline` is never refused by this check.
   - The invalid-feedback problem text and the rejected-verdict allowance and retry flow.
   - Notes: they may say anything.
   - The on_demand unread-hunk gate and all parts-mode review requests (prompts, catalog, tools).
   - The CLI. No limit, timeout or retry is raised.

Update the doc comments of `validate` and `reviewNoDefect` to say the check is on_demand-only and ignores quoted text. Existing tests that assert the parts-mode refusal are changed to the parts behavior above: `TestKRevodGateInvalidBlocker/validation` currently runs its table with no configuration (parts) and says "The rule holds in the default parts mode as well". Its parts-mode refusal is removed.

Acceptance runs real Builds through the build package's own test environment. `features/steps/k_nodefect_scope/harness.js` adds `features/steps/k_nodefect_scope/review_test.go.txt` to package `build` as `build/zz_k_nodefect_scope_acceptance_test.go`, using `go test -overlay` from a temporary folder. It runs `go test -overlay <tmp>/overlay.json -count=1 -v -run '^TestKNodefectScopeAcceptance$' ./build` from the repository root, with `KNODEFECT_SCOPE_MODE`, `KNODEFECT_SCOPE_MESSAGE` and `KNODEFECT_SCOPE_OUT` set. That test records the Build's outcome, review verdicts, invalid problems, reviewer feedback and whether HEAD moved, and the steps judge that record.

The Build must not create `build/zz_k_nodefect_scope_acceptance_test.go`. It must keep these test helpers of package `build` with their current signatures:
- `newEnv`
- the `env` methods `write`, `read`, `run`, `approve`, `head`, `reviewed`, `onlyRun`, `events`, `result` and the `prov` field
- the `fakeProvider` methods `then`, `calls`, `toolOutputs`
- `submit`, `finding`, `eventsNamed`, `jsonText`
- the `attempt` fields `exit` and method `shown`

## Decisions
- In on_demand review material mode, a blocking finding that names a concrete defect stays blocking even when it quotes text such as "no defects found" (source: request)
- A blocker that asserts no product defect (for example "inspection not completed") is still invalid feedback in on_demand mode and never becomes review_blocked (source: request)
- In parts mode no no-defect check applies; findings containing the phrase are handled as before K-REVOD-GATE (source: request)
- No CLI change; no limit, timeout or retry raised (source: request)

## Source comparison
- addressed: apply the no-defect check only in on-demand review material mode; source request "What to change"; contract prose rule 1, scenarios "In parts mode, …" (two), Tests lines 2 and 3.
- addressed: judge the finding's own claim, not a quoted phrase; the request's example "the scanner prints 'no defects found' for broken input"; source request "Why"/"What to change"/"Acceptance"; contract prose rule 2, scenarios "On demand, a concrete defect that quotes …" (single- and double-quoted), Tests lines 1 and 5.
- addressed: "inspection not completed" blocker still invalid feedback in on-demand mode, never review_blocked; source request "Must stay"/"Acceptance"; scenarios "On demand, an inspection-not-completed blocker …" and "On demand, an unquoted no-defect claim …", Tests line 4.
- addressed: parts mode reviews unchanged; source request "Must stay"; prose rule 3, Tests line 6 (existing TestKRevodGatePartsUnchanged).
- addressed: no CLI change, no limit/timeout/retry raised; source request "Must stay"; prose rule 3.
- addressed: all configured checks pass (format, compile, vet, go-test); source request "Acceptance"; Tests and Build verification.
- superseded: existing assertion that the no-defect rule "holds in the default parts mode as well" (build/k_revod_gate_test.go validation subtest); replaced per request; Tests line 3.
- addressed: repository split; the change lives in kogen-build package `build` (`build/review.go`, `build/k_revod_gate_test.go`); nothing in another repository is changed; source request "Repository split".
- addressed: host safety (temp dirs only, no process stopping by name, no synthetic load); the harness uses only a mkdtemp folder it removes and `go test`'s own temp dirs; source request "Host safety".

## Tests
- on_demand: a blocking finding whose no-defect phrase appears only inside quoted text stays a blocker: in `TestKRevodGateInvalidBlocker/validation` (build/k_revod_gate_test.go) run the table with `Config.ReviewMaterial.Mode = "on_demand"` and add valid cases quoting with `'…'` ("The scanner prints 'no defects found' for broken input: …"), `"…"`, `“…”`, `‘…’` and backticks, plus an apostrophe case ("The scanner's 'no defects found' line hides the parse error in app.txt.") (unit)
- parts mode applies no no-defect check: in the same subtest, with `Mode = "parts"` (and with no Config), every table message including the five current no-defect messages yields a `changes` verdict; remove the "rule holds in the default parts mode" expectation (unit)
- on_demand keeps refusing unquoted no-defect blockers and the protected-fixture stop stays a blocker: the existing table cases and fixtureStop, now run in on_demand mode, plus a mixed case ("Inspection incomplete; not a product defect. The tool prints 'no defects found'.") that stays invalid (unit)
- on_demand inspection-not-completed blocker ends review_invalid, never review_blocked: existing `TestKRevodGateInvalidBlocker/historical blocker ends review_invalid` (existing)
- the reported failure, a real on_demand Build whose only blocker quotes 'no defects found' while naming a concrete defect, stops review_blocked: add a subtest to `TestKRevodGateInvalidBlocker` using `revodGateLong` that reads the required parts and submits that finding, expecting exit 1, outcome review_blocked, one `changes` review event and unchanged HEAD (regression)
- parts mode review requests unchanged: `TestKRevodGatePartsUnchanged` with its golden unchanged (existing)
- end to end through real Builds in both modes: the six acceptance scenarios (scenario)

## Feasibility
- The overlay harness runs a real Build on the unchanged tree. A quoted "no defects found" blocker ends review_invalid in on_demand and in parts mode: the reported fault. A plain concrete blocker ends review_blocked. On the unchanged tree the acceptance gave 4 failed scenarios (the new behavior) and 2 passed (the preserved invalid-feedback behavior). A scratch patch (mode check plus quote stripping) made all 6 pass. With that patch, the existing parts-mode `validation` subtest fails as this Intent expects and must be updated.
- evidence: {"id":"harness-blocked","kind":"observer","inputs":["features/steps/k_nodefect_scope/harness.js","features/steps/k_nodefect_scope/review_test.go.txt"],"observers":["features/steps/k_nodefect_scope/harness.js","features/steps/k_nodefect_scope/review_test.go.txt"],"checks":[{"name":"good","command":"node features/steps/k_nodefect_scope/harness.js on_demand blocked 'Inspection of app.txt shows the parser drops the last record: a product defect.'","exit":0,"output":"ok: on_demand blocked: outcome review_blocked; review verdicts changes\n"},{"name":"bad","command":"node features/steps/k_nodefect_scope/harness.js on_demand blocked 'Inspection incomplete; this is not a product defect.'","exit":1,"output":"FAILED: on_demand blocked: outcome review_invalid, want review_blocked; review verdicts invalid,invalid, want changes; the reviewer was told the finding asserts no product defect\n"}]}
- evidence: {"id":"harness-invalid","kind":"observer","inputs":["features/steps/k_nodefect_scope/harness.js","features/steps/k_nodefect_scope/review_test.go.txt"],"observers":["features/steps/k_nodefect_scope/harness.js","features/steps/k_nodefect_scope/review_test.go.txt"],"checks":[{"name":"good","command":"node features/steps/k_nodefect_scope/harness.js on_demand invalid 'The required inspection of the complete pinned candidate source was not completed... no product defect is asserted by this finding.'","exit":0,"output":"ok: on_demand invalid: outcome review_invalid; review verdicts invalid,invalid\n"},{"name":"bad","command":"node features/steps/k_nodefect_scope/harness.js on_demand invalid 'Inspection of app.txt shows the parser drops the last record: a product defect.'","exit":1,"output":"FAILED: on_demand invalid: outcome review_blocked, want review_invalid; review verdicts changes, want only invalid; invalid problems [], want each to be the no-defect refusal\n"}]}
