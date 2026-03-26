# Changelog

All notable changes to this project will be documented in this file.

## [1.4.0rc] - 2026-03-26

### Added

- **Fix: Deploy stale — `services_reload`**: New configuration field `services_reload` in each environment. After every deploy and rollback, versaDeploy executes these commands on the remote server (e.g. `sudo systemctl reload php8.2-fpm`) to clear PHP-FPM OPcache and `realpath_cache`. Without this, deployed changes would not take effect until PHP-FPM workers recycled naturally (up to 120 seconds). See `deploy.example.yml` for Apache + PHP-FPM and Nginx examples.
- **Health Check post-deploy**: New `health_check` config block (`url`, `expected_status`, `timeout`, `retries`, `retry_delay`). After every deploy, versaDeploy performs an HTTP GET from the local machine to the configured URL. If the check fails after all retries, automatic rollback to the previous release is triggered.
- **Webhook Notifications**: New `notifications` config block (`webhook_url`, `on_success`, `on_failure`). versaDeploy sends a JSON POST to the configured webhook at the end of each deployment with project, environment, release, commit, status, error, duration, and timestamp fields.
- **Deploy Timeout enforcement**: New `deploy_timeout` config field (default: 600 seconds). The deployment pipeline now checks the deadline at key stages (build, upload, pre-deploy hooks, symlink switch) and aborts with an error if the timeout is exceeded.
- **Rollback to specific version (`--to`)**: New `--to` flag on `versa rollback <env> --to=<version>`. Validates the target release exists on the server, switches the symlink, and runs `services_reload`.
- **CLI `versa exec`**: New command to execute an arbitrary command on the remote server via SSH and print its output. Example: `versa exec production "df -h"`.
- **CLI `versa logs`**: New command to tail remote log files in real-time using `tail -f`. Defaults to Laravel's `storage/logs/laravel.log` inside the active release. Accepts `--lines N` flag. Example: `versa logs production /var/log/apache2/error.log`.
- **CLI `versa hooks`**: New command to re-execute `post_deploy` hooks on the currently active release. Accepts optional zero-based indices to run specific hooks. Example: `versa hooks production 0 2`.
- **TUI — Terminal SSH view (view 6)**: New interactive remote terminal view accessible via `←`/`→` navigation. Features: command history (↑/↓), remote working directory tracking (`cd` updates prompt), Tab completion for remote paths via `ls -1Ap`, mouse wheel scrolling, `Ctrl+L` to clear, `Esc` to exit. Commands run via SSH with output streamed to a scrollable viewport.
- **TUI — Operations: Re-execute post_deploy hooks (`h`)**: New quick action in the Operations view. Press `h` to re-run all configured `post_deploy` hooks against the currently active release.
- **TUI — Operations: Re-execute services_reload (`l`)**: New quick action in the Operations view. Press `l` to re-run all configured `services_reload` commands (e.g. PHP-FPM reload) without a full redeploy.
- **SSH `ExecuteCommandStreaming`**: New method on the SSH client that runs a command and streams stdout/stderr to provided `io.Writer` instances in real-time, with PTY allocation support.

### Changed

- **TUI — Navigation: removed number key shortcuts**: View switching by pressing `1`–`8` has been removed. Navigate between views exclusively using `←`/`→` arrow keys. This fixes a conflict where pressing `h` in the Operations view was incorrectly triggering "go to previous view" instead of "re-execute hooks".
- **TUI — Navigation: removed vim-style keys**: `h`/`l` (left/right) and `j`/`k` (up/down) keybindings removed from the global keymap to prevent conflicts with operation-specific keys.
- **TUI — Tab bar**: View labels no longer show number key hints (e.g. `1:Dashboard` → `Dashboard`).
- **TUI — Status bar**: Help string updated to `←/→:views` instead of `1-7:views`.
- **Deployer pipeline order**: `services_reload` runs after the symlink switch and before `post_deploy` hooks. `health_check` runs after `post_deploy` hooks.
- **Documentation — `doc/GETTING_STARTED.md`**: Added `EnableSendfile Off` / `EnableMMAP Off` Apache directives, sudoers configuration for the deploy user, and `services_reload` marked as REQUIRED for PHP-FPM setups.
- **Documentation — `doc/CLI_REFERENCE.md`**: Added full reference for `versa exec`, `versa logs`, `versa hooks`, and `versa rollback --to`.
- **Internal Version**: Version bumped to 1.4.0rc.

