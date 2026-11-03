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

### Golden files

Some tests compare their output with recorded golden files under `build/testdata`. To rewrite every golden from the current code's output, run:

```sh
go test ./... -update
```

`-update` is a flag of the tests only; the `kogen` program has no such option. Use it only when a change deliberately changes a golden's recorded output, or to regenerate the goldens on a combined tree after a merge or cherry-pick conflict in them. Then review the goldens' diff (the frozen goldens, such as `context-external-golden.json`, `reviewretry/*.wire.json`, `k-revod-gate/parts-requests.json` and `named-feedback-compat.json`, must not change without deliberate intent) and run the full checks above without the flag. It is never a way to make an unexplained failure pass.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
