# square-cli

[![CI](https://github.com/MikeJansen/square-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/MikeJansen/square-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/MikeJansen/square-cli.svg)](https://pkg.go.dev/github.com/MikeJansen/square-cli)
[![Go Report Card](https://goreportcard.com/badge/github.com/MikeJansen/square-cli)](https://goreportcard.com/report/github.com/MikeJansen/square-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A comprehensive, scriptable command-line interface for the [Square Connect API](https://developer.squareup.com/reference/square).

`square-cli` exhaustively mirrors the entire Square API. Every command maps
one-to-one to a Square API endpoint: its flags are the endpoint's parameters,
and its output is the raw JSON the API returns. This makes it ideal for shell
scripts, automation pipelines, and AI assistants that need a clean, predictable
wrapper around Square — an alternative to the
[Square MCP server](https://developer.squareup.com/docs/mcp).

The command tree and the typed request/response models are **generated directly
from Square's official OpenAPI specification**, so coverage stays exhaustive and
in sync with the API.

> **Unofficial.** This is a community project and is not affiliated with,
> endorsed by, or supported by Square/Block, Inc.

---

## Highlights

- **Exhaustive coverage** — every Square API namespace and endpoint
  (**43 APIs / 328 endpoints** at the current spec version).
- **1:1 mapping** — `square-cli <api> <operation>` mirrors the API exactly.
- **Typed validation** — request bodies are validated client-side against
  generated Go models before anything is sent.
- **Full-fidelity output** — responses are emitted as the API's raw JSON
  (pretty, compact, or raw), so nothing is lost.
- **Built-in docs** — append `--docs` to any endpoint command to print a link
  to its online reference; the same link appears in `--help`.
- **Any API version** — send any `Square-Version` you need via
  `--api-version` / `SQUARE_VERSION`.
- **Flexible config** — environment variables, a config file with named
  profiles, and global flags, in that order of precedence.

## Installation

### With `go install`

```sh
go install github.com/MikeJansen/square-cli/cmd/square-cli@latest
```

### Pre-built binaries

Download a binary for your platform from the
[Releases](https://github.com/MikeJansen/square-cli/releases) page.

### From source

```sh
git clone https://github.com/MikeJansen/square-cli
cd square-cli
make build          # produces ./bin/square-cli
```

Requires Go 1.26 or newer.

## Quick start

```sh
# 1. Authenticate (sandbox recommended while testing)
export SQUARE_ACCESS_TOKEN="EAAA…"
export SQUARE_ENVIRONMENT="sandbox"     # or "production" (default)

# 2. List your locations
square-cli locations list-locations

# 3. Create a payment (request body as JSON)
square-cli payments create-payment --body '{
  "idempotency_key": "'"$(uuidgen)"'",
  "source_id": "cnon:card-nonce-ok",
  "amount_money": { "amount": 100, "currency": "USD" }
}'

# 4. Not sure about an endpoint? Open its docs.
square-cli payments create-payment --docs
# → https://developer.squareup.com/reference/square/payments-api/create-payment
```

## Command structure

Commands are grouped by Square API namespace, and each subcommand is an
operation named exactly as in the API (kebab-cased):

```
square-cli <api-group> <operation> [flags]

square-cli catalog       list                 # (example groups)
square-cli customers     create-customer
square-cli orders        search-orders
square-cli inventory     batch-retrieve-counts
```

Discover everything with `--help` at any level:

```sh
square-cli --help                    # list all API groups
square-cli payments --help           # list operations in the Payments API
square-cli payments create-payment --help
```

### Parameters

- **Path parameters** become required flags, e.g. `--payment-id`.
- **Query parameters** become optional flags of the correct type, e.g.
  `--limit`, `--begin-time`, `--location-id`.
- **Request bodies** are supplied as JSON via one of:
  - `--body '<json>'` — inline JSON
  - `--body-file <path>` — read JSON from a file
  - `--body-file -` — read JSON from stdin

Request bodies are validated against the generated typed model (unknown fields
are rejected) before the request is sent:

```sh
$ square-cli payments create-payment --body '{"nope": 1}'
Error: invalid request body: json: unknown field "nope"
```

## Output

Responses are printed as the API's raw JSON. Choose a format with `-o/--output`:

| Format    | Description                          |
|-----------|--------------------------------------|
| `json`    | Pretty-printed JSON (default)        |
| `compact` | Single-line JSON                     |
| `raw`     | Exactly as received from the API     |

```sh
square-cli locations list-locations -o compact | jq '.locations[].id'
```

Add `--typed` to normalize the response through the generated model (stable
field ordering); by default output preserves the API payload verbatim.

## Authentication & configuration

Configuration is resolved from these sources, **lowest to highest precedence**:

1. **Config file profile**
2. **Environment variables**
3. **Global flags**

### Environment variables

| Variable               | Purpose                                   |
|------------------------|-------------------------------------------|
| `SQUARE_ACCESS_TOKEN`  | API access token                          |
| `SQUARE_ENVIRONMENT`   | `sandbox` or `production` (default)       |
| `SQUARE_VERSION`       | `Square-Version` header (e.g. 2026-05-20) |
| `SQUARE_BASE_URL`      | Override the API base URL                 |
| `SQUARE_PROFILE`       | Config profile to use                     |
| `SQUARE_CONFIG`        | Path to the config file                   |

### Global flags

```
--access-token   Square access token
--environment    sandbox | production
--api-version    Square-Version header to send
--base-url       Override the API base URL
--profile        Config profile to use
--config         Path to config file
-o, --output     json | compact | raw
-v, --verbose    Log requests to stderr
--docs           Print the endpoint's documentation URL and exit
--typed          Render the response through the typed model
```

### Config file (profiles)

Default location: `$XDG_CONFIG_HOME/square-cli/config.yaml`
(or `~/.config/square-cli/config.yaml`).

```yaml
current_profile: sandbox
profiles:
  sandbox:
    access_token: EAAA_sandbox_token
    environment: sandbox
    api_version: "2026-05-20"
  prod:
    access_token: EAAA_production_token
    environment: production
```

Inspect what's resolved:

```sh
square-cli config current     # resolved config (token redacted)
square-cli config profiles    # list profiles
square-cli config path        # config file path in use
```

## API versioning

Square uses date-based API versions via the `Square-Version` header. `square-cli`
sends whatever you configure (or nothing, letting Square use your application's
default):

```sh
square-cli --api-version 2026-05-20 catalog list
SQUARE_VERSION=2025-01-23 square-cli orders search-orders --body '{ … }'
```

## How it works

`square-cli` is generated from Square's
[OpenAPI specification](https://github.com/square/connect-api-specification),
vendored at [`api/openapi/square-api.json`](api/openapi/square-api.json):

- **`internal/codegen`** parses the spec and emits typed models and the cobra
  command tree.
- **`pkg/square`** holds the generated request/response/enum models.
- **`internal/commands`** holds the generated command groups plus a small,
  hand-written runtime.

To regenerate after updating the vendored spec:

```sh
make generate    # or: go generate ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the project layout and development
workflow.

## License

[MIT](LICENSE) © Mike Jansen.

The vendored Square OpenAPI specification is © Square, Inc. and distributed
under the Apache 2.0 license; see the file header in
[`api/openapi/square-api.json`](api/openapi/square-api.json).
