# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**versaDeploy** (`versa`) is a CLI deployment tool written in Go for deploying applications from Linux/Windows to remote Linux servers. It uses SSH/SFTP, supports PHP/Go/Frontend/Python multi-language builds, SHA256-based change detection, atomic symlink releases, and an optional BubbleTea TUI.

## Commands

```bash
# Build
go build -o versa ./cmd/versa

# Cross-compile (as done in CI)
GOOS=linux GOARCH=amd64 go build -o dist/versa_linux_amd64 ./cmd/versa

# Run tests
go test ./...
go test -v ./...
go test -v ./internal/deployer/...   # single package

# Run with race detector
go test -race ./...

# Run the binary
go run ./cmd/versa deploy [env]
go run ./cmd/versa --gui
```

## Architecture

Entry point: `cmd/versa/main.go` (Cobra CLI). All core logic lives under `internal/`.

### Deployment Pipeline

1. **Validation** — local tools, git repo, working directory cleanliness
2. **Change Detection** (`internal/changeset/`) — parallel SHA256 hashing of all files, categorized by language (PHP/Go/Frontend/Python), detects dependency manifest changes
3. **Build** (`internal/builder/`) — concurrent builds via `errgroup`; copies repo → artifact dir first, then each language builder runs. Language builders: `internal/builder/lang/{php,go,frontend,python}.go`
4. **Artifact** (`internal/artifact/`) — generates `manifest.json`, compresses to tar.gz in 100 MB chunks
5. **Upload** — SFTP with 4 parallel workers, 256 KB buffers
6. **Activation** — runs `pre_deploy_local` hooks locally, runs `pre_deploy_server` hooks on remote, atomically switches symlink `current → new-release`, runs `post_deploy` hooks
7. **Cleanup** — keeps 5 most recent releases (`ReleasesToKeep = 5` in deployer), updates `deploy.lock`

### Key Packages

| Package | Purpose |
|---|---|
| `internal/config` | YAML config parsing, env var interpolation (`${VAR}`), SSH key validation |
| `internal/state` | `deploy.lock` — persists last deployment's file hashes and metadata |
| `internal/changeset` | SHA256 change detection, parallel workers with 30s per-file timeout, O(1) ignore lookup |
| `internal/builder` | Orchestrates concurrent builds; `LanguageBuilder` interface |
| `internal/deployer` | Full deployment orchestration end-to-end |
| `internal/ssh` | SSH/SFTP client wrapper (`golang.org/x/crypto`, `github.com/pkg/sftp`) |
| `internal/tui` | BubbleTea TUI — 7 views (Dashboard, Releases, File Browser, Shared Paths, Operations, Config, Switch Config) |
| `internal/errors` | Custom error types with error codes, user-friendly messages, and fix suggestions |
| `internal/selfupdate` | Self-update mechanism from GitHub releases |

### TUI (`versa` or `versa --gui`)

TUI launches by default when running `versa` with no arguments. Use `--no-gui` to get help output instead. Uses BubbleTea (Elm architecture). State machine in `internal/tui/model.go`. Views accessible via keys 1–6 or ←/→ arrows. SSH connections are lazy per environment; reconnect with `c`. Styled with Lip Gloss (Tokyo Night palette). Logger switches to TUI mode when GUI is active.

**TUI views:** Dashboard (F5 refresh), Releases (Enter=browse files, R=rollback), Files (browser with symlinks), Shared (Enter=navigate dirs), Operations (D=deploy, R=rollback, s=ssh-test, u=self-update, t=status), Config.

### Hook System

Three hook types (replaces deprecated `hook_execution_mode`):
- `pre_deploy_local` — local commands run before cloning; abort deploy on failure
- `pre_deploy_server` — remote commands run before symlink switch; non-fatal (warnings only)
- `post_deploy` — remote commands run after symlink switch; rollback on failure

`hook_execution_mode` is deprecated. If set to `before_switch`, post_deploy hooks are automatically migrated to `pre_deploy_server`.

### Configuration (`deploy.yml`)

Supports multiple environments. Each environment has: SSH config, `remote_path`, `builds` (php/go/frontend/python), `shared_paths`, `ignored_paths`, `pre_deploy_local`/`pre_deploy_server`/`post_deploy` hooks, `hook_timeout`.

The tool auto-discovers `deploy*.yml` files in the current directory. Environment variables are interpolated at parse time.

### Python Builder

Supports `pip`/`poetry`/`pipenv`, virtual env reuse via hardlinks, PyInstaller binary builds, systemd service file generation, and web servers: Django, Flask, FastAPI/Uvicorn, Gunicorn.

### Go Builder

Supports configurable `deploy_path` for binary isolation (enables multiple services in one repo). Cross-compiles for `target_os`/`target_arch`.

## CI/CD

- `.github/workflows/test.yml` — runs `go test -v ./...` on push/PR
- `.github/workflows/release.yml` — builds binaries for Linux/Windows/Darwin × amd64/arm64 on tag push
