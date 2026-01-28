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

| Field           | Type         | Default | Description                                                                              |
| :-------------- | :----------- | :------ | :--------------------------------------------------------------------------------------- |
| `remote_path`   | string       | -       | **Required**. Absolute path on the remote server where the application will be deployed. |
| `hook_timeout`  | int          | `300`   | Timeout in seconds for each `post_deploy` hook.                                          |
| `route_files`   | list[string] | `[]`    | Files that, if changed, will trigger route cache regeneration flags.                     |
| `ignored_paths` | list[string] | `[...]` | Paths relative to project root that should be ignored by SHA256 tracking.                |

### 3. Build Configurations (`builds`)

versaDeploy supports three build engines: `php`, `go`, and `frontend`.

#### PHP (`php`)

| Field              | Type   | Default                | Description                                                               |
| :----------------- | :----- | :--------------------- | :------------------------------------------------------------------------ |
| `enabled`          | bool   | `false`                | Enable PHP build engine.                                                  |
| `root`             | string | `""`                   | Subdirectory where `composer.json` is located (relative to project root). |
| `composer_command` | string | `composer install ...` | Command to run for dependency installation.                               |

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

| Field             | Type   | Default      | Description                                                                                                   |
| :---------------- | :----- | :----------- | :------------------------------------------------------------------------------------------------------------ |
| `enabled`         | bool   | `false`      | Enable Frontend build engine.                                                                                 |
| `root`            | string | `""`         | Subdirectory where `package.json` is located.                                                                 |
| `npm_command`     | string | `npm ci ...` | Command to install dependencies.                                                                              |
| `compile_command` | string | -            | **Required** if enabled. Command to compile assets. Use `{file}` placeholder for individual file compilation. |

## Post-Deployment Hooks (`post_deploy`)

A list of commands to run on the **remote server** after the symlink has been switched to the new release.

- Commands are executed relative to the `current` directory on the server.
- If any command fails (non-zero exit code), the deployment is rolled back automatically.

```yaml
post_deploy:
  - "php artisan migrate --force"
  - "php artisan config:cache"
```

## Platform Considerations

### Local Development (Windows vs. Linux)

- versaDeploy automatically detects if it's running on Windows or Linux and uses the appropriate shell (`cmd.exe` via `COMSPEC` or `sh`).
- Build commands should ideally be platform-agnostic or use tools available on both (e.g., `npm`, `php`, `go`).

### Remote Server (Linux)

- The remote server is expected to be a Linux-based system with SSH/SFTP access.
- Hooks are executed in a standard shell. Use absolute paths or commands available in the environment's `PATH`.
