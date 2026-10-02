# 📖 CLI Reference

Every `versa` command, its arguments and flags, and what it does on your machine and on the server.

- [Conventions](#conventions)
- [Global flags](#global-flags)
- [Choosing the configuration file](#choosing-the-configuration-file)
- Commands
  - Setup: [`init`](#versa-init) · [`config validate`](#versa-config-validate) · [`ssh-test` / `info`](#versa-ssh-test--versa-info)
  - Deploying: [`deploy`](#versa-deploy) · [`diff`](#versa-diff) · [`rollback`](#versa-rollback) · [`status`](#versa-status)
  - Operating the server: [`exec`](#versa-exec) · [`logs`](#versa-logs) · [`hooks`](#versa-hooks) · [`services-reload`](#versa-services-reload) · [`service`](#versa-service)
  - The tool itself: [`version`](#versa-version) · [`self-update`](#versa-self-update)
- [Interactive TUI](#interactive-tui)
- [Which commands work in local mode](#which-commands-work-in-local-mode)
- [Exit codes and error output](#exit-codes-and-error-output)

---

## Conventions

```
versa <command> <environment> [arguments] [flags]
```

- `<environment>` is a key under `environments:` in your config file (`production`, `staging`, …). It is required by every command that touches a server.
- Run `versa` **from the root of your Git repository**. versa deploys the repository in the current directory and looks for the config file there.
- `versa <command> --help` prints the built-in help for any command.

## Global flags

Available on every command.

| Flag               | Default      | Description                                                                                 |
| :----------------- | :----------- | :------------------------------------------------------------------------------------------ |
| `--config <path>`  | `deploy.yml` | Configuration file to use. Disables auto-discovery (see below).                             |
| `--verbose`        | `false`      | More detailed output.                                                                       |
| `--debug`          | `false`      | Debug output (internal steps, remote commands).                                             |
| `--log-file <path>`| —            | Also write the log to this file. Handy to keep a history: `--log-file deploy.log`.          |
| `--no-gui`         | `false`      | With no command, print help instead of launching the TUI.                                   |
| `--gui`            | `false`      | Launch the TUI. Kept for backward compatibility: plain `versa` already does this.           |
| `-v`, `--version`  | —            | Print the version (same as `versa version`).                                                |
| `-h`, `--help`     | —            | Help for any command.                                                                       |

## Choosing the configuration file

Unless you pass `--config`, versa looks in the current directory for files matching any of these names (`.yml` or `.yaml`):

```
deploy.yml   deploy_*.yml   versa_deploy.yml   versa_deploy_*.yml   *_deploy.yml   *_deploy_*.yml
```

| Files found | What happens                                                          |
| :---------- | :-------------------------------------------------------------------- |
| None        | `deploy.yml` is used (and the command fails if it doesn't exist).     |
| One         | That file is used.                                                    |
| Several     | versa lists them and asks you to pick one by number.                  |

For scripts and CI, always pass `--config` so versa never stops to ask:

```bash
versa deploy production --config versa_deploy_frontend.yml
```

> [!NOTE]
> `versa init` does not auto-discover: it always writes to the `--config` path (default `deploy.yml`).

---

## `versa init`

Creates a commented starter configuration in the current directory.

```bash
versa init                       # asks: [1] VPS/server over SSH, [2] local hosting without SSH
versa init --local               # skip the question, generate a local-mode config
versa init --config deploy_staging.yml
```

| Flag      | Default | Description                                                                    |
| :-------- | :------ | :----------------------------------------------------------------------------- |
| `--local` | `false` | Generate a [local-mode](DEPLOY.md#local-mode-no-ssh) environment without asking. |

Fails if the target file already exists. Edit the generated file, then run `versa config validate`.

---

## `versa config validate`

Parses and validates the config file **without connecting to any server**.

```bash
versa config validate               # validate every environment
versa config validate production    # also print a summary of one environment
```

It checks required fields, that `ssh.key_path` exists (and on Linux/macOS, that it is `chmod 600`), that `remote_path` is absolute, that at least one build is enabled and that each enabled build has its required fields. With an environment name it prints:

```
✅ deploy.yml is valid (2 environment(s): production, staging)

Environment "production":
  mode:        ssh
  ssh:         deploy@10.0.0.5:22
  remote_path: /var/www/app
  builds:      php, frontend
  hooks:       pre_deploy_local=1 pre_deploy_server=0 post_deploy=2
```

---

## `versa ssh-test` / `versa info`

`info` is an alias of `ssh-test`. Connects to the server, prints what it finds, and checks that SFTP works.

```bash
versa info production
```

```
🔍 Testing SSH connection to production (deploy@10.0.0.5)...
✅ SSH connection established successfully!
🔍 Probing server...
   Host:            web01
   OS:              Ubuntu 22.04.4 LTS
   Kernel:          5.15.0-105-generic x86_64
   Init / libc:     systemd / glibc
   CPU:             Intel(R) Xeon(R) (4 cores)
   RAM:             2.1G / 7.8G
   Disk (deploy):   12G / 40G (30%)
   php:             8.2.18
   node:            —
   tools:           tar gzip timeout perl ...
🔍 Testing SFTP subsystem...
✅ SFTP subsystem working.
```

Run this first on a new server: it surfaces SSH, host-key and permission problems before a real deploy. When the environment has [`services`](DEPLOY.md#services-services), it also checks that the SSH user can restart them with passwordless sudo. It warns if `tar` is missing on the server, because deploys need it. The probe is plain POSIX and works on old servers too (RHEL/CentOS 5, busybox).

---

## `versa deploy`

Builds the committed state of your repository and deploys it to one or more environments.

```bash
versa deploy production
versa deploy production --initial-deploy     # first deploy to this environment
versa deploy production --dry-run            # show what would change; deploy nothing
versa deploy production --force              # redeploy even with no changes
versa deploy staging,production              # several environments, one after another
```

| Flag                 | Default | Description                                                                                                                                                                    |
| :------------------- | :------ | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--initial-deploy`   | `false` | Allow deploying when the server has no `deploy.lock` yet (first deploy). Also asks for confirmation before running `post_deploy` hooks, so you can put `.env` etc. in place. |
| `--dry-run`          | `false` | Connect, detect changes and list them, then stop. Nothing is built or uploaded and `pre_deploy_local` hooks are not run.                                                    |
| `--force`            | `false` | Deploy even when no file changed since the last deploy (e.g. to redeploy after a rollback).                                                                                  |
| `--skip-dirty-check` | `false` | Don't refuse to deploy when the working tree has uncommitted changes. Those changes are still **not** deployed: versa always deploys the last commit (`HEAD`).              |

### What a deploy does

1. **Local checks**: build tools are in `PATH`, the directory is a Git repository, `pre_deploy_local` hooks pass, and the working tree is clean (unless `--skip-dirty-check`).
2. **Clone**: `HEAD` is cloned to a temporary directory. Only committed files are deployed.
3. **Connect and lock**: opens SSH and creates `<remote_path>/.versa.lock` so two deploys can't run at once.
4. **Change detection**: SHA256 hashes of every file are compared with the server's `deploy.lock`. With no changes, versa prints `No changes detected` and stops (unless `--force`).
5. **Pre-flight**: probes the server (OS, `tar`, and for Go builds, whether the target fits the server's kernel/arch).
6. **Build**: runs the enabled builders (PHP/Go/Frontend/Python) concurrently.
7. **Upload**: compresses the release into chunks and uploads them over SFTP in parallel, into `releases/<version>/`. With `incremental_upload: true`, only changed files are sent.
8. **Prepare**: links `shared_paths`, reuses dependencies from the previous release (`vendor`, `node_modules`, `.venv` …) via hardlinks, restores `preserved_paths`, and for Python creates the virtualenv on the server when it couldn't be reused.
9. **Activate**: runs `pre_deploy_server` hooks, atomically switches `current → releases/<version>`, runs `services_reload`, runs `post_deploy` hooks, installs and restarts [`services`](DEPLOY.md#services-services) (checking they stay up), then the `health_check`.
10. **Finish**: writes the new `deploy.lock`, deletes old releases beyond `releases_to_keep`, sends the webhook notification, and releases the lock.

If a `post_deploy` hook, a service or the health check fails, `current` is switched back to the previous release (and services are restarted on it). See [DEPLOY.md](DEPLOY.md#hooks) for the hook rules.

### Multiple environments

`versa deploy staging,production` deploys to each environment in order, with a separate build per environment, and stops at the first failure. To build once and ship the same artifact to several servers (one config file per server), use multi-deploy in the [TUI](#interactive-tui) (Switch Config view).

### Local-mode environments

For an environment with `local: true`, `deploy` builds the project and writes a complete release to `local_path` (replacing what was there), then runs `post_deploy` hooks locally. Upload the contents of `local_path/app` to your hosting yourself.

---

## `versa diff`

Lists the files a deploy would ship: the committed `HEAD` compared with the server's `deploy.lock`.

```bash
versa diff production
```

It is a dry-run deploy that also skips the dirty check (uncommitted changes don't matter, since only `HEAD` is compared). The output groups files by type (PHP, Twig, Go, Frontend, Python, Other) and says which dependency manifests changed (`composer`, `package.json`, `go.mod`, Python requirements, routes) and will trigger a rebuild. Only for SSH environments.

---

## `versa rollback`

Points `current` back to an earlier release that is still on the server.

```bash
versa rollback production                       # to the most recent release other than the active one
versa rollback production --to 20261001-183807  # to a specific release
```

| Flag             | Default | Description                                                                                       |
| :--------------- | :------ | :------------------------------------------------------------------------------------------------ |
| `--to <release>` | —       | Release to activate. Release names are UTC timestamps `YYYYMMDD-HHMMSS`; list them with `versa status`. |

- Rollback only switches the symlink: nothing is rebuilt and `post_deploy` hooks are **not** run. Database migrations are not reverted.
- `services_reload` runs and [`services`](DEPLOY.md#services-services) are restarted after switching, so PHP-FPM and your Go/Python processes serve the restored release right away.
- Only releases still on the server can be restored (see `releases_to_keep`).
- `deploy.lock` is not changed. If you then want to redeploy the same commit, use `versa deploy <env> --force`.

---

## `versa status`

Shows the active release and every release on the server.

```bash
versa status production
```

```
[INFO] Status for production:
[INFO] Current release: 20261001-183807
[INFO] Available releases: 3
[INFO]     20260930-101500
[INFO]     20260930-170212
[INFO]   → 20261001-183807
```

`→` marks the release `current` points to.

---

## `versa exec`

Runs a command on the server over SSH and prints its output.

```bash
versa exec production "df -h"
versa exec production "sudo systemctl status php8.2-fpm"
versa exec production "tail -50 /var/log/nginx/error.log | grep 502"
```

- Everything after the environment is joined with spaces into one command. **Quote it** so your local shell doesn't interpret pipes, redirections or `&&`.
- It runs in the SSH user's login shell, starting in their home directory (not in the release). Use `cd /var/www/app/current/app && …` if you need the app directory.
- Limited to `hook_timeout` seconds (default 300).

---

## `versa logs`

Follows a log file on the server (`tail -f`). Press `Ctrl+C` to stop.

```bash
versa logs production                                   # <remote_path>/current/app/storage/logs/laravel.log
versa logs production /var/log/nginx/error.log
versa logs production /var/log/syslog --lines 200
```

| Argument / flag | Default                                                 | Description                                   |
| :-------------- | :------------------------------------------------------ | :-------------------------------------------- |
| `[path]`        | `<remote_path>/current/app/storage/logs/laravel.log`    | Absolute path of the file on the server.      |
| `--lines <n>`   | `50`                                                    | Lines of history to show before following.    |

The default path is Laravel's log. For other frameworks, always pass the path.

---

## `versa hooks`

Runs the `post_deploy` hooks again on the active release, without deploying.

```bash
versa hooks production          # all post_deploy hooks, in order
versa hooks production 0 2      # only hooks #0 and #2 (0-based, in the order they appear in the config)
```

Hooks run in `<active release>/app`, with `hook_timeout`. A `parallel:` block counts as one index. Unlike during a deploy, a failure here stops the command but does **not** roll back.

---

## `versa services-reload`

Runs the environment's `services_reload` commands (e.g. reload PHP-FPM and the web server) without deploying.

```bash
versa services-reload production
```

Each command has a 30 s timeout. Failures are reported as warnings. Useful when PHP keeps serving stale code, or after changing the server's PHP/web server config.

---

## `versa service`

Manages the processes listed under [`services`](DEPLOY.md#services-services) (a Go API, a Python web server…). Deploys already install, enable and restart them; use this to check on them or act by hand.

```bash
versa service production                     # status of every service (default action)
versa service production restart
versa service production logs --name api     # follow the log; Ctrl+C to stop
versa service production sudoers             # sudoers line the SSH user needs on this server
```

| Action      | What it does                                                                                       |
| :---------- | :------------------------------------------------------------------------------------------------- |
| `status`    | Whether each service is running (`systemctl status`, or the init script's `status`). Default.      |
| `start`     | Start, wait `start_wait` seconds and check it's still running.                                     |
| `stop`      | Stop.                                                                                              |
| `restart`   | Restart and check it stays up.                                                                     |
| `logs`      | Follow the log: `journalctl -u <name>` on systemd, `/var/log/<name>.log` otherwise.                 |
| `install`   | Rewrite the service files and (re)install + enable them at boot, without deploying. Use it after changing `services` in the config. |
| `uninstall` | Stop, disable at boot and remove from the init system.                                             |
| `sudoers`   | Print the sudoers line the SSH user needs, with this server's binary paths.                        |

| Flag          | Default | Description                                                                 |
| :------------ | :------ | :-------------------------------------------------------------------------- |
| `--name <n>`  | all     | Act on one service. Required for `logs` when there are several.             |
| `--lines <n>` | `50`    | `logs`: lines of history before following.                                  |

Unless `ssh.user` is `root`, the commands run with `sudo -n`: see [Permissions](DEPLOY.md#permissions-sudo).

---

## `versa version`

```bash
versa version      # versaDeploy 1.7.0
```

## `versa self-update`

Checks the [latest GitHub release](https://github.com/kriollo/versaDeploy/releases) and, if it is newer, downloads the binary for your OS/architecture, verifies its SHA256 checksum when one is published, and replaces the running `versa`.

```bash
versa self-update
```

You need write permission on the folder where `versa` is installed (on Linux, `sudo` if it's in `/usr/local/bin`).

---

## Interactive TUI

Running `versa` with no command opens a full-screen interface (`versa --no-gui` prints help instead). It loads the config file from the current directory if there is one; otherwise use the **Switch Config** view to choose one. The SSH connection to an environment opens the first time you need it.

**Everywhere**

| Key              | Action                                          |
| :--------------- | :---------------------------------------------- |
| `←` / `→`        | Previous / next view                            |
| `Tab`            | Focus the sidebar (pick a view or environment)  |
| `↑` / `↓`, `Enter` | Move, select                                  |
| `F5`             | Refresh the current view                        |
| `c`              | Connect / reconnect to the active environment   |
| `D`              | Go to Operations to deploy                      |
| `q`, `Ctrl+C`    | Quit                                            |

**Views**

| View              | What it shows                                    | Keys                                                                                                                         |
| :---------------- | :----------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------------- |
| **Dashboard**     | Server info (same as `versa info`), active release, runtimes and tools | `F5` refresh                                                                                       |
| **Releases**      | Releases on the server, active one marked        | `Enter` browse its files · `R` roll back to it (asks `y`/`n`; runs `services_reload` and restarts `services`)                                                                |
| **Files**         | Remote file browser (follows symlinks)           | `Enter` open dir / view file · `Backspace` up · `d` download · `u` upload (`Space` select files, `Enter` open dir) · `Delete` delete · `Esc` close |
| **Shared**        | Contents of `<remote_path>/shared`               | `Enter` open dir                                                                                                             |
| **Operations**    | Deploy options and live log                      | `↑`/`↓` + `Enter` toggle Dry run, Force, Initial deploy, Skip dirty check, Debug, log file · `D` deploy · `R` rollback · `s` SSH test · `t` status · `h` re-run hooks · `l` services reload · `u` self-update · `Esc` close log |
| **Terminal**      | Remote shell on the server                       | `Tab` completion · `Esc` leave                                                                                               |
| **Config**        | The loaded config file                           | `e` edit · `Ctrl+S` save · `Esc` cancel                                                                                      |
| **Switch Config** | Config files found in the directory              | `Enter` load · `Space` select several · `m` multi-deploy the selected configs with a single build · `Esc` back               |

---

## Which commands work in local mode

Environments with `local: true` have no server, so only these commands apply to them:

| Command                        | Local mode                                     |
| :----------------------------- | :--------------------------------------------- |
| `deploy`                       | ✅ Builds and writes the release to `local_path` |
| `config validate`              | ✅                                             |
| `diff`, `rollback`, `status`, `exec`, `hooks`, `services-reload`, `service`, `logs`, `ssh-test`/`info` | ❌ SSH environments only |

---

## Exit codes and error output

`versa` exits with `0` on success and `1` on any error, so it can be used in CI and scripts. Known errors are printed with a code, the underlying details and, when possible, a suggested fix:

```
[ERROR] Deployment lock already held
Code: CONFIG_INVALID
Details: sftp: "Failure" (SSH_FX_FAILURE)

Suggestion: Another deployment is currently in progress. If you are sure no one else
is deploying, manually remove the directory: /var/www/app/.versa.lock
```

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common errors.
