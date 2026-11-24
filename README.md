# kogen-build

Kogen's implementation workflow: `kogen intent build`, `resume` and `recover`.

It is a component of [Kogen](https://github.com/KogenAI/kogen), which pins an exact version of it and assembles the `kogen` program.

## Standalone developer Build command

Clone the repository and enter it:

```sh
git clone https://github.com/KogenAI/kogen-build.git
cd kogen-build
```

Build the entry point from this module, without an assembled `kogen` executable:

```sh
go build -o kogen-build ./cmd/kogen-build
./kogen-build build --project /path/to/checkout --slug my-intent
./kogen-build resume --project /path/to/checkout --id BUILD_ID
```

The command uses the existing provider configuration and credentials. Where verification
uses Kogen's Node, Quint or Cucumber tools, supply their existing paths via `KOGEN_NODE`,
`KOGEN_QUINT` and `KOGEN_CUCUMBER` (the latter is the `node_modules` directory).
Approval, independent review, verification, commit, interruption and resume remain the
ordinary Build behavior. Human progress and diagnostics go to stderr. Stdout is one JSON
object, also on refusal or stop:

- `exit_code`: integer phase exit code, also used as the process exit status.
- `branch`: current symbolic branch, or an empty string for detached HEAD.
- `tree`: candidate Git tree identity (checkout tree on a pre-run refusal).
- `checks`: ordered project check records from the latest verification pass, each with
  `name`, `command`, `status` and `output`; `[]` when no pass ran.
- `error`: null on success, otherwise an object with string `category`, `kind` and
  `message` when the phase reports a structured error.

## Packages

- `build`: the builder rounds, native and external executors, working context, verification gates, the independent review and repair stage, the commit stage, checkpoints, resume and recovery.
- `consumes/`: one test suite per Kogen dependency (kogen-coordinator, kogen-executor, kogen-loop, kogen-review, kogen-verification) stating exactly what `build` and `cmd/kogen-build` rely on from that producer, at the version `go.mod` pins; `consumes/README.md` holds the inventory. `contract/` states what this module provides.

`build` owns the builder's model-facing policy: its instructions (`build/prompts/builder.md`, `BuilderInstructions`) and its terminal tools (`finish`, `report_blocker`, `BuilderToolDefinitions`), recorded in `build/testdata/prompts_golden.json` and `defs_golden.json`.

The independent review and repair itself (the native and the external-executor reviewer: review, resume, the review loop, verdict evaluation and the review tool conversation) is the `review` package of kogen-review, with a narrow contract: `build` adapts its state to the review's input (the saved candidate, the exact approved Intent, the verification evidence, the permitted tools and the review checkpoint) and serves the review's `Env`; the review makes its own material from that input and returns its verdict, findings and repairs (`build/host.go`). The outer routing (verify → review → repair → commit, and the resume across those steps) is the `route` package of kogen-coordinator, which drives the Build's phases through `route.Steps` (`build/route.go`); `build` keeps its implementation loop and feedback policy. The persisted Build state and the run record are `build`'s own; what `build` and the review share is the `core` package of kogen-executor (token totals, the review record, an executor's state) and kogen-review's review material, which `build` keeps in the review's working window; `build` re-exports them under its own names. An external builder's structured submission (`SubmissionSchema`, `build/policy.go`) is the Build's own and reaches its executor as the task's schema.

## Testing

Use Go 1.27.2 (as specified in `go.mod`) and Node.js 24 with npm. From a public clone, install the real Cucumber tools used by some tests:

```sh
npm ci --prefix testdata/tools/cucumber
```

Then run:

```sh
go build ./... && go vet ./... && go test -race -count=1 ./...
```

For inquiries, contact contact@kogen.dev.

The [history audit](docs/public-history-audit.md) lists historical findings for an owner to review; it does not change Git history.

The `build` package's tests start many git processes, so `TestMain` (`build/main_test.go`) runs them in up to four processes of the test binary (`runtime.NumCPU()/3`); a run with a `-run` pattern, a profile, `-update` or another non-plain flag runs in one process, as does a host with fewer than six CPUs. On a 12-CPU host `go test -count=1 ./build` took 77 s split and 122 s in one process.

### Golden files

Some tests compare their output with recorded golden files under `build/testdata`. To rewrite every golden from the current code's output, run:

```sh
go test ./... -update
```

`-update` is a flag of the tests only; the `kogen` program has no such option. Use it only when a change deliberately changes a golden's recorded output, or to regenerate the goldens on a combined tree after a merge or cherry-pick conflict in them. Then review the goldens' diff (the frozen goldens, such as `context-external-golden.json`, `reviewretry/*.wire.json` and `named-feedback-compat.json`, must not change without deliberate intent) and run the full checks above without the flag. It is never a way to make an unexplained failure pass.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

## Native continuation evidence for fleet workers

`kogen-build` maintains the native adapter; it does not schedule fleet work. Prepare
without provider credentials, reservation, queue changes, or checkout edits:

```sh
./kogen-build prepare-continuation --project /path/to/checkout --id BUILD_ID --binding binding.json
```

`binding.json` is an object with string fields `host`, `claim`, `attempt`,
`request_revision`, and `request_hash`. These are non-secret caller identities,
not credentials or native approval identities. Preparation merely echoes them.
The ordinary JSON envelope includes `continuation` (also on refusal); successful
preparation exits with zero, a native hold with one and the native structured
category/kind/message. Diagnostics remain on stderr. Existing build/resume output
is unchanged. There is no standalone fleet execution authority switch: standalone
execution remains `resume`.

The typed library entry points on `build.ResumeInput` are:

```go
PrepareContinuation(ctx context.Context, s *sys.Session, binding build.ContinuationBinding) (build.ContinuationPlan, error)
ExecuteContinuation(ctx context.Context, s *sys.Session, plan build.ContinuationPlan, fresh build.ContinuationBinding, admit func(context.Context, build.ContinuationPlan, build.ContinuationBinding) error) (int, error)
```

Preparation returns evidence even on native refusal. `held_reason` is the native
refusal detail, not the optimistic inspect `resume_status`; inspect facts remain
available but are not continuation authority. The plan identifies the requested
Build, slug, checkout root, current and saved device/inode identity, saved candidate
and currently observed candidate tree, source start, saved approval and approved
contract, exact checkpoint bytes, stage, binding, and remaining roles/checks.
`approval_sha256` hashes saved approval bytes; `current_approval_sha256` hashes
current bytes when readable. `contract_sha256` is the approved baseline's
`project.Hash`, retained from that approval, not a fleet request hash. A malformed
or missing checkpoint yields only observed evidence. Empty work lists are arrays.

The remaining roles are in native order: builder continuations can need build then
review; resumed review routing can need review; commit-failed and already-landed
checkpoints need neither. Remaining configured check commands retain configuration
order, omitting only native ledger reuse or the native commit-failed/already-landed
no-rerun route. Reuse is tree/environment-bound, not a waiver for later edits.
These commands do not replace native acceptance/model gates, scoped-stage custody
and ordered stage gates, whole-Intent verification, independent review and repair,
or finishing/commit checks. Checkpoint identity binds the saved stage progress,
source provenance and custody; the adapter revalidates these through native resume.

Execution acquires the ordinary Build lock, revalidates native custody, and compares
the entire prepared evidence and fresh binding before admission or a new run. A
missing callback, stale plan, changed binding, native hold or callback refusal causes
no execution or evidence repair. The callback runs under that same lock, before any
new run or paid request. The trusted worker **must freshly validate and consume its
fenced claim**, ownership, host, attempt, revision/hash, and ordinary provider and
finishing admission there, including rejecting duplicate attempts that previously
stopped before landing. An echoed binding or plan is not authorization. Callback
errors are held pre-execution refusals. Native execution retains the lock without
a release/reacquire gap, uses supported resume, and reports ordinary exit/error and
`Outcome`; it never bypasses provider login, verification, review or commit gates.

The private fleet retains scheduling, policy, accounts, credentials, provider choice
and landing policy. This adapter transfers no checkout, rebinds no inode, reconstructs
no work and changes no customer records. The sponsors row remains held and its
separate unusable-checkout reconstruction item is not resolved or delivered here.