## [1.3.2rc] - 2026-03-25

### Added

- **TUI — Log view scrolling**: Deploy log viewport now supports user-controlled scrolling. Arrow keys (`↑`/`↓`) and `PgUp`/`PgDn` scroll through the log while a deploy is running or after completion. Auto-scroll to bottom resumes when the user scrolls back to the end. Footer shows `↑↓:scroll  PgUp/PgDn:page` hints.
- **TUI — File editor (View 3)**: Text files in the remote file browser can now be edited inline. Press `e` while viewing a text file to open an editable textarea, `Ctrl+S` to save back to the remote server via SFTP, and `Esc` to cancel. Only available for text files with valid UTF-8 content that are not truncated. Original file permissions are preserved after write.
- **TUI — Deploy options: Skip dirty check**: New toggle in the Operations view (`Skip dirty check`) to bypass the uncommitted-changes validation, equivalent to the CLI `--skip-dirty-check` flag.
- **TUI — Deploy options: Debug mode**: New toggle in the Operations view (`Debug mode`) that activates verbose debug logging during the deploy, equivalent to the CLI `--debug` flag.
- **TUI — Deploy options: Log file path**: New text field in the Operations view to specify an optional log file path. When set, deploy log output is written simultaneously to the TUI viewport and to the specified file via `io.MultiWriter`.
- **SSH `WriteRemoteBytes`**: New method on the SSH client to write arbitrary bytes to a remote file path via SFTP, with permission preservation via `Chmod`.

### Changed

- **TUI — Icons overhaul (Material Design Icons + colors)**: All file browser and sidebar icons replaced with Material Design Icons (Nerd Fonts v3, codepoints `\U000Fxxxx`) with language brand colors (Go `#00ADD8`, Python `#3776AB`, JS `#F7DF1E`, TS `#3178C6`, PHP `#777BB4`, Rust `#DEA584`, folder `#7aa2f7`, etc.). Status indicators use MDI glyphs: running `\U000F0765`, success `\U000F012C`, error `\U000F0159`.
- **TUI — Expanded directory icon mappings**: Added icon mappings for `test/tests/__tests__`, `docs/documentation`, `assets/static/public`, `scripts/bin`, `migrations`, `.vscode`, `tmp/temp/cache`, `deploy/deployments`, `api`.
- **Internal Version**: Version bumped to 1.3.2rc.

## [1.3.1rc] - 2026-03-24

### Fixed

- **Config editor scroll behavior**: Improved upward navigation in the Config editor by adding cursor margins so the viewport no longer follows too aggressively near the file end.
- **Self-update restart path**: Fixed restart resolution after `self-update` to avoid attempting to execute a removed `.old` binary path.
- **Config edit mode key isolation**: Global shortcuts are now blocked while editing config files, so keys like `c` are treated as text input instead of triggering commands.

### Changed

- **Internal Version**: Version bumped to 1.3.1rc.

## [1.3.0rc] - 2026-03-24

### Added

- **`--no-gui` flag**: TUI now launches by default when running `versa` with no subcommand. Use `--no-gui` to get the help output instead. `--gui` is kept for backward compatibility.
- **`pre_deploy_local` hooks**: New hook type that runs local shell commands before cloning the repo. Aborts the deployment on failure. Ideal for running tests, linters, or pre-flight checks locally.
- **`pre_deploy_server` hooks**: New hook type that runs remote commands before the symlink switch (non-fatal — warnings only). Ideal for gracefully stopping services before activation.
- **`hook_execution_mode` migration**: Deprecated `hook_execution_mode: before_switch` configurations are now automatically migrated to `pre_deploy_server` at validation time with a printed warning.

