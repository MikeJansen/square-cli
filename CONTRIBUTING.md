# Contributing to square-cli

Thanks for your interest in improving `square-cli`! Contributions of all kinds
are welcome — bug reports, documentation, and code.

## Ground rules

- Be respectful; this project follows the [Code of Conduct](CODE_OF_CONDUCT.md).
- Open an issue to discuss significant changes before investing a lot of work.
- Keep pull requests focused and small where possible.

## Development setup

You need **Go 1.26+**.

```sh
git clone https://github.com/MikeJansen/square-cli
cd square-cli
make build      # build ./bin/square-cli
make test       # run tests
make check      # fmt-check, vet, and test (what CI runs)
```

## Project layout

```
cmd/square-cli/          Program entrypoint (main).
internal/cli/            Root command, global flags, config→client wiring.
internal/commands/       Generated command groups (*_gen.go) + hand-written runtime.
internal/config/         Config file, profiles, env, and flag resolution.
internal/client/         Thin HTTP client (auth, Square-Version, errors).
internal/output/         Response rendering (json / compact / raw).
internal/codegen/        The code generator (reads the OpenAPI spec).
pkg/square/              Generated typed models (DO NOT EDIT).
api/openapi/             Vendored Square OpenAPI specification.
```

## Generated code

The command tree (`internal/commands/*_gen.go`), the model package
(`pkg/square/models_gen.go`), and the registry are **generated** — do not edit
them by hand. Instead, change the generator in `internal/codegen` and regenerate:

```sh
make generate      # or: go generate ./...
```

CI verifies that checked-in generated code matches the generator output, so
always commit regenerated files.

### Updating to a newer Square API version

1. Replace `api/openapi/square-api.json` with the latest spec from
   [square/connect-api-specification](https://github.com/square/connect-api-specification).
2. Run `make generate`.
3. Run `make check` and fix any generator issues.
4. Note the change in `CHANGELOG.md`.

## Design notes

`square-cli` is intentionally a **thin, exhaustive "L1" wrapper**: one command
per endpoint, flags for parameters, JSON bodies in and raw JSON out. Higher-level
convenience ("L2") commands may be layered on later, but the L1 surface should
remain a faithful mirror of the API.

- Request bodies are validated against the generated typed model before sending.
- Responses are emitted as the API's raw JSON by default to preserve fidelity.
- Every endpoint command carries a `--docs` link to its online reference.

## Coding conventions

- Run `gofmt` (via `make fmt`); CI rejects unformatted code.
- Keep the generator deterministic — sort maps before emitting so output is
  stable across runs.
- Prefer the standard library; the runtime has minimal dependencies
  (`cobra`, `yaml.v3`).

## Submitting a pull request

1. Fork and create a feature branch.
2. Make your change; add or update tests.
3. Run `make check` and `make generate` (commit any regenerated files).
4. Open a PR describing the change and linking any related issue.

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE).
