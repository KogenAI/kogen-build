# kogen-build

Kogen's implementation workflow: `kogen intent build`, `resume` and `recover`.

It is a component of [Kogen](https://github.com/KogenAI/kogen), which pins an exact version of it and assembles the `kogen` program.

## Packages

- `build`: the builder rounds, native and external executors, working context, verification gates, the independent review and repair stage, the commit stage, checkpoints, resume and recovery.

`build` owns the builder's model-facing policy: its instructions (`build/prompts/builder.md`, `BuilderInstructions`) and its terminal tools (`finish`, `report_blocker`, `BuilderToolDefinitions`), recorded in `build/testdata/prompts_golden.json` and `defs_golden.json`.

The independent review and repair itself (the native and the external-executor reviewer: review, resume, the review loop, verdict evaluation and the review tool conversation) is the `review` package of [kogen-review](https://github.com/KogenAI/kogen-review), with a narrow contract: `build` adapts its state to the review's input (the saved candidate, the exact approved Intent, the verification evidence, the permitted tools and the review checkpoint) and serves the review's `Env`; the review returns its verdict, findings and repairs (`build/host.go`). The outer routing (verify → review → repair → commit, and the resume across those steps) is the `route` package of [kogen-coordinator](https://github.com/KogenAI/kogen-coordinator), which drives the Build's phases through `route.Steps` (`build/route.go`); `build` keeps its implementation loop and feedback policy. The persisted Build state and the run record are `build`'s own; what `build` and the review share (token totals, the review record, an executor's state, the review material) is the `core` package of [kogen-executor](https://github.com/KogenAI/kogen-executor), which `build` re-exports under its own names.

## Testing

Some tests run the real Cucumber. Install them once:

```sh
npm ci --prefix testdata/tools/cucumber
```

Then run:

```sh
go build ./... && go vet ./... && go test -race -count=1 ./...
```

The module depends on private KogenAI modules: set `GOPRIVATE=github.com/KogenAI/*`.

### Golden files

Some tests compare their output with recorded golden files under `build/testdata`. To rewrite every golden from the current code's output, run:

```sh
go test ./... -update
```

`-update` is a flag of the tests only; the `kogen` program has no such option. Use it only when a change deliberately changes a golden's recorded output, or to regenerate the goldens on a combined tree after a merge or cherry-pick conflict in them. Then review the goldens' diff (the frozen goldens, such as `context-external-golden.json`, `reviewretry/*.wire.json`, `k-revod-gate/parts-requests.json` and `named-feedback-compat.json`, must not change without deliberate intent) and run the full checks above without the flag. It is never a way to make an unexplained failure pass.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