### TUI Overhaul

- **Real terminal height**: All views now receive the actual content height from the window size message instead of using the `heightFromWidth()` heuristic (`width / 3`, clamped 10–40). Layouts no longer break on wide or narrow terminals.
- **Esc handling — universal dismiss**:
  - File viewer (browser): `Esc` closes the viewer and returns to the directory listing.
  - Config view (read-only): `Esc` returns to the Dashboard.
  - Operations (running): `Esc` closes the log output even while an operation is in progress.
- **Left/Right navigation unblocked for config read-only**: Arrow key view-cycling now works from the Config view when not in edit mode.
- **Releases view — scroll window**: Added `viewStart` tracking so large release lists scroll correctly rather than overflowing the terminal.
- **Shared view — scroll window**: Same scroll window added; long shared-path lists no longer overflow.
- **Shared view — Nerd Font icons**: Replaced `📄`/`📁` emoji with `iconForEntry()` calls (consistent with the file browser).
- **Icon codepoint replacement**: Replaced all MDI-range Nerd Font codepoints (`\uf7xx`, `\uf9xx`, `\uea6c`, `\ufc1e`, `\ufd42`) — which render as empty boxes in many terminal/font combinations — with widely supported Font Awesome and Devicon codepoints. Key changes: folders use `\uf07b`/`\uf07c`, shell files use `\uf120` (terminal), lock files use `\uf023`, images use `\uf03e`, Docker uses `\ue7b0`, generic files use `\uf15b`.
- **Sidebar — connection indicators for all envs**: Every environment in the sidebar now shows its connection state dot (connected/connecting/error/idle), not just the active one.
- **Operations footer fix**: While an operation is running, the footer now reads `Esc=close   ←/→=switch views   (running…)` instead of the previous misleading message.
- **Operations emoji cleanup**: Replaced `▶`/`◀`/`⚡` section markers with plain-text `▸`/`◂`/`»` to avoid rendering issues.
- **Config footer fix**: Read-only mode footer now shows `Esc=back` and `←/→=switch views`.
- **Removed `heightFromWidth()`**: The width-based height heuristic function is gone; all views use real terminal dimensions.

### Fixed

- **Builder race condition**: Concurrent PHP/Go/Frontend/Python build goroutines previously wrote directly to `b.result` fields while running in parallel, creating a data race. Results are now collected in goroutine-local variables and merged into `b.result` after `errgroup.Wait()`.
- **Changeset channel deadlock**: Hash worker channels were sized to `len(filesToHash)` which could be arbitrarily large. Channel buffer is now capped at 1000 and jobs are sent in a separate goroutine, preventing blocked sends on repositories with many files.
- **Artifact single-chunk compression**: `Compress()` was calling `CompressChunked` with a 100 MB limit, potentially producing multiple chunks for large artifacts. Now uses `math.MaxInt64` to always produce a single `.tar.gz`.
- **Artifact defer-in-loop**: `defer f.Close()` inside a `filepath.Walk` callback was leaking file descriptors until the walk completed. Replaced with an explicit `f.Close()` immediately after `io.Copy`.
- **SSH agent connection leak**: The `net.Conn` to `SSH_AUTH_SOCK` was never stored and therefore never closed. It is now tracked in `Client.agentConn` and closed in `Client.Close()`.
- **`CheckDiskSpace` path injection**: Remote path was interpolated into the shell command without quoting (`%s`). Changed to `%q` to prevent command injection via unusual path characters.
- **Temp lock file collision**: Multiple concurrent deployments to different environments shared the same `deploy.lock` temp file path (`os.TempDir()/deploy.lock`). Now namespaced as `deploy-<envName>.lock`.

### Changed

