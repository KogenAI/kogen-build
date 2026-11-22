# Record cut sizes wherever the cut marker sits

Slug: build-cut-fields-follow-the-marker
Model: not applicable — one pure function over a string; the scenarios cover each outcome
Acceptance:
- .kogen/intents/build-cut-fields-follow-the-marker/acceptance.feature
- features/steps/build-cut-fields-follow-the-marker__steps.js
- features/steps/build-cut-fields-follow-the-marker__driver/acceptance_test.go.txt

`cutFields` in `build/records.go` fills `bytes_before_cut` and `bytes_delivered` of a tool record, but returns nothing unless the omitted-bytes marker starts between byte 4000 and byte 4096 of the delivered text. Since kogen-loop cuts at a line boundary, a cut shell result can carry its marker at byte 15, and its record then has no cut fields, so the context-spend analysis reads it as uncut. After this change a delivered tool result of at most 16384 bytes that contains the omitted-bytes marker line (a newline, the text `[... N bytes omitted ...]` with N a whole number, a newline) is recorded with `bytes_before_cut` equal to the delivered length, less the marker length, plus N, and `bytes_delivered` equal to the delivered length, wherever the marker starts. A result without the marker, a result over 16384 bytes, and a marker whose number does not parse as a whole number are recorded as before, with no cut fields. The comment above `cutFields` no longer claims a 4096 byte head.

The 4000 to 4096 window is removed, not kept beside the new rule. Scope: `build/records.go` and `build/records_test.go`. Out of scope: every other file, the kogen-loop pin, `go.mod`, `go.sum`, the cut itself and every other record field.

## Decisions
- The marker position check (`loc[0] > 4096 || loc[0] < 4000`) is deleted; the existing size bound of 16384 bytes, the marker pattern and the whole-number parse stay. (decision: D1, round 1) (evidence: build/records.go) (source: request)
- A delivered result of at most 16384 bytes that contains the marker is recorded with bytes_before_cut equal to the delivered length, less the marker length, plus N, and bytes_delivered equal to the delivered length, wherever the marker starts. The formula is unchanged. (decision: D2, round 1) (evidence: build/records.go; build/records_test.go) (source: request)
- A result without the marker, a result over 16384 bytes, and a marker whose number does not parse are recorded as before, with no cut fields; text that only contains the word omitted is not a marker. (decision: D3, round 1) (evidence: build/records.go) (source: request)
- The comment above cutFields describes the marker, not a head size, and no longer mentions 4096. (decision: D4, round 1) (evidence: build/records.go) (source: request)
- No setting is added; the bound and marker pattern stay constants. (decision: D5, round 1) (evidence: build/records.go) (source: request)
- Acceptance runs three driver Go tests that it copies into build/ for the run and removes afterwards, because the tests the Build adds to build/records_test.go are not known beforehand; the driver calls cutFields directly and uses tools.Cut for the existing 40960 figure. The Build also extends build/records_test.go with its own tests. (decision: D6, round 1) (evidence: build/records_test.go; build/noprogress_test.go) (assumption: the driver tests are a proof artifact separate from the tests the Build adds) (source: request)

## Source comparison
- Request acceptance 1: a 9000 byte result whose marker starts at byte 15 and states 31000 omitted bytes gets bytes_before_cut 9000 minus the marker length plus 31000 and bytes_delivered 9000; addressed by D1, D2 and scenario 1.
- Request acceptance 2: a marker starting at byte 4050 is recorded exactly as today, and the existing 40960 figure of the current cut test stays; addressed by D2 and scenario 2.
- Request acceptance 3: a result with no marker, one of 16385 bytes with a marker and one whose marker number is not a whole number carry neither field; addressed by D3 and scenario 3.
- Request: the comment no longer claims a 4096 byte head; addressed by D4 and scenario 4.
- Request: out of scope are every other file, the kogen-loop pin, go.mod, go.sum, the cut itself and every other record field; omitted from the behavior.

## Tests
- A marker at byte 15 (and elsewhere, up to a 16384 byte result) yields bytes_before_cut and bytes_delivered: a Go test in build/records_test.go (unit)
- A marker inside the former window and the cut of 40960 bytes keep their recorded values: a Go test in build/records_test.go (regression)
- No marker, 16385 bytes and a marker number that is not whole carry no fields: a Go test in build/records_test.go (unit)
- The comment above cutFields no longer mentions a 4096 byte head: acceptance.feature scenario (scenario)
- The existing TestCutFieldsOnlyForCutResults keeps passing unchanged: build/records_test.go (existing)

## Feasibility
- none
