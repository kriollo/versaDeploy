# Changelog

All notable changes to this project will be documented in this file.

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
