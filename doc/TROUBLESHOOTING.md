# 🩺 Troubleshooting

Common errors and how to fix them. Add `--debug` (and `--log-file deploy.log` to keep the output) to any command for more detail. `versa info <env>` is the quickest way to check SSH, the host key and SFTP.

- [Configuration](#configuration)
- [SSH connection](#ssh-connection)
- [Deploy](#deploy)
- [Builds](#builds)
- [Hooks](#hooks)
- [Services](#services)
- [After the deploy](#after-the-deploy)

---

## Configuration

### `Multiple configuration files found. Please select one`

More than one file in the directory matches the config names (`deploy*.yml`, `*_deploy*.yml`, …). Pick one, or pass it explicitly (required in scripts/CI): `--config deploy_production.yml`.

### `environment 'X' not found in configuration`

The name after the command must match a key under `environments:`. Check spelling and which file was loaded (`versa config validate`).

### `environment keys are mis-indented`

A setting (`ssh`, `remote_path`, `builds` …) is at the same indentation as the environment name. Indent every setting one level deeper than its environment.

### `ssh key not found` / `SSH key has insecure permissions`

`key_path` must point to an existing private key. On Linux/macOS the key must be private to you:

```bash
chmod 600 ~/.ssh/id_rsa
```

### `remote_path must be an absolute path`

Use a full path starting with `/`: `remote_path: "/var/www/app"`.

### `at least one build type must be enabled`

Enable at least one of `builds.php`, `builds.go`, `builds.frontend`, `builds.python`.

### A hook or value lost its `$...` part

`${NAME}`/`$NAME` anywhere in the config is replaced with **your local** environment variables, including inside hooks. For a variable of the server's shell write `$$NAME` (e.g. `echo $$HOME`). See [Environment variables](DEPLOY.md#environment-variables-in-the-config).

---

## SSH connection

### `Host key NOT verified (open ...known_hosts: ...)`

versa couldn't read the `known_hosts` file, so it connected without verifying the server's identity. Fix:

1. Connect once with `ssh user@host` and accept the key, or run `ssh-keyscan -p 22 host >> ~/.ssh/known_hosts`.
2. If you set `known_hosts_file`, check the path. `~/` is supported (versa ≤ 1.7.0 didn't expand `~` in this field; update with `versa self-update` or remove the line to use the default).
3. To refuse unverified connections, set `ssh.strict_host_key: true`.

### `knownhosts: key mismatch`

The key the server presented doesn't match `known_hosts`. Either the server was reinstalled / its keys regenerated, or something is intercepting the connection. Verify with the server's administrator, then remove the old entry and add the new one:

```bash
ssh-keygen -R your-server.com
ssh user@your-server.com      # accept the new key
```

### `ssh: handshake failed: ssh: no common algorithm for ...`

The server only supports old algorithms (typical of OpenSSH < 7, e.g. CentOS 5/6). Enable them for that environment:

```yaml
ssh:
  legacy_algorithms: true
```

For `no common algorithm for host key ... peer offered: [ssh-rsa ssh-dss]` with a `known_hosts` entry of type `ssh-rsa`, update versa (fixed after 1.7.0) or set `legacy_algorithms: true`.

### `SSH Authentication failed`

- The public key isn't in the server's `~/.ssh/authorized_keys` for that `user`, or `key_path` points to another key. Try `ssh -i <key_path> user@host`.
- On the server, `~/.ssh` must be `700` and `authorized_keys` `600`.
- If your key is in an agent, set `use_ssh_agent: true`.

### Connection timeout / `connection refused`

Check `host`, `port`, firewalls and VPN. `versa info <env>` shows whether TCP connects at all.

---

## Deploy

### `working directory has uncommitted changes`

versa deploys the last commit, and refuses to run while you have uncommitted changes so you don't think they were deployed. Commit them, or use `--skip-dirty-check` (uncommitted changes are still **not** deployed).

### `No changes detected - skipping deployment`

Nothing changed since the last deploy recorded in the server's `deploy.lock`. Did you commit? To deploy anyway (e.g. after a rollback, or to rebuild dependencies), use `--force`.

### `deploy.lock not found on remote server`

First deploy to this environment (or `remote_path` changed). Use `versa deploy <env> --initial-deploy`.

### `Deployment lock already held`

Another deploy is running, or a previous one was interrupted (Ctrl+C, lost connection) and left the lock behind. If you are sure nobody else is deploying:

```bash
versa exec production "rmdir /var/www/app/.versa.lock"
```

### `deployment aborted: timeout of 600s exceeded`

The whole deploy took longer than `deploy_timeout`. Raise it for large projects or slow links, or enable `incremental_upload`.

### Not enough disk space on the server

versa checks free space before uploading. Free space, or lower `releases_to_keep` so fewer old releases are kept.

### `tar not found` on the server

Install `tar` (and `gzip`) on the server; deploys need them.

---

## Builds

### `PHP build tool 'composer' not found` / `Frontend build tool '...' not found` / `Go compiler not found`

Builds run on **your** machine. Install the tool and make sure it's in `PATH` in the terminal you run versa from (open a new terminal after changing `PATH`).

### Dependencies weren't reinstalled

Dependencies are reinstalled when a manifest or its lockfile changes (`composer.json`/`composer.lock`, `package.json`/`package-lock.json`/`pnpm-lock.yaml`/`yarn.lock`, `go.mod`/`go.sum`, Python requirements/`pyproject.toml`/`poetry.lock`/`Pipfile`/`Pipfile.lock`), and only if that file was committed. To force a clean reinstall of `vendor`/`node_modules`, deploy with `--force`.

### Go: target doesn't match the server

Before building, versa compares `go.target_os`/`target_arch` with the server and with the minimum kernel your Go version supports. Set the right `target_arch` (`uname -m` on the server: `x86_64` → `amd64`, `aarch64` → `arm64`), or build with an older Go for very old kernels.

### A file is missing on the server

- Is it committed? Only tracked, committed files are deployed.
- Is it under `ignored_paths`? Those are removed from the release.
- Is it under `shared_paths`? Then the server's copy in `shared/` is used instead.

---

## Hooks

### `post-deploy hook failed (rolled back to ...)`

A `post_deploy` command exited with an error, so `current` was switched back. Run it by hand in the new release to see why:

```bash
versa exec production "cd /var/www/app/releases/<version>/app && php artisan migrate --force"
```

Common causes: the command isn't in the non-interactive shell's `PATH` (use the full path, e.g. `/usr/bin/php8.2`), missing `.env`, or permissions. Re-run hooks on the active release with `versa hooks <env>`.

### `pre_deploy_local hook failed: exec: "sh": executable file not found` (Windows)

Local hooks run with `sh`. Install Git for Windows and add `C:\Program Files\Git\bin` to `PATH`.

### A hook was killed after 300 s

Raise `hook_timeout` (seconds).

---

## Services

### `service install failed: the deploy user needs passwordless sudo`

The SSH user can't run the init commands with `sudo -n`. Print the line it needs and add it on the server:

```bash
versa service production sudoers
# on the server:
sudo visudo -f /etc/sudoers.d/versa
```

`versa info <env>` tells you whether it's fixed.

### `sudo: sorry, you must have a tty to run sudo`

The server has `Defaults requiretty` (default on RHEL/CentOS up to 7). Add the `Defaults:<user> !requiretty` line that `versa service <env> sudoers` prints.

### `sudo: sorry, a password is required` although the sudoers line is there

The rule must match the exact command and path. Regenerate it on that server with `versa service <env> sudoers` (it uses the server's paths) and replace the old line. On RHEL/CentOS 5 there is no `/etc/sudoers.d`: the lines go in `/etc/sudoers`.

### `services failed to start (rolled back to ...)` / `<name> is not running`

The process exited within `start_wait` seconds, so versa went back to the previous release. The output above the error shows the last log lines. Then:

```bash
versa service production logs --name <name>      # full log
versa exec production "cd /var/www/app/current/app && /bin/sh /var/www/app/.versa/<name>.sh"   # run it by hand
```

Usual causes: a missing variable in `shared/.env`, the port already in use (an old process started by hand: stop it), a binary built for the wrong architecture, or a missing Python package. If your app takes longer to fail or to start, raise `start_wait`.

### `failed to prepare the Python virtualenv`

- `python3 not found on the server`: install Python 3, or set `python.python_command` to its name or path (e.g. `python3.11`).
- `could not create the virtualenv`: install the venv module (`apt install python3-venv` on Debian/Ubuntu).
- A `pip install` error: the package needs build tools or system libraries on the server (`build-essential`, `python3-dev`, `libpq-dev` …).
- `poetry`/`pipenv not found`: install it on the server, or use `package_manager: pip`.

### `python.build_binary needs versa to run on Linux`

PyInstaller builds for the OS it runs on, so a binary built on Windows or macOS can't run on the server. Deploy from Linux (WSL or CI), or remove `build_binary` and let versa run the code with a virtualenv on the server.

### The service doesn't start after a reboot (SysV / OpenRC)

Check it's enabled: `chkconfig --list <name>` (RHEL/CentOS) or `rc-update show` (Alpine). versa enables it when installing; run `versa service <env> install` to repeat. Without systemd, crashed processes are not restarted automatically.

---

## After the deploy

### The site still shows the old version

PHP-FPM caches where `current` used to point. Add the reload to `services_reload` and run it now:

```yaml
services_reload:
  - "sudo systemctl reload php8.2-fpm"
```

```bash
versa services-reload production
```

With Nginx, also use `$realpath_root` in `SCRIPT_FILENAME`/`DOCUMENT_ROOT`. See [Getting Started](GETTING_STARTED.md#24-web-server).

### 404 / wrong document root

The app lives in `current/app/`, so the web root is `<remote_path>/current/app/public` (or your framework's public directory), not `<remote_path>/current/public`.

### `.env` became an empty directory

The shared path didn't exist in `shared/` on the first deploy, so versa created it as a directory. Fix it on the server:

```bash
rmdir /var/www/app/shared/.env && nano /var/www/app/shared/.env
```

### Rolled back, but PHP still serves the new code

Rollbacks run `services_reload`; check that it reloads PHP-FPM (see above) and that its commands succeed (a failure is printed as a warning). Run it again with `versa services-reload <env>`.
