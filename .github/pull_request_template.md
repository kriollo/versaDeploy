## What / Qué

<!-- What this changes and why. Link the issue: Fixes #123 -->

## Checklist

- [ ] Tests added or updated (a bug fix comes with a test that fails without it)
- [ ] `go test ./...`, `go vet ./...` and `gofmt -l cmd internal test` pass
- [ ] Docs updated: `doc/`, `deploy.example.yml` for new config fields
- [ ] A line under `## [Unreleased]` in `CHANGELOG.md`
- [ ] Remote commands still work on old servers (POSIX `sh`), and provisioning stays safe on servers in use (see [CONTRIBUTING.md](../CONTRIBUTING.md#rules-of-the-codebase))
- [ ] Ran the e2e test if this touches provisioning, services or the deploy pipeline
