# Review wide changes' parts without full texts

Slug: k-review-cost-wide
Model: not applicable — only the bytes one external review in parts carries change; its part sequence, custody and approval rules stay as they are
Acceptance:
- .kogen/intents/k-review-cost-wide/acceptance.feature
- features/steps/k_review_cost_wide_steps.js
- features/steps/k_review_cost_wide_test.go.txt
- features/steps/k_review_cost_wide_controls/good.json
- features/steps/k_review_cost_wide_controls/bad.json

An external review (Claude Code) whose complete one-session prompt exceeds the inclusive 524288-byte ceiling is reviewed in ordered parts. What the code does now (`build/review_external.go` `reviewInParts`, `build/review_parts.go`): every part is already its own fresh session that is sent the complete authority and one consecutive slice of the review source, each slice once; there is no shared conversation that re-sends earlier parts. The cost comes from what that source carries: for every changed file both its diff and its complete text, so a wide change that touches large wire and golden JSON fixtures carries them two or three times, and the per-part authority is repeated over many more parts than the hunks need. For K-MODELS-FILE the full text of the 80 changed files was about 1.97 MB of a 2.99 MB source.

What the product does after this change: on the parts route only, the review source that Kogen retains, hashes, cuts and sends is the parts source: today's source with the full text of every changed text file left out. Every hunk stays: the diff of every changed or deleted path is carried exactly as now, and an added text file keeps its full text (it is its own hunk). A reviewer reads a changed file's full text in the checkout, which holds the candidate; the catalog of every changed path with its Git object and SHA-256 stays in every part's header. Fewer and smaller parts follow, so the bytes sent grow with the hunks to read. Each part is still read once by its own fresh session.

The parts source is exactly today's source with only its third section changed:

```
# Check results of the final pass

<check results>

# Diff of existing/deleted paths against start HEAD <short start>

<diff, unchanged>
# Full text of every added file

Newly added files appear here once, with their mode, instead of being repeated in the diff. A changed file's hunks are in the diff above and its full text is in the checkout, so only its heading is listed here.

<one entry per changed path, in today's order, joined by "\n">
```

Each entry, per path:
- A changed text file becomes the single line `--- <path> (changed; full text in the checkout)`.
- An added text file stays `--- <path> (added; mode <mode>)` followed by its full text, as now.
- A deleted path stays `--- <path> (deleted)`.
- A binary file stays `<heading> (binary)`.
- A gitlink stays `<heading>` followed by `Subproject commit <id>`.

The parts source is made valid UTF-8 the same way as today's source.

Rules of the parts route:
- Routing is unchanged. It is still decided by the complete one-session prompt of today's full source against the ceiling. That size is what the progress line `the complete review prompt would be <n> bytes` reports. A prompt that fits is still reviewed in one session with today's full source and today's task text, full texts included.
- `review-<n>-source.txt`, the `SHA-256` and byte range in every part header, the `review_parts` event's `source_bytes` and every part's `from`/`to` refer to the parts source. The parts concatenated are exactly that source.
- The parts source is cut by the existing rules: each complete part prompt fits the ceiling inclusively, a path's material that fits one part is not split, and parts end after a newline or at a character boundary. The parter locates each path's material in the parts source: the diff block of every changed or deleted path and the text of every added file. The result may be a single part.
- The progress line is `kogen: reviewing (review <n>) in <k> parts: the complete review prompt would be <w> bytes, over the 524288-byte ceiling`, with `in 1 part:` when there is one part.
- The first paragraph of the parts task (`reviewPartsTask`) becomes exactly: `The candidate's source does not fit one review prompt, so it is reviewed in ordered parts, each by its own independent session. This message carries the complete authority and one part: the check results, every changed path and one consecutive piece of the exact review source, nothing clipped; the other pieces go to the other parts' sessions. The review source carries the diff of every changed or deleted path and the full text of every added file; a changed file's full text is not repeated there: read it in the checkout. Review against the approved Intent with this part as your focus and read any checkout file you need. Reviewers of earlier parts may have made verified repairs, so the checkout holds the current candidate.` Its tail is unchanged.
- The following stay as they are:
  - custody of the authority, the source and every part file (missing, changed or unsafe stops with `context_custody_invalid`);
  - approval only after the part was delivered in a completed round of its own session;
  - a blocking part stops the Build;
  - a verdictless part restarts the review from part 1;
  - a repair is verified before the next part;
  - the mandatory-room stop;
  - `writeReviewMaterial`'s files and modes;
  - usage accounting.

