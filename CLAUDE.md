# square-cli — Project Guide

A comprehensive, generated Go CLI that exhaustively wraps the entire Square
Connect API. Public repo: `github.com/MikeJansen/square-cli`.

## What this is

- An "L1" CLI: **one command per Square API endpoint**, flags mirror the
  endpoint's parameters, JSON body in, raw JSON out.
- Intended as a clean, scriptable wrapper for shell scripts and AI assistants.
- The command tree and typed models are **generated from Square's OpenAPI spec**.

## Architecture

```
cmd/square-cli/       Entry point.
internal/cli/         Root command, global flags, config→client wiring (hand-written).
internal/commands/    runtime.go (hand-written helpers) + *_gen.go (GENERATED command tree).
internal/config/      Config resolution: file profiles < env < flags.
internal/client/      Thin HTTP client: auth, Square-Version, error parsing.
internal/output/      json / compact / raw rendering.
internal/codegen/     The generator (main package; reads the spec).
pkg/square/           GENERATED typed models (models_gen.go). DO NOT hand-edit.
api/openapi/          Vendored Square OpenAPI spec (square-api.json).
```

## Key facts

- **Go 1.26+.** Deps: `spf13/cobra`, `gopkg.in/yaml.v3` only.
- Generated files end in `_gen.go` and carry a `DO NOT EDIT` header. Change the
  generator, not the output.
- Regenerate: `make generate` (or `go generate ./...`). CI fails if checked-in
  generated code drifts from generator output.
- The generator maps: top-level schemas → structs / string-enum types; operations
  (grouped by OpenAPI `tag`) → cobra command groups + subcommands named by the
  kebab-cased `operationId`.
- Dangling `$ref`s in the spec (e.g. `AppFeeAllocation`) fall back to
  `json.RawMessage` — see `definedSchemas` in the generator.
- Doc links are derived: `https://developer.squareup.com/reference/square/<tag>-api/<operation>`
  (tag/operation kebab-cased; `OAuth` overridden to `oauth`). Surfaced via `--docs`.

## Conventions

- Request bodies are validated against the typed model (`DisallowUnknownFields`)
  before sending; responses print the API's raw JSON by default (`--typed` to
  round-trip through the model).
- Keep the generator deterministic (sort maps before emitting).
- `make check` = gofmt-check + vet + tests. Run before committing.

## Common tasks

- **Bump Square API version:** replace `api/openapi/square-api.json`, run
  `make generate`, then `make check`.
- **Add L2 convenience commands:** add hand-written commands in a non-`_gen.go`
  file under `internal/commands` and register them (keep L1 untouched).
