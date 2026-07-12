# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Instead, report them privately using GitHub's
[private vulnerability reporting](https://github.com/MikeJansen/square-cli/security/advisories/new)
("Report a vulnerability" under the Security tab). You should receive an
acknowledgement within a few days.

Please include:

- A description of the vulnerability and its impact
- Steps to reproduce
- Any relevant version or environment details

## Handling of credentials

`square-cli` reads your Square access token from environment variables, global
flags, or a config file. A few things to keep in mind:

- **Never commit access tokens** to version control. The config file lives
  outside the repository by default (`$XDG_CONFIG_HOME/square-cli/config.yaml`).
- Prefer the `sandbox` environment while developing.
- The tool never logs your access token; `config current` redacts it.

## Supported versions

This project is pre-1.0. Security fixes are applied to the latest release only.
