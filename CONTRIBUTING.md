# Contributing to versaDeploy

Thanks for helping! Bug reports, docs fixes, new examples and code are all welcome. Issues and pull requests can be written in **Spanish or English**.

- [Reporting a bug](#reporting-a-bug) · [Proposing a feature](#proposing-a-feature) · [Development setup](#development-setup) · [Tests](#tests) · [Pull requests](#pull-requests) · [Rules of the codebase](#rules-of-the-codebase)

## Reporting a bug

Open an [issue](https://github.com/kriollo/versaDeploy/issues/new/choose) with the bug template. The most useful things to include:

- `versa version`, your OS, and the server's distro (`cat /etc/os-release`).
- The command you ran and its output with `--debug` (e.g. `versa deploy production --debug`).
- The relevant part of your `deploy.yml`, **without secrets**: replace hosts, passwords and tokens.

Security problems go **privately** through [SECURITY.md](SECURITY.md), never in a public issue.

## Proposing a feature

Open an issue with the feature template before writing a large change, so we can agree on the approach. versaDeploy aims to be a simple, secure, agentless (SSH-only) way to deploy to Linux servers: features that need an agent on the server, or that make the common path harder, are a hard sell.

## Development setup

You need Go (the version in [`go.mod`](go.mod)) and git. Some builders also need the toolchain they run (PHP + Composer, Node, Python).

```bash
git clone https://github.com/kriollo/versaDeploy.git
cd versaDeploy
go build -o versa ./cmd/versa
./versa version
```

Where things live: `cmd/versa` is the CLI (Cobra), everything else is under `internal/` — `deployer` (the deploy pipeline), `builder` (language builders), `provision` (the Services view's install scripts), `tui` (BubbleTea), `ssh`, `config`. [CLAUDE.md](CLAUDE.md) has a detailed map of the architecture.

## Tests

```bash
go test ./...            # unit tests (run on Linux, macOS and Windows in CI)
go test -race ./...      # with the race detector
gofmt -l cmd internal test   # must print nothing
go vet ./...
```

Remote scripts are tested by running them in `sh` with stubbed `sudo`/`systemctl`, so most of the logic is covered without a server.

The **end-to-end test** provisions real servers in Docker (systemd containers: Ubuntu, Debian, Rocky, Alma) and deploys real apps (Laravel, Next, Nuxt, FastAPI, Flask, Gin…). It isn't part of CI; run it when you touch provisioning, services or the deploy pipeline:

```bash
go test -tags e2e -v -timeout 90m ./test/e2e/                                    # every server
VERSA_E2E_SERVERS=ubuntu go test -tags e2e -v -timeout 60m -run '^TestProvisionAndDeploy$' ./test/e2e/   # one
```

It needs Docker, and Composer/Node locally for the PHP and Node apps.

## Pull requests

1. Branch from `main`, keep the change focused (one fix or feature per PR).
2. Add or update tests: a bug fix comes with a test that fails without it.
3. Update the docs in the same PR: [`doc/`](doc) (CLI_REFERENCE.md for commands and flags, DEPLOY.md for `deploy.yml` options, SERVICES.md for provisioning), [`deploy.example.yml`](deploy.example.yml) for new config fields, and a line under `## [Unreleased]` in [CHANGELOG.md](CHANGELOG.md).
4. Commit messages follow [Conventional Commits](https://www.conventionalcommits.org): `feat: …`, `fix(tui): …`, `docs: …`, `test: …`.
5. `go test ./...`, `go vet ./...` and `gofmt` must pass (CI checks them on every push).

## Rules of the codebase

These keep versaDeploy safe on servers that are already in production:

- **Old servers**: commands that run on the server must work on RHEL/CentOS 5 (coreutils 5.97, OpenSSH 4.3): POSIX `sh`, `df -Pk` instead of `-B1`, read `/proc` instead of `free -h`, don't rely on `/etc/os-release`.
- **Provisioning scripts are idempotent and safe on servers in use**: never install an alternative to what exists (nginx next to Apache, MariaDB over MySQL, a second PHP), never restart or reconfigure a running service, never change an existing database user's password, never upgrade a package unless the user asked (`U`/`S`).
- **Secrets never travel on a command line** (other users see them in `ps`): pass them in a private file or over stdin.
- **Errors** are user-facing: say what failed and what to do about it ([`internal/errors`](internal/errors)).

## Releases

Maintainers release by bumping `internal/version/version.go`, moving `[Unreleased]` to a version in the CHANGELOG, and pushing a `vX.Y.Z` tag: [the release workflow](.github/workflows/release.yml) builds the binaries for Linux, macOS and Windows and publishes them with their sha256.

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
