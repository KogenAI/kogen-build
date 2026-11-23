# Confine Run.Write to the run folder

Slug: run-write-path-confinement
Model: not applicable — Run.Write is a stateless file write with no interacting rules
Acceptance:
- .kogen/intents/run-write-path-confinement/acceptance.feature
- features/steps/run-write-path-confinement__steps.js
- features/steps/run-write-path-confinement__driver/run_write_acceptance_test.go.txt

`Run.Write` in `build/core_run.go` documents that it writes a named JSON file in the run folder, but it joins the name to the run directory without checking it, so a name with parent-directory components writes outside the run folder. `Run.Write` must enforce the documented boundary: a name that escapes the run folder through parent-directory components, or that is an absolute path, is rejected with an error and nothing is written; a valid single-component name is still written as JSON followed by a newline inside the run folder. Add focused regression coverage in `build/unit_test.go`. Out of scope: other run-record writers (for example `PublishCheckpoint`) and checkpoint publication.

## Decisions
- Run.Write rejects, with an error and without writing any file, a name that escapes the run folder through parent-directory components, and rejects an absolute path; the outside target remains unchanged. The mechanism of the check is the builder's choice. No setting, flag or environment variable is added. (evidence: build/core_run.go) (assumption: nested local names such as `sub/x.json` are not named by the request; they are neither required to work nor required to be rejected, and the acceptance does not exercise them) (source: request)
- Valid single-component names keep writing JSON followed by a newline inside the run folder. (evidence: build/core_run.go) (source: request)
- Only Run.Write and its regression test change; PublishCheckpoint and other run-record writers are untouched. (evidence: build/core_run.go; build/unit_test.go) (source: request)
- Run.Write confines names with filepath.IsLocal before writing; rejects parent-directory escapes and absolute paths with an error and writes nothing (decision: D1, round 1) (constraint: Only Run.Write changes; PublishCheckpoint and other writers untouched) (constraint: No new setting, flag or env var) (evidence: build/core_run.go; build/unit_test.go) (assumption: Nested local names such as sub/x.json are unspecified by the request and not exercised by acceptance. A scratch run of the overlay fixture against the correct and three wrong Write variants showed red on the unchanged tree and the expected pass/fail split.) (source: request)

## Source comparison
- Request acceptance 1 (parent-directory escape rejected, outside file unchanged): addressed by scenario "A name that escapes through parent-directory components is rejected".
- Request acceptance 2 (absolute path rejected, target unchanged): addressed by scenario "An absolute name is rejected and its target is unchanged".
- Request acceptance 3 (valid single-component names still written as JSON and newline): addressed by scenario "A valid single-component name is still written as JSON and a newline".
- Request scope build/core_run.go and build/unit_test.go: followed; other run-record writers and checkpoint publication are omitted as out of scope.
- Existing build/records_test.go covers blob custody, not Run.Write containment (stated in the request); it is unchanged.

## Tests
- A parent-directory escape name is rejected and the outside file is unchanged: add a Run.Write regression test in build/unit_test.go (unit)
- An absolute name is rejected and its target is unchanged: add a Run.Write regression test in build/unit_test.go (unit)
- A single-component name is still written as JSON plus a newline in the run folder: add a Run.Write compatibility test in build/unit_test.go (unit)
- The three rules above end to end through the real Run.Write on a real run folder: the acceptance scenarios running a Go fixture compiled against package build (scenario)

## Feasibility
- none
