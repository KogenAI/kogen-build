# kogen-build

Kogen's implementation workflow: `kogen intent build`, `resume` and `recover`.

It is a component of [Kogen](https://github.com/KogenAI/kogen), which pins an exact version of it and assembles the `kogen` program.

## Packages

- `build`: the builder rounds, native and external executors, working context, verification gates, the independent review and repair stage, the commit stage, checkpoints, resume and recovery.
- `build/review`: the independent review and repair: the native and the external-executor reviewer (review, resume, the review loop, verdict evaluation and the review tool conversation). It imports `build/core` and nothing else of this module.
- `build/core`: what `build` and `build/review` share: the persisted Build state, the run record, the review contract and the services the review uses. It imports nothing else of this module.

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