Out of scope and unchanged:
- the native reviewer (pinned parts, `review_material` on_demand) and `materialPartBytes`;
- the one-session external review;
- the 524288-byte ceiling and every other limit;
- all CLI commands, flags and configuration keys.

Generated fixtures are not summarised or dropped: their hunks are carried in full in the diff, like every other hunk.

The acceptance harness uses these build package test helpers and evidence fields, which the Build keeps:
- `newEnv`, `env.useLineup`, `env.write`, `env.run`, `env.approve`, `env.head`, `env.build`, `env.onlyRun`, `env.runPath`, `env.events`, `env.commitsSince`, `attempt.shown`;
- `newLineupExec` and its `build`, `review` and `starts`;
- `withLineup`, `writeFile`, `verdictJSON`, `eventsNamed`;
- the `review-<n>-material.json` fields `delivered` and `review_parts[].file`/`from`/`to`/`prompt_bytes`;
- the `review_parts` event fields `parts` and `source_bytes`.

## Decisions
- How the review is read once (the request leaves the choice between a fresh context per part with a running digest and on-demand fetching to the Build): each part keeps its own fresh session, with no running digest, and the reviewer fetches a changed file's full text on demand from the checkout instead of having it in the prompt (source: request)
- Large generated fixtures are not summarised (the request allows summarising only if review still verifies them, and never dropping them silently): their hunks stay in the diff in full and nothing is dropped (source: request)
- No new CLI command, flag or config key, and no raised limit (source: request)
- Regression cases are synthetic fixture repositories only (source: request)

## Source comparison
- Problem (K-MODELS-FILE, Build 69f25f32): about $17 review cost and 12,802,581 input tokens, 11,016,987 of them cached, plus 24,907 output tokens. The builder cost $8.71. The review prompt was 3,096,086 bytes against a 524,288-byte ceiling and was reviewed "in 9 parts". Addressed as the reproducing case. A synthetic equivalent is the acceptance `rewrite` and `hunk` fixtures.
- Problem claim "in one conversation ... each part re-sends every earlier part": conflicts with the current code, where every part is its own fresh session that is sent only its own slice (`build/review_external.go` 138-217). It is superseded by the observed cause, the changed files' full texts carried next to their diffs. The repeated reading the request describes happens inside each part's harness session, which re-reads its own prompt on every turn, so smaller and fewer part prompts reduce it.
- Need "cost grows about linearly ... each part read once, without repeated whole-history re-sends": addressed. The parts route sends the parts source, each byte once, one fresh session per part, with every part prompt within the ceiling. Acceptance scenarios 1 and 2 check `parts_cover_source_once`, `part_prompts_fit`, `sent_line_counts` and the part counts: 1 instead of 2, and 2 instead of 3.
- Need "catches the same defects as now: every hunk still read; approval requires every part read; an unread or changed part blocks approval": addressed. Every hunk stays in the parts source (scenarios 1 and 2). A changed part file still stops with `context_custody_invalid` (scenario 3). Delivery-before-approval, the verdictless-part restart and the repair-before-next-part rules are unchanged (existing tests).
- Need "measure it on this Build's log (MacBook ~/logs/k-models-file.log lines 77-114 and 144-145, run 69f25f32)": addressed by Shape's measurement, recorded here.
  - Sources: on this MacBook, `~/logs/k-models-file.log` now holds only 5 lines of a later shaping run, so lines 77-114 and 144-145 no longer exist there. The measurement uses run 69f25f32's green tree 408a1e4 on base 733363b in the operator's Kogen checkout. It also uses `~/logs/k-models-file-0240.log`, the rebuild Build a24eb434: 9 parts, a complete prompt of 3,369,823 bytes, review input=10,423,905, cached=8,521,667, output=28,974, $17.5013.
  - For 69f25f32: today's source is about 2,990,980 bytes. The implied one-session mandatory prompt is 105,106 bytes, consistent with 3,096,086. The parts source is about 1,020,764 bytes.
  - The existing cut rules, simulated with a per-part mandatory of about 125,577 bytes (catalog included), give 9 parts for today's source (as logged) and 3 for the parts source. First-prompt bytes sent fall from about 4.12 MB to about 1.40 MB.
  - Projected review input for the same change: about 4.3 million tokens instead of 12,802,581. That is 3 part sessions at the recorded average of about 1.42 million input tokens per part session; the 34% byte ratio gives about 4.35 million.
  - This is an estimate. No paid review was re-run. Verdict validation, custody and delivery checks are unchanged.
  - For a24eb434: today's source is about 3,252,184 bytes and the parts source about 1,122,834 bytes.
