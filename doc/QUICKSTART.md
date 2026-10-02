# ⚡ Quick Start

The short version, for when you already have a server with SSH access. For a step-by-step setup (server preparation, web server config, sudoers), see [Getting Started](GETTING_STARTED.md).

```bash
# 1. In the root of your Git repository
versa init                                   # choose [1] SSH server

# 2. Edit deploy.yml (host, user, key_path, remote_path, builds), then commit it
versa config validate production
versa info production                        # tests SSH + SFTP and shows server info

# 3. First deploy
versa deploy production --initial-deploy

# 4. From now on
git commit -am "changes"
versa deploy production
```

Minimal `deploy.yml`:

```yaml
project: "my-app"
environments:
  production:
    ssh:
      host: "your-server.com"
      user: "deploy"
      key_path: "~/.ssh/id_rsa"
    remote_path: "/var/www/app"
    builds:
      php:
        enabled: true
    shared_paths:
      - ".env"                               # create /var/www/app/shared/.env on the server first
    services_reload:
      - "sudo systemctl reload php8.2-fpm"
```

Point your web server at `/var/www/app/current/app/public`.

## What a deploy does

1. Checks the working tree is committed and clones `HEAD` to a temp directory.
2. Compares file hashes with the server's `deploy.lock`; stops if nothing changed.
3. Builds locally (composer, npm, go, pip…), only for what changed.
4. Uploads the release in parallel chunks to `releases/<timestamp>/`.
5. Links `shared_paths`, switches `current` atomically, reloads services, runs `post_deploy` hooks, restarts `services`.
6. Rolls back automatically if a `post_deploy` hook, a service or the health check fails.

## Cheat sheet

```bash
versa                                        # full-screen interface (TUI)
versa diff production                        # what would be deployed
versa deploy production --dry-run            # same, through the deploy flow
versa deploy production --force              # redeploy without changes
versa deploy staging,production              # several environments in a row
versa status production                      # releases on the server
versa rollback production                    # previous release
versa rollback production --to 20261001-183807
versa logs production                        # tail -f the Laravel log
versa exec production "df -h"                # any command on the server
versa hooks production                       # re-run post_deploy hooks
versa services-reload production             # re-run services_reload
versa service production                     # status of the Go/Python services
versa service production restart
versa service production logs --name api
versa deploy production --log-file deploy.log
```

All commands and flags: [CLI Reference](CLI_REFERENCE.md). All config options: [Configuration Reference](DEPLOY.md).
