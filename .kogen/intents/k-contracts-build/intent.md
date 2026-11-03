# Pin consumer contracts in the producer

Slug: k-contracts-build
Model: not applicable — this change adds producer-owned tests of existing stateful behavior, not new workflow states or transitions
Acceptance:
- .kogen/intents/k-contracts-build/acceptance.feature
- features/steps/k_contracts_build_steps.js

Add a real Go test package under contract/ that owns the only supplied consumer's contract. It must be included automatically by the existing GOFLAGS=-trimpath go test ./... check, without changing .kogen/project.yaml. This is a test-ownership feature, not an API redesign. Preserve production behavior and useful existing tests. No consumer checkout is needed or available: the kogen 9bf6073 snapshot supplied in the request is the complete consumer evidence.

## Decisions
- Derive and copy/adapt the consumer-referenced TestShapeBudgetRolesBuild cases and their synthetic setup into contract/, exercising the exported build API rather than private build helpers. Keep the original test useful and passing. (source: request)
- Synthetic fixtures only; no real model runs, external network, consumer repositories at test time, new CLI commands or flags, or raised timeouts. Loopback scripted HTTP transport, temporary Git repositories and a controllable provider clock as used by the existing tests are appropriate local fixtures. (source: request)
- The consumer surface is github.com/KogenAI/kogen-build/build: Build(ctx, session, Input{Deps: Deps{Client: client, Tools: tools, External: externalExecutor}, Project: project, Slug: slug, Context: diagnostic}), Resume(ctx, session, ResumeInput{Deps: Deps{Client: client, Tools: tools, External: externalExecutor}, Project: project, ID: id}), Recover(ctx, session, RecoverInput{Project: project, ID: id, Output: output}), each returning (int, error), and Diagnostic{Text: string(data), SourcePath: path, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}. Pin these callable signatures and field types with compiling external-package consumer-shaped usage, including nil optional executors and populated diagnostic input. Do not invent requirements for other exported symbols. (source: request)
- Document the snapshot-to-case mapping in contract/ and explain that deliberate contract changes require updating producer contracts together with paired consumer items. No remote consumer integration gate is added. (source: request)

## Source comparison
- addressed: request Problem/Need, producer-owned contract/ and ordinary fast check: Decisions, Tests and acceptance scenario run the new package, while the unchanged ./... check includes it. The snapshot's cli/assembly_test.go merely lists the module path; no additional runtime behavior is inferred from it.
- addressed: snapshot cli/engine.go and cli/main.go imports, calls, return shapes and Diagnostic literal: Decisions require external consumer-shaped compiling uses. No new wire formats are inferred from an import alone.
- addressed: cli/response_budget_test.go exact referenced TestShapeBudgetRolesBuild and its three subtests: Tests below preserve all three behaviors, their event fields and user-visible expiry text; copy/adapt build/response_budget_test.go rather than invoking a consumer suite or a private producer test subprocess.
- addressed: synthetic-only/offline/fast/no new CLI/no raised timeout constraints: Decisions and Tests. Existing local provider request/response fixtures and saved-run data used by these cases are copied or adapted into producer-owned fixtures; no private data or live provider responses.
- addressed: dead-code and compile-break planted-change counterexamples: no mutation scenario is required or introduced. Any mutation evidence added during implementation must keep every package compiling and change a behavior reached through Build or Resume in the copied cases, such as the response budget or response_budget record, never dead code or a signature compile break.
- omitted deliberately: running kogen's offline suite at each landing, and speculative contracts for files/wire formats not used by the supplied calls/tests. These conflict with isolation or exceed the sole supplied consumer evidence. No consumer production item is changed because no intentional interface change is requested.

## Tests
- Consumer call shapes and diagnostic construction remain callable from outside build: contract/ external-import Go coverage compiling the snapshot's Build, Resume, Recover and Diagnostic usage with their actual types (integration)
- The builder and independent reviewer send under ten minutes and the run records it once: copy/adapt the existing first budget subtest into contract/, execute Build with scripted local responses, require success (0, nil), one builder and one review exchange, each deadline between nine and ten minutes, client ResponseBudget ten minutes, and exactly one response_budget event with budget="10 minutes" and budget_ms=600000 (integration)
- A stalled builder expires unavailable naming ten minutes and its resume lands under a new budget: copy/adapt the second budget subtest, advance the synthetic clock by ten minutes plus one second, require exit 4 and visible "provider/unavailable: ChatGPT response wait exceeded 10 minutes\n" without "10m0s", one initial request, persisted outcome="unavailable", then successful Resume with one newly budgeted exchange, two run IDs and exactly one correctly valued budget event in each run (integration)
- A stalled review expires unavailable naming ten minutes and its resume answers it: copy/adapt the third budget subtest with the same synthetic expiry, exit and text checks, one initial review request, then successful Resume with a new budgeted exchange, two total review requests, "review: approved" output and one correctly valued budget event per run (integration)
- The producer owns and executes its copied contract independently: acceptance runs go test -count=1 ./contract; contract/ must contain actual Go tests, not a wrapper that runs the old build tests, and no consumer filesystem/module dependency. Retain the three source subtest descriptions in the adapted tests and map each to the consumer snapshot (integration)
- Ordinary checking includes the contracts without extra gates: the unchanged configured GOFLAGS=-trimpath go test ./... runs contract/ along with existing packages; keep runtime comparable to the original three synthetic cases, with no sleep for a real ten-minute timeout (existing)

## Feasibility
- none
