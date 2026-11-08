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
4. **Artifact** (`internal/artifact/`) — generates `manifest.json`, compresses to tar.gz in chunks (`chunk_size_mb`, default 10 MB); with `incremental_upload` only changed files are shipped over a hardlinked copy of the previous release
5. **Upload** — SFTP with `upload_workers` parallel workers (default 4), 256 KB buffers; disk space pre-checked with `df -Pk`
6. **Activation** — runs `pre_deploy_local` hooks locally, runs `pre_deploy_server` hooks on remote, atomically switches symlink `current → new-release`, runs `post_deploy` hooks
7. **Cleanup** — keeps `releases_to_keep` most recent releases (default 5), updates `deploy.lock`

### Key Packages

| Package | Purpose |
|---|---|
| `internal/config` | YAML config parsing, env var interpolation (`${VAR}`), SSH key validation |
| `internal/state` | `deploy.lock` — persists last deployment's file hashes and metadata |
| `internal/changeset` | SHA256 change detection, parallel workers with 30s per-file timeout, O(1) ignore lookup |
| `internal/builder` | Orchestrates concurrent builds; `LanguageBuilder` interface |
| `internal/deployer` | Full deployment orchestration end-to-end |
| `internal/ssh` | SSH/SFTP client wrapper (`golang.org/x/crypto`, `github.com/pkg/sftp`) |
| `internal/tui` | BubbleTea TUI — 8 views (Dashboard, Releases, File Browser, Shared Paths, Operations, Terminal, Config, Switch Config) |
| `internal/errors` | Custom error types with error codes, user-friendly messages, and fix suggestions |
| `internal/selfupdate` | Self-update mechanism from GitHub releases |

### TUI (`versa` or `versa --gui`)

TUI launches by default when running `versa` with no arguments. Use `--no-gui` to get help output instead. Uses BubbleTea (Elm architecture). State machine in `internal/tui/model.go`. Views accessible via ←/→ arrows or the sidebar (Tab). SSH connections are lazy per environment; reconnect with `c`. Styled with Lip Gloss (Tokyo Night palette). Mouse (`internal/tui/mouse.go`): click tabs and sidebar environments, wheel scrolls every view; the click geometry follows `View`/`renderTabBar`, covered by a test that clicks where each tab is drawn. Logger switches to TUI mode when GUI is active.

**TUI views:** Dashboard (F5 refresh), Releases (Enter=browse files, R=rollback), Files (browser with symlinks; Enter=view, e=edit + Ctrl+S, d=download, u=upload, Delete), Shared (shared/ + preserved_paths + missing shared_paths; Enter opens dirs and files — the `.env`/`config.json` editor), Operations (D=deploy, R=rollback, s=ssh-test, u=self-update, t=status, h=re-run hooks, l=services reload), Terminal (remote shell; Enter=start typing, Esc=stop typing — until then ←/→ pass through), Services (Space=select, I=install, P=prepare server for deploy, U=upgrade service, S=security updates, Ctrl+R=apt update, L=logs, B=DB backup now, X=reset DB password, w=save `provision:`, a=add services_reload), Config (e=edit, Ctrl+S=save), Switch Config (Space=select, m=multi-deploy with one build).

User-facing docs live in `doc/` (CLI_REFERENCE.md, DEPLOY.md for config options) and `deploy.example.yml`; update them when adding commands, flags or config fields.

### Hook System

Three hook types (replaces deprecated `hook_execution_mode`):
- `pre_deploy_local` — local commands run before cloning; abort deploy on failure
- `pre_deploy_server` — remote commands run before symlink switch; non-fatal (warnings only)
- `post_deploy` — remote commands run after symlink switch; rollback on failure

`hook_execution_mode` is deprecated. If set to `before_switch`, post_deploy hooks are automatically migrated to `pre_deploy_server`.

### Configuration (`deploy.yml`)

Supports multiple environments. Each environment has: SSH config, `remote_path`, `builds` (php/go/frontend/python), `shared_paths`, `ignored_paths`, `pre_deploy_local`/`pre_deploy_server`/`post_deploy` hooks, `hook_timeout`.

The tool auto-discovers `deploy*.yml` files in the current directory. Environment variables are interpolated at parse time.

### Old server compatibility

Remote commands generated by versa must work on old servers (RHEL/CentOS 5, coreutils 5.97, OpenSSH 4.3): use POSIX flags (`df -Pk`, not `-B1`), read `/proc` instead of `free -h`/`uptime -p`, and don't rely on `/etc/os-release`. `ssh.Client.ExecuteCommand` runs them under `/bin/sh -c`; user hooks go through `ExecuteCommandWithTimeout` in the login shell. `ssh.legacy_algorithms` enables SHA1/CBC algorithms for OpenSSH < 7.

### Python Builder

Locally it only writes `run_server.sh` (web servers: Django via gunicorn, Flask, FastAPI/Uvicorn, Gunicorn) and builds PyInstaller binaries (Linux only). Dependencies are installed **on the server**: `ensurePythonVenv` (`internal/deployer/python.go`) creates the release's virtualenv with pip/poetry/pipenv unless `reuseDependencies` hardlinked the previous one.

### Services

