# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-07-12

### Added

- Initial release of `square-cli`.
- Exhaustive "L1" command tree generated from Square's OpenAPI specification:
  one command group per API namespace, one subcommand per endpoint.
- Typed request/response/enum models generated into `pkg/square`.
- Client-side request-body validation against the typed models.
- Authentication via `SQUARE_ACCESS_TOKEN`, a config file with named profiles,
  and global flags, with clearly defined precedence.
- Sandbox/production environment selection and arbitrary `Square-Version`
  support.
- `--docs` flag on every endpoint command that prints a link to the online
  reference.
- `json`, `compact`, and `raw` output formats, plus optional `--typed` output.
- `config` subcommands to inspect the resolved configuration and profiles.

[Unreleased]: https://github.com/MikeJansen/square-cli/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/MikeJansen/square-cli/releases/tag/v0.1.0
