# 🚀 Getting Started

From zero to your first deploy, step by step. The example deploys a PHP (Laravel) app to an Ubuntu server with Nginx/Apache + PHP-FPM; other stacks follow the same steps with a different `builds` section (see the [Configuration Reference](DEPLOY.md#builds-builds)).

1. [Prerequisites](#1-prerequisites)
2. [Prepare the server](#2-prepare-the-server)
3. [Create the configuration](#3-create-the-configuration)
4. [Check everything before deploying](#4-check-everything-before-deploying)
5. [First deploy](#5-first-deploy)
6. [Everyday use](#6-everyday-use)
7. [Go or Python apps: run them as a service](#7-go-or-python-apps-run-them-as-a-service)

---

## 1. Prerequisites

### On your machine (where you run `versa`)

- **versa** installed: see [INSTALL.md](INSTALL.md).
- **Git**: versa deploys the last commit of the repository you run it from.
- **The build tools your project uses**, in `PATH`: `composer` for PHP, `npm`/`pnpm`/`yarn` for frontend, `go` for Go. Builds run locally, not on the server. (Python dependencies are the exception: they're installed on the server.)
- **Windows**: hooks that run locally (`pre_deploy_local`) use `sh`. Install Git for Windows and add `C:\Program Files\Git\bin` to `PATH`.
- An **SSH key** that can log in to the server.

### On the server

- Linux with SSH and SFTP enabled. Old distributions (e.g. CentOS 5) work too, see [`legacy_algorithms`](DEPLOY.md#ssh-connection-ssh).
- `tar` and `gzip` (present on virtually every distribution).
- A user that can write to the deploy directory.

## 2. Prepare the server

### 2.1 Deploy user and SSH key

From your machine, copy your public key to the server and accept its host key once:

```bash
ssh-copy-id deploy@your-server.com        # or append the .pub file to ~/.ssh/authorized_keys on the server
ssh deploy@your-server.com                # answer "yes" so the server is saved in ~/.ssh/known_hosts
```

> [!TIP]
> Connecting once with `ssh` saves the server in `~/.ssh/known_hosts`, which versa uses to verify the server. Without it versa still connects but warns `Host key NOT verified`.

### 2.2 Deploy directory

```bash
sudo mkdir -p /var/www/my-app
sudo chown -R deploy:www-data /var/www/my-app
```

versa creates `releases/`, `shared/` and the `current` symlink inside it.

### 2.3 Shared files

Files that must survive between releases (like `.env`) live in `shared/`. Create them **before** the first deploy; versa would otherwise create them as empty directories:

```bash
mkdir -p /var/www/my-app/shared
nano /var/www/my-app/shared/.env
```

### 2.4 Web server

Each release is stored as `releases/<version>/app/`, and `current` points to the active release. Your web root is therefore **`/var/www/my-app/current/app/<public dir>`**.

**Nginx**: use `$realpath_root` so PHP resolves the real release path, not the symlink:

```nginx
server {
    root /var/www/my-app/current/app/public;

    location ~ \.php$ {
        include fastcgi_params;
        fastcgi_pass unix:/run/php/php8.2-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        fastcgi_param DOCUMENT_ROOT $realpath_root;
    }
}
```

**Apache**:

```apache
<VirtualHost *:80>
    DocumentRoot /var/www/my-app/current/app/public

    <Directory /var/www/my-app/current/app/public>
        Options +FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>

    # Don't serve stale static files from the kernel cache after a switch
    EnableSendfile Off
    EnableMMAP Off

    <FilesMatch \.php$>
        SetHandler "proxy:unix:/run/php/php8.2-fpm.sock|fcgi://localhost"
    </FilesMatch>
</VirtualHost>
```

### 2.5 Allow reloading services without a password

PHP-FPM caches where `current` points, so it must be reloaded after each deploy (versa does it through [`services_reload`](DEPLOY.md#service-reload-services_reload)). Allow the deploy user to do that without a password, with `sudo visudo -f /etc/sudoers.d/deploy`:

```
deploy ALL=(ALL) NOPASSWD: /bin/systemctl reload php8.2-fpm
deploy ALL=(ALL) NOPASSWD: /bin/systemctl reload nginx
```

## 3. Create the configuration

In the root of your repository:

```bash
versa init
```

Choose `1` (server with SSH). This creates a commented `deploy.yml`. Edit it; a minimal Laravel setup looks like:

```yaml
project: "my-app"

environments:
  production:
    ssh:
      host: "your-server.com"
      user: "deploy"
      key_path: "~/.ssh/id_rsa"

    remote_path: "/var/www/my-app"

    builds:
      php:
        enabled: true
      frontend:
        enabled: true
        npm_command: "npm ci"
        compile_command: "npm run build"

    shared_paths:
      - ".env"
      - "storage"

    services_reload:
      - "sudo systemctl reload php8.2-fpm"

    post_deploy:
      - "php artisan migrate --force"
      - "php artisan config:cache"
```

Commit `deploy.yml` (and everything you want deployed): versa only deploys committed files.

## 4. Check everything before deploying

```bash
versa config validate production    # the file is valid (no connection)
versa info production               # SSH, host key, SFTP and server details
```

Fix whatever these report before going on; [TROUBLESHOOTING.md](TROUBLESHOOTING.md) covers the usual errors.

## 5. First deploy

The server has no `deploy.lock` yet, so tell versa this is the first deploy:

```bash
versa deploy production --initial-deploy
```

versa builds the project, uploads it, links `shared_paths`, switches `current`, and reloads PHP-FPM. Before running the `post_deploy` hooks it asks for confirmation: check that `shared/.env` is right, then answer `y`.

## 6. Everyday use

```bash
git commit -am "New feature"
versa deploy production             # builds and ships only if something changed
```

| I want to…                                  | Command                                     |
| :------------------------------------------ | :------------------------------------------ |
| See what would be deployed                  | `versa diff production`                     |
| See releases on the server                  | `versa status production`                   |
| Go back to the previous release             | `versa rollback production`                 |
| Go back to a specific release               | `versa rollback production --to 20261001-183807` |
| Redeploy even though nothing changed        | `versa deploy production --force`           |
| Follow the app log                          | `versa logs production`                     |
| Run a command on the server                 | `versa exec production "php artisan queue:restart"` |
| Check / restart a Go or Python service      | `versa service production [restart]`        |
| Do all of this from a full-screen interface | `versa`                                     |

## 7. Go or Python apps: run them as a service

A PHP app is served by PHP-FPM, but a Go binary or a Python server is a process that has to be running. Declare it under `services` and versa takes care of the rest: it installs it in the server's init system (systemd, or `/etc/init.d` on old servers), starts it at boot, restarts it on every deploy and rollback, and rolls the deploy back if it doesn't stay up.

**Go API:**

```yaml
environments:
  production:
    # ssh, remote_path …
    builds:
      go:
        enabled: true
        target_os: "linux"
        target_arch: "amd64"
        binary_name: "api"
        deploy_path: "bin"
    shared_paths:
      - ".env"
    services:
      - name: "myapi"
        env_file: ".env"            # /var/www/my-app/shared/.env
        environment:
          PORT: "8080"
```

**Python (FastAPI) API:** dependencies are installed on the server, in a virtualenv per release. The server needs `python3` and `python3-venv`.

```yaml
    builds:
      python:
        enabled: true
        web_server: true
        web_framework: "fastapi"
        entry_point: "main.py"      # main.py defines `app`
        web_port: 8000
    services:
      - name: "myapi"
        env_file: ".env"
```

Then:

```bash
versa service production sudoers    # prints the sudoers line the deploy user needs
# on the server: sudo visudo -f /etc/sudoers.d/versa   (paste the line)
versa info production               # confirms sudo is OK
versa deploy production
versa service production            # is it running?
versa service production logs       # follow its output
```

Put a reverse proxy (Nginx) in front of the port if the service must be reachable from outside. All options: [Services](DEPLOY.md#services-services).

### Next steps

- [CLI Reference](CLI_REFERENCE.md): every command and flag.
- [Configuration Reference](DEPLOY.md): hooks, health checks, notifications, incremental uploads, local mode.