`services` (`internal/deployer/service.go`) run Go/Python processes through the server's init system. `renderService` generates a wrapper + systemd unit + OpenRC + SysV script into `<remote_path>/.versa/`; `serviceScript` builds the POSIX scripts that detect the init system on the server, enable, start/stop/restart and verify. Init files are installed by root only (`installScript`, run by `versa service <env> setup | ssh root@… sh` or the Services view's sudoers step): a non-root deploy stops with "needs root setup" when `.versa/` differs from what's installed, and sudoers never grants `install` (the deploy user owns `.versa/`, so it would be root). Services can't run as root unless `ssh.user` is root. Activation: switch → services_reload → post_deploy → services restart → health check; `afterRollback` reloads + restarts after any rollback. The automatic rollbacks (`Deployer.rollback`) delete the failed release; `versa rollback`/the TUI go to `ReleaseBefore` current (never forward). Tests run the generated scripts in `sh` with stubbed `sudo`/`systemctl`.

### Services view / provisioning

`internal/provision` + `internal/tui/view_services.go` (user docs: `doc/SERVICES.md`). Detection is one POSIX round-trip over the deploy connection (or the admin one when it can't connect yet), like `serverinfo.go`. Installs run as root over `provision.ssh` (merged onto `ssh` by `Environment.ProvisionSSH()`) on apt (Debian/Ubuntu) and dnf (RHEL/Rocky/Alma 8+) servers — `common.sh` sets `PKG_MGR`, `pkg_install`/`pkg_available`/`dnf_candidate`, `selinux_label`; RHEL: nginx only (Apache/UFW refused in `Conflicts` and the scripts), PHP/Node from AppStream module streams, SELinux labels, firewalld; upgrades (U/S) stay apt only: the bundle (`run.sh` + `env` with secrets + rendered vhosts) is uploaded to a 0700 temp dir; each step (`scripts/<id>.sh`) runs as a function. Vhosts come from `templates/*.tmpl` (versaWYS configs). Python/Go/Node apps are NOT installed by provisioning: the view proposes their `services` entry (`provision.ProposedServices`, written by `w` via `config.SetServices`) and the sudoers step appends `deployer.SudoersScript`'s rules, so deploys install/restart them.

Scripts must be idempotent and safe on servers in use: `apt_install` skips installed packages (upgrades only via U/S); an already installed service keeps its config and is never restarted (reload only when needed); never install an alternative (nginx vs Apache, MariaDB over MySQL, a second PHP-FPM or Node major — `provision.Conflicts` + script guards); existing DB users keep their password unless `ResetDBPassword` (`X`); one vhost per domain. Several apps per server: per-project vhost, sudoers file (`<user>-<project>`), backup timer and cron file (`provision.cron` → `/etc/cron.d/versa-<project>`, jobs run as the deploy user in `current/app`). `provision.packages` adds OS packages in the base step; `provision.ssl_self_signed` replaces certbot for non-public hosts; `provision.dragonfly` installs Dragonfly (pinned release + sha256 in `scripts/dragonfly.sh`) instead of Redis (catalog group `cache`). Releases are extracted with `umask 002` and new PHP-FPMs get `UMask=0002`: the deploy user and the web server share files through group `www-data` on every distro. `config.SaveProvision`/`AddServicesReload`/`SetServices` edit deploy.yml through `yaml.Node` to keep comments.

E2E (Docker with systemd, not in CI): `go test -tags e2e -v -timeout 60m ./test/e2e/` — Ubuntu 24.04 (nginx+PostgreSQL+Redis), Rocky 9 (nginx+PostgreSQL+Redis) and Alma 9 (nginx+MariaDB+Dragonfly; both `Dockerfile.rhel`) each hosting fixture apps `test/e2e/apps/{php,python,go,node}` (Ubuntu and Alma also Next, Nuxt, Flask, FastAPI, Gin and Laravel), and Debian 12 (Apache+MariaDB+Dragonfly) with `../versaTodo` (`VERSA_E2E_SERVERS=rocky,alma` limits the servers); checks HTTP, aliases, backups, logs, sites, re-runs that change nothing, and refused alternatives.

### Shared paths and `.env`

`sharedLinkScript` (`internal/deployer/deployer.go`) seeds a missing `shared/<path>` once — from the release, else `<path>.example` (0640), else an empty `.env*` file (0640), else an empty g+w dir — and prints `new shared file:` for the deployer's warning. Never overwrites an existing shared path. `examples/deploy.*.yml` (Laravel, Next, Nuxt, Flask, FastAPI, Gin) are loaded with unknown-key checks by `internal/config/examples_test.go` and mirrored by the e2e `frameworkApps` (fixtures `test/e2e/apps/{flask,fastapi,gin}`, Laravel generated with `composer create-project`); keep both in sync.

### Node / SSR apps

`builds.frontend` builds locally; `server_install` (`ensureNodeDeps`, `internal/deployer/node.go`) drops `node_modules` from the artifact and installs it on the server before the switch, reused via `reuseDependencies` when the lockfile didn't change. SSR apps (Next, Nuxt) run as `services` behind the proxy; the nginx proxy template rate-limits `location /` but proxies static extensions without limit. E2E fixtures `test/e2e/apps/{next,nuxt}` on Ubuntu and Alma.

### Go Builder

Supports configurable `deploy_path` for binary isolation (enables multiple services in one repo). Cross-compiles for `target_os`/`target_arch` with `CGO_ENABLED=0` (static binary) unless `go.cgo: true`. Before building, `checkGoTarget` (`internal/deployer/gotarget.go`) validates the target against the server's `uname -srm` and the Go toolchain's minimum kernel.

## CI/CD

- `.github/workflows/test.yml` — runs `go test -v ./...` on push/PR
- `.github/workflows/release.yml` — builds binaries for Linux/Windows/Darwin × amd64/arm64 on tag push
