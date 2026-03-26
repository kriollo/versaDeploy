# 📖 CLI Command Reference

This document provides a detailed reference for all `versaDeploy` commands and their flags.

## Global Flags

These flags are available for all commands:

| Flag         | Shortcut | Default      | Description                               |
| :----------- | :------- | :----------- | :---------------------------------------- |
| `--config`   | -        | `deploy.yml` | Path to the configuration file.           |
| `--debug`    | -        | `false`      | Enable debug mode (detailed diagnostics). |
| `--verbose`  | -        | `false`      | Enable verbose output.                    |
| `--log-file` | -        | -            | Path to a file where logs will be saved.  |

---

## `versa init`

Initializes a new `deploy.yml` configuration file in the current directory.

---

## `versa deploy [environment]`

Deploys your application to the specified environment.

**Arguments:**

- `environment`: The name of the environment (e.g., `production`, `staging`).

**Flags:**
| Flag | Default | Description |
| :--- | :--- | :--- |
| `--initial-deploy` | `false` | Required for the very first deployment to an environment. |
| `--force` | `false` | Force a full build and redeploy even if no changes are detected. |
| `--skip-dirty-check` | `false` | Bypass the check for uncommitted changes (only committed code will be deployed). |
| `--dry-run` | `false` | Show what would be deployed without actually performing the deployment. |

---

## `versa rollback [environment]`

Rolls back to the previous stable release, or to a specific version using `--to`.

**Arguments:**

- `environment`: The name of the environment.

**Flags:**
| Flag | Default | Description |
| :--- | :--- | :--- |
| `--to` | - | Target a specific release version (e.g., `20240101_120000`). |

---

## `versa exec [environment] [command]`

Executes an arbitrary command on the remote server via SSH.

**Arguments:**

- `environment`: The name of the environment.
- `command`: The command to execute (all remaining arguments are joined).

**Examples:**

```bash
versa exec production "df -h"
versa exec production "sudo systemctl status php8.2-fpm"
versa exec production "cat /var/log/apache2/error.log | tail -50"
```

---

## `versa hooks [environment] [indices...]`

Re-executes post_deploy hooks on the currently active release.

**Arguments:**

- `environment`: The name of the environment.
- `indices` (optional): Zero-based indices of specific hooks to run. If omitted, all hooks run.

**Examples:**

```bash
versa hooks production          # Run all post_deploy hooks
versa hooks production 0 2      # Run only hooks at index 0 and 2
```

---

## `versa logs [environment] [path]`

Tail remote log files in real-time using `tail -f`. Press `Ctrl+C` to stop.

**Arguments:**

- `environment`: The name of the environment.
- `path` (optional): Absolute path to the log file on the server. Defaults to Laravel's `storage/logs/laravel.log` inside the active release.

**Flags:**

| Flag      | Default | Description                                      |
| --------- | ------- | ------------------------------------------------ |
| `--lines` | `50`    | Number of initial lines to show before following |

**Examples:**

```bash
versa logs production                                    # Default Laravel log
versa logs production /var/log/syslog                    # System log
versa logs production /var/log/apache2/error.log --lines 100
```

---

## `versa status [environment]`

Shows the current deployment status, active release, and history on the remote server.

---

## `versa ssh-test [environment]`

Tests the SSH connection and SFTP functionality for the specified environment.

---

## `versa self-update`

Checks for the latest version on GitHub and automatically updates the `versa` binary.

---

## `versa version`

Prints the current version of `versaDeploy`.
