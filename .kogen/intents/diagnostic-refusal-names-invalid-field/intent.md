# Name the invalid field in diagnostic refusals

Slug: diagnostic-refusal-names-invalid-field
Model: not applicable — the change only alters the detail text of a stateless validation refusal
Acceptance:
- .kogen/intents/diagnostic-refusal-names-invalid-field/acceptance.feature
- features/steps/diagnostic-refusal-names-invalid-field__steps.js
- features/steps/diagnostic-refusal-names-invalid-field__driver/main.go

When `Diagnostic.Validate` in `build/context.go` refuses invalid diagnostic data, the refusal detail names the offending field (the text, the source path or the SHA-256) instead of the same generic detail for every case. The refusal keeps category `intent`, kind `context_unavailable` and exit code 2, and still happens before any run is created or the checkout is changed. The validation rules themselves, the diagnostic size limit, the CLI flags and the diagnostic storage and delivery are unchanged. The change is limited to `build/context.go` and `build/unit_test.go`; the existing unit test that asserts the generic detail is updated to assert the field-specific details.

## Decisions
- For a diagnostic with one invalid field, the refusal detail names that field: the word "text" for an invalid, blank or oversized diagnostic text, the words "source path" for an empty, relative or invalid UTF-8 source path, and "SHA-256" for a digest that is invalid UTF-8 or does not equal the lower-case hex SHA-256 of the text. The detail does not name a field other than the offending one. (evidence: build/context.go; build/unit_test.go) (source: request)
- Category `intent`, kind `context_unavailable` and exit code 2 are preserved for every refusal; a nil Diagnostic keeps the existing generic detail because no field is offending. (evidence: build/context.go) (source: request)
- Validation stays the first step of the Build, before the checkout is opened, so a refusal creates no run directory and does not change the checkout. (evidence: build/build.go) (source: request)
- The accepted-input rules (UTF-8, nonblank, at most 262144 bytes, absolute source path, exact lower-case digest) are not changed. (evidence: build/context.go; build/unit_test.go) (source: request)

## Source comparison
- Invalid or blank text names the text field: addressed by the blank-text and invalid-UTF-8-text scenarios.
- Relative, empty or invalid UTF-8 source path names the source path field: addressed by the three source path scenarios.
- Digest mismatch names the SHA-256 field: addressed by the mismatched and upper-case digest scenarios.
- Same category and exit code, and no run or checkout change: asserted in every scenario.
- Diagnostic limits, accepted input rules, CLI flags and diagnostic storage or delivery: out of scope, untouched.

## Tests
- Each invalid field is named in the refusal detail, with intent/context_unavailable and exit code 2: update TestDiagnosticDataShapeAndDigestAreExact and TestDiagnosticDataIsBoundedOriginalUTF8 in build/unit_test.go to check the named field, and TestDiagnosticDataThatIsNotValidRefusesBeforeAnyEffect to expect the source path detail (unit)
- A refused Build creates no run and changes no checkout, for each field: scenarios running a compiled driver that calls the Build entry point in a scratch checkout (scenario)
- Existing tests of accepted diagnostic data and of delivery to the builder stay unchanged: build/unit_test.go (existing)

## Feasibility
- none
