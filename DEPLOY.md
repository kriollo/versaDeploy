# ⚙️ Configuration Guide (`deploy.yml`)

This document provides a detailed reference for all configuration options available in `deploy.yml`.

## Global Settings

| Field     | Type   | Description                                                                                     |
| :-------- | :----- | :---------------------------------------------------------------------------------------------- |
| `project` | string | **Required**. A unique identifier for your project. Used for logging and internal organization. |

## Environments

The `environments` map allows you to define different targets (e.g., `production`, `staging`).

### 1. SSH Configuration (`ssh`)

Settings for connecting to the remote server.

| Field              | Type   | Default              | Description                                                         |
| :----------------- | :----- | :------------------- | :------------------------------------------------------------------ |
| `host`             | string | -                    | **Required**. Hostname or IP address of the remote server.          |
| `user`             | string | -                    | **Required**. SSH username.                                         |
| `key_path`         | string | -                    | **Required**. Path to the private SSH key. Supports `~/` expansion. |
| `port`             | int    | `22`                 | SSH port.                                                           |
| `known_hosts_file` | string | `~/.ssh/known_hosts` | Path to the `known_hosts` file for host key verification.           |
| `use_ssh_agent`    | bool   | `false`              | If true, attempts to authenticate using an active SSH agent.        |

> [!TIP]
> **Windows Users**: You can use Windows-style paths like `C:\Users\Name\.ssh\id_rsa` or Unix-style `~/.ssh/id_rsa`.

### 2. General Environment Settings

| Field             | Type         | Default | Description                                                                                                       |
| :---------------- | :----------- | :------ | :---------------------------------------------------------------------------------------------------------------- |
| `remote_path`     | string       | -       | **Required**. Absolute path on the remote server where the application will be deployed.                          |
| `shared_paths`    | list[string] | `[]`    | Paths that persist across releases (e.g. `storage`, `uploads`). They are symlinked to a central `shared/` folder. |
| `preserved_paths` | list[string] | `[]`    | Files/folders on the server that **should not be updated** after the first deploy (e.g. `.env`, `config.php`).    |
| `hook_timeout`    | int          | `300`   | Timeout in seconds for each `post_deploy` hook.                                                                   |
| `route_files`     | list[string] | `[]`    | Files that, if changed, will trigger specific logic in your hooks via environment variables.                      |
| `ignored_paths`   | list[string] | `[...]` | Paths relative to project root that should be ignored when creating the artifact.                                 |

### 3. Build Configurations (`builds`)

versaDeploy supports three build engines: `php`, `go`, and `frontend`.

#### PHP (`php`)

| Field              | Type         | Default                | Description                                                                                                   |
| :----------------- | :----------- | :--------------------- | :------------------------------------------------------------------------------------------------------------ |
| `enabled`          | bool         | `false`                | Enable PHP build engine.                                                                                      |
| `root`             | string       | `""`                   | Subdirectory where `composer.json` is located.                                                                |
| `composer_command` | string       | `composer install ...` | Command to run for dependency installation.                                                                   |
| `reusable_paths`   | list[string] | `["vendor"]`           | Folders to reuse from the previous release via hardlinks if `composer.json` didn't change (speeds up deploy). |

#### Go (`go`)

| Field         | Type   | Default | Description                                                        |
| :------------ | :----- | :------ | :----------------------------------------------------------------- |
| `enabled`     | bool   | `false` | Enable Go build engine.                                            |
| `root`        | string | `""`    | Subdirectory where `go.mod` is located.                            |
| `target_os`   | string | -       | **Required** if enabled. Target OS (`linux`, `darwin`, `windows`). |
| `target_arch` | string | -       | **Required** if enabled. Target architecture (`amd64`, `arm64`).   |
| `binary_name` | string | -       | **Required** if enabled. Name of the resulting binary.             |
| `build_flags` | string | `""`    | Additional flags for `go build`.                                   |

#### Frontend (`frontend`)

| Field                | Type         | Default                 | Description                                                                                                    |
| :------------------- | :----------- | :---------------------- | :------------------------------------------------------------------------------------------------------------- |
| `enabled`            | bool         | `false`                 | Enable Frontend build engine.                                                                                  |
| `root`               | string       | `""`                    | Subdirectory where `package.json` is located.                                                                  |
| `npm_command`        | string       | `npm ci ...`            | Command to install dependencies.                                                                               |
| `compile_command`    | string       | -                       | **Required** if enabled. Command to compile assets.                                                            |
| `cleanup_dev_deps`   | bool         | `false`                 | If true, removes `node_modules` after build and runs `production_command`.                                     |
| `production_command` | string       | `pnpm install --prod`   | Command to install production-only dependencies if `cleanup_dev_deps` is true.                                 |
| `reusable_paths`     | list[string] | `["node_modules", ...]` | Folders to reuse from previous release if `package.json` didn't change (e.g. `node_modules`, `dist`, `build`). |

## Post-Deployment Hooks (`post_deploy`)

A list of commands to run on the **remote server** after the release is extracted and before the final symlink switch.

- Commands are executed relative to the `app` directory of the **new release**.
- Final activation of the `current` symlink happens **only if all hooks pass**.
- If a hook fails, the entire deployment is rolled back automatically.

```yaml
post_deploy:
  - "php versaCLI cache:clear"
  - "php artisan migrate --force"
```

## Platform Considerations

### Robust Change Detection

versaDeploy tracks changes using SHA256 hashes of your files. Even if a folder is in `ignored_paths` (like `src/`), if it contains files with critical extensions (`.vue`, `.ts`, `.php`), changes WILL be detected to trigger a new build and deployment.

### Reusable Paths & Optimization

To keep deployments fast, versaDeploy uses **Linux Hardlinks** (`cp -al`) to carry over large folders (like `vendor` or `node_modules`) between releases if their configuration hasn't changed. This avoids unnecessary network transfers and dependency re-installs.

### Absolute Symlinks

The `current` symlink and all internal symlinks (for `shared_paths`) use **absolute paths** on the remote server, ensuring they work regardless of how a shell session is initialized.
