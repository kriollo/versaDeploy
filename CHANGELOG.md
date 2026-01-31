# Changelog

All notable changes to this project will be documented in this file.

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
