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

Rolls back to the previous stable release.

**Arguments:**

- `environment`: The name of the environment.

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