- **`hook_execution_mode` deprecated**: The field is still parsed for backward compatibility but triggers a migration warning. Use `pre_deploy_local`, `pre_deploy_server`, and `post_deploy` instead.
- **Internal Version**: Version bumped to 1.3.0rc.

## [1.2.0rc] - 2026-03-23

### Added

- **Go Deploy Path Isolation**: Added `builds.go.deploy_path` to control where Go binaries are stored inside each release (default: `bin`). This enables safer multi-service deployments by separating runtime paths.
- **Hook Execution Mode**: Added `hook_execution_mode` with `after_switch` (default) and `before_switch` modes for rollout strategy control.
- **Runtime Pre-Activation Validation**: Deployments now validate critical runtime artifacts (Go binary, Python runtime script/binary, PHP vendor dependencies) before activating `current`.

### Changed

- **Go Rebuild Semantics**: Go binaries are now rebuilt only when Go files or `go.mod` change. A global `--force` no longer triggers Go rebuilds by itself.
- **Go/Python Runtime Reuse**: Enhanced reuse logic to carry over Go binaries and Python runtime assets/dependencies between releases when dependency inputs are unchanged.
- **Python Server Script Output**: Python web-server setup now always produces Linux-compatible `run_server.sh` artifacts for remote servers, regardless of local OS.
- **`versa init` Template**: Updated default generated `deploy.yml` with `hook_execution_mode`, `go.root`, and `go.deploy_path`.

### Improved

- **Configuration Defaults and Validation**: Added stricter validation and sane defaults for Go/Python deployment paths and hook execution mode.
- **Documentation**: Updated `doc/DEPLOY.md`, `doc/GETTING_STARTED.md`, `deploy.example.yml`, and default `deploy.yml` to reflect the new backend deployment model.

### Version

- **Internal Version**: Version bumped to 1.2.0rc.

## [1.1.0rc] - 2026-03-04

### Added

- **Interactive TUI (`--gui`)**: New terminal UI accessible via `versa --gui`. Provides a full-screen interactive dashboard without leaving the terminal.
  - **Sidebar**: Lists all configured environments with live connection indicators (connected / connecting / error / idle). Navigate with `↑/↓` and switch with `Tab`.
  - **View 1 — Dashboard**: Shows the current active release, remote disk usage, and total release count for the selected environment.
  - **View 2 — Releases**: Scrollable table of all releases with the active one marked. Press `r` to rollback to any selected release instantly.
  - **View 3 — File Browser**: SFTP-backed directory navigator rooted at `remote_path`. `Enter` descends into directories, `Backspace` goes up.
  - **View 4 — Shared Paths**: Lists all configured `shared_paths` with their disk usage (`du -sh`).
  - **View 5 — Deploy**: Toggle form for `dry-run`, `force`, and `initial-deploy` flags. Press `d` to launch the deployment and watch logs stream line-by-line in a scrollable viewport.
  - **Lazy SSH connections**: Each environment connects on first access; reconnect at any time with `c`.
  - **Tokyo Night color palette** via Lip Gloss.

### Improved

- **Logger TUI support**: `logger.NewTUILogger(w io.Writer, verbose, debug bool)` constructor added. When set, log output is written to the provided writer (used for deploy log streaming) instead of stdout.
- **SSH `ReadDir`**: Exposed `Client.ReadDir(path string) ([]os.FileInfo, error)` for SFTP directory listing used by the file browser.

### Dependencies

- Added `github.com/charmbracelet/bubbletea` (Elm-architecture TUI framework)
- Added `github.com/charmbracelet/lipgloss` (ANSI styling)
- Added `github.com/charmbracelet/bubbles` (viewport, spinner, key components)

### Changed

- **Internal Version**: Version bumped to 1.1.0rc.

## [1.0.7rc] - 2026-03-04

### Performance

