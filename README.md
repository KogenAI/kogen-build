# kogen-build

Kogen's implementation workflow: `kogen intent build`, `resume` and `recover`.

It is a component of [Kogen](https://github.com/KogenAI/kogen), which pins an exact version of it and assembles the `kogen` program.

## Packages

- `build`: the builder rounds, native and external executors, working context, verification gates, the independent review and repair stage, the commit stage, checkpoints, resume and recovery.

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

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