- Need "large generated fixtures may be summarised only if review still verifies them; never drop them silently": addressed by not summarising. Their hunks are carried in full (scenario 2 rewrites two wire fixtures on every line, and every rewritten line is sent once).
- Need "no new CLI command, flag or config key; no raised limits; synthetic regression cases only": addressed. The ceiling stays 524288 (feature background), and all scenarios use synthetic fixture repositories.
- Repository split: built in kogen-build only, where review lives. Nothing in kogen, kogen-shaping, kogen-verification, kogen-coordinator or kogen-executor changes. Earlier candidate diffs are not used.
- Omitted: the native reviewer's pinned-part route. The evidence is the Claude Code external review. A native-route change would be a follow-up item.

## Tests
- On the parts route a changed text file's full text is replaced by its `(changed; full text in the checkout)` heading. Added text files, deleted, binary and gitlink entries and the diff are unchanged, under the `# Full text of every added file` section: a new test that builds the material of a real Git fixture holding each kind of path and compares the parts source (integration)
- A wide change whose full source exceeds the ceiling is reviewed in fewer parts of the parts source, a one-line hunk in a large changed fixture in one part without its unchanged lines: acceptance scenario 1 and a new review-in-parts Build test with a large changed file (integration)
- One part is reported as `in 1 part`, and the parts task states that a changed file's full text is read in the checkout: acceptance scenario 1 (scenario)
- Fully rewritten wire fixtures send every rewritten line once, in 2 parts where today's source takes 3: acceptance scenario 2 (regression)
- The parts concatenated are exactly the retained parts source, each prompt within the ceiling, and `review_parts.source_bytes` is its length: acceptance scenarios 1 and 2 and the existing `TestReviewInPartsLands` (existing)
- A changed part file still blocks approval with `context_custody_invalid`: acceptance scenario 3 and the existing `TestReviewInPartsCustody` (existing)
- A part without a completed round is not delivered and restarts the review from part 1, a blocking part stops the Build, and a repair is verified before the next part: the existing `TestReviewInPartsVerdictlessPartRestartsFromPartOne`, `TestReviewInPartsBlockStopsLaterParts` and `TestReviewInPartsRepairIsVerifiedBeforeTheNextPart` (existing)
- A change whose complete prompt fits is still one session with every changed file's full text and today's task text: acceptance scenario 1 (first build) and the existing one-session review tests (existing)

## Feasibility
- These were scratch checks in this read-only external session, outside Kogen's probe records and outside the checkout.
  - The harness builds with `go test -c -vet=off -overlay` and runs each mode in about 2 s. On the unchanged tree all four modes run; the Cucumber run of this feature fails all 3 scenarios on missing-feature assertions: `hunk` 2 review sessions instead of 1, `rewrite` 3 instead of 2, `custody` 2 parts instead of 1.
  - A scratch prototype of this contract in a copy passed all 3 scenarios (14 steps) in about 8 s. With it, `go test -count=1 ./build` passed unchanged (386 s).
- The observer controls below take the real recorded harness runs: good from that prototype, bad from the unchanged tree. The good control accepts them against the feature's expected values. The bad control rejects:
  - the unchanged tree, a failed run, a run that did not start;
  - a missing or doubled observation line, a harness test that did not pass, another mode's observation, an altered fixture description;
  - every expected key altered or missing, and the task text without its expected sentence.

  A weakened observer, one that ignores `parts`, made the bad control fail. These are observer controls only, not product evidence.
- evidence: {"id":"review-cost-wide-observer","kind":"observer","inputs":["features/steps/k_review_cost_wide_controls/good.json","features/steps/k_review_cost_wide_controls/bad.json","features/steps/k_review_cost_wide_test.go.txt",".kogen/intents/k-review-cost-wide/acceptance.feature"],"observers":["features/steps/k_review_cost_wide_steps.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/k_review_cost_wide_steps.js --control good","exit":0,"output":"accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/k_review_cost_wide_steps.js --control bad","exit":1,"output":"rejected\n"}]}