- **Faster Release Sorting**: Replaced O(n²) bubble sort in `SortReleases` with `sort.Slice` (O(n log n)).
- **Logger Lazy Formatting**: `Debug()` now returns early when debug mode is off, eliminating unnecessary `fmt.Sprintf` allocations in production.
- **Faster Change Detection**: Pre-allocated `filesToHash` slice (capacity 512) to reduce GC pressure on large repos. Ignored paths now use a `map[string]struct{}` for O(1) exact-match lookups instead of linear scans.
- **Hash Timeout**: Added a 30-second per-file context timeout to file hashing goroutines, preventing indefinite hangs on stale NFS mounts or broken file permissions.
- **Parallel Repo Copy**: `copyEntireRepo` now uses a `runtime.NumCPU()` worker pool to copy files in parallel, significantly reducing build preparation time on large repos.
- **Parallel Directory Upload**: `UploadDirectory` now creates remote directories sequentially and uploads files with 4 concurrent workers, reducing SFTP transfer time.
- **Atomic Symlink in One Round-Trip**: `CreateSymlink` now executes the full create-rename-verify sequence in a single SSH command, reducing network round-trips from 3 to 1.
- **Larger Upload Buffer**: `uploadFile` now uses `io.CopyBuffer` with a 256 KB buffer to reduce syscall overhead when uploading large artifacts.
- **Reduced Artifact Memory Peak**: `Compress` default chunk size reduced from 1 GB to 100 MB, lowering peak memory usage during compression.

### Improved

- **Shared `CalculateDirSize`**: Extracted duplicate directory-size calculation into a new `internal/fsutil` package, used by both `builder` and `deployer`.
- **Robust Network Error Detection**: `errors.Wrap` now uses `errors.As(*net.OpError)` for network error classification (timeout, connection refused) before falling back to string matching, making it more reliable across library versions.

### Changed

- **Internal Version**: Version bumped to 1.0.7rc.

## [1.0.6rc] - 2026-01-31

### Added

- **Documentation Overhaul**: Centralized documentation strategy by moving guides to the `doc/` directory.
- **New Guides**: Added `doc/GETTING_STARTED.md` and `doc/CLI_REFERENCE.md` for better onboarding and reference.

### Improved

- **Logging Audit**: Full audit of application logs to standardize levels and improve readability. Debug logs are now properly suppressed unless `--debug` is used.
- **SSH Diagnostics**: Added logger support to the SSH client for better troubleshooting of remote operations.
- **Config Documentation**: Expanded comments and examples in `deploy.example.yml`.
- **Builder Stability**: Improved failure reporting in the build phase with more detailed context.

### Changed

- **Internal Version**: Version bumped to 1.0.6rc.

## [1.0.5rc] - 2026-01-31

### Added

- **Version Command**: Added `versa version` command to display the current version of versaDeploy.

### Fixed

- **Self-Update**: Fixed a bug where the self-update command would fail to detect the latest version from GitHub.

## [1.0.4rc] - 2026-01-31

### Fixed

- **Self-Update**: Fixed a bug where the self-update command would fail to detect the latest version from GitHub.

## [1.0.3rc] - 2026-01-31

### Added

- **Skip Dirty Check**: Added `--skip-dirty-check` flag to the `deploy` command. This allows deployments even when the local repository has uncommitted changes, ensuring that only the last committed state is deployed to the server.

## [1.0.2rc] - 2026-01-30

### Added

- **Parallel Chunked Uploads**: Implemented parallel artifact uploading using 10MB chunks. This significantly reduces deployment time for large artifacts.
- **Deployment Locking**: Added a distributed locking mechanism using atomic remote directory creation to prevent concurrent deployments to the same environment.
- **Release Sorting**: Added deterministic release sorting logic to ensure correct order for rollbacks and cleanup operations.
- **Safer Shell Execution**: Improved remote command security by using proper quoting (`%q`) for all user-provided and generated paths.

### Improved

- **SFTP Performance**: Optimized SFTP throughput by increasing the maximum packet size.
- **Diagnostic Logging**: Enhanced error messages with more context when file preservation or hook execution fails.
- **Repository Validation**: Strengthened internal checks for repository integrity before starting a build.

