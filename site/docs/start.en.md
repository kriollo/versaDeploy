# Getting started

From a brand-new VPS to your app online. If you'd rather watch it, every step is in the [videos](videos.md).

!!! tip "What you need"
    - A **VPS** running Ubuntu, Debian, Rocky, Alma or RHEL, with **root over SSH** (any provider gives you that: Hetzner, DigitalOcean, Vultr, OVH…).
    - Your project in **git** on your machine, with what it builds with: `composer`, `npm`, `go`… (Python is installed on the server).
    - Optional: a **domain** pointing to the VPS's IP, for HTTPS with Let's Encrypt.

## 1. Install versa

=== "Linux and macOS"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/kriollo/versaDeploy/main/install.sh | sh
    versa version
    ```

=== "Windows"

    Download `versa_windows_amd64.exe` from [Releases](https://github.com/kriollo/versaDeploy/releases/latest), rename it to `versa.exe` and put it in a folder on your `PATH`. Install Git for Windows for local hooks. Details in [Install](reference/INSTALL.md).

## 2. Your SSH key on the server

If you don't have an SSH key yet, create it and copy it to the server (once):

```bash
ssh-keygen -t ed25519                 # Enter to everything
ssh-copy-id root@YOUR_VPS_IP          # asks for root's password one last time
ssh root@YOUR_VPS_IP 'echo ok'        # must log in without a password
```

## 3. The `deploy.yml` file

In your project's folder, start from your framework's example:

```bash
curl -fsSL -o deploy.yml https://raw.githubusercontent.com/kriollo/versaDeploy/main/examples/deploy.laravel.yml
```

There are examples for Laravel, Next.js, Nuxt, FastAPI, Flask and Gin in [Examples](examples.md). Open it and change what's marked `# change`:

- `project`: your project's name.
- `ssh.host`: your VPS's IP or domain.
- `provision.domain`: your app's domain (or keep it and use the IP meanwhile).
- `remote_path`, `db_name`, `db_user`: usually replacing `myapp` with your name is enough.

Passwords don't go in the file: `db_password: "${DB_PASS}"` is read from a variable.

```bash
export DB_PASS='a-strong-password'
versa config validate production
git add deploy.yml && git commit -m "Configure versaDeploy"
```

!!! info "versa deploys what's committed"
    versa takes the last commit, not your unsaved files: commit before each deploy.

## 4. Set up the server

```bash
versa
```

The interface opens. Press **→** until the **Services** view: versa detects the server and ticks what your app needs (nginx, PHP, the database, the `deploy` user with your key, backups, fail2ban…). Check it, then press **I** and **Y** to install. It takes a few minutes. Quit with **q**.

If the server already runs things, versa respects them: it won't replace nginx with Apache, install a second PHP, restart services or change existing passwords.

## 5. First deploy

```bash
versa deploy production --initial-deploy
```

versa builds on your machine, uploads the project and switches to it. On the first deploy it asks whether to run the `post_deploy` hooks (the migrations): answer **N**, the `.env` isn't set yet.

## 6. The production `.env`

The first deploy created the server's `.env` from your `.env.example`. Edit it:

```bash
versa
```

**Shared** view → `.env` → **Enter** → **e** to edit → **Ctrl+S** to save. Put the production settings in (Laravel: `APP_ENV=production`, `APP_DEBUG=false`, `APP_URL`, `DB_*`…). If your framework needs a key (Laravel: `APP_KEY`), generate it on the server:

```bash
versa exec production 'cd /var/www/myapp/current/app && php artisan key:generate --force'
```

## 7. Deploy

```bash
versa deploy production --force
```

Now the migrations and caches run, and the health check confirms the app answers. Done!

## From now on

```bash
git commit -am "My change"
versa deploy production     # uploads only what changed
versa history production    # who deployed what, and when
versa rollback production   # back to the previous release, instantly
```

Next steps:

- **HTTPS**: with a domain pointing to the VPS, tick **SSL** in the Services view.
- **Notifications** to Slack, Discord or Telegram, **staging**, **tags** and **CI**: [Configuration](reference/DEPLOY.md).
- How it all works: [How it works](flows.md). Something failed? [Troubleshooting](reference/TROUBLESHOOTING.md).
