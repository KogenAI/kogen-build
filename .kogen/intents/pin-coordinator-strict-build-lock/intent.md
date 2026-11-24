# Pin the signed Coordinator revision

Slug: pin-coordinator-strict-build-lock
Model: not applicable — the dependency pin changes no state-transition rules
Acceptance:
- .kogen/intents/pin-coordinator-strict-build-lock/acceptance.feature
- features/steps/pin-coordinator-strict-build-lock__steps.js

Replace kogen-build's direct `github.com/KogenAI/kogen-coordinator` pin in `go.mod`, currently `v0.0.0-20261116203159-d0dab3d6a948` (evidence: go.mod), with `v0.0.0-20261124131633-b427b145576a` from the GitHub-verified signed Coordinator commit `b427b145576a5bcb096b3b203483471e0d69eb8f`. Limit the implementation diff to `go.mod` and necessary `go.sum` changes. The real `GOWORK=off GOFLAGS= go list -m -mod=readonly github.com/KogenAI/kogen-coordinator` from the module root must succeed and print the exact module and version line in acceptance, without a replacement suffix.

This is the dependency prerequisite for held `kogen-build/native-supersede-revised-scope` item `5f1ba719-1736-4f5a-9da6-9466e4afeac7` request revision 3. Do not implement supersession or alter retained Builds, their custody, or queue states.

Use normal native approval and the configured setup and checks on the delivery candidate: `npm ci --prefix testdata/tools/cucumber`, then `format` (`test -z "$(gofmt -l .)"`), `compile` (`go build ./...`), `vet` (`go vet ./...`), and `go-test` (`GOFLAGS=-trimpath go test ./...`), plus the configured Cucumber acceptance runner (evidence: .kogen/project.yaml). Use Go 1.27.2 and Node.js 24 with npm (evidence: go.mod; README.md).

Independent Review verifies the candidate's implementation diff against its Build starting tree and the pin evidence. Retain native approval, check and Review evidence. Deliver through a signed commit and GitHub verified landing on `kogen-build` main; record the landed commit SHA and verification evidence separately from upstream signature verification. On failure retain failure evidence and keep the consumer held. Landing this pin does not release the consumer: the consumer owner separately confirms the pin on landed main before fresh Shape.

## Decisions
- Pin Coordinator to v0.0.0-20261124131633-b427b145576a from GitHub-verified signed commit b427b145576a5bcb096b3b203483471e0d69eb8f and verify exact resolution with go list -m. (source: request)
- Limit delivery to go.mod and necessary go.sum changes; do not implement supersession or alter retained Builds, their custody, or queue states. (source: request)
- Run the repository's configured setup and checks; deliver through normal native approval, independent Review, signed commit, and GitHub verified landing. (evidence: .kogen/project.yaml) (source: request)
- On failure retain evidence and keep the consumer held. The consumer owner separately confirms the pin on landed main before fresh Shape. (source: request)

## Source comparison
- Addressed: The Request's exact version, commit and go list verification appear in the pin paragraph, resolution scenario and upstream provenance evidence.
- Superseded: The old Coordinator requirement in go.mod becomes the requested pin; no other dependency update is in scope.
- Addressed: The Request's file and state exclusions and held consumer slug, item ID and revision appear in the scope paragraphs and Review gate. This Intent is only a prerequisite, not the consumer change.
- Addressed: The Request's setup, checks, approval, independent Review, signed commit and GitHub verified landing govern delivery. Evidence retention on failure, the continued hold and the separate owner's landed-main confirmation before fresh Shape remain explicit. Local acceptance does not substitute for these delivery gates.

## Tests
- Exact Coordinator selection without replacement: Resolve the exact Coordinator pin from the consumer module (integration)

## Feasibility
- The unchanged tree resolved the old pin. Go 1.27.2 downloaded the requested version with Origin.Hash b427b145576a5bcb096b3b203483471e0d69eb8f. (probe: exit 0, 3.4 s)
- GitHub GET /repos/KogenAI/kogen-coordinator/commits/b427b145576a5bcb096b3b203483471e0d69eb8f returned that SHA with commit.verification.verified true and reason valid. (probe: exit 0, 0.7 s)
- Scratch go get at the exact version changed only the Coordinator requirement and added the two new Coordinator checksums. Exact resolution, configured npm setup and all four configured checks passed. This is not native delivery evidence. (probes: exit 0, 2.5 s; exit 0, 260.5 s)
- Scratch controls used the real manifests. The replacement overlay printed a replacement suffix pointing to the older Coordinator. The second capture isolated the JSON after the first included diagnostics. (probes: exit 0, 2.9 s; exit 0, 2.8 s)