### Fixed

- **Version Consistency**: Synchronized internal versioning with the release candidate tag.
- **Artifact Generator**: Fixed edge cases in manifest validation for complex directory structures.

## [1.0.1 RC] - 2026-01-30

### Added

- **Force Redeploy Flag**: Added `--force` flag to the `deploy` command to trigger full builds (Composer/NPM/Go) even when no file changes are detected.

### Fixed

- **Frontend Cleanup Logic**: Fixed a bug where `node_modules` cleanup was skipped if `package.json` was not modified, ensuring production-only dependencies are always enforced after a build.

### Improved

- **SSH Cleanup Diagnostics**: The `CleanupOldReleases` function now captures and reports the exact output from the server if a release deletion fails.
- **Builder Diagnostics**: Added real-time directory size measurement (before/after cleanup) and improved command logging in the build process for easier troubleshooting.

## [1.0.0 RC] - 2026-01-29

### Added

- **Concurrent File Hashing**: Significantly improved change detection speed (4-8x faster) by parallelizing SHA256 calculations using a worker pool.
- **Parallel Builds**: Added concurrent execution of PHP (Composer), Go, and Frontend (NPM/Yarn) builds, reducing total build time by up to 60%.
- **Compression Progress Bar**: Added real-time visual feedback during the artifact compression phase.
- **Enhanced Progress Visibility**: Separated compression and upload phases in the UI for better status tracking.

### Optimized

- **Performance Overhaul**: Replaced O(n²) bubble sort with O(n log n) standard library sorting for release management.
- **Code Efficiency**: Removed unused functions and streamlined internal build logic.
- **Resource Management**: Optimized I/O operations for large file handling.

### Fixed

- **CLI Help Duplication**: Fixed an issue where available commands were listed twice in the `--help` output.
- **Concurrent Build Isolation**: Ensured independent build processes do not interfere with each other.

## [0.9.0-beta] - 2026-01-29

### Added

- **Self-Update**: Added `versa self-update` command to automatically detect, download, and install the latest version from GitHub. Includes atomic binary replacement and automatic application restart.
- **Improved Versioning**: Added `versa version` command and centralized version management.
- **Standardized Naming**: Release assets are now named `versa_{os}_{arch}` for better predictability and multi-platform compatibility.
- **Preserved Paths**: New feature to lock specific files or directories (e.g., `.env`, `config.php`) to their server-side versions after the initial deployment.
- **Reusable Build Assets**: Implementation of `reusable_paths` to recover large folders (like `vendor`, `node_modules`, `dist`) from the previous release using fast Linux hardlinks.
- **Shared Paths**: Added support for persistent directories across releases using symlinks to a central `shared/` folder.
- **Intelligent Change Detection**: The deployment tracker now monitors critical file extensions (`.vue`, `.ts`, `.php`) even within `ignored_paths`.
- **Automatic Build Dependencies**: Local builder now automatically runs installation commands (e.g., `pnpm install`) if dependencies are missing during compilation.
- **Absolute Symlinks**: Re-implemented `current` symlink and shared path linking using absolute paths for cross-environment robustness.

### Fixed

- **Post-Deploy Hook Permissions**: Added automatic permission normalization (0775 for directories, 0664 for files) to ensure group write access.
- **Hook Context**: Fixed path resolution for post-deploy hooks to always execute within the absolute path of the new release's `app/` directory.
- **Windows Git Path**: Improved Git executable resolution on Windows to handle non-standard installation paths.
- **Artifact Structure**: Standardized the artifact layout to use a consistent `app/` subdirectory.

### Improved

- **Deployment Logs**: Enhanced logging with clearer status messages for rollbacks, hook execution, and file preservation.
- **Automatic Rollback**: Strengthened the automatic rollback mechanism to ensure environment stability on hook failures.
- **Exhaustive Documentation**: Updated `DEPLOY.md` and `deploy.example.yml` with comprehensive configuration guides.
