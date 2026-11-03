# Prepare public repository documentation

Slug: k-public-ready-build
Model: not applicable — documentation cleanup has no meaningful state
Acceptance:
- .kogen/intents/k-public-ready-build/acceptance.feature
- features/steps/k_public_ready_build_steps.js

Make KogenAI/kogen-build read correctly as a public repository. Change only README and documentation files; preserve executable behavior, useful assertions, goldens, dependencies and all existing checks. Remove private setup instructions and unavailable workflows rather than documenting their unavailability. Describe this component's implementation-workflow role in Kogen, standalone build, public-clone setup and testing accurately. Preserve useful CLI output and golden-update documentation. The Go version must agree with go.mod; retain the real Cucumber installation prerequisite rather than suggesting that Go alone installs Node tools.

Audit the entire tracked tree, including old Intent documentation, for private URLs, actual runner/home paths, host identifiers and internal landing/delivery notes. Remove those from documentation. Paths needed by fixtures must be synthetic; do not weaken assertions. If a required cleanup cannot be achieved in documentation, stop and report the scope conflict rather than editing executable files. Do not change .kogen/project.yaml or any other protected file. The host-local lineup override is not repository content and is not a cleanup target. Current main uses ordinary commands including `GOFLAGS=-trimpath go test ./...` and has no private-module setup.

Every link in the tree must resolve publicly, including relative links. Kogen-wide links must use https://kogen.dev or https://github.com/KogenAI/kogen; replace README's sibling-component links with the public Kogen entry point, retaining useful architectural explanation without depending on private workspace knowledge. Verify external targets anonymously, following redirects; record unreachable targets honestly rather than claiming success. Preserve public third-party license and dependency-registry links. Inquiries go to contact@kogen.dev exactly once in README. Do not add SECURITY.md, CONTRIBUTING.md or BRANDING.md, contribution refusals, maintenance promises, or prose about disabled collaboration features.

Never rewrite Git history. Inspect reachable history for the same categories and provide an owner-facing report at docs/public-history-audit.md, with commit identity, affected file and category/description for each hit, deduplicating identical commit/file/category entries. Do not copy raw private originals into that public report: use category descriptions, not sensitive literal matches. Include the history traversal command and its boundary so the report is not an unsupported exhaustive claim. This report is an audit result, not internal delivery instructions. The final result must point the owner to it for their decision; no removal from history is authorized.

## Decisions
- Documentation-only implementation scope; .kogen/project.yaml and protected files stay untouched; host override paths do not count as repository leaks. (source: request)
- Both Issues and Pull requests have already been disabled by an owner/admin; no documentation edit or unauthenticated check claims to enforce that setting. (source: answer 1)
- Leave the historical executable harness's GOPRIVATE default unchanged; private-module-setup prohibition applies to public-facing documentation, while private URL, real path and internal-note prohibitions still apply tree-wide. (source: answer 1)
- Preserve ordinary behavior and checks, never rewrite history, and report historical hits for the owner rather than removing them. (source: request)

## Source comparison
- Addressed: request Problem and Need, public README role/setup/tests and removal of private setup, by the opening outcome, README acceptance scenarios and configured unchanged checks. Existing README line 54 is superseded, not retained as a prerequisite.
- Addressed: Need public links and contact literal, by the link audit requirement and README checks; the existing README sibling-component links are superseded by the two allowed Kogen-wide entry points. Public third-party and relative links remain valid targets.
- Addressed: Need absent policy files, refusal and maintenance copy, by the prohibited-policy scenario and the prose constraint; no new contribution policy or feature-status prose is authorized.
- Addressed: Need current-tree sanitation, including historical Intent documentation containing real machine references, by the tracked-tree scenario and full documentation audit. Diagnostic patterns in the new acceptance specification are not real machine references. Synthetic fixture paths are permitted, not actual user directories.
- Addressed: Need unchanged behavior and assertions by documentation-only scope, unchanged executable files and full configured checks. No golden regeneration or dependency change is authorized.
- Addressed: Need untouched history and owner list of commit/file/finding, by docs/public-history-audit.md and its executable minimum report checks; independent review must compare the report to actual reachable history, not merely its headings.
- Superseded: original broad private-module-setup interpretation, by the latest answer 1 keeping features/steps/k_models_file_build/harness.js unchanged. The README private setup is still removed.
- Addressed: re-filed scope and main configuration clarification by the explicit protected-config exclusion. Host-local override commit is excluded, not counted as a repository defect.
- Addressed: latest settings answer 1 records the completed authenticated handoff. No further settings change or credentials are needed.
- Omitted: no requested behavior. Anonymous link resolution and clean public dependency availability remain delivery verification obligations, not claims established by local cached dependencies.

## Tests
- Public README explains the component, public setup/test commands and one contact without private setup: README content scenarios (integration)
- Tree documentation removes actual machine references and forbidden policy files: tracked-tree scan and file-absence scenarios (integration)
- Public links and no unavailable workflow/refusal/maintenance copy: full document and link audit during independent review plus README link restrictions (integration)
- Historical findings identify commits, files and categories without rewriting history: report checks and independent comparison with reachable Git history (integration)
- Existing executable behavior and assertions remain unchanged: configured format, compile, vet and Go test checks with documentation-only diff review (existing)

## Feasibility
- Anonymous `git -c credential.helper= -c core.askPass= ls-remote https://github.com/KogenAI/kogen-coordinator.git HEAD` returned a HEAD successfully; this establishes repository visibility only, not availability of every pinned module or a clean public-clone test pass. `go version` reported go1.27.2 darwin/arm64. (probe: exit 0, 1.0 s)
- Documentation search for Go module proxy documentation returned unrelated results; fetch of https://go.dev/ref/mod returned retrieved, untrusted documentation explaining GOPROXY and private-module environment controls. No exhaustive discovery or clean-clone proof is claimed.
