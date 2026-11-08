# Follow the Claude Code 2.1.318 contract

Slug: b-consumes-e-claude-code-2-1-318
Model: not applicable — a dependency pin and fixture literals have no meaningful state
Acceptance:
- .kogen/intents/b-consumes-e-claude-code-2-1-318/acceptance.feature
- features/steps/b-consumes-e-claude-code-2-1-318__steps.js
- features/steps/b-consumes-e-claude-code-2-1-318__observer.js

Follow the producer's new main without compatibility behavior. Required implementation edits are go.mod and go.sum for the producer upgrade, including required indirect module metadata, and build/context_external_test.go for the fake runtime's --version response and init record's claude_code_version. Replace both fixture literals, and any tests asserting the replaced contract, from 2.1.312 to 2.1.318. Do not preserve the old producer pin or either old fixture version. These are requested changes, not unchanged-code expectations. Preserve the existing first-prompt test's assertions and both context paths; only its version fixtures change. Update any other fixture version field to the same new contract and remove old-version alternatives. Keep every other test green and make no unrelated behavior changes. Update documentation only where it describes the replaced contract. Control overlays qualify acceptance, not permission to reformat or rewrite implementation tests.

## Decisions
- Adopt github.com/KogenAI/kogen-executor, package executor/claudecode: the request states "the dependency is github.com/KogenAI/kogen-executor (package executor/claudecode)". (source: request)
- Use only 2.1.318: the request states "every fixture version field in this repo must read 2.1.318 and no old-version alternative remains" and specifies runtime.go `Version = "2.1.318"`. (source: request)
- Move the producer pin using go get against the mirror: the request says "move this repository's go.mod pin of the producer to that main (go get against the mirror) in the same change". Do not add fallback behavior: "There is no compatibility layer and none is wanted". (source: request)
- Preserve scope: "change the fixture version literals 2.1.312 to 2.1.318, nothing else" and "Keep every other test green". (source: request)

## Source comparison
- Addressed: Request Problem's TestContextExternalFirstPrompt/context_false and context_true reproductions are covered by the producer-adoption scenario, including both subtests through the real first-prompt integration (evidence: build/context_external_test.go).
- Addressed: Request Need's new-main go.mod pin and delivery note's executor/claudecode Version literal are covered by the producer-adoption scenario. Replace the old go.mod dependency pin using go get github.com/KogenAI/kogen-executor/executor/claudecode@main against the configured mirror, retaining its required go.sum and indirect dependency changes. No replacement module is allowed. The old pin is explicitly superseded, not retained (evidence: go.mod).
- Addressed: Retry notes' fake runtime and fixture-only 2.1.312 to 2.1.318 replacement is covered by the fixture scenario and first-prompt execution. The fixture scenario specifies the replacement values, not the old values to preserve. The existing --version and claude_code_version fields to replace are in build/context_external_test.go (evidence: build/context_external_test.go). No old-version acceptance or fallback is permitted.
- Addressed: "every test in it passes" and "Nothing else changes" remain whole-change requirements; retain configured checks and the README full regression command (evidence: .kogen/project.yaml; README.md). No unrelated refactor, golden rewrite or test weakening is authorized.
- Addressed: delivery notes' instruction not to fetch the web; use the request and repository. Probe findings are not product authority and are not Decisions or acceptance expectations.
- Addressed: delivery note's form requirement, "mark anything the request does not quote with (assumption: ...) before that tag." Decisions above quote the request rather than introducing unconfirmed product assumptions, and each ends with the required source tag.
- Omitted: no requested behavior. Superseded: the old producer pin and old fixture version only. Conflicting: none.

## Tests
- New producer pin: verify the pinned producer exports the requested Claude Code Version without a local replacement (integration)
- New fixture contract only: verify fake runtime version literals (regression)
- First prompt remains correct with and without context: run TestContextExternalFirstPrompt and require both subtests to pass (regression)
- All unrelated behavior stays green: retain configured checks and run go build ./... && go vet ./... && go test -race -count=1 ./... (existing)

## Feasibility
- evidence: {"id":"first-prompt-observer","kind":"observer","inputs":["features/steps/b-consumes-e-claude-code-2-1-318__observer.js"],"observers":["features/steps/b-consumes-e-claude-code-2-1-318__observer.js"],"checks":[{"name":"good","command":"\"$KOGEN_NODE\" features/steps/b-consumes-e-claude-code-2-1-318__observer.js good","exit":0,"output":"observer accepted\n"},{"name":"bad","command":"\"$KOGEN_NODE\" features/steps/b-consumes-e-claude-code-2-1-318__observer.js bad","exit":1,"output":"observer rejected\n"}]}
